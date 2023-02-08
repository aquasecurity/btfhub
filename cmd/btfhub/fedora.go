package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type fedoraRepo struct {
	archs      map[string]string
	repos      map[string][]string
	minVersion kernelVersion
}

var centosArchives = []string{
	"https://archives.fedoraproject.org/pub/archive/fedora/linux/releases/%s/Everything/%s/debug/tree/Packages/k/",
	"https://archives.fedoraproject.org/pub/archive/fedora/linux/updates/%s/Everything/%s/debug/Packages/k/",
}
var centosDownload = []string{
	"https://dl.fedoraproject.org/pub/fedora/linux/releases/%s/Everything/%s/debug/tree/Packages/k/",
}

func newFedoraRepo() Repository {
	return &fedoraRepo{
		archs: map[string]string{
			"x86_64": "x86_64",
			"arm64":  "aarch64",
		},
		repos: map[string][]string{
			"24": centosArchives,
			"25": centosArchives,
			"26": centosArchives,
			"27": centosArchives,
			"28": centosArchives,
			"29": centosArchives,
			"30": centosArchives,
			"31": centosArchives,
			//"32": centosArchives,
			//"33": centosArchives,
			//"34": centosDownload,
		},
		minVersion: newKernelVersion("3.10.0-957"),
	}
}

func (d *fedoraRepo) GetKernelPackages(ctx context.Context, dir string, release string, arch string, jobchan chan<- Job) error {
	altArch := d.archs[arch]
	var repos []string
	for _, r := range d.repos[release] {
		repos = append(repos, fmt.Sprintf(r, release, altArch))
	}

	var links []string
	for _, repo := range repos {
		rlinks, err := getLinks(repo)
		if err != nil {
			//return fmt.Errorf("list packages: %s", err)
			log.Printf("ERROR: list packages: %s\n", err)
			continue
		}
		links = append(links, rlinks...)
	}

	var pkgs []Package
	kre := regexp.MustCompile(fmt.Sprintf(`kernel-debuginfo-([0-9].*\.%s)\.rpm`, altArch))
	for _, l := range links {
		match := kre.FindStringSubmatch(l)
		if match != nil {
			name := strings.TrimSuffix(match[0], ".rpm")
			p := &fedoraPackage{
				name:         name,
				filename:     match[1],
				architecture: altArch,
				url:          l,
				version:      newKernelVersion(match[1]),
			}
			if p.Version().Less(d.minVersion) {
				continue
			}
			pkgs = append(pkgs, p)
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

type fedoraPackage struct {
	name         string
	architecture string
	version      kernelVersion
	filename     string
	url          string
}

func (pkg *fedoraPackage) Filename() string {
	return pkg.filename
}

func (pkg *fedoraPackage) Version() kernelVersion {
	return pkg.version
}

func (pkg *fedoraPackage) String() string {
	return pkg.name
}

func (pkg *fedoraPackage) Download(ctx context.Context, dir string) (string, error) {
	localFile := fmt.Sprintf("%s.rpm", pkg.filename)
	rpmpath := filepath.Join(dir, localFile)
	if exists(rpmpath) {
		return rpmpath, nil
	}

	if err := downloadFile(ctx, pkg.url, rpmpath); err != nil {
		os.Remove(rpmpath)
		return "", fmt.Errorf("downloading rpm package: %s", err)
	}
	return rpmpath, nil
}

func (pkg *fedoraPackage) ExtractKernel(ctx context.Context, pkgpath string, vmlinuxPath string) error {
	return extractVmlinuxFromRPM(ctx, pkgpath, vmlinuxPath)
}
