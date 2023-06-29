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
	repo        map[string]string // map[altArch]url
	debugRepo   string            // url
	kernelTypes map[string]string // map[signed,unsigned]regex
	archs       map[string]string // map[arch]altArch
}

func NewUbuntuRepo() Repository {
	return &UbuntuRepo{
		repo: map[string]string{
			"amd64": "http://archive.ubuntu.com/ubuntu",
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

// GetKernelPackages downloads Packages.xz from the main, updates and universe,
// from the debug repo and parses the list of kernel packages to download. It
// then filters out kernel packages that we already have or failed to download.
// It then process the list of kernel packages: they will be downloaded and then
// the btf files will be extracted from them.
func (uRepo *UbuntuRepo) GetKernelPackages(
	ctx context.Context,
	workDir string,
	release string,
	arch string,
	force bool,
	jobChan chan<- job.Job,
) error {

	altArch := uRepo.archs[arch]

	// Get Packages.xz from main, updates and universe repos

	repoURL := uRepo.repo[altArch]

	rawPkgs, err := pkg.GetPackageList(ctx, repoURL, release, altArch)
	if err != nil {
		return fmt.Errorf("main: %s", err)
	}

	// Get the list of kernel packages to download from those repos

	kernelPkgs, err := pkg.ParseAPTPackages(rawPkgs, repoURL, release)
	if err != nil {
		return fmt.Errorf("parsing main package list: %s", err)
	}

	// Filter out kernel packages that we already have or failed to download

	var filteredKernelPkgs []*pkg.UbuntuPackage

	for _, restr := range uRepo.kernelTypes {
		re := regexp.MustCompile(fmt.Sprintf("%s$", restr))
		for _, p := range kernelPkgs {
			match := re.FindStringSubmatch(p.Name)
			if match == nil {
				continue
			}
			if !force && (pkg.PackageBTFExists(p, workDir) || pkg.PackageFailed(p, workDir)) {
				continue
			}
			// match = [filename = linux-image-{unsigned}-XXX, flavor = generic, gke, aws, ...]
			p.Flavor = match[1]
			filteredKernelPkgs = append(filteredKernelPkgs, p)
		}
	}

	// Get Packages.xz from debug repo

	dbgRawPkgs, err := pkg.GetPackageList(ctx, uRepo.debugRepo, release, altArch)
	if err != nil {
		return fmt.Errorf("ddebs: %s", err)
	}

	// Get the list of kernel packages to download from debug repo

	kernelDbgPkgs, err := pkg.ParseAPTPackages(dbgRawPkgs, uRepo.debugRepo, release)
	if err != nil {
		return fmt.Errorf("parsing debug package list: %s", err)
	}

	// Filter out kernel packages that we already have or failed to download

	filteredKernelDbgPkgMap := make(map[string]*pkg.UbuntuPackage) // map[filename]package

	for _, restr := range uRepo.kernelTypes {
		re := regexp.MustCompile(fmt.Sprintf("%s-dbgsym", restr))

		for _, p := range kernelDbgPkgs {
			match := re.FindStringSubmatch(p.Name)
			if match == nil {
				continue
			}
			if p.Size < 10_000_000 { // ignore smaller than 10MB (signed vs unsigned emptiness)
				continue
			}
			if !force && (pkg.PackageBTFExists(p, workDir) || pkg.PackageFailed(p, workDir)) {
				continue
			}
			// match = [filename = linux-image-{unsigned}-XXX-dbgsym, flavor = generic, gke, aws, ...]
			p.Flavor = match[1]
			if dp, ok := filteredKernelDbgPkgMap[p.Filename()]; !ok {
				filteredKernelDbgPkgMap[p.Filename()] = p
			} else {
				log.Printf("DEBUG: duplicate %s filename from %s (other %s)", p.Filename(), p, dp)
			}
		}
	}

	// Check if debug package exists for each kernel package and, if not,
	// add pseudo-packages for the missing entries (try pull-lp-ddebs later on)

	for _, p := range filteredKernelPkgs {
		_, ok := filteredKernelDbgPkgMap[p.Filename()]
		if !ok {
			log.Printf("DEBUG: adding launchpad package for %s\n", p.Name)
			filteredKernelDbgPkgMap[p.Filename()] = &pkg.UbuntuPackage{
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

	log.Printf("DEBUG: %d %s packages\n", len(filteredKernelDbgPkgMap), arch)

	// type: signed/unsigned
	// flavor: generic, gcp, aws, ...

	pkgsByKernelFlavor := make(map[string][]pkg.Package)

	for _, p := range filteredKernelDbgPkgMap { // map[filename]package
		pkgSlice, ok := pkgsByKernelFlavor[p.Flavor]
		if !ok {
			pkgSlice = make([]pkg.Package, 0, 1)
		}
		pkgSlice = append(pkgSlice, p)
		pkgsByKernelFlavor[p.Flavor] = pkgSlice
	}

	log.Printf("DEBUG: %d %s flavors\n", len(pkgsByKernelFlavor), arch)

	for flavor, pkgSlice := range pkgsByKernelFlavor {
		sort.Sort(pkg.ByVersion(pkgSlice)) // so kernels can be skipped if previous has BTF already
		log.Printf("DEBUG: %s %s flavor %d kernels\n", arch, flavor, len(pkgSlice))
	}

	g, ctx := errgroup.WithContext(ctx)

	for flavor, pkgSlice := range pkgsByKernelFlavor {
		theFlavor := flavor
		thePkgSlice := pkgSlice

		// Start a goroutine for each flavor to process all of its packages

		g.Go(func() error {
			log.Printf("DEBUG: start kernel flavor %s %s (%d pkgs)\n", theFlavor, arch, len(thePkgSlice))
			err := uRepo.processPackages(ctx, workDir, thePkgSlice, force, jobChan)
			log.Printf("DEBUG: end kernel flavor %s %s\n", theFlavor, arch)
			return err
		})
	}

	return g.Wait()
}

// processPackages processes a list of packages, sending jobs to the job channel.
func (d *UbuntuRepo) processPackages(
	ctx context.Context,
	workDir string,
	pkgs []pkg.Package,
	force bool,
	jobChan chan<- job.Job,
) error {

	for i, pkg := range pkgs {
		log.Printf("DEBUG: start pkg %s (%d/%d)\n", pkg, i+1, len(pkgs))

		// Jobs about to be created:
		//
		// 1. Download package and extract vmlinux file
		// 2. Extract BTF info from vmlinux file

		if err := processPackage(ctx, pkg, workDir, force, jobChan); err != nil {
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
