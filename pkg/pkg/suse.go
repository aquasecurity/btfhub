package pkg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aquasecurity/btfhub/pkg/kernel"
	"github.com/aquasecurity/btfhub/pkg/utils"
)

type SUSEPackage struct {
	Name          string
	NameOfFile    string
	NameOfBTFFile string
	Architecture  string
	KernelVersion kernel.Version
	Repo          string
	Flavor        string
	Downloaddir   string
}

func (pkg *SUSEPackage) Filename() string {
	return pkg.NameOfFile
}

func (pkg *SUSEPackage) BTFFilename() string {
	return pkg.NameOfBTFFile
}

func (pkg *SUSEPackage) Version() kernel.Version {
	return pkg.KernelVersion
}

func (pkg *SUSEPackage) String() string {
	return fmt.Sprintf("%s-%s.%s", pkg.Name, pkg.KernelVersion.String(), pkg.Architecture)
}

func (pkg *SUSEPackage) ExtractKernel(ctx context.Context, pkgpath string, vmlinuxPath string) error {
	// vmlinux at: /usr/lib/debug/boot/vmlinux-<ver>-<type>.debug
	return utils.ExtractVmlinuxFromRPM(ctx, pkgpath, vmlinuxPath)
}

func (pkg *SUSEPackage) Download(ctx context.Context, _ string, force bool) (string, error) {
	localFile := fmt.Sprintf("%s-%s.%s.rpm", pkg.Name, pkg.KernelVersion.String(), pkg.Architecture)
	rpmpath := filepath.Join(pkg.Downloaddir, localFile)
	if !force && utils.Exists(rpmpath) {
		return rpmpath, nil
	}

	if err := zypperDownload(ctx, fmt.Sprintf("%s=%s", pkg.Name, pkg.KernelVersion.String())); err != nil {
		os.Remove(rpmpath)
		return "", fmt.Errorf("zypper download: %s", err)
	}

	return rpmpath, nil
}

func zypperDownload(ctx context.Context, pkg string) error {
	stdout, err := utils.RunZypperCMD(ctx, "-q", "install", "-y", "--no-recommends", "--download-only", pkg)
	_, _ = fmt.Fprint(os.Stdout, stdout.String())
	return err
}
