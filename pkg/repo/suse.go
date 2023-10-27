package repo

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/sync/errgroup"

	"github.com/aquasecurity/btfhub/pkg/job"
	"github.com/aquasecurity/btfhub/pkg/kernel"
	"github.com/aquasecurity/btfhub/pkg/pkg"
	"github.com/aquasecurity/btfhub/pkg/utils"
)

type suseRepo struct {
	archs       map[string]string
	repoAliases map[string]string
}

func NewSUSERepo() Repository {
	return &suseRepo{
		archs: map[string]string{
			"x86_64": "x86_64",
			"arm64":  "aarch64",
		},
		repoAliases: map[string]string{},
	}
}

func (d *suseRepo) GetKernelPackages(ctx context.Context, dir string, release string, arch string, force bool, jobchan chan<- job.Job) error {
	var repos []string

	switch release {
	case "12.3":
		repos = append(repos, fmt.Sprintf("SUSE_Linux_Enterprise_Server_12_SP3_%s:SLES12-SP3-Debuginfo-Pool", arch))
		repos = append(repos, fmt.Sprintf("SUSE_Linux_Enterprise_Server_12_SP3_%s:SLES12-SP3-Debuginfo-Updates", arch))
	case "12.5":
		repos = append(repos, fmt.Sprintf("SUSE_Linux_Enterprise_Server_%s:SLES12-SP5-Debuginfo-Pool", arch))
		repos = append(repos, fmt.Sprintf("SUSE_Linux_Enterprise_Server_%s:SLES12-SP5-Debuginfo-Updates", arch))
	case "15.1":
		repos = append(repos, fmt.Sprintf("Basesystem_Module_15_SP1_%s:SLE-Module-Basesystem15-SP1-Debuginfo-Pool", arch))
		repos = append(repos, fmt.Sprintf("Basesystem_Module_15_SP1_%s:SLE-Module-Basesystem15-SP1-Debuginfo-Updates", arch))
	case "15.2":
		repos = append(repos, fmt.Sprintf("Basesystem_Module_%s:SLE-Module-Basesystem15-SP2-Debuginfo-Pool", arch))
		repos = append(repos, fmt.Sprintf("Basesystem_Module_%s:SLE-Module-Basesystem15-SP2-Debuginfo-Updates", arch))
	case "15.3":
		repos = append(repos, fmt.Sprintf("Basesystem_Module_%s:SLE-Module-Basesystem15-SP3-Debuginfo-Pool", arch))
		repos = append(repos, fmt.Sprintf("Basesystem_Module_%s:SLE-Module-Basesystem15-SP3-Debuginfo-Updates", arch))
	case "15.4":
		repos = append(repos, fmt.Sprintf("Basesystem_Module_%s:SLE-Module-Basesystem15-SP4-Debuginfo-Pool", arch))
		repos = append(repos, fmt.Sprintf("Basesystem_Module_%s:SLE-Module-Basesystem15-SP4-Debuginfo-Updates", arch))
	}
	for _, r := range repos {
		if _, err := utils.RunZypperCMD(ctx, "modifyrepo", "--enable", r); err != nil {
			return err
		}
	}

	if err := d.getRepoAliases(ctx); err != nil {
		return fmt.Errorf("repo aliases: %s", err)
	}

	// packages are named kernel-<type>-debuginfo
	// possible types are: default, azure
	searchOut, err := zypperSearch(ctx, "kernel-*-debuginfo")
	if err != nil {
		return err
	}

	pkgs, err := d.parseZypperPackages(searchOut, arch)
	if err != nil {
		return fmt.Errorf("parse package listing: %s", err)
	}

	pkgsByKernelType := make(map[string][]pkg.Package)
	for _, p := range pkgs {
		ks, ok := pkgsByKernelType[p.Flavor]
		if !ok {
			ks = make([]pkg.Package, 0, 1)
		}
		ks = append(ks, p)
		pkgsByKernelType[p.Flavor] = ks
	}

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
			err := d.processPackages(ctx, dir, cks, force, jobchan)
			log.Printf("DEBUG: end kernel type %s %s\n", ckt, arch)
			return err
		})
	}
	return g.Wait()
}

