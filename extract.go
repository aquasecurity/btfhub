package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

type kernelExtractionJob struct {
	pkg   Package
	dir   string
	reply chan interface{}
}

func (j *kernelExtractionJob) Do(ctx context.Context) error {
	vmlinuxName := fmt.Sprintf("vmlinux-%s", j.pkg.Filename())
	vmlinuxPath := filepath.Join(j.dir, vmlinuxName)
	if exists(vmlinuxPath) {
		j.reply <- vmlinuxPath
		return nil
	}

	dstart := time.Now()
	log.Printf("DEBUG: downloading %s\n", j.pkg)
	pkgpath, err := j.pkg.Download(ctx, j.dir)
	if err != nil {
		os.Remove(pkgpath)
		return err
	}
	log.Printf("DEBUG: finished downloading %s in %s\n", j.pkg, time.Since(dstart))

	estart := time.Now()
	log.Printf("DEBUG: extracting vmlinux from %s\n", pkgpath)
	err = j.pkg.ExtractKernel(ctx, pkgpath, vmlinuxPath)
	if err != nil {
		os.Remove(vmlinuxPath)
		return fmt.Errorf("extracting vmlinux from %s: %s", vmlinuxPath, err)
	}
	log.Printf("DEBUG: finished extracting from %s in %s\n", pkgpath, time.Since(estart))
	os.Remove(pkgpath)
	j.reply <- vmlinuxPath
	return nil
}

func (j *kernelExtractionJob) Reply() chan<- interface{} {
	return j.reply
}
