package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/aquasecurity/btfhub/pkg/job"
	"github.com/aquasecurity/btfhub/pkg/manifest"
	"github.com/aquasecurity/btfhub/pkg/preflight"
	"github.com/aquasecurity/btfhub/pkg/repo"
)

// ubuntuArchiveDir maps APT codenames (used for downloads) to on-disk segments
// under archive/ubuntu/ in btfhub-archive. The published repo lists blobs under
// e.g. ubuntu/20.04/...; ubuntu/focal is only a symlink, so placeholders from the
// GitHub tree would not be visible under ubuntu/focal/... and kernels looked
// "missing" without this mapping.
var ubuntuArchiveDir = map[string]string{
	"xenial": "16.04",
	"bionic": "18.04",
	"focal":  "20.04",
}

// debianArchiveDir maps suite codenames (used for APT metadata) to on-disk
// segments under archive/debian/ in btfhub-archive. The GitHub tree lists blobs
// under debian/9, debian/10, debian/bullseye — not under stretch/buster.
var debianArchiveDir = map[string]string{
	"stretch":  "9",
	"buster":   "10",
	"bullseye": "bullseye",
}

// archiveLayoutDir returns the archive subdirectory name for (distro, release).
func archiveLayoutDir(distro, release string) string {
	if distro == "ubuntu" {
		if d, ok := ubuntuArchiveDir[release]; ok {
			return d
		}
	}
	if distro == "debian" {
		if d, ok := debianArchiveDir[release]; ok {
			return d
		}
	}
	return release
}

func archiveWorkDir(archiveRoot, distro, release, arch string) string {
	return filepath.Join(archiveRoot, distro, archiveLayoutDir(distro, release), arch)
}

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
var preflightMode bool
var indexFile string
var manifestOut string
var manifestIn string

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
	flag.BoolVar(&preflightMode, "preflight", false, "check if any kernel packages need processing vs index file; exit 0 if none, 1 if work needed (no downloads)")
	flag.StringVar(&indexFile, "index", "", "path to archive path list (one repo-relative path per line); required with -preflight")
	flag.StringVar(&manifestOut, "manifest-out", "", "with -preflight: append JSONL of every kernel that would be processed (all distros); exit 1 if any line written")
	flag.StringVar(&manifestIn, "manifest", "", "normal mode: process only kernels listed in this JSONL (from preflight -manifest-out)")
}

func main() {
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	err := run(ctx)
	if err != nil {
		if preflightMode && errors.Is(err, preflight.ErrWorkFound) {
			os.Exit(1)
		}
		log.Fatal(err)
	}
	if preflightMode {
		// No packages need processing.
		os.Exit(0)
	}
}

func materializeArchiveIndex(archiveDir, indexPath string) error {
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return fmt.Errorf("read index: %w", err)
	}
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, line)
	}
	// Longest paths first so we never create a file at e.g. ubuntu/xenial before
	// ubuntu/xenial/amd64/foo (which would leave xenial as a regular file and
	// break MkdirAll for workDir ubuntu/xenial/x86_64).
	sort.Slice(lines, func(i, j int) bool { return len(lines[i]) > len(lines[j]) })
	for _, line := range lines {
		p := filepath.Join(archiveDir, filepath.FromSlash(line))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(p), err)
		}
		f, err := os.Create(p)
		if err != nil {
			return fmt.Errorf("create placeholder %s: %w", p, err)
		}
		if err := f.Close(); err != nil {
			return err
		}
	}
	return nil
}

// mkdirArchiveWorkDir creates workDir under archive, removing empty regular
// files that would block a path component (e.g. CI placeholders created in
// wrong order, or stale zero-byte stubs).
func mkdirArchiveWorkDir(workDir string) error {
	const perm = 0775
	for attempts := 0; attempts < 128; attempts++ {
		err := os.MkdirAll(workDir, perm)
		if err == nil {
			return nil
		}
		removed := false
		for p := workDir; p != "/" && p != "."; p = filepath.Dir(p) {
			st, err2 := os.Lstat(p)
			if err2 != nil {
				continue
			}
			if st.Mode().IsRegular() && st.Size() == 0 {
				if rmErr := os.Remove(p); rmErr == nil {
					removed = true
					break
				}
			}
		}
		if !removed {
			return err
		}
	}
	return fmt.Errorf("mkdir %s: exhausted retries removing blocking stubs", workDir)
}

