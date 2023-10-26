package pkg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aquasecurity/btfhub/pkg/kernel"
	"github.com/aquasecurity/btfhub/pkg/utils"
)

type CentOSPackage struct {
	Name          string
	Architecture  string
	KernelVersion kernel.Version
	NameOfFile    string
	URL           string
}

func (pkg *CentOSPackage) Filename() string {
	return pkg.NameOfFile
}

func (pkg *CentOSPackage) BTFFilename() string {
	return pkg.NameOfFile
}

func (pkg *CentOSPackage) Version() kernel.Version {
	return pkg.KernelVersion
}

func (pkg *CentOSPackage) String() string {
	return pkg.Name
}

func (pkg *CentOSPackage) Download(ctx context.Context, dir string, force bool) (string, error) {
	localFile := fmt.Sprintf("%s.rpm", pkg.NameOfFile)
	rpmpath := filepath.Join(dir, localFile)
	if !force && utils.Exists(rpmpath) {
		return rpmpath, nil
	}

	if err := utils.DownloadFile(ctx, pkg.URL, rpmpath); err != nil {
		os.Remove(rpmpath)
		return "", fmt.Errorf("downloading rpm package: %s", err)
	}
	return rpmpath, nil
}

func (pkg *CentOSPackage) ExtractKernel(ctx context.Context, pkgpath string, vmlinuxPath string) error {
	return utils.ExtractVmlinuxFromRPM(ctx, pkgpath, vmlinuxPath)
}
