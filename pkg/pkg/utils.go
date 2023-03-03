package pkg

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aquasecurity/btfhub/pkg/kernel"
	"github.com/aquasecurity/btfhub/pkg/utils"
)

func TarballBTF(ctx context.Context, btf string, out string) error {
	return utils.RunCMD(ctx, filepath.Dir(btf), "tar", "cvfJ", out, filepath.Base(btf))
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

// ubuntu

func indexPackages(pkgs []*UbuntuPackage) map[string]*UbuntuPackage {
	mp := make(map[string]*UbuntuPackage, len(pkgs))
	for _, p := range pkgs {
		mp[p.Filename()] = p
	}
	return mp
}

func GetPackageList(ctx context.Context, repo string, release string, arch string) (*bytes.Buffer, error) {
	rawPkgs := &bytes.Buffer{}
	if err := utils.Download(ctx, fmt.Sprintf("%s/dists/%s/main/binary-%s/Packages.xz", repo, release, arch), rawPkgs); err != nil {
		return nil, fmt.Errorf("download base package list: %s", err)
	}
	if err := utils.Download(ctx, fmt.Sprintf("%s/dists/%s-updates/main/binary-%s/Packages.xz", repo, release, arch), rawPkgs); err != nil {
		return nil, fmt.Errorf("download updates main package list: %s", err)
	}
	if err := utils.Download(ctx, fmt.Sprintf("%s/dists/%s-updates/universe/binary-%s/Packages.xz", repo, release, arch), rawPkgs); err != nil {
		return nil, fmt.Errorf("download updates universe package list: %s", err)
	}
	return rawPkgs, nil
}

func ParseAPTPackages(r io.Reader, baseurl string, release string) ([]*UbuntuPackage, error) {
	var pkgs []*UbuntuPackage
	p := &UbuntuPackage{Release: release}
	bio := bufio.NewScanner(r)
	bio.Buffer(make([]byte, 4096), 128*1024)
	for bio.Scan() {
		line := bio.Text()
		if len(line) == 0 {
			// between packages
			if strings.HasPrefix(p.Name, "linux-image-") && p.isValid() {
				pkgs = append(pkgs, p)
			}
			p = &UbuntuPackage{Release: release}
			continue
		}
		if line[0] == ' ' {
			continue
		}
		name, val, found := strings.Cut(line, ": ")
		if !found {
			continue
		}
		switch name {
		case "Package":
			p.Name = val
			fn := strings.TrimPrefix(val, "linux-image-")
			fn = strings.TrimSuffix(fn, "-dbgsym")
			fn = strings.TrimSuffix(fn, "-dbg")
			p.NameOfFile = strings.TrimPrefix(fn, "unsigned-")
		case "Architecture":
			p.Architecture = val
		case "Version":
			p.KernelVersion = kernel.NewKernelVersion(val)
		case "Filename":
			p.URL = fmt.Sprintf("%s/%s", baseurl, val)
		case "Size":
			sz, err := strconv.ParseUint(val, 10, 64)
			if err == nil {
				p.Size = sz
			}
		default:
			continue
		}
	}
	if err := bio.Err(); err != nil {
		return nil, err
	}
	if p.isValid() && strings.HasPrefix(p.Name, "linux-image-") {
		pkgs = append(pkgs, p)
	}

	return pkgs, nil
}
