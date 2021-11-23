## How to generate BTF information ?

BTF information is usually
[encoded within ELF sections `.BTF` and `.BTF.ext`](https://github.com/aquasecurity/btfhub/blob/main/docs/btfgen-internals.md#btf-elf-sections)
OR in a raw BTF file. BTF might be encoded by 2 different ways:

1. The [pahole](https://lwn.net/Articles/762847/) tool (from the
   [dwarves](https://github.com/acmel/dwarves)) project): using existing
   non-stripped ELF file [DWARF debug data]((https://en.wikipedia.org/wiki/DWARF).
   The ELF file can be the kernel (check next topic) or a regular eBPF ELF
   object. Two extra ELF sections, with BTF encoded, will be added to the same
   ELF used as input OR, more recently, to an external RAW BTF file (to feed
   libbpf).

2. LLVM: The 2 ELF sections `.BTF` and `.BTF.ext` are created automatically by
   LLVM during eBPF code compilation. Pahole isn't capable of generating BTF
   relocation information, like LLVM, something that libbpf will need in order
   to make CO-RE relocations to work.

## Embedded BTF information

If your running kernel supports embedded BTF (and most of the current
distributions do have kernels supporting it) you will have the following sysfs
file: `/sys/kernel/btf/vmlinux`.

You may read its contents by executing:

```
$ bpftool btf dump file /sys/kernel/btf/vmlinux format raw

[1] INT 'long unsigned int' size=8 bits_offset=0 nr_bits=64 encoding=(none)
[2] CONST '(anon)' type_id=1
[3] ARRAY '(anon)' type_id=1 index_type_id=18 nr_elems=2
[4] PTR '(anon)' type_id=6
[5] INT 'char' size=1 bits_offset=0 nr_bits=8 encoding=(none)
[6] CONST '(anon)' type_id=5
[7] INT 'unsigned int' size=4 bits_offset=0 nr_bits=32 encoding=(none)
[8] CONST '(anon)' type_id=7
[9] INT 'signed char' size=1 bits_offset=0 nr_bits=8 encoding=(none)
[10] TYPEDEF '__u8' type_id=11
...
```
> format might be "c" or "raw", depending on what you need.

or through the pahole tool:

```
$ pahole /sys/kernel/btf/vmlinux
struct list_head {
	struct list_head *         next;                 /*     0     8 */
	struct list_head *         prev;                 /*     8     8 */

	/* size: 16, cachelines: 1, members: 2 */
	/* last cacheline: 16 bytes */
};
...
```

If you don't have this file available in your current kernel, OR in the kernel
you would like your eBPF application to support, then you will likely need
BTFhub.

> If the kernel isn't compiled with `DEBUG_INFO_BTF` kconfig option enabled,
> it won't have that file available. This happens in most of the kernels
> present in BTFhub-Archive files.

## How does Linux Kernel do it ?

By using pahole, Linux kernel is able to encode
[BTF information](https://github.com/aquasecurity/btfhub/blob/main/docs/btfgen-internals.md#btf-external-files)
for its ELF file. This is done by the
[link-vmlinux.sh](https://elixir.bootlin.com/linux/v5.15.4/source/scripts/link-vmlinux.sh#L205)
script, during build time.

## What should I do to my eBPF project ?

You should always compile your eBPF project using LLVM/Clang (>= 10). This will
guarantee that your eBPF CO-RE object will have all needed relocation
information embedded on the ELF file.

## Using pahole to convert DWARF to BTF format

This is the **core** idea of [BTFhub](https://github.com/aquasecurity/btfhub/)
and [BTFhub-Archive](https://github.com/aquasecurity/btfhub-archive/). The
[update.sh](https://github.com/aquasecurity/btfhub/blob/main/tools/update.sh)
script is responsible to download all existing debug kernel packages, for the
supported distributions, and convert the DWARF information, contained in those
debug packages, into BTF information, needed by libbpf in order to run CO-RE
capable eBPF applications.

> **BTFhub** runs the **update.sh** script in a cron like job, every day, and
> uploads generated BTF files into **BTFhub-Archive**, in order for it to be
> consumed by your project.

If you want to try doing things by hand, you may add a `.BTF` ELF section to
the non-stripped **vmlinuz** (uncompressed) kernel file by doing:

```
pahole -J vmlinux
```

or generate an external raw BTF file, by doing:

```
pahole --btf_encode_detached external.btf vmlinux
```

If you chose to generate a new .BTF ELF section within the **vmlinuz file**,
you may be able to extract it later, into an external raw BTF file, called
vmlinux.btf, for example, by doing:

```
llvm-objcopy --only-section=.BTF --set-section-flags .BTF=alloc,readonly vmlinux vmlinux.btf
```
