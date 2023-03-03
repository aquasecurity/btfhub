package repo

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"regexp"
	"sort"

	"github.com/aquasecurity/btfhub/pkg/job"
	"github.com/aquasecurity/btfhub/pkg/pkg"
	"github.com/aquasecurity/btfhub/pkg/utils"
	"golang.org/x/sync/errgroup"
)

type UbuntuRepo struct {
	repo        map[string]string // arch to url
	debugRepo   string
	kernelTypes map[string]string
	archs       map[string]string
}

func NewUbuntuRepo() Repository {
	return &UbuntuRepo{
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

func (d *UbuntuRepo) GetKernelPackages(ctx context.Context, dir string, release string, arch string, jobchan chan<- job.Job) error {
	altArch := d.archs[arch]

	// get main apt kernel list
	rawPkgs, err := pkg.GetPackageList(ctx, d.repo[altArch], release, altArch)
	if err != nil {
		return fmt.Errorf("main: %s", err)
	}
	pkgs, err := pkg.ParseAPTPackages(rawPkgs, d.repo[altArch], release)
	if err != nil {
		return fmt.Errorf("parsing main package list: %s", err)
	}

	var filteredPkgs []*pkg.UbuntuPackage
	for _, restr := range d.kernelTypes {
		re := regexp.MustCompile(fmt.Sprintf("%s$", restr))
		for _, p := range pkgs {
			match := re.FindStringSubmatch(p.Name)
			if match == nil {
				continue
			}
			if pkg.PackageBTFExists(p, dir) || pkg.PackageFailed(p, dir) {
				continue
			}
			p.Flavor = match[1]
			filteredPkgs = append(filteredPkgs, p)
		}
	}

	// get ddebs package list
	dbgRawPkgs, err := pkg.GetPackageList(ctx, d.debugRepo, release, altArch)
	if err != nil {
		return fmt.Errorf("ddebs: %s", err)
	}
	dbgPkgs, err := pkg.ParseAPTPackages(dbgRawPkgs, d.debugRepo, release)
	if err != nil {
		return fmt.Errorf("parsing debug package list: %s", err)
	}
	dbgPkgMap := make(map[string]*pkg.UbuntuPackage)
	for _, restr := range d.kernelTypes {
		re := regexp.MustCompile(fmt.Sprintf("%s-dbgsym", restr))
		for _, p := range dbgPkgs {
			match := re.FindStringSubmatch(p.Name)
			if match == nil {
				continue
			}
			if p.Size < 10_000_000 {
				continue
			}
			if pkg.PackageBTFExists(p, dir) || pkg.PackageFailed(p, dir) {
				continue
			}
			p.Flavor = match[1]
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
			log.Printf("DEBUG: adding launchpad package for %s\n", p.Name)
			dbgPkgMap[p.Filename()] = &pkg.UbuntuPackage{
				// always use unsigned, because signed never has the actual kernel
				Name:          fmt.Sprintf("linux-image-unsigned-%s-dbgsym", p.Filename()),
				Architecture:  p.Architecture,
				KernelVersion: p.KernelVersion,
				NameOfFile:    p.NameOfFile,
				Size:          math.MaxUint64,
				Flavor:        p.Flavor,
				URL:           "pull-lp-ddebs",
			}
		}
	}

	log.Printf("DEBUG: %d %s packages\n", len(dbgPkgMap), arch)
	pkgsByKernelType := make(map[string][]pkg.Package)
	for _, p := range dbgPkgMap {
		ks, ok := pkgsByKernelType[p.Flavor]
		if !ok {
			ks = make([]pkg.Package, 0, 1)
		}
		ks = append(ks, p)
		pkgsByKernelType[p.Flavor] = ks
	}

	log.Printf("DEBUG: %d %s flavors\n", len(pkgsByKernelType), arch)
	for kt, ks := range pkgsByKernelType {
		sort.Sort(pkg.ByVersion(ks))
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

func (d *UbuntuRepo) processPackages(ctx context.Context, dir string, pkgs []pkg.Package, jobchan chan<- job.Job) error {
	for i, pkg := range pkgs {
		log.Printf("DEBUG: start pkg %s (%d/%d)\n", pkg, i+1, len(pkgs))
		if err := processPackage(ctx, pkg, dir, jobchan); err != nil {
			if errors.Is(err, utils.ErrHasBTF) {
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
