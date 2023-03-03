package repo

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aquasecurity/btfhub/pkg/job"
	"github.com/aquasecurity/btfhub/pkg/kernel"
	"github.com/aquasecurity/btfhub/pkg/pkg"
	"github.com/aquasecurity/btfhub/pkg/utils"
	"golang.org/x/exp/maps"
)

func parseYumPackages(rdr io.Reader, minVersion kernel.Version) ([]pkg.Package, error) {
	pkgs := map[string]pkg.Package{}
	bio := bufio.NewScanner(rdr)
	for bio.Scan() {
		line := bio.Text()
		if !strings.HasPrefix(line, "kernel-debuginfo-") {
			continue
		}
		if strings.HasPrefix(line, "kernel-debuginfo-common-") {
			continue
		}
		name, _, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		name = strings.TrimSpace(name)
		filename := strings.TrimPrefix(name, "kernel-debuginfo-")
		lastdot := strings.LastIndex(filename, ".")
		if lastdot == -1 {
			continue
		}
		p := &pkg.RHELPackage{
			Name:          name,
			NameOfFile:    filename,
			KernelVersion: kernel.NewKernelVersion(filename[:lastdot]),
			Architecture:  filename[lastdot+1:],
		}
		if !minVersion.IsZero() && p.Version().Less(minVersion) {
			continue
		}
		if _, ok := pkgs[p.Name]; !ok {
			pkgs[p.Name] = p
		}
	}
	if err := bio.Err(); err != nil {
		return nil, err
	}
	return maps.Values(pkgs), nil
}

func yumSearch(ctx context.Context, pkg string) (*bytes.Buffer, error) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd := exec.CommandContext(ctx, "sudo", "yum", "search", "--showduplicates", pkg)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("yum search %s: %s\n%s", pkg, err, stderr.String())
	}
	return stdout, nil
}

func processPackage(ctx context.Context, pkg pkg.Package, dir string, jobchan chan<- job.Job) error {
	btfName := fmt.Sprintf("%s.btf", pkg.Filename())
	btfPath := filepath.Join(dir, btfName)
	btfTarName := fmt.Sprintf("%s.btf.tar.xz", pkg.Filename())
	btfTarPath := filepath.Join(dir, btfTarName)
	if utils.Exists(btfTarPath) {
		log.Printf("SKIP: %s exists\n", btfTarName)
		return nil
	}

	var vmlinuxPath string
	kj := &job.KernelExtractionJob{
		Pkg:       pkg,
		Dir:       dir,
		ReplyChan: make(chan interface{}),
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case jobchan <- kj:
	}

	reply := <-kj.ReplyChan
	switch v := reply.(type) {
	case error:
		return v
	case string:
		vmlinuxPath = v
	}

	hasBTF, err := utils.HasBTFSection(vmlinuxPath)
	if err != nil {
		return fmt.Errorf("btf check: %s", err)
	}
	if hasBTF {
		// removing here is bad for re-runs, because it has to re-download
		os.Remove(vmlinuxPath)
		return utils.ErrHasBTF
	}

	job := &job.BTFGenerationJob{
		VmlinuxPath: vmlinuxPath,
		BTFPath:     btfPath,
		BTFTarPath:  btfTarPath,
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case jobchan <- job:
	}
	return nil
}