func validateDistroFlags() error {
	if distro != "" {
		if _, ok := distroReleases[distro]; !ok {
			return fmt.Errorf("invalid distribution %s", distro)
		}
		if release != "" {
			found := false
			for _, r := range distroReleases[distro] {
				if r == release {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("invalid release %s for %s", release, distro)
			}
		}
	} else {
		release = ""
	}
	return nil
}

func validateManifestFlags() error {
	if preflightMode {
		if manifestIn != "" {
			return fmt.Errorf("-manifest cannot be used with -preflight")
		}
		return nil
	}
	if manifestOut != "" {
		return fmt.Errorf("-manifest-out requires -preflight")
	}
	return nil
}

func run(ctx context.Context) error {
	if err := validateDistroFlags(); err != nil {
		return err
	}
	if err := validateManifestFlags(); err != nil {
		return err
	}

	if preflightMode {
		if indexFile == "" {
			return fmt.Errorf("-preflight requires -index")
		}
		return runPreflight(ctx)
	}

	distros := []string{"ubuntu", "debian", "fedora", "centos", "ol"} // RHEL needs subscription
	if distro != "" {
		distros = []string{distro}
	}

	archs := []string{"x86_64", "arm64"}
	if arch != "" {
		archs = []string{arch}
	}

	basedir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("pwd: %s", err)
	}
	archiveDir := path.Join(basedir, "archive")

	var filt *manifest.Filter
	if manifestIn != "" {
		var lerr error
		filt, lerr = manifest.LoadFilter(manifestIn)
		if lerr != nil {
			return lerr
		}
	}

	if numWorkers == 0 {
		numWorkers = runtime.NumCPU() - 1
		if numWorkers > 12 {
			numWorkers = 12
		}
	}

	jobChan := make(chan job.Job)
	consume, consCtx := errgroup.WithContext(ctx)

	log.Printf("Using %d workers\n", numWorkers)
	for i := 0; i < numWorkers; i++ {
		consume.Go(func() error {
			return job.StartWorker(consCtx, jobChan)
		})
	}

	produce, prodCtx := errgroup.WithContext(ctx)
	if filt != nil {
		prodCtx = manifest.WithFilter(prodCtx, filt)
	}

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
					workDir := archiveWorkDir(archiveDir, distro, release, arch)
					if err := mkdirArchiveWorkDir(workDir); err != nil {
						return fmt.Errorf("arch dir: %s", err)
					}
					rp := repoCreators[distro]()
					return rp.GetKernelPackages(prodCtx, workDir, release, arch, force, jobChan)
				})
			}
		}
	}

	err = produce.Wait()
	close(jobChan)
	if err != nil {
		return err
	}

	return consume.Wait()
}

func runPreflight(ctx context.Context) error {
	ctx = preflight.WithContext(ctx)

	var mw *manifest.Writer
	if manifestOut != "" {
		w, err := manifest.NewAppender(manifestOut)
		if err != nil {
			return fmt.Errorf("manifest-out: %w", err)
		}
		mw = w
		defer func() { _ = mw.Close() }()
	}

	distros := []string{"ubuntu", "debian", "fedora", "centos", "ol"}
	if distro != "" {
		distros = []string{distro}
	}

	archs := []string{"x86_64", "arm64"}
	if arch != "" {
		archs = []string{arch}
	}

	basedir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("pwd: %s", err)
	}
	archiveDir := path.Join(basedir, "archive")
	if err := os.RemoveAll(archiveDir); err != nil {
		return fmt.Errorf("reset archive dir: %w", err)
	}
	if err := materializeArchiveIndex(archiveDir, indexFile); err != nil {
		return err
	}

	jobChan := make(chan job.Job)
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
					workDir := archiveWorkDir(archiveDir, distro, release, arch)
					if err := mkdirArchiveWorkDir(workDir); err != nil {
						return fmt.Errorf("arch dir: %s", err)
					}
					subCtx := prodCtx
					if mw != nil {
						subCtx = manifest.WithWriter(subCtx, mw)
						subCtx = manifest.WithTemplate(subCtx, manifest.Template{
							Distro:  distro,
							Release: release,
							Arch:    arch,
						})
					}
					rp := repoCreators[distro]()
					return rp.GetKernelPackages(subCtx, workDir, release, arch, force, jobChan)
				})
			}
		}
	}

	err = produce.Wait()
	close(jobChan)
	if err != nil {
		return err
	}
	if mw != nil && mw.Count() > 0 {
		return preflight.ErrWorkFound
	}
	return nil
}
