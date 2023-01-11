package main

import "context"

type Package interface {
	String() string
	Filename() string
	Version() kernelVersion
	Download(ctx context.Context, dir string) (string, error)
	ExtractKernel(ctx context.Context, pkgpath string, vmlinuxPath string) error
}

type ByVersion []Package

func (a ByVersion) Len() int      { return len(a) }
func (a ByVersion) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByVersion) Less(i, j int) bool {
	return a[i].Version().Less(a[j].Version())
}
