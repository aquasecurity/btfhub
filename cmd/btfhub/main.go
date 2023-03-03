package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"

	"github.com/aquasecurity/btfhub/pkg/job"
	"github.com/aquasecurity/btfhub/pkg/repo"
	"golang.org/x/sync/errgroup"
)

var distroReleases = map[string][]string{
	"ubuntu": {"xenial", "bionic", "focal"},
	"debian": {"stretch", "buster"},
	"fedora": {"24", "25", "26", "27", "28", "29", "30", "31"},
	"centos": {"7", "8"},
	"ol":     {"7", "8"},
	"rhel":   {"7", "8"},
	"amzn":   {"1", "2"},
}

type repoFunc func() repo.Repository

var repoCreators = map[string]repoFunc{
	"ubuntu": repo.NewUbuntuRepo,
	"debian": repo.NewDebianRepo,
	"fedora": repo.NewFedoraRepo,
	"centos": repo.NewCentOSRepo,
	"ol":     repo.NewOracleRepo,
	"rhel":   repo.NewRHELRepo,
	"amzn":   repo.NewAmazonRepo,
}

var distro, release, arch string
var numWorkers int

func init() {
	flag.StringVar(&distro, "distro", "", "distribution to update (ubuntu,debian,centos,fedora,ol,rhel,amazon)")
	flag.StringVar(&distro, "d", "", "distribution to update (ubuntu,debian,centos,fedora,ol,rhel,amazon)")
	flag.StringVar(&release, "release", "", "distribution release to update, requires specifying distribution")
	flag.StringVar(&release, "r", "", "distribution release to update, requires specifying distribution")
	flag.StringVar(&arch, "arch", "", "architecture to update (x86_64,arm64)")
	flag.StringVar(&arch, "a", "", "architecture to update (x86_64,arm64)")
	flag.IntVar(&numWorkers, "workers", 0, "number of concurrent workers (defaults to runtime.NumCPU() - 1)")
	flag.IntVar(&numWorkers, "j", 0, "number of concurrent workers (defaults to runtime.NumCPU() - 1)")
}

func main() {
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := run(ctx); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	if distro != "" {
		if _, ok := distroReleases[distro]; !ok {
			return fmt.Errorf("invalid distribution %s", distro)
		}
		if release != "" {
			found := false
			for _, r := range distroReleases[distro] {
				found = r == release
				if found {
					break
				}
			}
			if !found {
				return fmt.Errorf("invalid distribution release %s for %s", release, distro)
			}
		}
	} else {
		// cannot select specific version, if no specific distro is selected
		release = ""
	}

	basedir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("pwd: %s", err)
	}
	archiveDir := path.Join(basedir, "archive")

	// RHEL excluded here because we don't have direct external access to repos
	distros := []string{"ubuntu", "debian", "fedora", "centos", "ol"}
	if distro != "" {
		distros = []string{distro}
	}
	archs := []string{"x86_64", "arm64"}
	if arch != "" {
		archs = []string{arch}
	}

	jobchan := make(chan job.Job)
	consume, cctx := errgroup.WithContext(ctx)
	if numWorkers == 0 {
		numWorkers = runtime.NumCPU() - 1
	}
	log.Printf("Using %d workers\n", numWorkers)
	for i := 0; i < numWorkers; i++ {
		consume.Go(func() error {
			return job.StartWorker(cctx, jobchan)
		})
	}

	produce, pctx := errgroup.WithContext(ctx)
	for _, d := range distros {
		releases := distroReleases[d]
		if release != "" {
			releases = []string{release}
		}
		for _, r := range releases {
			cr := r
			for _, a := range archs {
				ca := a
				newd := d
				produce.Go(func() error {
					wd := filepath.Join(archiveDir, newd, cr, ca)
					if err := os.MkdirAll(wd, 0775); err != nil {
						return fmt.Errorf("arch dir: %s", err)
					}

					repo := repoCreators[newd]()
					return repo.GetKernelPackages(pctx, wd, cr, ca, jobchan)
				})
			}
		}
	}
	err = produce.Wait()
	close(jobchan)
	if err != nil {
		return err
	}
	return consume.Wait()
}
