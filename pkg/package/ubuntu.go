package pkg

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aquasecurity/btfhub/pkg/kernel"
	"github.com/aquasecurity/btfhub/pkg/utils"
	"pault.ag/go/debian/deb"
)

type UbuntuPackage struct {
	Name          string
	Architecture  string
	KernelVersion kernel.KernelVersion
	NameOfFile    string
	URL           string
	Size          uint64
	Release       string
	Flavor        string
}

func (pkg *UbuntuPackage) isValid() bool {
	return pkg.Name != "" && pkg.URL != "" && pkg.NameOfFile != "" && pkg.KernelVersion.String() != ""
}

func (pkg *UbuntuPackage) Filename() string {
	return pkg.NameOfFile
}

func (pkg *UbuntuPackage) Version() kernel.KernelVersion {
	return pkg.KernelVersion
}

func (pkg *UbuntuPackage) String() string {
	return fmt.Sprintf("%s %s", pkg.Name, pkg.Architecture)
}

func (pkg *UbuntuPackage) Download(ctx context.Context, dir string) (string, error) {
	localFile := fmt.Sprintf("%s.ddeb", pkg.NameOfFile)
	ddebpath := filepath.Join(dir, localFile)
	if utils.Exists(ddebpath) {
		return ddebpath, nil
	}

	if pkg.URL == "pull-lp-ddebs" {
		if err := pkg.pullLaunchpadDdeb(ctx, dir, ddebpath); err != nil {
			os.Remove(ddebpath)
			return "", fmt.Errorf("downloading ddeb package: %s", err)
		}
		return ddebpath, nil
	}

	if err := utils.DownloadFile(ctx, pkg.URL, ddebpath); err != nil {
		os.Remove(ddebpath)
		return "", fmt.Errorf("downloading ddeb package: %s", err)
	}
	return ddebpath, nil
}

func (pkg *UbuntuPackage) ExtractKernel(ctx context.Context, pkgpath string, vmlinuxPath string) error {
	vmlinuxName := fmt.Sprintf("vmlinux-%s", pkg.NameOfFile)
	debpath := fmt.Sprintf("./usr/lib/debug/boot/%s", vmlinuxName)
	ddeb, closer, err := deb.LoadFile(pkgpath)
	if err != nil {
		return fmt.Errorf("deb load: %s", err)
	}
	defer closer()

	rdr := ddeb.Data
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		hdr, err := rdr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("deb reader next: %s", err)
		}
		if hdr.Name == debpath {
			vmf, err := os.Create(vmlinuxPath)
			if err != nil {
				return fmt.Errorf("create vmlinux file: %s", err)
			}

			counter := &utils.ProgressCounter{Op: "Extract", Name: hdr.Name, Size: uint64(hdr.Size)}
			if _, err := io.Copy(vmf, io.TeeReader(rdr, counter)); err != nil {
				vmf.Close()
				os.Remove(vmlinuxPath)
				return fmt.Errorf("copy file: %s", err)
			}
			vmf.Close()
			return nil
		}
	}
	return fmt.Errorf("%s file not found in ddeb", debpath)
}

func (pkg *UbuntuPackage) pullLaunchpadDdeb(ctx context.Context, dir string, dest string) error {
	fmt.Printf("Downloading %s from launchpad\n", pkg.Name)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd := exec.CommandContext(ctx, "pull-lp-ddebs", "--arch", pkg.Architecture, pkg.Name, pkg.Release)
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pull-lp-ddebs: %s\n%s\n%s", err, stdout.String(), stderr.String())
	}

	scan := bufio.NewScanner(stdout)
	for scan.Scan() {
		line := scan.Text()
		if strings.HasPrefix(line, "Downloading ") {
			fields := strings.Fields(line)
			debpath := filepath.Join(dir, fields[1])
			if err := os.Rename(debpath, dest); err != nil {
				return fmt.Errorf("rename %s to %s: %s", debpath, dest, err)
			}
			return nil
		}
	}
	if scan.Err() != nil {
		return scan.Err()
	}
	errline := stderr.String()
	if len(errline) > 0 {
		return fmt.Errorf(strings.TrimSpace(errline))
	}
	return fmt.Errorf("download path not found in pull-lp-ddebs output")
}
