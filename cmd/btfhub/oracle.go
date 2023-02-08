package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
)

type oracleRepo struct {
	archs      map[string]string
	repos      map[string]string
	minVersion kernelVersion
}

func newOracleRepo() Repository {
	return &oracleRepo{
		archs: map[string]string{
			"arm64":  "aarch64",
			"x86_64": "x86_64",
		},
		repos: map[string]string{
			"7": "https://oss.oracle.com/ol7/debuginfo/",
			"8": "https://oss.oracle.com/ol8/debuginfo/",
		},
		minVersion: newKernelVersion("3.10.0-957"),
	}
}

func (d *oracleRepo) GetKernelPackages(ctx context.Context, dir string, release string, arch string, jobchan chan<- Job) error {
	repo := d.repos[release]
	altArch := d.archs[arch]

	links, err := getLinks(repo)
	if err != nil {
		return fmt.Errorf("repo links: %s", err)
	}

	var pkgs []Package
	kre := regexp.MustCompile(fmt.Sprintf(`kernel(?:-uek)?-debuginfo-([0-9].*\.%s)\.rpm`, altArch))
	for _, l := range links {
		match := kre.FindStringSubmatch(l)
		if match != nil {
			p := &centosPackage{
				name:         strings.TrimSuffix(match[0], ".rpm"),
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