func (d *suseRepo) getRepoAliases(ctx context.Context) error {
	repos, err := zypperRepos(ctx)
	if err != nil {
		return err
	}
	bio := bufio.NewScanner(repos)
	for bio.Scan() {
		line := bio.Text()
		fields := strings.FieldsFunc(line, func(r rune) bool {
			return unicode.IsSpace(r) || r == '|'
		})
		if len(fields) < 3 {
			continue
		}
		// first field must be a number
		if _, err := strconv.Atoi(fields[0]); err != nil {
			continue
		}
		alias, name := fields[1], fields[2]
		d.repoAliases[name] = alias
	}
	return bio.Err()
}

func (d *suseRepo) processPackages(ctx context.Context, dir string, pkgs []pkg.Package, force bool, jobchan chan<- job.Job) error {
	for i, p := range pkgs {
		log.Printf("DEBUG: start pkg %s (%d/%d)\n", p, i+1, len(pkgs))
		if err := processPackage(ctx, p, dir, force, jobchan); err != nil {
			if errors.Is(err, utils.ErrHasBTF) {
				log.Printf("INFO: kernel %s has BTF already, skipping later kernels\n", p)
				return nil
			}
			if errors.Is(err, context.Canceled) {
				return nil
			}
			log.Printf("ERROR: %s: %s\n", p, err)
			continue
		}
		log.Printf("DEBUG: end pkg %s (%d/%d)\n", p, i+1, len(pkgs))
	}
	return nil
}

func (d *suseRepo) parseZypperPackages(rdr io.Reader, arch string) ([]*pkg.SUSEPackage, error) {
	var pkgs []*pkg.SUSEPackage
	kre := regexp.MustCompile(`^kernel-([^-]+)-debuginfo$`)
	bio := bufio.NewScanner(rdr)
	for bio.Scan() {
		line := bio.Text()
		fields := strings.FieldsFunc(line, func(r rune) bool {
			return unicode.IsSpace(r) || r == '|'
		})
		if len(fields) < 5 {
			continue
		}
		name, ver, pkgarch, repo := fields[0], fields[2], fields[3], fields[4]
		if pkgarch != arch {
			continue
		}
		match := kre.FindStringSubmatch(name)
		if match != nil {
			alias, ok := d.repoAliases[repo]
			if !ok {
				return nil, fmt.Errorf("unknown repo %s", repo)
			}
			flavor := match[1]
			if flavor == "preempt" {
				continue
			}

			// remove final .x because it is just a build counter and not included in `uname -r`
			parts := strings.Split(ver, ".")
			btfver := strings.Join(parts[:len(parts)-1], ".")

			p := &pkg.SUSEPackage{
				Name:          name,
				NameOfFile:    fmt.Sprintf("%s-%s", ver, flavor),
				NameOfBTFFile: fmt.Sprintf("%s-%s", btfver, flavor),
				KernelVersion: kernel.NewKernelVersion(ver),
				Architecture:  pkgarch,
				Repo:          repo,
				Flavor:        flavor,
				Downloaddir:   fmt.Sprintf("/var/cache/zypp/packages/%s/%s", alias, arch),
			}
			pkgs = append(pkgs, p)
		}
	}
	if err := bio.Err(); err != nil {
		return nil, err
	}
	return pkgs, nil
}

func zypperRepos(ctx context.Context) (*bytes.Buffer, error) {
	return utils.RunZypperCMD(ctx, "repos")
}

func zypperSearch(ctx context.Context, pkg string) (*bytes.Buffer, error) {
	return utils.RunZypperCMD(ctx, "search", "-s", pkg)
}
