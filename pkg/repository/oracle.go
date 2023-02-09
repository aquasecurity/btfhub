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
	pkg "github.com/aquasecurity/btfhub/pkg/package"
	"github.com/aquasecurity/btfhub/pkg/utils"
)

type oracleRepo struct {
	archs      map[string]string
	repos      map[string]string
	minVersion kernel.KernelVersion
}

func NewOracleRepo() Repository {
	return &oracleRepo{
		archs: map[string]string{
			"arm64":  "aarch64",
			"x86_64": "x86_64",
		},
		repos: map[string]string{
			"7": "https://oss.oracle.com/ol7/debuginfo/",
			"8": "https://oss.oracle.com/ol8/debuginfo/",
		},
		minVersion: kernel.NewKernelVersion("3.10.0-957"),
	}
}

func (d *oracleRepo) GetKernelPackages(ctx context.Context, dir string, release string, arch string, jobchan chan<- job.Job) error {
	repo := d.repos[release]
	altArch := d.archs[arch]

	links, err := utils.GetLinks(repo)
	if err != nil {
		return fmt.Errorf("repo links: %s", err)
	}

	var pkgs []pkg.Package
	kre := regexp.MustCompile(fmt.Sprintf(`kernel(?:-uek)?-debuginfo-([0-9].*\.%s)\.rpm`, altArch))
	for _, l := range links {
		match := kre.FindStringSubmatch(l)
		if match != nil {
			p := &pkg.CentOSPackage{
				Name:          strings.TrimSuffix(match[0], ".rpm"),
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
