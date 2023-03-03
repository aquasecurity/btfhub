package pkg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aquasecurity/btfhub/pkg/kernel"
	"github.com/aquasecurity/btfhub/pkg/utils"
)

type FedoraPackage struct {
	Name          string
	Architecture  string
	KernelVersion kernel.Version
	NameOfFile    string
	URL           string
}

func (pkg *FedoraPackage) Filename() string {
	return pkg.NameOfFile
}

func (pkg *FedoraPackage) Version() kernel.Version {
	return pkg.KernelVersion
}

func (pkg *FedoraPackage) String() string {
	return pkg.Name
}

func (pkg *FedoraPackage) Download(ctx context.Context, dir string) (string, error) {
	localFile := fmt.Sprintf("%s.rpm", pkg.NameOfFile)
	rpmpath := filepath.Join(dir, localFile)
	if utils.Exists(rpmpath) {
		return rpmpath, nil
	}

	if err := utils.DownloadFile(ctx, pkg.URL, rpmpath); err != nil {
		os.Remove(rpmpath)
		return "", fmt.Errorf("downloading rpm package: %s", err)
	}
	return rpmpath, nil
}

func (pkg *FedoraPackage) Extract(ctx context.Context, pkgpath string, vmlinuxPath string) error {
	return utils.ExtractVmlinuxFromRPM(ctx, pkgpath, vmlinuxPath)
}

func (pkg *FedoraPackage) ExtractKernel(ctx context.Context, pkgpath string, vmlinuxPath string) error {
	return fmt.Errorf("not implemented")
}
