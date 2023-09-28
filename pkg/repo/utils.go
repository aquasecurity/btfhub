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

	"golang.org/x/exp/maps"

	"github.com/aquasecurity/btfhub/pkg/job"
	"github.com/aquasecurity/btfhub/pkg/kernel"
	"github.com/aquasecurity/btfhub/pkg/pkg"
	"github.com/aquasecurity/btfhub/pkg/utils"
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
	binary, args := utils.SudoCMD("yum", "search", "--showduplicates", pkg)
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("yum search %s: %s\n%s", pkg, err, stderr.String())
	}
	return stdout, nil
}

// processPackage creates a kernel extraction job and waits for the reply. It
// then creates a BTF generation job and sends it to the worker. It returns
func processPackage(
	ctx context.Context,
	p pkg.Package,
	workDir string,
	force bool,
	jobChan chan<- job.Job,
) error {

	btfName := fmt.Sprintf("%s.btf", p.Filename())
	btfPath := filepath.Join(workDir, btfName)
	btfTarName := fmt.Sprintf("%s.btf.tar.xz", p.Filename())
	btfTarPath := filepath.Join(workDir, btfTarName)

	if pkg.PackageHasBTF(p, workDir) {
		return utils.ErrHasBTF
	}
	if !force && utils.Exists(btfTarPath) {
		log.Printf("SKIP: %s exists\n", btfTarName)
		return nil
	}

	// 1st job: Extract kernel vmlinux file

	kernelExtJob := &job.KernelExtractionJob{
		Pkg:       p,
		WorkDir:   workDir,
		ReplyChan: make(chan interface{}),
		Force:     force,
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case jobChan <- kernelExtJob: // send vmlinux file extraction job to worker
	}

	reply := <-kernelExtJob.ReplyChan // wait for reply

	var vmlinuxPath string

	switch v := reply.(type) {
	case error:
		return v
	case string:
		vmlinuxPath = v // receive vmlinux path from worker
	}

	// Check if BTF is already present in vmlinux (will skip further packages)

	hasBTF, err := utils.HasBTFSection(vmlinuxPath)
	if err != nil {
		return fmt.Errorf("BTF check: %s", err)
	}
	if hasBTF {
		pkg.MarkPackageHasBTF(p, workDir)
		// Removing here is bad for re-runs (it has to re-download)
		os.Remove(vmlinuxPath)
		return utils.ErrHasBTF
	}

	// 2nd job: Generate BTF file from vmlinux file

	job := &job.BTFGenerationJob{
		VmlinuxPath: vmlinuxPath,
		BTFPath:     btfPath,
		BTFTarPath:  btfTarPath,
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case jobChan <- job: // send BTF generation job to worker
	}

	return nil
}
