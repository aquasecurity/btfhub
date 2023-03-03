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
	archs      map[string]string
	repos      map[string][]string
	minVersion kernel.Version
}

var centosArchives = []string{
	"https://archives.fedoraproject.org/pub/archive/fedora/linux/releases/%s/Everything/%s/debug/tree/Packages/k/",
	"https://archives.fedoraproject.org/pub/archive/fedora/linux/updates/%s/Everything/%s/debug/Packages/k/",
}
var centosDownload = []string{
	"https://dl.fedoraproject.org/pub/fedora/linux/releases/%s/Everything/%s/debug/tree/Packages/k/",
}

func NewFedoraRepo() Repository {
	return &FedoraRepo{
		archs: map[string]string{
			"x86_64": "x86_64",
			"arm64":  "aarch64",
		},
		repos: map[string][]string{
			"24": centosArchives,
			"25": centosArchives,
			"26": centosArchives,
			"27": centosArchives,
			"28": centosArchives,
			"29": centosArchives,
			"30": centosArchives,
			"31": centosArchives,
			//"32": centosArchives,
			//"33": centosArchives,
			//"34": centosDownload,
		},
		minVersion: kernel.NewKernelVersion("3.10.0-957"),
	}
}

func (d *FedoraRepo) GetKernelPackages(ctx context.Context, dir string, release string, arch string, jobchan chan<- job.Job) error {
	altArch := d.archs[arch]
	var repos []string
	for _, r := range d.repos[release] {
		repos = append(repos, fmt.Sprintf(r, release, altArch))
	}

	var links []string
	for _, repo := range repos {
		rlinks, err := utils.GetLinks(repo)
		if err != nil {
			//return fmt.Errorf("list packages: %s", err)
			log.Printf("ERROR: list packages: %s\n", err)
			continue
		}
		links = append(links, rlinks...)
	}

	var pkgs []pkg.Package
	kre := regexp.MustCompile(fmt.Sprintf(`kernel-debuginfo-([0-9].*\.%s)\.rpm`, altArch))
	for _, l := range links {
		match := kre.FindStringSubmatch(l)
		if match != nil {
			name := strings.TrimSuffix(match[0], ".rpm")
			p := &pkg.FedoraPackage{
				Name:          name,
				NameOfFile:    match[1],
				Architecture:  altArch,
				URL:           l,
				KernelVersion: kernel.NewKernelVersion(match[1]),
			}
			if p.Version().Less(d.minVersion) {
				continue
			}
			pkgs = append(pkgs, p)
		}
	}

	sort.Sort(pkg.ByVersion(pkgs))

	for _, pkg := range pkgs {
		err := processPackage(ctx, pkg, dir, jobchan)
		if err != nil {
			if errors.Is(err, utils.ErrHasBTF) {
				log.Printf("INFO: kernel %s has BTF already, skipping later kernels\n", pkg)
				return nil
			}
			return err
		}
	}
	return nil
}
