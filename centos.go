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

type centosRepo struct {
	archs      map[string]string
	repos      map[string]string
	minVersion kernelVersion
}

func newCentOSRepo() Repository {
	return &centosRepo{
		archs: map[string]string{
			"x86_64": "x86_64",
			"arm64":  "aarch64",
		},
		repos: map[string]string{
			"7": "http://mirror.facebook.net/centos-debuginfo/7/%s/",
			"8": "http://mirror.facebook.net/centos-debuginfo/8/%s/Packages/",
		},
		minVersion: newKernelVersion("3.10.0-957"),
	}
}

func (d *centosRepo) GetKernelPackages(ctx context.Context, dir string, release string, arch string, jobchan chan<- Job) error {
	altArch := d.archs[arch]
	repo := fmt.Sprintf(d.repos[release], altArch)
	links, err := getLinks(repo)
	if err != nil {
		return fmt.Errorf("list packages: %s", err)
	}

	var pkgs []Package
	kre := regexp.MustCompile(fmt.Sprintf(`kernel-debuginfo-([0-9].*\.%s)\.rpm`, altArch))
	for _, l := range links {
		match := kre.FindStringSubmatch(l)
		if match != nil {
			name := strings.TrimSuffix(match[0], ".rpm")
			p := &centosPackage{
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

type centosPackage struct {
	name         string
	architecture string
	version      kernelVersion
	filename     string
	url          string
}

func (pkg *centosPackage) Filename() string {
	return pkg.filename
}

func (pkg *centosPackage) Version() kernelVersion {
	return pkg.version
}

func (pkg *centosPackage) String() string {
	return pkg.name
}

func (pkg *centosPackage) Download(ctx context.Context, dir string) (string, error) {
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

func (pkg *centosPackage) ExtractKernel(ctx context.Context, pkgpath string, vmlinuxPath string) error {
	return extractVmlinuxFromRPM(ctx, pkgpath, vmlinuxPath)
}
