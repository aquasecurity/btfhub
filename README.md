## BPF CO-RE programs and BTF files

Portable [BPF](https://www.kernel.org/doc/html/latest/bpf/index.html) programs,
called [CO-RE](https://facebookmicrosites.github.io/bpf/blog/2020/02/19/bpf-portability-and-co-re.html) (
Compile Once - Run Everywhere), are portable because all needed symbol
relocations are done by [libbpf](https://github.com/libbpf/libbpf) before
loading the BPF programs into the running kernel.

Running kernels need to have some kind of debug information, either DWARF or the
[BTF](https://www.kernel.org/doc/html/latest/bpf/btf.html) type format,
available during runtime in order to allow libbpf to know how to relocate
existing symbols.

Because BTF type format is compressed/deduplicated, recent kernels are able to
provide this information, during runtime, through a special sysfs file in BTF
raw format. You can dump this file, converted from raw to text format, using:

```bpftool btf dump file /sys/kernel/btf/vmlinux format c```

> If you're using a recent kernel and cannot find such file it means that your
kernel has not been compiled with the `DEBUG_INFO_BTF` option enabled.

> Older kernels, that did not have BPF type format (BTF) support backported, don't
provide this sysfs raw file but may still support eBPF. This is the case for
some distributions that use newer kernels in old userland, like Ubuntu.

Before CO-RE was developed, the only option for someone using eBPF was to
compile each eBPF object to every supported kernel. This led implementations to
rely on 'runtime' compilations, so there would always be a compatible object
available for the running kernel.

Unfortunately that approach is far from optimal: running environment always
depend on having compilers available, making the software distribution a burden.

**BTFHub** provides you a way to overcome this problem: it does the dirty job
for your eBPF application. Instead of compiling your code into a specific kernel
version, it generates and extract the BTF files/info out of an existing and
published kernel, and makes it available so your application can become CO-RE
capable AND rely on relocations (that depend on these BTF files).

## How is this possible ?

Thanks to [@acme](https://github.com/acmel), and other contributors, we can
convert [DWARF symbols](https://en.wikipedia.org/wiki/DWARF) - existent in
distribution's debugging kernel packages - into the BPF type format (BTF).

With **pahole**, a tool distributed together with the **dwarves** project, you
may either add an `.BTF` ELF section to the vmlinux kernel, by doing:

```
pahole -J vmlinux
```
or generate an external raw BTF file, by doing:
```
pahole -j external.btf -J vmlinux
```

If you chose to generate a new .BTF ELF section within vmlinux file, you may be
able to extract it later, into a vmlinux.btf, for example, by doing:

```
llvm-objcopy --only-section=.BTF --set-section-flags .BTF=alloc,readonly vmlinux vmlinux.btf
```

> This HUB can only provide BTF files for distributions that already provide
> debug symbols for kernel binary packages. In the future, if needed, it may
> be able to include 3rd party BTF files (opened to suggestions, open an issue).

## Supported Kernels and Distribution Versions

This is a list of existing distributions and their current status on **eBPF**
and **BTF** supportability over their kernel versions.

> It is **highly recommended** that you update to your latest distribution's
> kernel version in order to use eBPF latest available features.

In the tables bellow you will find BPF, BTF and HUB columns:

* BPF - kernel has support for BPF programs (with or without embedded BTF info)
* BTF - kernel is compiled with DEBUG_INFO_BTF (/sys/kernel/bpf/vmlinux avail)
* HUB - there is a 1:1 kernel <-> BTF file available in this hub

### [CentOS](https://en.wikipedia.org/wiki/CentOS)

#### CentOS 7

| Centos   | RHEL | Release Date | RHEL Date  | Kernel      | BPF | BTF | HUB |
|----------|------|--------------|------------|-------------|-----|-----|-----|
| 7.0.1406 | 7.0  | 2014-07      | 2014-06-09 | 3.10.0-123  |  -  |  -  |  Y  |
| 7.1.1503 | 7.1  | 2015-03      | 2015-03-05 | 3.10.0-229  |  -  |  -  |  Y  |
| 7.2.1511 | 7.2  | 2015-11      | 2015-11-19 | 3.10.0-327  |  -  |  -  |  Y  |
| 7.3.1611 | 7.3  | 2016-11      | 2016-11-03 | 3.10.0-514  |  -  |  -  |  Y  |
| 7.4.1708 | 7.4  | 2017-08      | 2017-07-31 | 3.10.0-693  |  -  |  -  |  Y  |
| 7.5.1804 | 7.5  | 2018-04      | 2018-04-10 | 3.10.0-862  |  -  |  -  |  Y  |
| 7.6.1810 | 7.6  | 2018-10      | 2018-10-30 | 3.10.0-957  |  Y  |  -  |  Y  |
| 7.7.1908 | 7.7  | 2019-08      | 2019-08-06 | 3.10.0-1062 |  Y  |  -  |  Y  |
| 7.8.2003 | 7.8  | 2020-03      | 2020-03-31 | 3.10.0-1127 |  Y  |  -  |  Y  |
| 7.9.2009 | 7.9  | 2020-09      | 2020-09-29 | 3.10.0-1160 |  Y  |  -  |  Y  |

> **Note**: Latest centos7 kernels support BPF, and might support BTF, but they
> lack some eBPF features. With that, eBPF programs capable of running in those
> systems are very limited.
>
> Check out eBPF features your code use [HERE](https://github.com/iovisor/bcc/blob/master/docs/kernel-versions.md)

#### CentOS 8

| Centos   | RHEL | Release Date | RHEL Date  | Kernel      | BPF | BTF | HUB |
|----------|------|--------------|------------|-------------|-----|-----|-----|
| 8.0.1905 | 8.0  | 2019-09-24   | 2019-05-07 | 4.18.0-80   |  -  |  -  |  Y  |
| 8.1.1911 | 8.1  | 2020-01-15   | 2019-11-05 | 4.18.0-147  |  -  |  -  |  Y  |
| 8.2.2004 | 8.2  | 2020-06-15   | 2020-04-28 | 4.18.0-193  |  Y  |  Y  |  Y  |
| 8.3.2011 | 8.3  | 2020-12-07   | 2020-11-03 | 4.18.0-240  |  Y  |  Y  |  Y  |
| 8.4.2105 | 8.4  | 2021-06-03   | 2021-05-18 | 4.18.0-305  |  Y  |  Y  |  Y  |
| ...      | ...  | ...          | ...        | ...         |  Y  |  Y  |  Y  |

> **Note**: **ALL** latest CentOS 8 releases have BPF & BTF support enabled!

#### CentOS Stream 8

| Stream   | RHEL | Release Date | RHEL Date  | Kernel      | BPF | BTF | HUB |
|----------|------|--------------|------------|-------------|-----|-----|-----|
| 8.3      | 8.3  | 2021-01-14   | 2020-11-03 | 4.18.0-240  |  Y  |  Y  |  -  |
| 8.4      | 8.4  | 2021-01-14   | 2020-11-03 | 4.18.0-240  |  Y  |  Y  |  -  |

> **Note**: **ALL** CentOS Stream 8 releases have BPF & BTF support enabled

----

### [Alma](https://en.wikipedia.org/wiki/AlmaLinux)

| Alma     | RHEL | Release Date | RHEL Date  | Kernel      | BPF | BTF | HUB |
|----------|------|--------------|------------|-------------|-----|-----|-----|
| 8.3      | 8.3  | 2021-03-30   | 2020-11-03 | 4.18.0-240  |  Y  |  Y  |  -  |
| 8.4      | 8.4  | 2021-05-26   | 2021-05-18 | 4.18.0-305  |  Y  |  Y  |  -  |
| ...      | ...  | ...          | ...        | ...         |  Y  |  Y  |  -  |

> **Note**: **ALL** Alma releases have BPF & BTF support enabled!

----

### [Fedora](https://en.wikipedia.org/wiki/Fedora_version_history)

| Fedora | Release Date | Kernel  | BPF | BTF | HUB |
|--------|--------------|---------|-----|-----|-----|
| 29     | 2018-10-30   | 4.18    |     |     |  Y  |
| 30     | 2019-05-07   | 5.0     |     |     |  Y  |
| 31     | 2019-10-29   | 5.3     |     |     |  Y  |
| 32     | 2020-04-28   | 5.6     |  Y  |  Y  |  -  |
| 33     | 2020-10-27   | 5.8     |  Y  |  Y  |  -  |
| 34     | 2021-04-27   | 5.11    |  Y  |  Y  |  -  |
| ...    | -            | -       |  Y  |  Y  |  -  |

> **Note**: All supported future Fedora releases will have BPF & BTF support enabled.

----

### [Ubuntu](https://en.wikipedia.org/wiki/Ubuntu_version_history)

| Ubuntu Ver | Num     | Release Date | Kernel  | BPF | BTF | HUB |
|------------|---------|--------------|---------|-----|-----|-----|
| Bionic     | 18.04.2 | 2018-04-26   | 4.15.0  |  -  |  -  |  -  |
| Bionic HWE | -       | -            | 5.4.0   |  Y  |  -  |  Y  |
| Focal      | 20.04.2 | 2020-04-23   | 5.4.0   |  Y  |  -  |  Y  |
| Focal HWE  | -       | -            | 5.8.0   |  Y  |  -  |  Y  |
| Groovy     | 20.10   | 2020-10-22   | 5.8.0   |  Y  |  Y  |  -  |
| Groovy HWE | 20.10   | -            | 5.11.0  |  Y  |  Y  |  -  |
| Hirsute    | 21.04   | 2021-04-22   | 5.11.0  |  Y  |  Y  |  -  |
| ...        | ...     | ...          | ...     |  Y  |  Y  |  -  |

> **Notes**: Bionic HWE, Focal and Focal HWE kernels need this HUB. All other
> future Ubuntu releases will have BPF & BTF support enabled.

### Disclaimer

All BTF files from this repository were built from the DWARF symbols of their
correspondent debug symbols package, from each of the supported distribution,
using [LLVM](https://github.com/llvm/llvm-project) and pahole tool from
the [dwarves project](https://github.com/acmel/dwarves).

> No manipulation of those binary files was made during the process and all
> the files, or scripts, responsible for generating the binary files are
> available in this repository.

Debug Symbols Repositories Used:

1. [debuginfo.centos.org](http://debuginfo.centos.org/)
2. [ddebs.ubuntu.com](http://ddebs.ubuntu.com/)
3. [archives.fedoraproject.org](https://archives.fedoraproject.org/)
