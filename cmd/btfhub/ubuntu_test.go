package main

import (
	"bytes"
	"log"
	"sort"
	"testing"
)

const testpkgs = `
Package: linux-image-unsigned-5.4.0-92-generic-dbgsym
Architecture: amd64
Version: 5.4.0-92.103~18.04.2
Priority: optional
Section: devel
Source: linux-hwe-5.4
Maintainer: Ubuntu Kernel Team <kernel-team@lists.ubuntu.com>
Installed-Size: 6273842
Provides: linux-hwe-5.4-debug
Filename: pool/main/l/linux-hwe-5.4/linux-image-unsigned-5.4.0-92-generic-dbgsym_5.4.0-92.103~18.04.2_amd64.ddeb
Size: 922281236
MD5sum: 670795ae4248008e44ef131b403fd105
SHA1: b3c308f7487fa412338e12cb8556784ebc5ef724
SHA256: 570e1a3c693ef786f40ce6f469048330c9250858234ec13777925f5d8c4be67b
SHA512: 715cb42792981183af1f30381078ad1519eaedd9b6b06c4ed3813cdf2b7bbdc9e57c0f0cdcd86b68179d4affd6911f10c6990aed01259951dc2f04e515b39799
Description: Linux kernel debug image for version 5.4.0 on 64 bit x86 SMP
 This package provides the unsigned kernel debug image for version 5.4.0 on
 64 bit x86 SMP.
 .
 This is for sites that wish to debug the kernel.
 .
 The kernel image contained in this package is NOT meant to boot from. It
 is uncompressed, and unstripped. This package also includes the
 unstripped modules.

Package: linux-image-unsigned-5.4.0-90-lowlatency-dbgsym
Architecture: amd64
Version: 5.4.0-90.101~18.04.1
Priority: optional
Section: devel
Source: linux-hwe-5.4
Maintainer: Ubuntu Kernel Team <kernel-team@lists.ubuntu.com>
Installed-Size: 6277903
Provides: linux-hwe-5.4-debug
Filename: pool/main/l/linux-hwe-5.4/linux-image-unsigned-5.4.0-90-lowlatency-dbgsym_5.4.0-90.101~18.04.1_amd64.ddeb
Size: 922123560
MD5sum: 63998a1157192ce804b282d143ae0855
SHA1: 989f8093dcc56dc3325c631d28bc2ab9a5640026
SHA256: 2dd8f364c8976ffbef64c5895a0ce2bbcfa681512deb2bcf29e7e9ddc8f5771c
SHA512: f50ae9cb9b6b86d1fcb30959b80bce7792c3d4ce71b0863ccee25af92fb5c113d1c8c496d4b688eaa60f1a0bc8573a195fa3de0d23f9539350a2569548ea6c99
Description: Linux kernel debug image for version 5.4.0 on 64 bit x86 SMP
 This package provides the unsigned kernel debug image for version 5.4.0 on
 64 bit x86 SMP.
 .
 This is for sites that wish to debug the kernel.
 .
 The kernel image contained in this package is NOT meant to boot from. It
 is uncompressed, and unstripped. This package also includes the
 unstripped modules.
`

func TestParsePackages(t *testing.T) {
	bb := bytes.NewBuffer([]byte(testpkgs))
	d := &ubuntuDownloader{}
	pkgs, err := d.parsePackages(bb)
	if err != nil {
		log.Fatal(err)
	}
	if len(pkgs) != 2 {
		log.Fatalf("incorrect number of packages: %d", len(pkgs))
	}

	sort.Sort(UbuntuByVersion(pkgs))
	if pkgs[0].Name != "linux-image-unsigned-5.4.0-90-lowlatency-dbgsym" {
		log.Fatalf("invalid version sort")
	}
}
