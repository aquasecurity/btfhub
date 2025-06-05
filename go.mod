module github.com/aquasecurity/btfhub

go 1.24

toolchain go1.24.2

require (
	github.com/DataDog/zstd v1.5.7
	github.com/cavaliergopher/cpio v1.0.1
	github.com/cavaliergopher/rpm v1.3.0
	github.com/therootcompany/xz v1.0.1
	golang.org/x/exp v0.0.0-20250408133849-7e4ce0ab07d0
	golang.org/x/sync v0.15.0
	pault.ag/go/debian v0.18.0
)

require github.com/klauspost/compress v1.18.0 // indirect

require (
	github.com/dustin/go-humanize v1.0.1
	github.com/kjk/lzma v0.0.0-20161016003348-3fd93898850d // indirect
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	pault.ag/go/topsort v0.1.1 // indirect
)
