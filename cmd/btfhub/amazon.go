package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
)

type amazonRepo struct {
	archs map[string]string
}

func newAmazonRepo() Repository {
	return &amazonRepo{
		archs: map[string]string{
			"x86_64": "x86_64",
			"arm64":  "aarch64",
		},
	}
}

func (d *amazonRepo) GetKernelPackages(ctx context.Context, dir string, release string, arch string, jobchan chan<- Job) error {
	searchOut, err := yumSearch(ctx, "kernel-debuginfo")
	if err != nil {
		return err
	}
	pkgs, err := parseYumPackages(searchOut, newKernelVersion(""))
	if err != nil {
		return fmt.Errorf("parse package listing: %s", err)
	}
	sort.Sort(ByVersion(pkgs))

	for _, pkg := range pkgs {
		err := processPackage(ctx, pkg, dir, jobchan)
		if err != nil {
			if errors.Is(err, ErrHasBTF) {
				log.Printf("INFO: kernel %s has BTF already, skipping later kernels\n", pkg)
				return nil
			}
			return err
		}
	}

	return nil
}
