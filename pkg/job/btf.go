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

func (j *BTFGenerationJob) Do(ctx context.Context) error {
	log.Printf("DEBUG: generating BTF from %s\n", j.VmlinuxPath)
	gstart := time.Now()
	if err := GenerateBTF(ctx, j.VmlinuxPath, j.BTFPath); err != nil {
		os.Remove(j.BTFPath)
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return fmt.Errorf("btf gen: %s", err)
	}
	log.Printf("DEBUG: finished generating BTF from %s in %s\n", j.VmlinuxPath, time.Since(gstart))

	log.Printf("DEBUG: compressing BTF into %s\n", j.BTFTarPath)
	tstart := time.Now()
	if err := pkg.TarballBTF(ctx, j.BTFPath, j.BTFTarPath); err != nil {
		os.Remove(j.BTFTarPath)
		return fmt.Errorf("btf.tar.xz gen: %s", err)
	}
	log.Printf("DEBUG: finished compressing BTF into %s in %s\n", j.BTFTarPath, time.Since(tstart))

	// only remove valid files on success, to enable resuming
	os.Remove(j.BTFPath)
	os.Remove(j.VmlinuxPath)

	return nil
}

func (j *BTFGenerationJob) Reply() chan<- interface{} {
	return nil
}
