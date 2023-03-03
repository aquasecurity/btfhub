package job

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/aquasecurity/btfhub/pkg/pkg"
	"github.com/aquasecurity/btfhub/pkg/utils"
)

type KernelExtractionJob struct {
	Pkg       pkg.Package
	Dir       string
	ReplyChan chan interface{}
}

func (j *KernelExtractionJob) Do(ctx context.Context) error {
	vmlinuxName := fmt.Sprintf("vmlinux-%s", j.Pkg.Filename())
	vmlinuxPath := filepath.Join(j.Dir, vmlinuxName)
	if utils.Exists(vmlinuxPath) {
		j.ReplyChan <- vmlinuxPath
		return nil
	}

	dstart := time.Now()
	log.Printf("DEBUG: downloading %s\n", j.Pkg)
	pkgpath, err := j.Pkg.Download(ctx, j.Dir)
	if err != nil {
		os.Remove(pkgpath)
		return err
	}
	log.Printf("DEBUG: finished downloading %s in %s\n", j.Pkg, time.Since(dstart))

	estart := time.Now()
	log.Printf("DEBUG: extracting vmlinux from %s\n", pkgpath)
	err = j.Pkg.ExtractKernel(ctx, pkgpath, vmlinuxPath)
	if err != nil {
		os.Remove(vmlinuxPath)
		return fmt.Errorf("extracting vmlinux from %s: %s", vmlinuxPath, err)
	}
	log.Printf("DEBUG: finished extracting from %s in %s\n", pkgpath, time.Since(estart))
	os.Remove(pkgpath)
	j.ReplyChan <- vmlinuxPath
	return nil
}

func (j *KernelExtractionJob) Reply() chan<- interface{} {
	return j.ReplyChan
}
