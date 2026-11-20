# Using a BTFhub file with libbpf

`example.bpf.c` traces `openat()` and reads the caller's parent pid from `task_struct`, whose layout changes between kernels. libbpf takes that layout from the running kernel's BTF, `/sys/kernel/btf/vmlinux`. A kernel built without BTF has no such file, and there the loader, `example.c`, takes a BTF file instead (`-b`), either a complete one from [BTFhub-archive](https://github.com/aquasecurity/btfhub-archive) or one tailored to this object by `tools/btfgen.sh`.

## Build

You need clang, bpftool, and libbpf 1.0 or later with its headers and pkg-config file, on a build host whose kernel has BTF:

```
make
```

`vmlinux.h` is generated from the build host's BTF. Set `VMLINUX_BTF=<file>` to generate it from another BTF file.

## Run on a kernel with BTF

```
sudo ./example
```

## Run on a kernel without BTF

Take the running kernel's BTF file from BTFhub-archive. Files are stored as `<distro>/<release>/<arch>/<kernel>.btf.tar.xz`, for example for Ubuntu 20.04 on x86_64:

```
curl -fLO https://github.com/aquasecurity/btfhub-archive/raw/main/ubuntu/20.04/x86_64/$(uname -r).btf.tar.xz
tar xJf $(uname -r).btf.tar.xz
sudo ./example -b $(uname -r).btf
```

A complete BTF file takes a few megabytes per kernel. A tailored one holds only the types this object uses, a few hundred bytes. To make one for every kernel in the archive, from the root of this repository, with the archive in `./archive` (see [generating tailored BTFs](../docs/generating-tailored-btfs.md)):

```
./tools/btfgen.sh -a x86_64 -o example/example.bpf.o
sudo ./example/example -o example/example.bpf.o -b custom-archive/ubuntu/20.04/x86_64/$(uname -r).btf
```

## Other loaders

- [libbpfgo](https://github.com/aquasecurity/libbpfgo): set `BTFObjPath` in the `NewModuleArgs` given to `NewModuleFromFileArgs`.
- [cilium/ebpf](https://github.com/cilium/ebpf): load the file with `btf.LoadSpec` and set it as `KernelTypes` in `ebpf.ProgramOptions`.
