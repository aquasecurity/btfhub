package repo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/aquasecurity/btfhub/pkg/job"
	"github.com/aquasecurity/btfhub/pkg/pkg"
	"github.com/aquasecurity/btfhub/pkg/utils"
)

type DebianRepo struct {
	archs          map[string]string
	repos          map[string][]string
	releaseNumbers map[string]string
}

var oldRepos = []string{
	"http://ftp.debian.org/debian/dists/%s/main/binary-%s/Packages.gz",
	"http://ftp.debian.org/debian/dists/%s-updates/main/binary-%s/Packages.gz",
	"http://security.debian.org/debian-security/dists/%s/updates/main/binary-%s/Packages.gz",
}

var newRepos = []string{
	"http://ftp.debian.org/debian/dists/%s/main/binary-%s/Packages.xz",
	"http://ftp.debian.org/debian/dists/%s-updates/main/binary-%s/Packages.xz",
	"http://security.debian.org/debian-security/dists/%s-security/main/binary-%s/Packages.xz",
}

func NewDebianRepo() Repository {
	return &DebianRepo{
		archs: map[string]string{
			"x86_64": "amd64",
			"arm64":  "arm64",
		},
		repos: map[string][]string{
			"stretch":  oldRepos,
			"buster":   oldRepos,
			"bullseye": newRepos,
		},
		releaseNumbers: map[string]string{
			"stretch":  "9",
			"buster":   "10",
			"bullseye": "11",
		},
	}
}

func (d *DebianRepo) GetKernelPackages(
	ctx context.Context,
	workDir string,
	release string,
	arch string,
	jobchan chan<- job.Job,
) error {
	altArch := d.archs[arch]

	var pkgs []pkg.Package

	for _, r := range d.repos[release] {
		rawPkgs := &bytes.Buffer{}

		// Get Packages.xz from main, updates and security

		repo := fmt.Sprintf(r, release, altArch) // ..debian/dists/%s/%s/main.../Packages.gz

		if err := utils.Download(ctx, repo, rawPkgs); err != nil {
			return fmt.Errorf("download package list %s: %s", repo, err)
		}

		// Get the list of kernel packages to download from those repos

		repoURL, err := url.Parse(repo)
		if err != nil {
			return fmt.Errorf("repo url parse: %s", err)
		}

		// Get the list of kernel packages to download from debug repo

		repoURL.Path = "/" + strings.Split(repoURL.Path, "/")[1]
		kernelDbgPkgs, err := pkg.ParseAPTPackages(rawPkgs, repoURL.String(), release)
		if err != nil {
			return fmt.Errorf("parsing package list: %s", err)
		}

		// Filter out packages that aren't debug kernel packages

		re := regexp.MustCompile(`linux-image-[0-9]+\.[0-9]+\.[0-9].*-dbg`)

		for _, p := range kernelDbgPkgs {
			match := re.FindStringSubmatch(p.Name)
			if match == nil {
				continue
			}
			pkgs = append(pkgs, p)
		}
	}

	sort.Sort(pkg.ByVersion(pkgs))
	for i, pkg := range pkgs {
		log.Printf("DEBUG: start pkg %s (%d/%d)\n", pkg, i+1, len(pkgs))

		// Jobs about to be created:
		//
		// 1. Download package and extract vmlinux file
		// 2. Extract BTF info from vmlinux file

		err := processPackage(ctx, pkg, workDir, jobchan)
		if err != nil {
			if errors.Is(err, utils.ErrHasBTF) {
				log.Printf("INFO: kernel %s has BTF already, skipping later kernels\n", pkg)
				return nil
			}
			if errors.Is(err, context.Canceled) {
				return nil
			}

			log.Printf("ERROR: %s: %s\n", pkg, err)
			continue
		}

		log.Printf("DEBUG: end pkg %s (%d/%d)\n", pkg, i+1, len(pkgs))
	}
	return nil
}
