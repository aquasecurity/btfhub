package job

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aquasecurity/btfhub/pkg/pkg"
)

type BTFGenerationJob struct {
	VmlinuxPath string
	BTFPath     string
	BTFTarPath  string
}

// Do implements the Job interface, and is called by the worker. It generates a
// BTF file from a vmlinux file, compresses it into a .tar.xz file, and removes
// the vmlinux file.
func (job *BTFGenerationJob) Do(ctx context.Context) error {

	// Generate the BTF file from the vmlinux file

	log.Printf("DEBUG: generating BTF from %s\n", job.VmlinuxPath)
	btfGenStart := time.Now()

	if err := GenerateBTF(ctx, job.VmlinuxPath, job.BTFPath); err != nil {
		os.Remove(job.BTFPath)
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return fmt.Errorf("btf gen: %s", err)
	}

	log.Printf("DEBUG: finished generating BTF from %s in %s\n", job.VmlinuxPath, time.Since(btfGenStart))

	// Compress BTF file into a .tar.xz file

	log.Printf("DEBUG: compressing BTF into %s\n", job.BTFTarPath)
	tarCompressStart := time.Now()

	if err := pkg.TarballBTF(ctx, job.BTFPath, job.BTFTarPath); err != nil {
		os.Remove(job.BTFTarPath)
		return fmt.Errorf("btf.tar.xz gen: %s", err)
	}

	log.Printf("DEBUG: finished compressing BTF into %s in %s\n", job.BTFTarPath, time.Since(tarCompressStart))

	// Remove valid files on success (keep files on fail to enable resuming)

	os.Remove(job.BTFPath)
	os.Remove(job.VmlinuxPath)

	return nil
}

func (job *BTFGenerationJob) Reply() chan<- interface{} {
	return nil
}
