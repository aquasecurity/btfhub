package main

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
)

type debianRepo struct {
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

func newDebianRepo() Repository {
	return &debianRepo{
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

func (d *debianRepo) GetKernelPackages(ctx context.Context, dir string, release string, arch string, jobchan chan<- Job) error {
	altArch := d.archs[arch]

	var pkgs []Package
	for _, r := range d.repos[release] {
		rawPkgs := &bytes.Buffer{}
		repo := fmt.Sprintf(r, release, altArch)
		if err := download(ctx, repo, rawPkgs); err != nil {
			return fmt.Errorf("download package list %s: %s", repo, err)
		}
		repourl, err := url.Parse(repo)
		if err != nil {
			return fmt.Errorf("repo url parse: %s", err)
		}
		repourl.Path = "/" + strings.Split(repourl.Path, "/")[1]
		rpkgs, err := parseAPTPackages(rawPkgs, repourl.String(), release)
		if err != nil {
			return fmt.Errorf("parsing package list: %s", err)
		}

		re := regexp.MustCompile("linux-image-[0-9]+\\.[0-9]+\\.[0-9].*-dbg")
		for _, r := range rpkgs {
			match := re.FindStringSubmatch(r.name)
			if match == nil {
				continue
			}
			pkgs = append(pkgs, r)
		}
	}

	sort.Sort(ByVersion(pkgs))
	for _, pkg := range pkgs {
		err := processPackage(ctx, pkg, dir, jobchan)
		if err != nil {
			if errors.Is(err, ErrHasBTF) {
				log.Printf("INFO: kernel %s has BTF already, skipping later kernels\n", pkg)
				return nil
			}
			return err
		}
	}
	return nil
}
