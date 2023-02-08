package main

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/DataDog/zstd"
	"github.com/cavaliergopher/cpio"
	"github.com/cavaliergopher/rpm"
	fastxz "github.com/therootcompany/xz"
)

func extractVmlinuxFromRPM(ctx context.Context, rpmpath string, vmlinuxPath string) error {
	f, err := os.Open(rpmpath)
	if err != nil {
		return err
	}
	defer f.Close()

	rpkg, err := rpm.Read(f)
	if err != nil {
		return fmt.Errorf("rpm read: %s", err)
	}
	var crdr io.Reader
	switch rpkg.PayloadCompression() {
	case "xz":
		crdr, err = fastxz.NewReader(f, 0)
		if err != nil {
			return fmt.Errorf("xz reader: %s", err)
		}
	case "zstd":
		zrdr := zstd.NewReader(f)
		defer zrdr.Close()
		crdr = zrdr
	case "gzip":
		grdr, err := gzip.NewReader(f)
		if err != nil {
			return fmt.Errorf("xz reader: %s", err)
		}
		defer grdr.Close()
		crdr = grdr
	default:
		return fmt.Errorf("unsupported compression: %s", rpkg.PayloadCompression())
	}

	if format := rpkg.PayloadFormat(); format != "cpio" {
		return fmt.Errorf("unsupported payload format: %s", format)
	}
	cpioReader := cpio.NewReader(crdr)

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		hdr, err := cpioReader.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("cpio next: %s", err)
		}

		if !hdr.Mode.IsRegular() {
			continue
		}
		if strings.Contains(hdr.Name, "vmlinux") {
			outf, err := os.Create(vmlinuxPath)
			if err != nil {
				return err
			}

			counter := &progressCounter{Op: "Extract", Name: hdr.Name, Size: uint64(hdr.Size)}
			if _, err := io.Copy(outf, io.TeeReader(cpioReader, counter)); err != nil {
				outf.Close()
				os.Remove(vmlinuxPath)
				return fmt.Errorf("cpio file copy: %s", err)
			}
			outf.Close()
			return nil
		}
	}
	return fmt.Errorf("vmlinux file not found in rpm")
}
