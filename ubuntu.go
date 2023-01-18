package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/sync/errgroup"
	"pault.ag/go/debian/deb"
)

type ubuntuRepo struct {
	repo        map[string]string // arch to url
	debugRepo   string
	kernelTypes map[string]string
	archs       map[string]string
}

func newUbuntuRepo() Repository {
	return &ubuntuRepo{
		repo: map[string]string{
			"amd64": "http://us-east-1.ec2.archive.ubuntu.com/ubuntu",
			"arm64": "http://ports.ubuntu.com",
		},
		debugRepo: "http://ddebs.ubuntu.com",
		kernelTypes: map[string]string{
			"signed":   "linux-image-[0-9.]+-.*-(generic|azure|gke|gkeop|gcp|aws)",
			"unsigned": "linux-image-unsigned-[0-9.]+-.*-(generic|azure|gke|gkeop|gcp|aws)",
		},
		archs: map[string]string{
			"x86_64": "amd64",
			"arm64":  "arm64",
		},
	}
}

func indexPackages(pkgs []*ubuntuPackage) map[string]*ubuntuPackage {
	mp := make(map[string]*ubuntuPackage, len(pkgs))
	for _, p := range pkgs {
		mp[p.Filename()] = p
	}
	return mp
}

func getPackageList(ctx context.Context, repo string, release string, arch string) (*bytes.Buffer, error) {
	rawPkgs := &bytes.Buffer{}
	if err := download(ctx, fmt.Sprintf("%s/dists/%s/main/binary-%s/Packages.xz", repo, release, arch), rawPkgs); err != nil {
		return nil, fmt.Errorf("download base package list: %s", err)
	}
	if err := download(ctx, fmt.Sprintf("%s/dists/%s-updates/main/binary-%s/Packages.xz", repo, release, arch), rawPkgs); err != nil {
		return nil, fmt.Errorf("download updates main package list: %s", err)
	}
	if err := download(ctx, fmt.Sprintf("%s/dists/%s-updates/universe/binary-%s/Packages.xz", repo, release, arch), rawPkgs); err != nil {
		return nil, fmt.Errorf("download updates universe package list: %s", err)
	}
	return rawPkgs, nil
}

func (d *ubuntuRepo) GetKernelPackages(ctx context.Context, dir string, release string, arch string, jobchan chan<- Job) error {
	altArch := d.archs[arch]

	// get main apt kernel list
	rawPkgs, err := getPackageList(ctx, d.repo[altArch], release, altArch)
	if err != nil {
		return fmt.Errorf("main: %s", err)
	}
	pkgs, err := parseAPTPackages(rawPkgs, d.repo[altArch], release)
	if err != nil {
		return fmt.Errorf("parsing main package list: %s", err)
	}

	var filteredPkgs []*ubuntuPackage
	for _, restr := range d.kernelTypes {
		re := regexp.MustCompile(fmt.Sprintf("%s$", restr))
		for _, p := range pkgs {
			match := re.FindStringSubmatch(p.name)
			if match == nil {
				continue
			}
			if packageBTFExists(p, dir) || packageFailed(p, dir) {
				continue
			}
			p.flavor = match[1]
			filteredPkgs = append(filteredPkgs, p)
		}
	}

	// get ddebs package list
	dbgRawPkgs, err := getPackageList(ctx, d.debugRepo, release, altArch)
	if err != nil {
		return fmt.Errorf("ddebs: %s", err)
	}
	dbgPkgs, err := parseAPTPackages(dbgRawPkgs, d.debugRepo, release)
	if err != nil {
		return fmt.Errorf("parsing debug package list: %s", err)
	}
	dbgPkgMap := make(map[string]*ubuntuPackage)
	for _, restr := range d.kernelTypes {
		re := regexp.MustCompile(fmt.Sprintf("%s-dbgsym", restr))
		for _, p := range dbgPkgs {
			match := re.FindStringSubmatch(p.name)
			if match == nil {
				continue
			}
			if p.size < 10_000_000 {
				continue
			}
			if packageBTFExists(p, dir) || packageFailed(p, dir) {
				continue
			}
			p.flavor = match[1]
			if dp, ok := dbgPkgMap[p.Filename()]; !ok {
				dbgPkgMap[p.Filename()] = p
			} else {
				log.Printf("DEBUG: duplicate %s filename from %s (other %s)", p.Filename(), p, dp)
			}
		}
	}

	// add pseudo-packages for missing entries to try pull-lp-ddebs
	for _, p := range filteredPkgs {
		_, ok := dbgPkgMap[p.Filename()]
		if !ok {
			log.Printf("DEBUG: adding launchpad package for %s\n", p.name)
			dbgPkgMap[p.Filename()] = &ubuntuPackage{
				// always use unsigned, because signed never has the actual kernel
				name:         fmt.Sprintf("linux-image-unsigned-%s-dbgsym", p.Filename()),
				architecture: p.architecture,
				version:      p.version,
				filename:     p.filename,
				size:         math.MaxUint64,
				flavor:       p.flavor,
				url:          "pull-lp-ddebs",
			}
		}
	}

	log.Printf("DEBUG: %d %s packages\n", len(dbgPkgMap), arch)
	pkgsByKernelType := make(map[string][]Package)
	for _, p := range dbgPkgMap {
		ks, ok := pkgsByKernelType[p.flavor]
		if !ok {
			ks = make([]Package, 0, 1)
		}
		ks = append(ks, p)
		pkgsByKernelType[p.flavor] = ks
	}

	log.Printf("DEBUG: %d %s flavors\n", len(pkgsByKernelType), arch)
	for kt, ks := range pkgsByKernelType {
		sort.Sort(ByVersion(ks))
		log.Printf("DEBUG: %s %s flavor %d kernels\n", arch, kt, len(ks))
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

func parseAPTPackages(r io.Reader, baseurl string, release string) ([]*ubuntuPackage, error) {
	var pkgs []*ubuntuPackage
	p := &ubuntuPackage{release: release}
	bio := bufio.NewScanner(r)
	bio.Buffer(make([]byte, 4096), 128*1024)
	for bio.Scan() {
		line := bio.Text()
		if len(line) == 0 {
			// between packages
			if strings.HasPrefix(p.name, "linux-image-") && p.isValid() {
				pkgs = append(pkgs, p)
			}
			p = &ubuntuPackage{release: release}
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
	release      string
	flavor       string
}

func (pkg *ubuntuPackage) isValid() bool {
	return pkg.name != "" && pkg.url != "" && pkg.filename != "" && pkg.version.String() != ""
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

	if pkg.url == "pull-lp-ddebs" {
		if err := pkg.pullLaunchpadDdeb(ctx, dir, ddebpath); err != nil {
			os.Remove(ddebpath)
			return "", fmt.Errorf("downloading ddeb package: %s", err)
		}
		return ddebpath, nil
	}

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

func (pkg *ubuntuPackage) pullLaunchpadDdeb(ctx context.Context, dir string, dest string) error {
	fmt.Printf("Downloading %s from launchpad\n", pkg.name)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd := exec.CommandContext(ctx, "pull-lp-ddebs", "--arch", pkg.architecture, pkg.name, pkg.release)
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
