package repo

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"

	"github.com/aquasecurity/btfhub/pkg/job"
	"github.com/aquasecurity/btfhub/pkg/kernel"
	"github.com/aquasecurity/btfhub/pkg/pkg"
	"github.com/aquasecurity/btfhub/pkg/utils"
)

type RHELRepo struct {
	archs           map[string]string
	releaseVersions map[string]string
	minVersion      kernel.Version
}

func NewRHELRepo() Repository {
	return &RHELRepo{
		archs: map[string]string{
			"x86_64": "x86_64",
			"arm64":  "aarch64",
		},
		releaseVersions: map[string]string{
			"7:x86_64":  "7.9",
			"7:aarch64": "7Server",
			"8:x86_64":  "8.1",
			"8:aarch64": "8.1",
		},
		minVersion: kernel.NewKernelVersion("3.10.0-957"),
	}
}

func (d *RHELRepo) GetKernelPackages(
	ctx context.Context,
	workDir string,
	release string,
	arch string,
	force bool,
	jobChan chan<- job.Job,
) error {
	altArch := d.archs[arch]
	rver := d.releaseVersions[release+":"+altArch]
	binary, args := utils.SudoCMD("subscription-manager", "release", fmt.Sprintf("--set=%s", rver))
	if err := utils.RunCMD(ctx, "", binary, args...); err != nil {
		return err
	}

	searchOut, err := yumSearch(ctx, "kernel-debuginfo")
	if err != nil {
		return err
	}
	pkgs, err := parseYumPackages(searchOut, d.minVersion)
	if err != nil {
		return fmt.Errorf("parse package listing: %s", err)
	}
	sort.Sort(pkg.ByVersion(pkgs))

	for _, pkg := range pkgs {
		err := processPackage(ctx, pkg, workDir, force, jobChan)
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
