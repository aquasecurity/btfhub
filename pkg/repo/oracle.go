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

type oracleRepo struct {
	archs      map[string]string
	repos      map[string]string
	minVersion kernel.Version
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

func (d *oracleRepo) GetKernelPackages(
	ctx context.Context,
	workDir string,
	release string,
	arch string,
	force bool,
	jobChan chan<- job.Job,
) error {
	var pkgs []pkg.Package

	altArch := d.archs[arch]

	// Pick all the links that match the kernel-debuginfo pattern

	repoURL := d.repos[release]

	links, err := utils.GetLinks(ctx, repoURL)
	if err != nil {
		return fmt.Errorf("ERROR: list packages: %s", err)
	}

	kre := regexp.MustCompile(fmt.Sprintf(`kernel(?:-uek)?-debuginfo-([0-9].*\.%s)\.rpm`, altArch))

	for _, l := range links {
		match := kre.FindStringSubmatch(l)
		if match != nil {

			// Create a package object from the link and add it to pkgs list

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

	sort.Sort(pkg.ByVersion(pkgs)) // so kernels can be skipped if previous has BTF already

	for i, pkg := range pkgs {
		log.Printf("DEBUG: start pkg %s (%d/%d)\n", pkg, i+1, len(pkgs))

		// Jobs about to be created:
		//
		// 1. Download package and extract vmlinux file
		// 2. Extract BTF info from vmlinux file

		err := processPackage(ctx, pkg, workDir, force, jobChan)
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
