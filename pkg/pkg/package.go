package pkg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aquasecurity/btfhub/pkg/kernel"
	"github.com/aquasecurity/btfhub/pkg/utils"
)

type Package interface {
	String() string
	Filename() string
	BTFFilename() string
	Version() kernel.Version
	Download(ctx context.Context, dir string, force bool) (string, error)
	ExtractKernel(ctx context.Context, pkgpath string, vmlinuxPath string) error
}

func PackageBTFExists(p Package, workDir string) bool {
	fp := filepath.Join(workDir, fmt.Sprintf("%s.btf.tar.xz", p.BTFFilename()))
	return utils.Exists(fp)
}

func PackageFailed(p Package, workDir string) bool {
	fp := filepath.Join(workDir, fmt.Sprintf("%s.failed", p.BTFFilename()))
	return utils.Exists(fp)
}

func hasBTFPath(p Package, workDir string) string {
	return filepath.Join(workDir, fmt.Sprintf("%s.hasbtf", p.BTFFilename()))
}

func MarkPackageHasBTF(p Package, workDir string) error {
	fp := hasBTFPath(p, workDir)
	f, err := os.Create(fp)
	if err != nil {
		return fmt.Errorf("mark hasbtf %s: %s", fp, err)
	}
	f.Close()
	return nil
}

func PackageHasBTF(p Package, workDir string) bool {
	fp := hasBTFPath(p, workDir)
	return utils.Exists(fp)
}

type ByVersion []Package

func (a ByVersion) Len() int      { return len(a) }
func (a ByVersion) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByVersion) Less(i, j int) bool {
	return a[i].Version().Less(a[j].Version())
}
