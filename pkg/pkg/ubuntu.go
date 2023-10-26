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

	"pault.ag/go/debian/deb"

	"github.com/aquasecurity/btfhub/pkg/kernel"
	"github.com/aquasecurity/btfhub/pkg/utils"
)

// UbuntuPackage represents a package in Ubuntu
type UbuntuPackage struct {
	Name          string
	Architecture  string
	KernelVersion kernel.Version
	NameOfFile    string
	URL           string
	Size          uint64
	Release       string
	Flavor        string // generic, gcp, aws, azure
}

func (pkg *UbuntuPackage) isValid() bool {
	return pkg.Name != "" && pkg.URL != "" && pkg.NameOfFile != "" && pkg.KernelVersion.String() != ""
}

func (pkg *UbuntuPackage) Filename() string {
	return pkg.NameOfFile
}

func (pkg *UbuntuPackage) BTFFilename() string {
	return pkg.NameOfFile
}

func (pkg *UbuntuPackage) Version() kernel.Version {
	return pkg.KernelVersion
}

func (pkg *UbuntuPackage) String() string {
	return fmt.Sprintf("%s %s", pkg.Name, pkg.Architecture)
}

// Download downloads the package to the specified directory and returns the
// path to the downloaded file.
func (pkg *UbuntuPackage) Download(ctx context.Context, dir string, force bool) (
	string, error,
) {
	localFile := fmt.Sprintf("%s.ddeb", pkg.NameOfFile)
	ddebPath := filepath.Join(dir, localFile)

	if !force && utils.Exists(ddebPath) {
		return ddebPath, nil
	}

	// Deal with meta packages that didn't have a direct ddeb associated
	// (download them using pull-lp-ddebs, which will pick them directly from
	// the launchpad archive)

	if pkg.URL == "pull-lp-ddebs" {
		if err := pkg.pullLaunchpadDdeb(ctx, dir, ddebPath); err != nil {
			os.Remove(ddebPath)
			return "", fmt.Errorf("downloading ddeb package: %s", err)
		}
		return ddebPath, nil
	}

	if err := utils.DownloadFile(ctx, pkg.URL, ddebPath); err != nil {
		os.Remove(ddebPath)
		return "", fmt.Errorf("downloading ddeb package: %s", err)
	}

	return ddebPath, nil
}

// ExtractKernel extracts the vmlinux file from the package and saves it to
// vmlinuxPath. It returns an error if the package is not a ddeb or if the
// vmlinux file is not found.
func (pkg *UbuntuPackage) ExtractKernel(ctx context.Context, pkgPath string, vmlinuxPath string) error {

	vmlinuxName := fmt.Sprintf("vmlinux-%s", pkg.NameOfFile)
	debpath := fmt.Sprintf("./usr/lib/debug/boot/%s", vmlinuxName)

	ddeb, closer, err := deb.LoadFile(pkgPath)
	if err != nil {
		return fmt.Errorf("deb load: %s", err)
	}
	defer closer()

	rdr := ddeb.Data // tar reader for the deb package

	// Iterate over the files in the deb package to find the vmlinux file

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

		// Found the vmlinux file, extract it

		if hdr.Name == debpath {
			vmlinuxFile, err := os.Create(vmlinuxPath)
			if err != nil {
				return fmt.Errorf("create vmlinux file: %s", err)
			}
			counter := &utils.ProgressCounter{
				Ctx:  ctx,
				Op:   "Extract",
				Name: hdr.Name,
				Size: uint64(hdr.Size),
			}
			_, err = io.Copy(vmlinuxFile, io.TeeReader(rdr, counter))
			if err != nil {
				vmlinuxFile.Close()
				os.Remove(vmlinuxPath)
				return fmt.Errorf("copy file: %s", err)
			}
			vmlinuxFile.Close()

			return nil
		}
	}

	return fmt.Errorf("%s file not found in ddeb", debpath)
}

// pullLaunchpadDdeb downloads a ddeb package from launchpad using pull-lp-ddebs
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

	// pull-lp-ddebs will download the ddeb package to the current directory

	scan := bufio.NewScanner(stdout)
	for scan.Scan() {
		line := scan.Text()
		if strings.HasPrefix(line, "Downloading ") {
			fields := strings.Fields(line)
			debPath := filepath.Join(dir, fields[1])
			if err := os.Rename(debPath, dest); err != nil {
				return fmt.Errorf("rename %s to %s: %s", debPath, dest, err)
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
