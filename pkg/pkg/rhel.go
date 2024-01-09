package pkg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aquasecurity/btfhub/pkg/kernel"
	"github.com/aquasecurity/btfhub/pkg/utils"
)

type RHELPackage struct {
	Name          string
	Architecture  string
	KernelVersion kernel.Version
	NameOfFile    string
}

func (pkg *RHELPackage) Filename() string {
	return pkg.NameOfFile
}

func (pkg *RHELPackage) BTFFilename() string {
	return pkg.NameOfFile
}

func (pkg *RHELPackage) Version() kernel.Version {
	return pkg.KernelVersion
}

func (pkg *RHELPackage) String() string {
	return pkg.Name
}

func (pkg *RHELPackage) ExtractKernel(ctx context.Context, pkgpath string, vmlinuxPath string) error {
	return utils.ExtractVmlinuxFromRPM(ctx, pkgpath, vmlinuxPath)
}

func (pkg *RHELPackage) Download(ctx context.Context, dir string, force bool) (string, error) {

	localFile := fmt.Sprintf("%s.rpm", pkg.Name)
	rpmpath := filepath.Join(dir, localFile)

	if !force && utils.Exists(rpmpath) {
		return rpmpath, nil
	}

	err := yumDownload(ctx, pkg.Name, pkg.Architecture, dir)
	if err != nil {
		os.Remove(rpmpath)
		return "", fmt.Errorf("rpm download: %s", err)
	}

	commonArch := fmt.Sprintf("kernel-debuginfo-common-%s-", pkg.Architecture)
	commonRPMPath := strings.ReplaceAll(localFile, "kernel-debuginfo-", commonArch)

	os.Remove(commonRPMPath) // no need for common rpm

	return rpmpath, nil
}
