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
	WorkDir   string
	ReplyChan chan interface{}
	Force     bool
}

// Do implements the Job interface, and is called by the worker. It downloads
// the kernel package, extracts the vmlinux file, and replies with the path to
// the vmlinux file in the reply channel.
func (job *KernelExtractionJob) Do(ctx context.Context) error {

	vmlinuxName := fmt.Sprintf("vmlinux-%s", job.Pkg.Filename())
	vmlinuxPath := filepath.Join(job.WorkDir, vmlinuxName)

	if !job.Force && utils.Exists(vmlinuxPath) {
		job.ReplyChan <- vmlinuxPath // already extracted, reply with path
		return nil
	}

	// Download the kernel package

	downloadStart := time.Now()
	log.Printf("DEBUG: downloading %s\n", job.Pkg)

	kernPkgPath, err := job.Pkg.Download(ctx, job.WorkDir, job.Force)
	if err != nil {
		os.Remove(kernPkgPath)
		return err
	}

	log.Printf("DEBUG: finished downloading %s in %s\n", job.Pkg, time.Since(downloadStart))

	// Extract downloaded kernel package

	extractStart := time.Now()
	log.Printf("DEBUG: extracting vmlinux from %s\n", kernPkgPath)

	err = job.Pkg.ExtractKernel(ctx, kernPkgPath, vmlinuxPath)
	if err != nil {
		os.Remove(vmlinuxPath)
		return fmt.Errorf("extracting vmlinux from %s: %s", vmlinuxPath, err)
	}

	log.Printf("DEBUG: finished extracting from %s in %s\n", kernPkgPath, time.Since(extractStart))

	os.Remove(kernPkgPath) // remove downloaded kernel package

	// Reply with the path to the extracted vmlinux file

	job.ReplyChan <- vmlinuxPath

	return nil
}

func (job *KernelExtractionJob) Reply() chan<- interface{} {
	return job.ReplyChan
}
