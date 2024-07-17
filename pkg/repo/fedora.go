package repo

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"

	"github.com/aquasecurity/btfhub/pkg/job"
	"github.com/aquasecurity/btfhub/pkg/kernel"
	"github.com/aquasecurity/btfhub/pkg/pkg"
	"github.com/aquasecurity/btfhub/pkg/utils"
)

type FedoraRepo struct {
	archs map[string]string
	repos map[string][]string
}

var olderRepoOrganization = []string{
	"https://archives.fedoraproject.org/pub/archive/fedora/linux/releases/%s/Everything/%s/debug/tree/Packages/k/",
	"https://archives.fedoraproject.org/pub/archive/fedora/linux/updates/%s/%s/debug/k/",
}

var oldRepoOrganization = []string{
	"https://archives.fedoraproject.org/pub/archive/fedora/linux/releases/%s/Everything/%s/debug/tree/Packages/k/",
	"https://archives.fedoraproject.org/pub/archive/fedora/linux/updates/%s/%s/debug/Packages/k/",
}

var repoOrganization = []string{
	"https://archives.fedoraproject.org/pub/archive/fedora/linux/releases/%s/Everything/%s/debug/tree/Packages/k/",
	"https://archives.fedoraproject.org/pub/archive/fedora/linux/updates/%s/Everything/%s/debug/Packages/k/",
}

func NewFedoraRepo() Repository {
	return &FedoraRepo{
		archs: map[string]string{
			"x86_64": "x86_64",
			"arm64":  "aarch64",
		},
		repos: map[string][]string{
			"24": olderRepoOrganization, // amd64
			"25": oldRepoOrganization,   // amd64
			"26": oldRepoOrganization,   // amd64
			"27": oldRepoOrganization,   // amd64
			"28": repoOrganization,      // amd64, arm64
			"29": repoOrganization,      // amd64, arm64
			"30": repoOrganization,      // amd64, arm64
			"31": repoOrganization,      // amd64, arm64
			// "32": repoOrganization,
			// "33": repoOrganization,
			// "34": repoOrganization,
			// "35": repoOrganization,
			// "36": repoOrganization,
			// ...
		},
	}
}

func (d *FedoraRepo) GetKernelPackages(
	ctx context.Context,
	workDir string,
	release string,
	arch string,
	force bool,
	jobChan chan<- job.Job,
) error {

	if release == "24" || release == "25" || release == "26" || release == "27" {
		if arch == "arm64" {
			log.Printf("INFO: Fedora %s does not have arm64 packages\n", release)
			return nil
		}
	}

	var pkgs []pkg.Package
	var links []string
	var repos []string

	altArch := d.archs[arch]

	for _, r := range d.repos[release] {
		repoURL := fmt.Sprintf(r, release, altArch)
		repos = append(repos, repoURL)
	}

	// Pick all the links from multiple repositories

	for _, repo := range repos {
		rlinks, err := utils.GetLinks(ctx, repo)
		if err != nil {
			log.Printf("ERROR: list packages: %s\n", err)
			continue
		}
		links = append(links, rlinks...)
	}

	// Only links that match the kernel-debuginfo pattern

	kre := regexp.MustCompile(fmt.Sprintf(`kernel-debuginfo-([0-9].*\.%s)\.rpm`, altArch))

	for _, l := range links {
		match := kre.FindStringSubmatch(l)
		if match != nil {
			name := strings.TrimSuffix(match[0], ".rpm")

			// Create a package object from the link and add it to pkgs list

			p := &pkg.FedoraPackage{
				Name:          name,
				NameOfFile:    match[1],
				Architecture:  altArch,
				URL:           l,
				KernelVersion: kernel.NewKernelVersion(match[1]),
			}

			pkgs = append(pkgs, p)
		}
	}

	sort.Sort(pkg.ByVersion(pkgs)) // so kernels can be skipped if previous has BTF already

	for i, pkg := range pkgs {
		log.Printf("DEBUG: start pkg %s (%d/%d)\n", pkg, i+1, len(pkgs))

		// Jobs about to be created:
		//
		// 1. Download package and extract vmlinux file
		// 2. Extract BTF info from vmlinux file

		err := processPackage(ctx, pkg, workDir, force, jobChan)
		if err != nil {
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
