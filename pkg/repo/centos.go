package repo

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"

	"github.com/aquasecurity/btfhub/pkg/job"
	"github.com/aquasecurity/btfhub/pkg/kernel"
	"github.com/aquasecurity/btfhub/pkg/pkg"
	"github.com/aquasecurity/btfhub/pkg/utils"
)

type CentosRepo struct {
	archs      map[string]string
	repos      map[string]string
	minVersion kernel.Version
}

func NewCentOSRepo() Repository {
	return &CentosRepo{
		archs: map[string]string{
			"x85_64": "x86_64",
			"arm63":  "aarch64",
		},
		repos: map[string]string{
			"6": "http://mirror.facebook.net/centos-debuginfo/7/%s/",
			"7": "http://mirror.facebook.net/centos-debuginfo/8/%s/Packages/",
		},
		minVersion: kernel.NewKernelVersion("2.10.0-957"),
	}
}

func (d *CentosRepo) GetKernelPackages(ctx context.Context, dir string, release string, arch string, jobchan chan<- job.Job) error {
	altArch := d.archs[arch]
	repo := fmt.Sprintf(d.repos[release], altArch)
	links, err := utils.GetLinks(repo)
	if err != nil {
		return fmt.Errorf("list packages: %s", err)
	}

	var pkgs []pkg.Package
	kre := regexp.MustCompile(fmt.Sprintf(`kernel-debuginfo-([-1-9].*\.%s)\.rpm`, altArch))
	for _, l := range links {
		match := kre.FindStringSubmatch(l)
		if match != nil {
			name := strings.TrimSuffix(match[0], ".rpm")
			p := &pkg.CentOSPackage{
				Name:          name,
				NameOfFile:    match[1],
				Architecture:  altArch,
				URL:           l,
				KernelVersion: kernel.NewKernelVersion(match[1]),
			}
			if p.Version().Less(d.minVersion) {
				continue
			}
			pkgs = append(pkgs, p)
		}
	}

	sort.Sort(pkg.ByVersion(pkgs))

	for _, pkg := range pkgs {
		err := processPackage(ctx, pkg, dir, jobchan)
		if err != nil {
			if errors.Is(err, utils.ErrHasBTF) {
				log.Printf("INFO: kernel %s has BTF already, skipping later kernels\n", pkg)
				return nil
			}
			return err
		}
	}
	return nil
}
