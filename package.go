package main

import (
	"context"
	"fmt"
	"path/filepath"
)

type Package interface {
	String() string
	Filename() string
	Version() kernelVersion
	Download(ctx context.Context, dir string) (string, error)
	ExtractKernel(ctx context.Context, pkgpath string, vmlinuxPath string) error
}

func packageBTFExists(p Package, dir string) bool {
	fp := filepath.Join(dir, fmt.Sprintf("%s.btf.tar.xz", p.Filename()))
	return exists(fp)
}

func packageFailed(p Package, dir string) bool {
	fp := filepath.Join(dir, fmt.Sprintf("%s.failed", p.Filename()))
	return exists(fp)
}

type ByVersion []Package

func (a ByVersion) Len() int      { return len(a) }
func (a ByVersion) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByVersion) Less(i, j int) bool {
	return a[i].Version().Less(a[j].Version())
}
