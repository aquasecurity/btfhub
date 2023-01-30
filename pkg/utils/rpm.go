package utils

import (
	"compress/bzip2"
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

func ExtractVmlinuxFromRPM(ctx context.Context, rpmPath string, vmlinuxPath string) error {
	file, err := os.Open(rpmPath)
	if err != nil {
		return err
	}
	defer file.Close()

	rpmPkg, err := rpm.Read(file)
	if err != nil {
		return fmt.Errorf("rpm read: %s", err)
	}

	var crdr io.Reader

	// Find out about RPM package compression

	switch rpmPkg.PayloadCompression() {
	case "xz":
		crdr, err = fastxz.NewReader(file, 0)
		if err != nil {
			return fmt.Errorf("xz reader: %s", err)
		}
	case "zstd":
		zrdr := zstd.NewReader(file)
		defer zrdr.Close()
		crdr = zrdr
	case "gzip":
		grdr, err := gzip.NewReader(file)
		if err != nil {
			return fmt.Errorf("gzip reader: %s", err)
		}
		defer grdr.Close()
		crdr = grdr
	case "bzip2":
		crdr = bzip2.NewReader(file)
	default:
		return fmt.Errorf("unsupported compression: %s", rpmPkg.PayloadCompression())
	}

	if format := rpmPkg.PayloadFormat(); format != "cpio" {
		return fmt.Errorf("unsupported payload format: %s", format)
	}

	// Read from cpio archive

	cpioReader := cpio.NewReader(crdr)

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		cpioHeader, err := cpioReader.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("cpio next: %s", err)
		}

		if !cpioHeader.Mode.IsRegular() {
			continue
		}

		// Extract vmlinux file

		if strings.Contains(cpioHeader.Name, "vmlinux") {
			outFile, err := os.Create(vmlinuxPath)
			if err != nil {
				return err
			}

			counter := &ProgressCounter{
				Ctx:  ctx,
				Op:   "Extract",
				Name: cpioHeader.Name,
				Size: uint64(cpioHeader.Size),
			}

			_, err = io.Copy(outFile, io.TeeReader(cpioReader, counter))

			if err != nil {
				outFile.Close()
				os.Remove(vmlinuxPath)
				return fmt.Errorf("cpio file copy: %s", err)
			}

			outFile.Close()

			return nil
		}
	}
	return fmt.Errorf("vmlinux file not found in rpm")
}
