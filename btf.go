package main

import (
	"bytes"
	"context"
	"debug/elf"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var ErrHasBTF = errors.New("vmlinux has .BTF section")

func hasBTFSection(name string) (bool, error) {
	ef, err := elf.Open(name)
	if err != nil {
		return false, fmt.Errorf("elf open: %s", err)
	}
	return ef.Section(".BTF") != nil, nil
}

func generateBTF(ctx context.Context, vmlinux string, out string) error {
	return runCmd(ctx, "", "pahole", "--btf_encode_detached", out, vmlinux)
}

func tarballBTF(ctx context.Context, btf string, out string) error {
	return runCmd(ctx, filepath.Dir(btf), "tar", "cvfJ", out, filepath.Base(btf))
}

func runCmd(ctx context.Context, cwd string, binary string, args ...string) error {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = cwd
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %s\n%s\n%s", binary, strings.Join(args, " "), err, stdout.String(), stderr.String())
	}
	return nil
}

type btfGenerationJob struct {
	vmlinuxPath string
	btfPath     string
	btfTarPath  string
}

func (j *btfGenerationJob) Do(ctx context.Context) error {
	log.Printf("DEBUG: generating BTF from %s\n", j.vmlinuxPath)
	gstart := time.Now()
	if err := generateBTF(ctx, j.vmlinuxPath, j.btfPath); err != nil {
		os.Remove(j.btfPath)
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return fmt.Errorf("btf gen: %s", err)
	}
	log.Printf("DEBUG: finished generating BTF from %s in %s\n", j.vmlinuxPath, time.Since(gstart))

	log.Printf("DEBUG: compressing BTF into %s\n", j.btfTarPath)
	tstart := time.Now()
	if err := tarballBTF(ctx, j.btfPath, j.btfTarPath); err != nil {
		os.Remove(j.btfTarPath)
		return fmt.Errorf("btf.tar.xz gen: %s", err)
	}
	log.Printf("DEBUG: finished compressing BTF into %s in %s\n", j.btfTarPath, time.Since(tstart))

	// only remove valid files on success, to enable resuming
	os.Remove(j.btfPath)
	os.Remove(j.vmlinuxPath)

	return nil
}

func (j *btfGenerationJob) Reply() chan<- interface{} {
	return nil
}
