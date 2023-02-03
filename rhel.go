package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type rhelRepo struct {
	archs           map[string]string
	releaseVersions map[string]string
	minVersion      kernelVersion
}

func newRHELRepo() Repository {
	return &rhelRepo{
		archs: map[string]string{
			"x86_64": "x86_64",
			"arm64":  "aarch64",
		},
		releaseVersions: map[string]string{
			"7:x86_64":  "7.7",
			"7:aarch64": "7Server",
			"8:x86_64":  "8.1",
			"8:aarch64": "8.1",
		},
		minVersion: newKernelVersion("3.10.0-957"),
	}
}

func (d *rhelRepo) GetKernelPackages(ctx context.Context, dir string, release string, arch string, jobchan chan<- Job) error {
	altArch := d.archs[arch]
	rver := d.releaseVersions[release+":"+altArch]
	if err := runCmd(ctx, "", "sudo", "subscription-manager", "release", fmt.Sprintf("--set=%s", rver)); err != nil {
		return err
	}

	searchOut, err := yumSearch(ctx, "kernel-debuginfo")
	if err != nil {
		return err
	}
	pkgs, err := parseYumPackages(searchOut, d.minVersion)
	if err != nil {
		return fmt.Errorf("parse package listing: %s", err)
	}
	sort.Sort(ByVersion(pkgs))

	for _, pkg := range pkgs {
		err := processPackage(ctx, pkg, dir, jobchan)
		if err != nil {
			if errors.Is(err, ErrHasBTF) {
				log.Printf("INFO: kernel %s has BTF already, skipping later kernels\n", pkg)
				return nil
			}
			return err
		}
	}

	return nil
}

func parseYumPackages(rdr io.Reader, minVersion kernelVersion) ([]Package, error) {
	var pkgs []Package
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
		p := &rhelPackage{
			name:         name,
			filename:     filename,
			version:      newKernelVersion(filename[:lastdot]),
			architecture: filename[lastdot+1:],
		}
		if !minVersion.IsZero() && p.Version().Less(minVersion) {
			continue
		}
		pkgs = append(pkgs, p)
	}
	if err := bio.Err(); err != nil {
		return nil, err
	}
	return pkgs, nil
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

type rhelPackage struct {
	name         string
	filename     string
	architecture string
	version      kernelVersion
}

func (pkg *rhelPackage) Filename() string {
	return pkg.filename
}

func (pkg *rhelPackage) Version() kernelVersion {
	return pkg.version
}

func (pkg *rhelPackage) String() string {
	return pkg.name
}

func (pkg *rhelPackage) ExtractKernel(ctx context.Context, pkgpath string, vmlinuxPath string) error {
	return extractVmlinuxFromRPM(ctx, pkgpath, vmlinuxPath)
}

func (pkg *rhelPackage) Download(ctx context.Context, dir string) (string, error) {
	localFile := fmt.Sprintf("%s.rpm", pkg.name)
	rpmpath := filepath.Join(dir, localFile)
	if exists(rpmpath) {
		return rpmpath, nil
	}

	if err := yumDownload(ctx, pkg.name, dir); err != nil {
		os.Remove(rpmpath)
		return "", fmt.Errorf("rpm download: %s", err)
	}
	// we don't need the common RPM file
	commonrpmpath := strings.ReplaceAll(localFile, "kernel-debuginfo-", fmt.Sprintf("kernel-debuginfo-common-%s-", pkg.architecture))
	os.Remove(commonrpmpath)

	return rpmpath, nil
}

func yumDownload(ctx context.Context, pkg string, destdir string) error {
	stderr := &bytes.Buffer{}
	cmd := exec.CommandContext(ctx, "sudo", "yum", "install", "-y", "--downloadonly", fmt.Sprintf("--downloaddir=%s", destdir), pkg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("yum download %s: %s\n%s", pkg, err, stderr.String())
	}
	return nil
}
