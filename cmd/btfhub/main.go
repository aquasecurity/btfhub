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

	"golang.org/x/sync/errgroup"

	"github.com/aquasecurity/btfhub/pkg/job"
	"github.com/aquasecurity/btfhub/pkg/repo"
)

var distroReleases = map[string][]string{
	"ubuntu": {"xenial", "bionic", "focal"},
	"debian": {"stretch", "buster", "bullseye"},
	"fedora": {"24", "25", "26", "27", "28", "29", "30", "31"},
	"centos": {"7", "8"},
	"ol":     {"7", "8"},
	"rhel":   {"7", "8"},
	"amzn":   {"1", "2"},
	"sles":   {"12.3", "12.5", "15.1", "15.2", "15.3", "15.4"},
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
	"sles":   repo.NewSUSERepo,
}

var distro, release, arch string
var numWorkers int
var force bool

func init() {
	flag.StringVar(&distro, "distro", "", "distribution to update (ubuntu,debian,centos,fedora,ol,rhel,amazon,sles)")
	flag.StringVar(&distro, "d", "", "distribution to update (ubuntu,debian,centos,fedora,ol,rhel,amazon,sles)")
	flag.StringVar(&release, "release", "", "distribution release to update, requires specifying distribution")
	flag.StringVar(&release, "r", "", "distribution release to update, requires specifying distribution")
	flag.StringVar(&arch, "arch", "", "architecture to update (x86_64,arm64)")
	flag.StringVar(&arch, "a", "", "architecture to update (x86_64,arm64)")
	flag.IntVar(&numWorkers, "workers", 0, "number of concurrent workers (defaults to runtime.NumCPU() - 1)")
	flag.IntVar(&numWorkers, "j", 0, "number of concurrent workers (defaults to runtime.NumCPU() - 1)")
	flag.BoolVar(&force, "f", false, "force update regardless of existing files (defaults to false)")
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
				return fmt.Errorf("invalid release %s for %s", release, distro)
			}
		}
	} else {
		release = "" // no release if no distro is selected
	}

	// Distributions

	distros := []string{"ubuntu", "debian", "fedora", "centos", "ol"} // RHEL needs subscription
	if distro != "" {
		distros = []string{distro}
	}

	// Architectures

	archs := []string{"x86_64", "arm64"}
	if arch != "" {
		archs = []string{arch}
	}

	// Environment

	basedir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("pwd: %s", err)
	}
	archiveDir := path.Join(basedir, "archive")

	if numWorkers == 0 {
		numWorkers = runtime.NumCPU() - 1
		if numWorkers > 12 {
			numWorkers = 12 // limit to 12 workers max (for bigger machines)
		}
	}

	// Workers: job consumers (pool)

	jobChan := make(chan job.Job)
	consume, consCtx := errgroup.WithContext(ctx)

	log.Printf("Using %d workers\n", numWorkers)
	for i := 0; i < numWorkers; i++ {
		consume.Go(func() error {
			return job.StartWorker(consCtx, jobChan)
		})
	}

	// Workers: job producers (per distro, per release)

	produce, prodCtx := errgroup.WithContext(ctx)

	for _, d := range distros {
		releases := distroReleases[d]
		if release != "" {
			releases = []string{release}
		}
		for _, r := range releases {
			release := r
			for _, a := range archs {
				arch := a
				distro := d
				produce.Go(func() error {
					// workDir example: ./archive/ubuntu/focal/x86_64
					workDir := filepath.Join(archiveDir, distro, release, arch)
					if err := os.MkdirAll(workDir, 0775); err != nil {
						return fmt.Errorf("arch dir: %s", err)
					}

					// pick the repository creator and get the kernel packages
					repo := repoCreators[distro]()

					return repo.GetKernelPackages(prodCtx, workDir, release, arch, force, jobChan)
				})

			}
		}
	}

	// Cleanup

	err = produce.Wait()
	close(jobChan)
	if err != nil {
		return err
	}

	return consume.Wait()
}
