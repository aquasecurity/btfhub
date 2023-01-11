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
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/sync/errgroup"
	"pault.ag/go/debian/deb"
)

type ubuntuRepo struct {
	repo           string
	kernelVersions map[string][]string
	kernelTypes    map[string]string
	archs          map[string]string
}

func newUbuntuRepo() Repository {
	return &ubuntuRepo{
		repo: "http://ddebs.ubuntu.com",
		kernelVersions: map[string][]string{
			"xenial": {"4.4.0", "4.15.0"},
			"bionic": {"4.15.0", "4.18.0", "5.4.0"},
			"focal":  {"5.4.0", "5.8.0", "5.11.0"},
		},
		kernelTypes: map[string]string{
			"signed":   "linux-image-%s-.*-(generic|azure|gke|gcp|aws)-dbgsym",
			"unsigned": "linux-image-unsigned-%s-.*-(generic|azure|gke|gcp|aws)-dbgsym",
		},
		archs: map[string]string{
			"x86_64": "amd64",
			"arm64":  "arm64",
		},
	}
}

func (d *ubuntuRepo) GetKernelPackages(ctx context.Context, dir string, release string, arch string, jobchan chan<- Job) error {
	ktypes := []string{"signed", "unsigned"}
	altArch := d.archs[arch]

	rawPkgs := &bytes.Buffer{}
	if err := download(ctx, fmt.Sprintf("%s/dists/%s/main/binary-%s/Packages", d.repo, release, altArch), rawPkgs); err != nil {
		return fmt.Errorf("download base package list: %s", err)
	}
	if err := download(ctx, fmt.Sprintf("%s/dists/%s-updates/main/binary-%s/Packages", d.repo, release, altArch), rawPkgs); err != nil {
		return fmt.Errorf("download updates package list: %s", err)
	}

	pkgs, err := parseAPTPackages(rawPkgs, d.repo)
	if err != nil {
		return fmt.Errorf("parsing package list: %s", err)
	}
	log.Printf("DEBUG: %d packages\n", len(pkgs))

	pkgsByKernelType := make(map[string][]Package)
	for _, uv := range d.kernelVersions[release] {
		for _, kt := range ktypes {
			re := regexp.MustCompile(fmt.Sprintf(d.kernelTypes[kt], uv))
			for _, p := range pkgs {
				match := re.FindStringSubmatch(p.name)
				if match == nil {
					continue
				}

				kernelType := match[1]
				ks, ok := pkgsByKernelType[kernelType]
				if !ok {
					ks = make([]Package, 0, 1)
				}
				ks = append(ks, p)
				pkgsByKernelType[kernelType] = ks
			}
		}
	}

	log.Printf("DEBUG: %d flavors\n", len(pkgsByKernelType))
	for kt, ks := range pkgsByKernelType {
		sort.Sort(ByVersion(ks))
		log.Printf("DEBUG: %s flavor %d kernels\n", kt, len(ks))
	}

	g, ctx := errgroup.WithContext(ctx)
	for kt, ks := range pkgsByKernelType {
		ckt := kt
		cks := ks
		g.Go(func() error {
			log.Printf("DEBUG: start kernel type %s %s (%d pkgs)\n", ckt, arch, len(cks))
			err := d.processPackages(ctx, dir, cks, jobchan)
			log.Printf("DEBUG: end kernel type %s %s\n", ckt, arch)
			return err
		})
	}
	return g.Wait()
}

func (d *ubuntuRepo) processPackages(ctx context.Context, dir string, pkgs []Package, jobchan chan<- Job) error {
	for i, pkg := range pkgs {
		log.Printf("DEBUG: start pkg %s (%d/%d)\n", pkg, i+1, len(pkgs))
		if err := processPackage(ctx, pkg, dir, jobchan); err != nil {
			if errors.Is(err, ErrHasBTF) {
				log.Printf("INFO: kernel %s has BTF already, skipping later kernels\n", pkg)
				return nil
			}
			if errors.Is(err, context.Canceled) {
				return nil
			}
			log.Printf("ERROR: %s: %s\n", pkg, err)
			continue
		}
		log.Printf("DEBUG: end pkg %s (%d/%d)\n", pkg, i+1, len(pkgs))
	}
	return nil
}

func parseAPTPackages(r io.Reader, baseurl string) ([]*ubuntuPackage, error) {
	var pkgs []*ubuntuPackage
	p := &ubuntuPackage{}
	bio := bufio.NewScanner(r)
	bio.Buffer(make([]byte, 4096), 128*1024)
	for bio.Scan() {
		line := bio.Text()
		if len(line) == 0 {
			// between packages
			if strings.HasPrefix(p.name, "linux-image-") && p.isValid() {
				pkgs = append(pkgs, p)
			}
			p = &ubuntuPackage{}
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
			p.name = val
			fn := strings.TrimPrefix(val, "linux-image-")
			fn = strings.TrimSuffix(fn, "-dbgsym")
			fn = strings.TrimSuffix(fn, "-dbg")
			p.filename = strings.TrimPrefix(fn, "unsigned-")
		case "Architecture":
			p.architecture = val
		case "Version":
			p.version = newKernelVersion(val)
		case "Filename":
			p.url = fmt.Sprintf("%s/%s", baseurl, val)
		case "Size":
			sz, err := strconv.ParseUint(val, 10, 64)
			if err == nil {
				p.size = sz
			}
		default:
			continue
		}
	}
	if err := bio.Err(); err != nil {
		return nil, err
	}
	if p.isValid() && strings.HasPrefix(p.name, "linux-image-") {
		pkgs = append(pkgs, p)
	}

	return pkgs, nil
}

type ubuntuPackage struct {
	name         string
	architecture string
	version      kernelVersion
	filename     string
	url          string
	size         uint64
}

func (pkg *ubuntuPackage) isValid() bool {
	return pkg.name != "" && pkg.url != "" && pkg.filename != "" && pkg.version.String() != "" && pkg.size > 10_000_000
}

func (pkg *ubuntuPackage) Filename() string {
	return pkg.filename
}

func (pkg *ubuntuPackage) Version() kernelVersion {
	return pkg.version
}

func (pkg *ubuntuPackage) String() string {
	return fmt.Sprintf("%s %s", pkg.name, pkg.architecture)
}

func (pkg *ubuntuPackage) Download(ctx context.Context, dir string) (string, error) {
	localFile := fmt.Sprintf("%s.ddeb", pkg.filename)
	ddebpath := filepath.Join(dir, localFile)
	if exists(ddebpath) {
		return ddebpath, nil
	}

	// TODO check for existing ddeb file and check checksum, skip if valid
	if err := downloadFile(ctx, pkg.url, ddebpath); err != nil {
		os.Remove(ddebpath)
		return "", fmt.Errorf("downloading ddeb package: %s", err)
	}
	return ddebpath, nil
}

func (pkg *ubuntuPackage) ExtractKernel(ctx context.Context, pkgpath string, vmlinuxPath string) error {
	vmlinuxName := fmt.Sprintf("vmlinux-%s", pkg.filename)
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

			counter := &progressCounter{Op: "Extract", Name: hdr.Name, Size: uint64(hdr.Size)}
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
