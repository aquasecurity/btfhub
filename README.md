## BPF CO-RE programs and BTF files

Portable [BPF](https://www.kernel.org/doc/html/latest/bpf/index.html) programs, called
[CO-RE](https://facebookmicrosites.github.io/bpf/blog/2020/02/19/bpf-portability-and-co-re.html)
(Compile Once - Run Everywhere), are portable because all needed symbol rellocations are done by
[libbpf](https://github.com/libbpf/libbpf) before loading the BPF programs into the running kernel.

Running kernels need to have [BTF](https://www.kernel.org/doc/html/latest/bpf/btf.html) type format
available during runtime in order to allow libbpf to know how to do the rellocations. For the recent
kernels this information is available through `/sys/kernel/btf/vmlinux` file:

```bpftool btf dump file /sys/kernel/btf/vmlinux format c```

as they have CONFIG_DEBUG_INFO_BTF option enabled, creating this sysfs interface.

Older kernels don't provide this sysfs BTF interface, but are still capable of running BPF binaries.
One of the things preventing older kernels to have CO-RE BPF binaries to run is the lack of BTF type
format.

**This is exactly what this HUB provides you:** an API that will feed your CO-RE BPF program - based
on the libbpf library - with the needed btf(s) file(s) to make it portable, now even in older
kernels.

## How is this possible ?

You might be wondering, how is this possible... if the kernels did not have the BTF feature during
compilation time ? Thanks to [@acme](https://github.com/acmel), the Linux Perf maintainer, and other
contributors, we can convert [DWARF symbols](https://en.wikipedia.org/wiki/DWARF) - available from
existing kernel binary packages debug symbols - into a BTF only binary files.

By using pahole, you may add an `.BTF` ELF section to the vmlinux kernel by doing:

```
pahole -J vmlinux
```

And then, with LLVM 10 or newer, you can extract the .BTF section into another ELF file:

```
llvm-objcopy-10 --only-section=.BTF --set-section-flags .BTF=alloc,readonly vmlinux vmlinux.btf
```

The result will be a 'vmlinux.btf' file containing BTF only ELF section that will feed libbpf for
the needed symbols rellocations in that kernel. This means that you can create one BPF binary and
run in all existing (and supported) kernels: new ones, because they have BTF data embedded, and old
ones, because this HUB will provide you BTF data you need.

> This HUB can only provide BTF files for distributions that already provide debug symbols for
> kernel binary packages.

### Recent Pahole Update

Newer pahole (dwarves project) tool versions are capable of extracting BTF data from DWARF symbols
into RAW files (instead of ELF files containing .BTF section). Those RAW files are capable of being
used by libbpf so the CO-RE BPF relocations can be made.

## Supported Kernels and Distribution Versions

From this HUB you will always find the latest kernel patched version for the supported distribution.
If your installation is outdated, then y our kernel might have been already replaced within your
distribution repository. It is recommended that you upgrade to latest kernel version (patch version)
for the Linux Distribution (and version) you are using.

> `BPF = y` -- The kernel has support for BPF programs (not all BPF enabled kernels were BTF capable).

> `BTF = Y` -- The kernel already has support for BTF (no files from this HUB are needed).

> `BPF = y` and `BTF = -` and `HUB = Y` -- The kernel does not support BTF but is capable of BPF.
> External files from this HUB are needed and btfhub.io API will provide you with needed BTF file.

### [Alma](https://en.wikipedia.org/wiki/AlmaLinux)

| Alma Ver  | RHEL | Release Date  | RHEL Date  | Kernel     | BPF | BTF | HUB |
|-----------|------|---------------|------------|------------|-----|-----|-----|
| 8.3 🏁    | 8.3  | 2021-03-30    | 2020-11-03 | 4.18.0-240 |  Y  |  Y  |  -  |
| 8.4       | 8.4  | 2021-05-26    | 2021-05-18 | 4.18.0-305 |  Y  |  Y  |  -  |
| ...       | ...  | ...           | ...        | ...        |  Y  |  Y  |  -  |

> **Note**: All kernels from Alma Linux releases have BTF support enabled

### [CentOS](https://en.wikipedia.org/wiki/CentOS)

##### CentOS 7 (and 7 Plus)

| Centos Ver    | RHEL | Release Date | RHEL Date  | Kernel      | BPF | BTF | HUB |
|---------------|------|--------------|------------|-------------|-----|-----|-----|
| 7.0.1406 🏁   | 7.0  | 2014-07      | 2014-06-09 | 3.10.0-123  |  -  |  -  |  -  |
| 7.1.1503 🏁   | 7.1  | 2015-03      | 2015-03-05 | 3.10.0-229  |  -  |  -  |  -  |
| 7.2.1511 🏁   | 7.2  | 2015-11      | 2015-11-19 | 3.10.0-327  |  -  |  -  |  -  |
| 7.3.1611 🏁   | 7.3  | 2016-11      | 2016-11-03 | 3.10.0-514  |  -  |  -  |  -  |
| 7.4.1708 🏁   | 7.4  | 2017-08      | 2017-07-31 | 3.10.0-693  |  -  |  -  |  -  |
| 7.5.1804 🏁   | 7.5  | 2018-04      | 2018-04-10 | 3.10.0-862  |  -  |  -  |  -  |
| 7.6.1810 🏁   | 7.6  | 2018-10      | 2018-10-30 | 3.10.0-957  |  Y  |  -  |  Y  |
| 7.7.1908 🏁   | 7.7  | 2019-08      | 2019-08-06 | 3.10.0-1062 |  Y  |  -  |  Y  |
| 7.8.2003 🏁   | 7.8  | 2020-03      | 2020-03-31 | 3.10.0-1127 |  Y  |  -  |  Y  |
| 7.9.2009      | 7.9  | 2020-09      | 2020-09-29 | 3.10.0-1160 |  Y  |  -  |  Y  |

🏁  End-of-Life (BTF for kernels released and in repository until EOL date)

> **Note**: Latest centos7 kernels support BPF, and have BTFs available, but they lack BPF features
> and the eBPF programs capable of running in those systems are very limited. It might be impossible
> to run your eBPF program in such systems, make sure to look for features your eBPF code use and
> check [HERE](https://github.com/iovisor/bcc/blob/master/docs/kernel-versions.md) if there are
> chances for them to be supported in a centos7 kernel.

##### CentOS 8 (and 8 Plus)

| Centos Ver    | RHEL | Release Date | RHEL Date  | Kernel      | BPF | BTF | HUB |
|---------------|------|--------------|------------|-------------|-----|-----|-----|
| 8.0.1905 🏁   | 8.0  | 2019-09-24   | 2019-05-07 | 4.18.0-80   |     |  -  |  Y  |
| 8.1.1911 🏁   | 8.1  | 2020-01-15   | 2019-11-05 | 4.18.0-147  |     |  -  |  Y  |
| 8.2.2004 🏁   | 8.2  | 2020-06-15   | 2020-04-28 | 4.18.0-193  |  Y  |  Y  |  -  |
| 8.3.2011 🏁   | 8.3  | 2020-12-07   | 2020-11-03 | 4.18.0-240  |  Y  |  Y  |  -  |
| 8.4.2105      | 8.4  | 2021-06-03   | 2021-05-18 | 4.18.0-305  |  Y  |  Y  |  -  |
| ...           | ...  | ...          | ...        | ...         |  Y  |  Y  |  -  |

🏁  End-of-Life (BTF for kernels released and in repository until EOL date)

> **Note**: Next CentOS 8 releases will likely have BPF & BTF support enabled

##### CentOS Stream 8

| Stream Ver | RHEL | Release Date | RHEL Date  | Kernel      | BPF | BTF | HUB |
|------------|------|--------------|------------|-------------|-----|-----|-----|
| 8.3    🏁  | 8.3  | 2021-01-14   | 2020-11-03 | 4.18.0-240  |  Y  |  Y  |  -  |
| 8.4        | 8.4  | 2021-01-14   | 2020-11-03 | 4.18.0-240  |  Y  |  Y  |  -  |

🏁  End-of-Life.

> **Note**: All supported CentOS Stream 8 releases have BPF & BTF support enabled

### [Fedora](https://en.wikipedia.org/wiki/Fedora_version_history)

| Fedora | Release Date | Kernel  | BPF | BTF | HUB |
|--------|--------------|---------|-----|-----|-----|
| 29 🏁  | 2018-10-30   | 4.18    |     |     |  Y  |
| 30 🏁  | 2019-05-07   | 5.0     |     |     |  Y  |
| 31 🏁  | 2019-10-29   | 5.3     |     |     |  Y  |
| 32 🏁  | 2020-04-28   | 5.6     |  Y  |  Y  |  -  |
| 33     | 2020-10-27   | 5.8     |  Y  |  Y  |  -  |
| 34     | 2021-04-27   | 5.11    |  Y  |  Y  |  -  |
| ...    | -            | -       |  Y  |  Y  |  -  |

> **Note**: All supported future Fedora releases will have BPF & BTF support enabled.

🏁  End-of-Life (first and last released kernels BTF available to each EOL version)

### [Ubuntu](https://en.wikipedia.org/wiki/Ubuntu_version_history)

| Ubuntu Ver    | Num     | Release Date | Kernel  | BPF | BTF | HUB |
|---------------|---------|--------------|---------|-----|-----|-----|
| Bionic        | 18.04.2 | 2018-04-26   | 4.15.0  |  -  |  -  |  -  |
| Bionic HWE    | -       | -            | 5.4.0   |  Y  |  -  |  Y  |
| ...           | -       | -            | -       |  -  |  -  |  -  |
| Focal         | 20.04.2 | 2020-04-23   | 5.4.0   |  Y  |  -  |  Y  |
| Focal HWE     | -       | -            | 5.8.0   |  Y  |  -  |  Y  |
| Groovy     🏁 | 20.10   | 2020-10-22   | 5.8.0   |  Y  |  Y  |  -  |
| Groovy HWE 🏁 | 20.10   | -            | 5.11.0  |  Y  |  Y  |  -  |
| Hirsute       | 21.04   | 2021-04-22   | 5.11.0  |  Y  |  Y  |  -  |
| ...           | ...     | ...          | ...     |  Y  |  Y  |  -  |

🏁 End-of-Life (BTF for kernels in repository until EOL date)

> **Note**: Cosmic, Disco and Eoan were not considered.
> **Note**: All supported future Ubuntu releases will have BPF & BTF support enabled.

### Disclaimer

All the BTF files and their content were built from the DWARF symbols of their correspondent debug
symbols package, from each of the supported distribution, using
[LLVM](https://github.com/llvm/llvm-project) and pahole tool from the [dwarves
project](https://github.com/acmel/dwarves).

> No manipulation of those binary files was made during the process and all the files responsible
> for generating the binary files are available in this repository.

Debug Symbols Repositories:

1. [debuginfo.centos.org](http://debuginfo.centos.org/)
2. [ddebs.ubuntu.com](http://ddebs.ubuntu.com/)

