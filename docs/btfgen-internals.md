# eBPF BTF GENERATOR: The road to truly portable CO-RE eBPF programs

## INTRODUCTION

CO-RE requires to have BTF information describing the types of the kernel in order to perform the relocations. This is usually provided by the kernel itself when it's configured with CONFIG\_DEBUG\_INFO\_BTF. However, this configuration is not enabled in all the distributions and it is not available on older kernels.

It's possible to use CO-RE in kernels without CONFIG\_DEBUG\_INFO\_BTF support by providing the BTF information from an external source. [BTFHUB](https://github.com/aquasecurity/btfhub/) contains BTF files to each released kernel not supporting BTF, for the most popular distributions.

Providing this BTF file for a given kernel has some challenges:

1. Each BTF file is a few MBs big, then it's not possible to ship the eBPF program with the all the BTF files needed to run in different kernels. (The BTF files will be in the order of GBs if you want to support a high number of kernels)

2. Downloading the BTF file for the current kernel at runtime delays the start of the program and it isn't always possible to reach an external host to download such a file.

Providing the BTF file with the information about all the data types of the kernel for running an eBPF program is an overkill in many of the cases. Usually the eBPF programs access only some kernel fields.

Our proposal is: to extend libbpf to provide an API to generate a BTF file with only the types that are needed by an eBPF object. These generated files are very small compared to the ones that contain all the kernel types. This allows to ship an eBPF program together with the BTF information that it needs to run for many different kernels.

This idea was discussed during the [Towards truly portable eBPF](https://www.youtube.com/watch?v=igJLKyP1lFk&t=2418s) presentation a Linux Plumbers 2021. We prepared a [BTFGen](https://github.com/kinvolk/btfgen) repository with an example of how this API can be used. Our plan is to include this **support in bpftool once it's merged in libbpf**.

There is also a [good example](https://github.com/aquasecurity/btfhub/tree/main/tools) on how to use BTFGen and BTFHub together to generate multiple BTF files, to each existing/supported kernel, tailored to one application. For example: a complex bpf object might support nearly 400 kernels by having BTF files summing only 1.5 MB.

> When you have sometime, go ahead and watch **Linux Plumbers 2021 presentation**, responsible to explain the difficulties in having portable eBPF code: [Towards truly portable eBPF](https://www.youtube.com/watch?v=igJLKyP1lFk&t=734s).

> At the end of the presentation, after [we demonstrate all blockers in making an eBPF application to support multiple kernels](https://t.co/e5YjqdxQDU), there is a a demonstration on [how BTFHUB can be used](https://youtu.be/igJLKyP1lFk?t=2315) AND a [discussion about the future steps](https://youtu.be/igJLKyP1lFk?t=2418).

## AUTHORS AND DISCLAIMER

The **btfgen** tool was only created thanks to the effort of:<BR>
<BR>
Mauricio Vasquez Bernal (Kinvolk/Microsoft) - main author<BR>
Rafael David Tinoco (Aqua Security) - fixes and review<BR>
Lorenzo Fontana (Elastic) - fixes and review<BR>
Itay Shakury (Aqua Security) - ideas, support and management<BR>
Marga Manterola (Kinvolk/Microsoft) - support and management<BR>
<BR>
**This document** was created and reviewed by:<BR>
<BR>
Rafael David Tinoco (Aqua Security) - main author<BR>
Mauricio Vasquez Bernal (Kinvolk/Microsoft) - review and fixes<BR>
Lorenzo Fontana (Elastic) - review and fixes<BR>
Yaniv Agman (Aqua Security) - review<BR>

> The code has not been upstreamed yet and it is being developed at:
>
> [https://github.com/kinvolk/libbpf/tree/btfgen](https://github.com/kinvolk/libbpf/tree/btfgen)<BR>
> [https://github.com/kinvolk/btfgen](https://github.com/kinvolk/btfgen)<BR>
>
> The intent was to [go upstream](https://x-lore.kernel.org/bpf/20211027203727.208847-1-mauricio@kinvolk.io/) with **libbpf** changes and create a **btfgen** as a sub-function in the bpftool tool.

## eBPF CO-RE: ENGINE BEHIND EBPF PORTABILITY

As you might have read in the pointed documents, or already knew, eBPF portability highly depends on [code relocation](https://en.wikipedia.org/wiki/Relocation_(computing)). Architectures have memory relocations made either during [link or load time](https://stffrdhrn.github.io/hardware/embedded/openrisc/2019/11/29/relocs.html), so does BPF arch.

> If you would like to revisit concepts about "linkers & loaders and relocations", go ahead and visit [this post about it](https://rafaeldtinoco.com/toolchain-relocations-overview/)

### EBPF RELOCATIONS (INTRODUCTION)

A nice quote from Brendan Gregg to explain why relocations are needed for eBPF:

> It's not just a matter of saving the BPF bytecode in ELF and then sending it to any other kernel. Many BPF programs walk kernel structs that can change from one kernel version to another. Your BPF bytecode may still execute on different kernels, but it may be reading the wrong struct offsets and printing garbage output!

> This is an issue of relocation, and both BTF and CO-RE solve this for BPF binaries. BTF provides type information so that struct offsets and other details can be queried as needed, and CO-RE records which parts of a BPF program need to be rewritten, and how.

The eBPF ELF object is not organized the same way a regular ELF objects (and it is not an executable, so it does not contain **program header table** entries, like explained in the previous session. In summary: eBPF ELF files have _different sections_ than the ones created by GCC/CLANG when dealing with other architectures.

In a same eBPF object file you might have _multiple different_ eBPF programs. Each non-inlined function will be a different program. Each eBPF program will have its own ELF section (TEXT), differently than an ELF executable which has a single .text section, as well as a relocation table section (REL) only for it. All the maps declared in your eBPF object will be placed in a .maps (DATA) section, and so on.

Look at the following example:

![](docs/image03.png)

The image above is an example of how the eBPF object of the [BTFHUB example](https://github.com/aquasecurity/btfhub/tree/main/example) looks like. Taking a look at the [source code](https://github.com/aquasecurity/btfhub/blob/main/example/example.bpf.c) you will see we have 2 *inlined functions* and 1 *non-inlined one*. As the reader probably knows, the inline function will become part of its callee (the compiler won't arrange the stack with a new frame), so at the end we will have only 1 eBPF program:

1. a kprobe eBPF program triggered by the `do_sys_openat2` kernel function.

> Note that this was done for education purposes only and a single call to `open` will likely trigger the eBPF program execution. Intent here was to show different program types and what their ELF sections would look like.

For each eBPF program ELF section, just one in this simple example, we have a correspondent section for all its local relocation info:

![](docs/image04.png)

The information about the **types, functions** (and needed dynamic relocations) used in this eBPF object is **contained in two ELF sections**: .BTF and [.BTF.ext](https://www.kernel.org/doc/html/latest/bpf/btf.html#btf-ext-section), both explained in details later on.

![](docs/image05.png)

After this introduction, this document will concentrate most, if not all, of its efforts in those structures and explain what the **BTF GENERATOR** tool is, and how eBPF programmers, seeking for code portability by using eBPF CO-RE, can benefit from it.

## BPF TYPE FORMAT (or BTF)

Perhaps the best document out there describing BTF is [Andrii's - BTF deduplication and Linux Kernel BTF](https://nakryiko.com/posts/btf-dedup/). In here it is good to mention that, without BTF, eBPF CO-RE would be very hard (or impossible) to be achieved.

In there you will find the following diagram:

![](docs/image02.png)

illustrating the **BTF type graph**. As you can see, BTF consists in **type descriptors** to describe, using **BTF types**, all types being used in your eBPF object. Each **BTF type** has a certain **KIND** and might point to another **BTF type** or not.

You can imagine BTF as a memory chunk with:

- a header
- a chunk of null terminated strings
- concatenated **BTF types**

### BTF KINDS

By the time this document was written, the following BTF **kinds** exist:

- [BTF\_KIND\_INT](https://www.kernel.org/doc/html/latest/bpf/btf.html#btf-kind-int): integer.
- [BTF\_KIND\_FLOAT](https://www.kernel.org/doc/html/latest/bpf/btf.html#btf-kind-float): float.
- [BTF\_KIND\_PTR](https://www.kernel.org/doc/html/latest/bpf/btf.html#btf-kind-ptr): points to another type.
- [BTF\_KIND\_ARRAY](https://www.kernel.org/doc/html/latest/bpf/btf.html#btf-kind-array): array of a certain type, using same/other type as index.
- [BTF\_KIND\_STRUCT](https://www.kernel.org/doc/html/latest/bpf/btf.html#btf-kind-struct): has members of a certain type.
- [BTF\_KIND\_UNION](https://www.kernel.org/doc/html/latest/bpf/btf.html#btf-kind-union): same as struct.
- [BTF\_KIND\_ENUM](https://www.kernel.org/doc/html/latest/bpf/btf.html#btf-kind-union): enumerator of a certain type.
- [BTF\_KIND\_FWD](https://www.kernel.org/doc/html/latest/bpf/btf.html#btf-kind-fwd): forward-declaration to another type.
- [BTF\_KIND\_TYPEDEF](https://www.kernel.org/doc/html/latest/bpf/btf.html#btf-kind-typedef): typedef to another type.
- [BTF\_KIND\_VOLATILE](https://www.kernel.org/doc/html/latest/bpf/btf.html#btf-kind-volatile): volatile.
- [BTF\_KIND\_CONST](https://www.kernel.org/doc/html/latest/bpf/btf.html#btf-kind-const): constant.
- [BTF\_KIND\_RESTRICT](https://www.kernel.org/doc/html/latest/bpf/btf.html#btf-kind-restrict): restrict.
- [BTF\_KIND\_FUNC](https://www.kernel.org/doc/html/latest/bpf/btf.html#btf-kind-func): function. not a type. defines a subprogram/function.
- [BTF\_KIND\_FUNC\_PROTO](https://www.kernel.org/doc/html/latest/bpf/btf.html#btf-kind-func-proto): function prototype/signature type.
- [BTF\_KIND\_VAR](https://www.kernel.org/doc/html/latest/bpf/btf.html#btf-kind-var): variable. (points to variable type).
- [BTF\_KIND\_DATASEC](https://www.kernel.org/doc/html/latest/bpf/btf.html#btf-kind-datasec): elf data section.

BTF kinds are encoded as binary. They're placed one after another and, depending on its kind, they might have, or not, an addended structure containing more information (like STRUCTS that will contain addends for each existing STRUCT member).

Example of memory organization for the most important **BTF type kinds**:

![](docs/image06.png)

### BTF ELF SECTIONS

Each existing variable, or function, in your eBPF object will have its type described in the ELF section **.BTF**, by using one of the BTF kinds described here.

The `.BTF` and `.BTF.ext` ELF sections can be encoded by 2 different ways:

1. The [pahole](https://lwn.net/Articles/762847/) tool ([dwarves package](https://github.com/acmel/dwarves)): using existing non-stripped ELF file [DWARF debug data]((https://en.wikipedia.org/wiki/DWARF). The ELF file can be the kernel (check next topic) or a regular ELF file. Two extra ELF sections will be added to the same ELF used as input OR, more recently, to an external RAW BTF file (to feed libbpf).

2. LLVM: The 2 ELF sections are created automatically by LLVM, since it [added support for BTF generation](https://reviews.llvm.org/D53261), when compiling eBPF programs, to provide BTF for the types the eBPF programs use.

As said previously, difference among these 2 ELF sections is:

1. .BTF - contains information about all BTF types used within this object
2. [.BTF.ext](https://www.kernel.org/doc/html/latest/bpf/btf.html#btf-ext-section) - contains debug information about function prototypes, line numbers and, when encoded by LLVM, information about needed relocations to load the ELF object.

And you're able to visualize that information by executing **bpftool** (using the object generated by the [BTFHUB example](https://github.com/aquasecurity/btfhub/tree/main/example)):

```c
$ bpftool btf dump file ./example.bpf.o

[1] PTR '(anon)' type_id=2
[2] STRUCT 'pt_regs' size=168 vlen=21
        'r15' type_id=3 bits_offset=0
        'r14' type_id=3 bits_offset=64
        'r13' type_id=3 bits_offset=128
        'r12' type_id=3 bits_offset=192
        'bp' type_id=3 bits_offset=256
        'bx' type_id=3 bits_offset=320
        'r11' type_id=3 bits_offset=384
        'r10' type_id=3 bits_offset=448
        'r9' type_id=3 bits_offset=512
        'r8' type_id=3 bits_offset=576
        'ax' type_id=3 bits_offset=640
        'cx' type_id=3 bits_offset=704
        'dx' type_id=3 bits_offset=768
        'si' type_id=3 bits_offset=832
        'di' type_id=3 bits_offset=896
        'orig_ax' type_id=3 bits_offset=960
        'ip' type_id=3 bits_offset=1024
        'cs' type_id=3 bits_offset=1088
        'flags' type_id=3 bits_offset=1152
        'sp' type_id=3 bits_offset=1216
        'ss' type_id=3 bits_offset=1280
[3] INT 'long unsigned int' size=8 bits_offset=0 nr_bits=64 encoding=(none)
[4] FUNC_PROTO '(anon)' ret_type_id=5 vlen=1
        'ctx' type_id=1
[5] INT 'int' size=4 bits_offset=0 nr_bits=32 encoding=SIGNED
[6] FUNC 'do_sys_openat2' type_id=4 linkage=global
[7] STRUCT 'task_struct' size=9472 vlen=229
        'thread_info' type_id=8 bits_offset=0
        'state' type_id=12 bits_offset=192
        'stack' type_id=14 bits_offset=256
        'usage' type_id=15 bits_offset=320
        'flags' type_id=11 bits_offset=352
        'ptrace' type_id=11 bits_offset=384
        'on_cpu' type_id=5 bits_offset=416
        'wake_entry' type_id=19 bits_offset=448
        'cpu' type_id=11 bits_offset=576
...
[8] STRUCT 'thread_info' size=24 vlen=3
        'flags' type_id=3 bits_offset=0
        'syscall_work' type_id=3 bits_offset=64
        'status' type_id=9 bits_offset=128
[9] TYPEDEF 'u32' type_id=10
[10] TYPEDEF '__u32' type_id=11
[11] INT 'unsigned int' size=4 bits_offset=0 nr_bits=32 encoding=(none)
[12] VOLATILE '(anon)' type_id=13
[13] INT 'long int' size=8 bits_offset=0 nr_bits=64 encoding=SIGNED
[14] PTR '(anon)' type_id=0
[15] TYPEDEF 'refcount_t' type_id=16
[16] STRUCT 'refcount_struct' size=4 vlen=1
        'refs' type_id=17 bits_offset=0
[17] TYPEDEF 'atomic_t' type_id=18
[18] STRUCT '(anon)' size=4 vlen=1
        'counter' type_id=5 bits_offset=0
[19] STRUCT '__call_single_node' size=16 vlen=4
        'llist' type_id=20 bits_offset=0
        '(anon)' type_id=22 bits_offset=64
        'src' type_id=23 bits_offset=96
        'dst' type_id=23 bits_offset=112
[20] STRUCT 'llist_node' size=8 vlen=1
        'next' type_id=21 bits_offset=0
...
```

The **bpftool** command displays the `.BTF` ELF section information about all types used by a given eBPF object. It gets all the **BTF types** and extract their data and the needed strings from the string chunk (since some BTF types have a "name\_offset" pointer as an offset to the string buffer).

If you remember the graph showed earlier in this document, it can be obtained by following a specific BTF type declaration until its final resolution. One example, by focusing on a specific initial type (a struct):

```c
The BTF_TYPE id 311 is a BTF_KIND_STRUCT and describes a STRUCT
called  "swregs_state":

	[311] STRUCT 'swregs_state' size=136 vlen=16
	        'cwd' type_id=9 bits_offset=0
	        'swd' type_id=9 bits_offset=32
	        'twd' type_id=9 bits_offset=64
	        'fip' type_id=9 bits_offset=96
	        'fcs' type_id=9 bits_offset=128
	        'foo' type_id=9 bits_offset=160
	        'fos' type_id=9 bits_offset=192
	        'st_space' type_id=302 bits_offset=224
	        'ftop' type_id=58 bits_offset=864
	        'changed' type_id=58 bits_offset=872
	        'lookahead' type_id=58 bits_offset=880
	        'no_update' type_id=58 bits_offset=888
	        'rm' type_id=58 bits_offset=896
	        'alimit' type_id=58 bits_offset=904
	        'info' type_id=312 bits_offset=960
	        'entry_eip' type_id=9 bits_offset=1024

The struct 'swregs_state' has a field 'entry_eip'. This field
is a BTF_MEMBER of the BTF_TYPE id 311 (struct). The member
points to BTF_TYPE id 9.

	[9] TYPEDEF 'u32' type_id=10

The BTF_TYPE id 9 is a BTF_KIND_TYPEDEF and describes a typedef
called 'u32'. It points to the BTF_TYPE id 10.

	[10] TYPEDEF '__u32' type_id=11

The BTF_TYPE id 10 is a BTF_KIND_TYPEDEF and describes a typedef
called '__u32'. It points to the BTF_TYPE id 11.

	[11] INT 'unsigned int' size=4 bits_offset=0 nr_bits=32 encoding=(none)

The BTF_TYPE id 11 is a BTF_KIND_INT and describes an integer of
type "unsigned int". It is not a modifier BTF type, so it does not
point to anything more.

The BTF_TYPE id 11 is a BTF_KIND_INT and describes a “32 bits signed integer”
called "unsigned int". It is not a modifier BTF type, so it does not point to
anything more.
```

### BTF EXTERNAL FILES

As previously said, the [pahole](https://lwn.net/Articles/762847/) tool ([dwarves package](https://github.com/acmel/dwarves)) is able to encode BTF information for an ELF. [This is how the Linux kernel gets BTF ELF sections nowadays](https://elixir.bootlin.com/linux/v5.14.14/source/scripts/link-vmlinux.sh#L218), since the GCC compiler isn't able to generate those.

> If you are wondering why the Linux kernel ELF file also has BTF information, then we reached into an important mark in this document. The eBPF relocations, done by libbpf when loading an eBPF object into the kernel, will use both BTFs - from the kernel and from the eBPF object - to calculate/speculate the relocations needed for the eBPF programs contained in the object to run into the kernel eBPF VM.

> After [this change](https://lwn.net/Articles/790177/), BTF information is also used to create [eBPF maps](https://ebpf.io/what-is-ebpf#maps). The BPF object contains enough information about the maps (ELF section `maps`) and its types (ELF section `.BTF`) so it can create the eBPF map to be used by both: eBPF programs and userland code.

Before [this commit](https://github.com/acmel/dwarves/commit/89be5646a03435bfc6d2b3f208989d17f7d39312), pahole only supported encoding this information into the ELF sections described. This commit added support for encoding BTF information into a detached file (which we will refer from now on as _external BTF file_, or _raw BTF file_).

The reasoning behind having external, to ELF, files was that libbpf [needed to be able to load external BTF files](https://lore.kernel.org/bpf/1626180159-112996-2-git-send-email-chengshuyi@linux.alibaba.com/) to describe a current kernel that did not have BTF information available. Unfortunately some [older distros might not be able to have kernels (even recent ones) with BTF information](https://bugs.launchpad.net/ubuntu/+source/linux/+bug/1926330), so being able to generate BTF information from existing DWARF symbols - a thing that pahole does - is extremely necessary in those cases.

## BTFHUB

After the previous concept was stablished, of external BTF files, we stop here to understand **what BTFHUB is and why it was created**: external BTF files should be used for those kernels not supporting embedded BTF info (within its ELF sections) that keep debug information available somewhere (to be converted from DWARF format to BTF format).

From that need [BTFHUB](https://github.com/aquasecurity/btfhub/) was created. The hub contains BTF files to each existing kernels (of the supported distributions). Its [README.md](https://github.com/aquasecurity/btfhub/blob/main/README.md) file describes process of creating external BTF files, as well as enumerates some of the most used Linux distributions and if their supported (or some EOL) kernels support BTF or not.

> Currently there is a big need to use external BTF files for: CentOS7 (7.5.1804, kernel: 3.10.0-862), CentoOS 8 (8.1.1911, kernel: 4.18.0-147), Fedora 29 (kernel: 4.18), Fedora 30 (kernel: 5.0), Fedora 31 (kernel: 5.3), Ubuntu Bionic (with HWE kernels: 5.4 and 5.8) and Ubuntu Focal (kernel: 5.4 and HWE kernels: 5.8 and 5.11). If you're using newer versions of those distributions, there is a high change you don't need external BTF files as your kernel might already have its ELF .BTF section information. You can check that by executing: `bpftool btf dump file /sys/kernel/btf/vmlinux format raw` and seeing if it produces desired results.

> **Note**: BTFHUB is opened to contributions so, if you project requires external BTF files, you can always submit suggestions to BTFHUB for it to include your BTF files.

If you would like to give a try on how to use external BTF files, you may do the following:

```
$ git clone --recurse-submodules git@github.com:aquasecurity/btfhub.git
$ cd ./btfhub/example
$ make
```

Then you can execute `example-c-static` binary either using the **existing kernel BTF** information (from /sys/kernel/btf/vmlinux), or by **giving an external BTF file**, so libbpf (used by `example` code) can calculate needed relocations for running the eBPF object/programs in the current kernel.

First let's show how to **give an external BTF file** to the binary. I'm currently using the file that is provided by the running kernel, which is what libbpf does by default, just to test it:

```
$ sudo EXAMPLE_BTF_FILE=/sys/kernel/btf/vmlinux ./example-c-static

Foreground mode...<Ctrl-C> or or SIG_TERM to end it.
libbpf: loading example.bpf.o
...
```

And now, in another older kernel, from Ubuntu Bionic, I can run the exact **same binary** if **I provide a BTF file from the BTFHUB repository**:

```
$ uname -a
Linux bionic 5.4.0-87-generic #98~18.04.1-Ubuntu SMP Wed Sep 22 10:45:04 UTC 2021 x86_64 x86_64 x86_64 GNU/Linux

# Copy compressed BTF file and uncompress it:

$ cp /sources/ebpf/aquasec-btfhub/ubuntu/bionic/x86_64/$(uname -r).btf.tar.xz .
$ tar xvfJ ./$(uname -r).btf.tar.xz
5.4.0-87-generic.btf

# Execute ./example-c-static with the BTF for the current kernel

$ sudo EXAMPLE_BTF_FILE=./5.4.0-87-generic.btf ./example-c-static
Foreground mode...<Ctrl-C> or or SIG_TERM to end it.
libbpf: loading example.bpf.o
..
```

### BTFHUB AS THE SOLUTION AND WITH A PROBLEM

A perceptive reader might have already made the question: Okay, isn't that too expensive ? What about the size of the BTF files ? How many external BTF files my project needs to be fully portable ? What if I need to package my eBPF application, would I have to include ALL existing BTF files from the hub to have the binary being able to run in any exiting kernel ?

> That brings us back to the Linux Plumbers 2021 presentation: [Towards truly portable eBPF](https://www.youtube.com/watch?v=igJLKyP1lFk&t=1520s), but, now, specifically at the end of it: the [discussion about next steps](https://youtu.be/igJLKyP1lFk?t=2825).

And, yes, that was the biggest problem and one of the main reasons why **btfgen**, being detailed explained in this document, was created.

```
[user@aquasec-btfhub]$ ls -1 ./ubuntu/bionic/x86_64 | head -10
4.15.0-20-generic.btf.tar.xz
4.15.0-22-generic.btf.tar.xz
4.15.0-23-generic.btf.tar.xz
4.15.0-24-generic.btf.tar.xz
4.15.0-29-generic.btf.tar.xz
4.15.0-30-generic.btf.tar.xz
4.15.0-32-generic.btf.tar.xz
4.15.0-33-generic.btf.tar.xz
4.15.0-34-generic.btf.tar.xz
4.15.0-36-generic.btf.tar.xz

[user@aquasec-btfhub]$ du -sh .
1.5G	.
```

The entire BTFHUB nowadays has 1.5GB of compressed BTF files (1MB average). Unfortunately including all those files in a packaged eBPF based software is undoable. Our idea, discussed in the Linux Plumbers conference, was to **minimize each existing BTF file size by making them to contain ONLY the relocations needed for a certain eBPF application.**

> If that is not clear: instead of having ALL the kernel types, each external BTF file would contain only those types that are REALLY needed for libbpf to calculate the eBPF relocations during the code execution.

> After our presentation, Mauricio, from Kinvolk/Microsoft, has contacted us, Aqua Security Open Source Team, to tell he had already started in that idea and inviting us to collaborate with the amazing work he had already put together.

The results from **btfgen**, reducing the size needs to create a portable eBPF application, will be presented in the next sections.

But, first, let's understand BPF relocations.

## eBPF CO-RE AND BPF RELOCATIONS (continuation)

> Quotes within this session are from Andrii's blog (bpf-core-reference-guide).

Now that you're familiarized with how BTF information is organized we can continue in our quest to understand [BPF LLVM relocations](https://www.kernel.org/doc/html/latest/bpf/llvm_reloc.html). For eBPF objects, [libbpf](https://github.com/libbpf/libbpf) is the linker acting like a *dynamic linker*. With BTF information from both sides: the **eBPF object** and the **target kernel**, libbpf solves all the relocations before loading the eBPF object into the kernel.

Andrii has added CO-RE (Compile Once - Run Everywhere) support in libbpf in [this patchset](https://x-lore.kernel.org/bpf/20190724192742.1419254-1-andriin@fb.com/). As [presented in 2019](http://vger.kernel.org/bpfconf2019_talks/bpf-core.pdf), BPF-CORE overview is a sum of:

1. Self-describing kernel (BTF)
2. Clang w/ emitted relocations (\_\_builtin\_preserve\_access\_index() feature)
3. libbpf as relocating loader

> As this document was being made, Andrii created an [incredible reference to eBPF CO-RE](https://nakryiko.com/posts/bpf-core-reference-guide/) that you **should visit and read together with this document**. It will improve your understanding of eBPF CO-RE feature and help putting together the idea of why BTF generator was created.

**So, with CO-RE, the same eBPF object can run in multiple target kernels without recompilation.**

Like regular ELF relocations, eBPF bytecode also needs relocations to TEXT (instructions) and DATA segments to be done before eBPF bytecode can be JIT'ed by the in-kernel BPF JIT VM
and finally executed as instructions of the architecture you're running your kernel on.

There are different kinds of relocations currently supported by libbpf:

1. Local Relocations
2. Field Based Relocations
3. Type Based Relocations
4. ENUM Value Based Relocations

> Do not confuse *different kinds of relocations* being said here with different types of relocations supported by an architecture (for x64 arch: `R_X86_64_NONE`, `R_X86_64_64`, `R_X86_64_PC32`, so on).

### 1. LOCAL RELOCATIONS

Local relocations are inherent of how the compiler/architecture works. Those relocations are the ones dealing with global variables (including eBPF MAPs) and function symbol names, for example. They are the ones using different *types of relocations* supported by the **architecture** (and not different kinds of relocations supported by **libbpf**).

eBPF architecture supports the following 6 **relocation types**:

```
Enum  ELF Reloc Type     Description      BitSize  Offset        Calculation
0     R_BPF_NONE         None
1     R_BPF_64_64        ld_imm64 insn    32       r_offset + 4  S + A
2     R_BPF_64_ABS64     normal data      64       r_offset      S + A
3     R_BPF_64_ABS32     normal data      32       r_offset      S + A
4     R_BPF_64_NODYLD32  .BTF[.ext] data  32       r_offset      S + A
10    R_BPF_64_32        call insn        32       r_offset + 4  (S + A) / 8 - 1
```

and those relocations types will be used on eBPF relocation tables to instruct JIT compiler on how to relocate local (to the object) types.

In our BTFHUB code example, we can see these local relocations by doing:

```
$ llvm-readelf -r ./example.bpf.o

Relocation section '.relkprobe/do_sys_openat2' at offset 0x6f88 contains 1 entries:
    Offset             Info             Type               Symbol's Value  Symbol's Name
0000000000000290  0000000500000001 R_BPF_64_64            0000000000000000 events

Relocation section '.rel.BTF' at offset 0x6f98 contains 2 entries:
    Offset             Info             Type               Symbol's Value  Symbol's Name
0000000000003a40  0000000300000000 R_BPF_NONE             0000000000000000 LICENSE
0000000000003a58  0000000500000000 R_BPF_NONE             0000000000000000 events

Relocation section '.rel.BTF.ext' at offset 0x6fb8 contains 42 entries:
    Offset             Info             Type               Symbol's Value  Symbol's Name
000000000000002c  0000000200000000 R_BPF_NONE             0000000000000000 kprobe/do_sys_openat2
0000000000000040  0000000200000000 R_BPF_NONE             0000000000000000 kprobe/do_sys_openat2
0000000000000050  0000000200000000 R_BPF_NONE             0000000000000000 kprobe/do_sys_openat2
0000000000000060  0000000200000000 R_BPF_NONE             0000000000000000 kprobe/do_sys_openat2
...
```

This is very similar to a regular ELF load-time/link-time relocations that can either be solved during compilation OR runtime, by a linker when reading the symbols tables:

```
$ llvm-readelf --symbols ./example.bpf.o

Symbol table '.symtab' contains 6 entries:
   Num:    Value          Size Type    Bind   Vis       Ndx Name
     0: 0000000000000000     0 NOTYPE  LOCAL  DEFAULT   UND
     1: 00000000000002c8     0 NOTYPE  LOCAL  DEFAULT     2 LBB0_2
     2: 0000000000000000     0 SECTION LOCAL  DEFAULT     2 kprobe/do_sys_openat2
     3: 0000000000000000     4 OBJECT  GLOBAL DEFAULT     4 LICENSE
     4: 0000000000000000   728 FUNC    GLOBAL DEFAULT     2 do_sys_openat2
     5: 0000000000000000    20 OBJECT  GLOBAL DEFAULT     3 events
```

The difference is that, in a regular ELF file all the relocations are usually placed in ELF section .rela.text, while in the eBPF object ELF file they are placed in different sections (with names starting with .relXXXX).

### EBPF CO-RE RELATED RELOCATIONS

After we talked about local relocations it is good to clarify that the subsequent relocation kinds are specific to eBPF, using BTF information, and they're done by libbpf before loading the eBPF object. They are **load-time** relocations with peculiarities for eBPF object. By traversing the eBPF ELF object, instructions & data can be changed by those relocations, depending on its kind.

#### 2. FIELD BASED RELOCATIONS

After local relocations are done, the eBPF object isn't ready yet to be loaded. That happens because there are other relocations to be solved: field-type based relocations. Programmer will explicit enumerate them in the source code. There are MACROs to help the use of eBPF helper functions with **builtin\_preserve\_access\_index** feature.

When compiling CO-RE (Compile Once - Run Everywhere) BPF architecture objects, LLVM BPF backend records each relocation in an **ELF structure** containing only relocation information. Those structures are placed in the correspondent RELO section (.BTF.ext) so they can be used by libbpf during load time.

> This is only possible thanks to [this feature](https://clang.llvm.org/docs/LanguageExtensions.html#builtin-preserve-access-index) called **builtin\_preserve\_access\_index** (used by bpf\_core\_read() helper function). By using this keyword, when accessing a kernel pointer, you are instructing LLVM to **keep the relocation information into the generated ELF file** so the **kernels where the BPF object will run knows how to relocate the symbols ** 

Example of a header file:

```c
#pragma clang attribute push (__attribute__((preserve_access_index)), apply_to = record)

struct task_struct {
	pid_t pid;
	pid_t tgid;
}
```

And how to use it and force LLVM to save relocation information:

```c
pid_t pid = __builtin_preserve_access_index(({ task->pid; }));
```

Basic idea is this: you tell the compiler the types and fields you want to access through the helper function. It will use the internal (to LLVM) feature to keep track of everything that can be relocated (as BTF information) so, whenever the generated object is loaded, it solves relocations before running the code.

#### 3. TYPE BASED RELOCATIONS

> One of the very common problems BPF applications must deal with is the need to perform feature detection. I.e., detecting if a particular host kernel supports some new and optional feature, which BPF application can use to get more information or improve the efficiency.

Type based relocations are meant for the eBPF programs to discover more about the running environment. With this type of relocation, the running eBPF code is able to:

**- BPF\_TYPE\_ID\_LOCAL** - get BTF type ID of specified type using local BTF information.<BR>
**- BPF\_TYPE\_ID\_TARGET** - get BTF type id of specified type using target BTF information.<BR>
**- BPF\_TYPE\_EXISTS** - Check if provided named type (struct/union/enum/typedef) exists in target.<BR>
**- BPF\_TYPE\_SIZE** - Get the byte size of a provided named type (struct/union/enum/typedef) in a target kernel.<BR>

> **BTF generator** currently does not support this kind of relocations.

#### 4. ENUM VALUE BASED RELOCATIONS

> One interesting challenge that some BPF applications run into is the need to work with "unstable" internal kernel enums. That is, enums which don't have a fixed set of constants and/or integer values assigned to them.

Enum based relocations are meant to allow eBPF programs to get the exact ENUM value from the running kernel (through an ebpf helper function).

> **BTF generator** currently does not support this kind of relocations.

### EBPF RELATED RELOCATIONS FAILURE

> It's not unusual for some fields to be missing on some kernels. If a BPF program attempts a to read a missing field with `BPF_CORE_READ()`, it will result in an error during BPF verification. Similarly, CO-RE relocations will fail when getting enum value (or type size) of an enumerator (or a type) that doesn't exist in the host kernel.

Like said previously, [libbpf will poison instruction](https://nakryiko.com/posts/bpf-core-reference-guide/#guarding-potentially-failing-relocations) containing bad relocation whenever relocation can't be done during load time. The result will be something like:

```
1: (85) call unknown#195896080
invalid func unknown#195896080
```

> That 195896080 is 0xbad2310 in hex (for "bad relo") and is a constant that libbpf uses to mark instructions that failed CO-RE relocation. The reason libbpf doesn't just report such errors immediately is because missing field/type/enum and corresponding failing CO-RE relocation can be handled by the BPF application gracefully, if desired. This makes it possible to accommodate very drastic changes in kernel types with just a single BPF application (which is a crucial goal of "Compile Once – Run Everywhere" philosophy).

## BPF RELOCATION SPECULATION AND RESOLUTION

> With BPF CO-RE relocations there are always two BTF types involved. One is the BPF program's local expectation of the type definition (e.g., vmlinux.h types or types defined manually with preserve\_access\_index attribute). This **local BTF type** provides the means for libbpf to **know what to search for in the kernel BTF**. As such, it can be a minimal definition of the type/field/enum with only a necessary subset of fields and enumerators.

> Libbpf then can use **local BTF type definition** to **find a matching actual complete kernel BTF type**. The above helpers allow capturing BTF type IDs for both types involved in a CO-RE relocation. They could be useful for distinguishing different kernel or local types at runtime, for debugging and logging purposes, or potentially for future BPF APIs that would accept BTF type IDs as input arguments. Such APIs don't exist yet, but they are coming for sure soon.

Here I'd like to point important ideas of how libbpf does the relocations. This will help reader to understand how **BTF generator** was implemented using the logic already existent in libbpf.

When loading an eBPF object, the execution path until the CO-RE relocation logic is done is:

`bpf_prog_log_xattr()` or `bpf_object__load_skeleton()`
 - `bpf_object__load_xattr()`
	- `bpf_object__relocate()`
		- **`bpf_object__relocate_core()`**
			- `bpf_core_apply_relo()`

The function **`bpf_object__relocate_core()`** is the one responsible for walking the `.BTF.ext` section and apply the relocations that were flagged by LLVM compiler within the eBPF object's BTF data.

The basic relocation information (`bpf_core_relo`) is:

- `insn_off`: instruction offset (bytes) within a BPF program that needs its insn-\>imm field to be relocated.
- `type_id`: BTF type ID of the root entity of a relocatable type or field.
- `access_str_off`: offset of .BTF string section (string interpretation depends on the relocation kind: field-based, type-based or enum value-based).

**This information is crucial to the BTF generator logic, and this will be showed shortly.**

`bpf_object__relocate_core()` will try to execute each relocation that was placed into `.BTF.ext`, one by one. It even has a loop for each `BTF.ext` CO-RE relocation structure =\> apply the relocation for the given instruction.

This takes us to the second most important function (or couple of functions) to understand **BTF generator**: `bpf_core_apply_relo()` and its sister function `bpf_core_apply_relo_insn()`. Both are called with the `bpf_core_relo` structure given as an argument.

> Remember:
> local = the eBPF object you will load into the kernel
> target = the kernel we are trying to load the eBPF into

The first function, `bpf_core_apply_relo()` will get the **local type ID and size** and **initiate a cache for target types** that could satisfy given relocation. This cache is called `cand_cache` (candidates cache).

The second function, `bpf_core_apply_relo_insn()` will take care of the following:

1. Turn `bpf_core_relo` into low-level and high-level representation of a speculation and keep this named as **`local_spec`**.

2. Check each relocation candidate, from `cand_cache` if they really satisfy the relocation needs AND, if they do, generate a low and a high-level representation of the target speculation, named as **`targ_spec`**.

	![](docs/image07-01.png)

3. Call `bpf_core_calc_relo()` function to calculate the relocation for a given `local_spec` (local speculation) and a `targ_spec` (target speculation). Depending on the type of relocation being worked with it will call the appropriate handling function:

	- `bpf_core_calc_field_relo()`
	- `bpf_core_calc_type_relo()`
	- `bpf_core_calc_enumval_relo()`

	and this will result into a structure called **`targ_res`**, containing the **target resolution** for the *local and target speculations*. A `bpf_core_relo_res`, that represents the resolution of the 2 `bpf_core_relo`, contains, among other values, the following:

	1. original value within the instruction (expected value)
	2. new value that needs to be patched up to
	3. a bool to warn if that relocation was poisoned (due to some error)
	4. original size of the type id
	5. original type
	6. new size of the new type id
	7. new type id

		![](docs/image07-02.png)

4. Patch the instruction (`bpf_core_patch_insn()`) related to the relocation given by using `local_spec`AND `targ_res` information.

## BTF GENERATOR AND HOW IT WORKS

By now the reader has a clear (hopefully) picture of:

- eBPF CO-RE and how BTF information is generated and used
- BTF information structures: how types are linked to each other
- eBPF relocation kinds: field, type and enum-val based.
- eBPF relocation speculation based on local (eBPF object) and target (kernel) BTF information.

Now, let's stop a bit and re-think about BTFHUB and its biggest issue: size!

The main idea is:

1. You have an eBPF program and it can run in multiple recent kernels (**local BTF and relocations**).
2. You have BTFHUB with tons of BTFs for old kernels (**target BTFs**)

Why not to "filter in/out" the types being used by your eBPF object and only keep those that are interesting for you in the **target BTF** (the ones representing the kernels). This way, to fully support old kernels, you don't need a 1.5MB file to each of them.

When **Mauricio (Kinvolk/Microsoft)** approached us with his proof-of-concept code, he had already solved a problem we were about to start dealing with. Our main intent was to get a **TARGET BTF** file and make it small by containing only types being used by OUR ebpf object.

Unfortunately, simply getting existing BTF types from the local BTF and trying to create a target BTF with those won't work. You need to calculate relocations first and generate a target BTF **with the result of those relocations**. This will make sure that all types you need **at the end** (when running in the old kernel that does not have a BTF) exist.

In summary, by receiving a range of external BTF files, for different kernels, and a range of eBPF object files, from different eBPF based applications, as arguments, **BTF generator is responsible** for:

1. Calculate all relocations from eBPF obj file to each existing kernel BTF files
2. Generate partial BTF files to each existing/given kernel containing only types being used by an eBPF object (this way one can distribute an application with a bundle of BTF files and make it to support all old kernels for multiple distros).

**How BTF generator does this ?**

- By [patching libbpf](https://github.com/kinvolk/libbpf/tree/btfgen) to create specific BTF generation code (based on relocations)
- By [patching bpftool](https://github.com/kinvolk/btfgen) to create specific sub-function to generate BTF files

### WHAT DO WE NEED TO BUILD A BTF FILE

Libbpf already provides functions to easily manipulate BTF information. If you think about how BTF types are organized, based on previous picture examples, you will see that creating a BTF file is simply a question of creating an empty BTF (`btf__new_empty()`) information structure and add BTF types to it.

There are different ways to add a BTF type to an existing empty BTF structure. You might chose to either add the type through specific BTF type kind functions:

- `btf__add_int()`
- `btf__add_float()`
- `btf__add_ref_kind()` (PTR, TYPEDEF, CONST/VOLATILE/RESTRICT)
- `btf__add_ptr()`
- `btf__add_array()`
- `btf__add_composite()` (STRUCT/UNION by providing existing fields)
- `btf__add_struct()` (STRUCT with no fields)
- `btf__add_union()` (UNION with no fields)
- `btf__add_enum()` (ENUM with no enum values)
- `btf__add_fwd()` (FWD declaration to STRUCT, UNION or ENUM)
- `btf__add_typedef()`
- `btf__add_volatile()`
- `btf__add_const()`
- `btf__add_restrict()`
- `btf__add_func()`
- `btf__add_func_proto()` (FUNCTION prototype with no arguments)
- `btf__add_var()`
- `btf__add_datasec()`

and by populating those with:

- `btf__add_field()` (STRUCT/UNION new field)
- `btf__add_enum_value()` (ENUM new value)
- `btf__add_func_param()` (FUNCTION arguments)
- `btf__add_datasec_var_info()`

But there is also a generic way of adding types to a BTF in-memory structure:

- `btf__add_type()`

### HOW TO USE LIBBPF CO-RE RELOCATIONS TO BUILD A BTF FILE

Like said previously, we need to construct a BTF file containing only the types that are result from the eBPF relocation. We have everything we need right before libbpf applies the relocation to the instruction:

- A `bpf_core_relo` that represents 1 relocation from the origin (the eBPF object) = `local_spec`
- A `bpf_core_relo` that represents 1 type candidate to match the relocation type = `targ_spec`
- A `bpf_core_relo_res` that represents the resolution of the 2 `bpf_core_relo`.

All we need is to walk the relocation, type by type, member/field by member/field, and add found types - and field/member relationships - to a recently in-memory created BTF file. This way, at the end, our resulted BTF file will be a small subset of the original big BTF file. Exactly what we wanted to solve BTFHUB sizing issue.

So, let's do this, let's walk all the relocations. To each relocation, let's walk the BTF types being represented by the relocation. And let's understand **BTF generator** internals.

By starting the **BTF generator** tool with debug messages, having an external BTF file for kernel 5.4.0-87 (Ubuntu Bionic), to a complex eBPF object ([Tracee](https://github.com/aquasecurity/tracee/blob/main/tracee-ebpf/tracee/tracee.bpf.c)), we will see a list of all the relocations calculated from the given eBPF object for this object to run in a 5.4.0-87 kernel:

```
RELOCATION: [26219] struct bpf_raw_tracepoint_args.args[1] (0:0:1 @ offset 8)
RELOCATION: [28233] struct task_struct.real_parent (0:68 @ offset 2256)
RELOCATION: [28233] struct task_struct.pid (0:65 @ offset 2240)
RELOCATION: [28233] struct task_struct.nsproxy (0:105 @ offset 2760)
RELOCATION: [28421] struct nsproxy.pid_ns_for_children (0:4 @ offset 32)
RELOCATION: [28409] struct pid_namespace.level (0:6 @ offset 72)
RELOCATION: [28233] struct task_struct.thread_pid (0:75 @ offset 2344)
RELOCATION: [28411] struct pid.numbers (0:5 @ offset 80)
RELOCATION: [28408] struct upid.nr (0:0 @ offset 0)
RELOCATION: [28421] struct nsproxy.pid_ns_for_children (0:4 @ offset 32)
...
RELOCATION: [198] struct pt_regs.di (0:14 @ offset 112)
RELOCATION: [198] struct pt_regs.si (0:13 @ offset 104)
RELOCATION: [3484] struct socket.sk (0:4 @ offset 24)
RELOCATION: [3012] struct sock.__sk_common.skc_family (0:0:3 @ offset 16)
RELOCATION: [47942] struct inet_sock.sk.__sk_common.skc_rcv_saddr (0:0:0:0:1:1 @ offset 4)
RELOCATION: [47942] struct inet_sock.sk.__sk_common.skc_num (0:0:0:2:1:1 @ offset 14)
RELOCATION: [47942] struct inet_sock.sk.__sk_common.skc_daddr (0:0:0:0:1:0 @ offset 0)
RELOCATION: [47942] struct inet_sock.sk.__sk_common.skc_dport (0:0:0:2:1:0 @ offset 12)
RELOCATION: [47875] struct sockaddr_in.sin_family (0:0 @ offset 0)
RELOCATION: [47875] struct sockaddr_in.sin_port (0:1 @ offset 2)
RELOCATION: [47875] struct sockaddr_in.sin_addr.s_addr (0:2:0 @ offset 4)
RELOCATION: [49718] struct unix_sock.addr (0:1 @ offset 760)
RELOCATION: [49716] struct unix_address.len (0:1 @ offset 4)
RELOCATION: [49716] struct unix_address.name (0:3 @ offset 12)
RELOCATION: [3012] struct sock.__sk_common.skc_state (0:0:4 @ offset 18)
RELOCATION: [47942] struct inet_sock.pinet6 (0:1 @ offset 760)
...
```

Picking one relocation as example:

```
RELOCATION: [47942] struct inet_sock.sk.__sk_common.skc_num (0:0:0:2:1:1 @ offset 14)
```

We can see that this relocation happens because of `struct inet_sock`. This is the **root entity** of the relocation. All the rest are either members or fields for the relocation. The `struct inet_sock` is a `BTF_TYPE` with id == 47942.

By executing bpftool we're able to follow that:

```
$ bpftool btf dump file ./btfs/5.4.0-87-generic.btf format raw
...
[47942] STRUCT 'inet_sock' size=968 vlen=30
        'sk' type_id=3012 bits_offset=0
        'pinet6' type_id=47944 bits_offset=6080
        'inet_saddr' type_id=2996 bits_offset=6144
        'uc_ttl' type_id=16 bits_offset=6176
        'cmsg_flags' type_id=18 bits_offset=6192
        'inet_sport' type_id=2995 bits_offset=6208
        'inet_id' type_id=18 bits_offset=6224
        'inet_opt' type_id=47938 bits_offset=6272
        'rx_dst_ifindex' type_id=21 bits_offset=6336
        'tos' type_id=13 bits_offset=6368
        'min_ttl' type_id=13 bits_offset=6376
        'mc_ttl' type_id=13 bits_offset=6384
        'pmtudisc' type_id=13 bits_offset=6392
        'recverr' type_id=13 bits_offset=6400 bitfield_size=1
        'is_icsk' type_id=13 bits_offset=6401 bitfield_size=1
        'freebind' type_id=13 bits_offset=6402 bitfield_size=1
        'hdrincl' type_id=13 bits_offset=6403 bitfield_size=1
        'mc_loop' type_id=13 bits_offset=6404 bitfield_size=1
        'transparent' type_id=13 bits_offset=6405 bitfield_size=1
        'mc_all' type_id=13 bits_offset=6406 bitfield_size=1
        'nodefrag' type_id=13 bits_offset=6407 bitfield_size=1
        'bind_address_no_port' type_id=13 bits_offset=6408 bitfield_size=1
        'defer_connect' type_id=13 bits_offset=6409 bitfield_size=1
        'rcv_tos' type_id=13 bits_offset=6416
        'convert_csum' type_id=13 bits_offset=6424
        'uc_index' type_id=21 bits_offset=6432
        'mc_index' type_id=21 bits_offset=6464
        'mc_addr' type_id=2996 bits_offset=6496
        'mc_list' type_id=47945 bits_offset=6528
        'cork' type_id=47941 bits_offset=6592
...
```

And this is the moment you realize the external BTF file `5.4.0-87-generic.btf` is HUGE as it contains all types used by the kernel image. Let's continue. In our relocation we had `sk` string as the member of `struct inet_sock`. In the BTF dump we can find the member `sk` and check which BTF\_TYPE id it points to:

```
        'sk' type_id=3012 bits_offset=0
```

Continuing, now we must find BTF type id == 3012:

```
[3012] STRUCT 'sock' size=760 vlen=88
        '__sk_common' type_id=4507 bits_offset=0
        'sk_lock' type_id=4492 bits_offset=1088
        'sk_drops' type_id=79 bits_offset=1344
        'sk_rcvlowat' type_id=21 bits_offset=1376
        'sk_error_queue' type_id=3582 bits_offset=1408
        'sk_rx_skb_cache' type_id=3247 bits_offset=1600
        'sk_receive_queue' type_id=3582 bits_offset=1664
        'sk_backlog' type_id=4510 bits_offset=1856
        'sk_forward_alloc' type_id=21 bits_offset=2048
        'sk_ll_usec' type_id=9 bits_offset=2080
        'sk_napi_id' type_id=9 bits_offset=2112
        'sk_rcvbuf' type_id=21 bits_offset=2144
        'sk_filter' type_id=4514 bits_offset=2176
        '(anon)' type_id=4511 bits_offset=2240
        'sk_policy' type_id=4515 bits_offset=2304
        'sk_rx_dst' type_id=3382 bits_offset=2432
        'sk_dst_cache' type_id=3382 bits_offset=2496
        'sk_omem_alloc' type_id=79 bits_offset=2560
        'sk_sndbuf' type_id=21 bits_offset=2592
        'sk_wmem_queued' type_id=21 bits_offset=2624
        'sk_wmem_alloc' type_id=790 bits_offset=2656
...
```

And it's a `struct sock`. So, we currently know we had a relocation for a `struct inet_sock`-\>`struct sock`-\>XXX. Now, the 3rd member of this relocation is `__sk_common` and we can find it right in the beginning of the `struct sock` BTF information:

```
        '__sk_common' type_id=4507 bits_offset=0
```

The BTF type id 4507 is:

```
[4507] STRUCT 'sock_common' size=136 vlen=25
        '(anon)' type_id=4496 bits_offset=0
        '(anon)' type_id=4497 bits_offset=64
        '(anon)' type_id=4500 bits_offset=96
        'skc_family' type_id=19 bits_offset=128
        'skc_state' type_id=2991 bits_offset=144
        'skc_reuse' type_id=14 bits_offset=152 bitfield_size=4
        'skc_reuseport' type_id=14 bits_offset=156 bitfield_size=1
        'skc_ipv6only' type_id=14 bits_offset=157 bitfield_size=1
        'skc_net_refcnt' type_id=14 bits_offset=158 bitfield_size=1
        'skc_bound_dev_if' type_id=21 bits_offset=160
        '(anon)' type_id=4501 bits_offset=192
        'skc_prot' type_id=4509 bits_offset=320
        'skc_net' type_id=3613 bits_offset=384
        'skc_v6_daddr' type_id=3289 bits_offset=448
        'skc_v6_rcv_saddr' type_id=3289 bits_offset=576
        'skc_cookie' type_id=81 bits_offset=704
        '(anon)' type_id=4502 bits_offset=768
        'skc_dontcopy_begin' type_id=106 bits_offset=832
        '(anon)' type_id=4504 bits_offset=832
        'skc_tx_queue_mapping' type_id=19 bits_offset=960
        'skc_rx_queue_mapping' type_id=19 bits_offset=976
        '(anon)' type_id=4505 bits_offset=992
        'skc_refcnt' type_id=790 bits_offset=1024
        'skc_dontcopy_end' type_id=106 bits_offset=1056
        '(anon)' type_id=4506 bits_offset=1056
```

By now our relocation `inet_sock.sk.__sk_common.skc_num (0:0:0:2:1:1 @ offset 14) ` only had structs as members. The next field is `skc_num` but there is a catch. We won't find `skc_num` as a member of the type 4507. That happens because that is an anonymous type. Now its time to pay attention to the fields, not only the member types. In the relocation

- `inet_sock.sk.__sk_common.skc_num (0:0:0:**2:1:1** @ offset 14)`

we have numbers at the end that tells us what fields to use if they're unnamed (which is the case here). The 4th member has field #2, which is:

```
        '(anon)' type_id=4500 bits_offset=96
```

Checking the BTF type id it points to:

```
[4500] UNION '(anon)' size=4 vlen=2
        'skc_portpair' type_id=4493 bits_offset=0
        '(anon)' type_id=4499 bits_offset=0
```

We will also must rely in the 5th member field #1 to know the relocation field:

```
        '(anon)' type_id=4499 bits_offset=0
```

And checking BTF type id 4499, also using the 6th member field #1:

```
[4499] STRUCT '(anon)' size=4 vlen=2
        'skc_dport' type_id=2995 bits_offset=0
        'skc_num' type_id=18 bits_offset=16
```

We will find the member named `skc_num`.

So, as showed, to each given relocation we must use contained information, of types, root entities, members, and fields, to construct another BTF file with the resulted relocations. This another BTF file will be a subset of the given external BTF file for kernel 5.4.0-87, but it will only contain the BTF types we need. This way we can use this generated BTF as an input of being an external BTF file to libbpf and load our eBPF program into a 5.4.0-87 kernel with a very small external BTF file (no need to have the big one). Of course, this external BTF file is tailor made to this eBPF object and other eBPF objects won't work. Idea is exactly that: your eBPF application can generate a bundle of BTF files representing all supported kernels together with its binaries, and be able to run your code EVERYWHERE (as CO-**RE** means **Run Everywhere**

### HOW TO ORGANIZE DATA WE NEED

**BTF generator** organizes all its information in three structs called `btf_reloc_info` and `btf_reloc_type` and `btf_reloc_member`. It also uses already existent `btf_type` and `btf_member` structures.

Look on how the data is organized:

![](docs/image08.png)

A single `BTF_RELOC_INFO` structure is created, and it contains:

- `SRC_BTF`: a pointer to the the source (eBPF object) BTF file
- `TYPES`: a hashmap of all `BTF_RELOC_TYPE` structures
- `IDS_MAP`: a hashmap containing a NEW TYPE ID value to each existent OLD TYPE ID value.

Focusing into `BTF_RELOC_TYPE` for now, it contains:

- `BTF_TYPE`: A ptr to the BTF type of the root entity of a relocation (`inet_sock` from the previous example). It can be any BTF type kind (STRUCT/UNION, INT, FLOAT, PTR, ...).
- `ID`: the BTF type id of the root entity of the relocation (47942 from the previous example)
- `MEMBERS`: a hashmap of all `BTF_RELOC_MEMBERS` (one per existing BTF type member)

So, if the `BTF_RELOC_TYPE` represents a UNION or a STRUCT, we will have `BTF_RELOC_MEMBER` to each existing member within the relocation. The `BTF_RELOC_MEMBER` contains:

- A pointer to `BTF_MEMBER` structure representing the member from that relocation.

At the end we have: All relocations are resolved so each type, from each field or member of that relocation, is added as a new type of the final BTF. Each existing type has its own `BTF_RELOC_TYPE` structure that can contain, or not, `BTF_RELOC_MEMBER`s.

It is important to keep the **"root entity type" =\> "members"** relationship because that is what will give us the final BTF graph. If the types are simple, then no members or any other structure is needed to be appended to the root entity BTF type. If the types are complex, then we would must initially add the types, and then add a field/member, one by one, to each complex type (like structs and unions) added.

Gladly libbpf allows us to add a complex BTF type with members already (through `btf__add_type()`). This is what function `bpf_reloc_info__get_btf()` does at its **first pass** through all existing types from `BTF_RELOC_INFO` structure.

Unfortunately, whenever we add a BTF type to a new BTF file it also gets a new BTF type ID. This means that the relationship between the BTF types (root entities) and existing fields and members are broken. Check it out:

```
$ bpftool btf dump file ./generated/5.4.0-87-generic.btf format raw

[1] PTR '(anon)' type_id=28278
[2] TYPEDEF 'u32' type_id=23
[3] TYPEDEF '__be16' type_id=18
[4] PTR '(anon)' type_id=47943
[5] TYPEDEF '__u8' type_id=14
[6] PTR '(anon)' type_id=49716
[7] STRUCT 'mnt_namespace' size=120 vlen=1
        'ns' type_id=1949 bits_offset=64
[8] TYPEDEF '__kernel_gid32_t' type_id=9
[9] STRUCT 'iovec' size=16 vlen=2
        'iov_base' type_id=103 bits_offset=0
        'iov_len' type_id=48 bits_offset=64
[10] PTR '(anon)' type_id=28881
[11] STRUCT '(anon)' size=8 vlen=2
        'skc_daddr' type_id=2996 bits_offset=0
        'skc_rcv_saddr' type_id=2996 bits_offset=32
[12] TYPEDEF '__u64' type_id=27
[13] PTR '(anon)' type_id=36422
[14] TYPEDEF 'pid_t' type_id=45
[15] PTR '(anon)' type_id=28284
[16] PTR '(anon)' type_id=8
...
```

So, despite having all BTF types being used by our eBPF object, the complex types point to non-existent types. For example:

```
        'skc_daddr' type_id=2996 bits_offset=0
```

There is no such BTF type id == 2996 in the generated BTF file. That is the reason why, in the data organization picture you will find a hashmap for OLD and NEW TYPE IDs. All BTF type ids being pointed to are fixed by using this hashmap of OLD and NEW TYPE IDs and this is done by the **second pass** of function `bpf_reloc_info__get_btf()`.

Look how the generated file look like after this **second pass**:

```
[1] PTR '(anon)' type_id=97
[2] TYPEDEF 'u32' type_id=35
[3] TYPEDEF '__be16' type_id=22
[4] PTR '(anon)' type_id=51
[5] TYPEDEF '__u8' type_id=82
[6] PTR '(anon)' type_id=29
[7] STRUCT 'mnt_namespace' size=120 vlen=1
        'ns' type_id=71 bits_offset=64
[8] TYPEDEF '__kernel_gid32_t' type_id=74
[9] STRUCT 'iovec' size=16 vlen=2
        'iov_base' type_id=16 bits_offset=0
        'iov_len' type_id=84 bits_offset=64
[10] PTR '(anon)' type_id=57
[11] STRUCT '(anon)' size=8 vlen=2
        'skc_daddr' type_id=80 bits_offset=0
        'skc_rcv_saddr' type_id=80 bits_offset=32
[12] TYPEDEF '__u64' type_id=87
[13] PTR '(anon)' type_id=7
[14] TYPEDEF 'pid_t' type_id=104
[15] PTR '(anon)' type_id=62
[16] PTR '(anon)' type_id=0
...
```

You will realize that the same example is now fixed and pointing to the correct BTF type id:

```
        'skc_daddr' type_id=80 bits_offset=0
```

BTF type id == 80 is:

```
[80] TYPEDEF '__be32' type_id=35
```

And it points to btf type id == 35:

```
[35] TYPEDEF '__u32' type_id=74
```

Which points to type id 74:

```
[74] INT 'unsigned int' size=4 bits_offset=0 nr_bits=32 encoding=(none)
```

Which is a simple type and does not need to point anywhere.

## TIME TO TEST BTFGEN AND BTFHUB

The reader can opt to use BTFHUB in different ways:

1. To download an ENTIRE BTF file for a specific kernel version and use it as an external BTF file when loading your app using libbpf. Examples:

	- [tracee-ebpf](https://github.com/aquasecurity/tracee/blob/main/tracee-ebpf/main.go#L155)
	- [inspektor-gadget](https://github.com/kinvolk/inspektor-gadget/commit/f7a807e86ea90d4df1bfea013139381c07aab6d2)

	To each different kernel you will have to download the correspondent BTF file available in BTFHUB. This is the old, problematic (big), way of doing CO-RE for kernels that don't support BTF.

or

2. To clone **BTFHUB** and use **btfgen** to generate **ALL BTF files** for the kernels *you would like to support for your app*. Example:

	```
	[user@host:~/.../aquasec-btfhub/tools][main]$ ./btfgen.sh ~/aquasec-tracee/tracee-ebpf/dist/tracee.bpf.core.o
	```

If you visit the [tools](https://github.com/aquasecurity/btfhub/tree/main/tools) directory within [BTFHUB](https://github.com/aquasecurity/btfhub) you will see instructions on how to use a non-upstreamed (and statically compiled version) of **btfgen** (the **BTF generator**).

[btfgen.sh](https://github.com/aquasecurity/btfhub/blob/main/tools/btfgen.sh) script will generate **multiple BTF files and symlinks** at `tools/output/{centos,fedora,ubuntu}/` containing ONLY the types being used by the given eBPF object (`tracee.bpf.core.o` in this example), with the relocations for each specific kernel version already recalculated.

Then you can execute your application by loading the correspondent - to your running kernel - partial generated BTF file:

Example:

```
$ uname -a
Linux bionic 5.4.0-87-generic #98~18.04.1-Ubuntu SMP Wed Sep 22 10:45:04 UTC 2021 x86_64 x86_64 x86_64 GNU/Linux

$ lsb_release -a
No LSB modules are available.
Distributor ID:	Ubuntu
Description:	Ubuntu 18.04.6 LTS
Release:	18.04
Codename:	bionic

$ bpftool btf dump file ./generated/5.4.0-87-generic.btf format raw | grep "^\[" | wc -l
122

[user@host:~/.../aquasec-tracee/tracee-ebpf]$ sudo TRACEE_BTF_FILE=~/aquasec-btfhub/tools/output/ubuntu/18.04/x86_64/5.4.0-87-generic.btf ./dist/tracee-ebpf --debug --trace event=execve,execveat,uname

OSInfo: VERSION: "18.04.6 LTS (Bionic Beaver)"
OSInfo: ID: ubuntu
OSInfo: ID_LIKE: debian
OSInfo: PRETTY_NAME: "Ubuntu 18.04.6 LTS"
OSInfo: VERSION_ID: "18.04"
OSInfo: VERSION_CODENAME: bionic
OSInfo: KERNEL_RELEASE: 5.4.0-87-generic
BTF: bpfenv = false, btfenv = true, vmlinux = false
BPF: using embedded BPF object
BTF: using BTF file from environment: ~/aquasec-btfhub/tools/output/ubuntu/18.04/x86_64/5.4.0-87-generic.btf
unpacked CO:RE bpf object file into memory

TIME             UID    COMM             PID     TID     RET              EVENT                ARGS
05:08:45:175699  1000   bash             5176    5176    0                execve               pathname: /bin/ls, argv: [ls --color=auto]
05:08:45:188780  1000   bash             5180    5180    0                execve               pathname: /usr/bin/git, argv: [git branch]
05:08:45:189986  1000   bash             5181    5181    0                execve               pathname: /bin/sed, argv: [sed -e /^[^*]/d -e s/* \(.*\)/\1/]
05:08:45:971635  1000   bash             5183    5183    0                execve               pathname: /bin/ps, argv: [ps]
05:08:46:015186  1000   bash             5186    5186    0                execve               pathname: /usr/bin/git, argv: [git branch]
05:08:46:015415  1000   bash             5187    5187    0                execve               pathname: /bin/sed, argv: [sed -e /^[^*]/d -e s/* \(.*\)/\1/]

End of events stream
Stats: {EventCount:6 ErrorCount:0 LostEvCount:0 LostWrCount:0 LostNtCount:0}
```

## FURTHER DOCUMENTATION

Here you will find some other sources of information about eBPF CO-RE:

1. [https://ebpf.io/what-is-ebpf](https://ebpf.io/what-is-ebpf)
2. [https://github.com/libbpf/libbpf](https://github.com/libbpf/libbpf)
3. [https://nakryiko.com/posts/bpf-portability-and-co-re/](https://nakryiko.com/posts/bpf-portability-and-co-re/)
4. [https://nakryiko.com/posts/bpf-portability-and-co-re/#btf](https://nakryiko.com/posts/bpf-portability-and-co-re/#btf)

Other great links that might be worth reading are:

1. [Introduction to eBPF](https://ebpf.io/what-is-ebpf#introduction-to-ebpf) (from: ebpf.io/what-is-ebpf)
2. [Development Toolchains](https://ebpf.io/what-is-ebpf#development-toolchains) (from: ebpf.io/what-is-ebpf)
3. [BCC to libbpf conversion guide](https://nakryiko.com/posts/bcc-to-libbpf-howto-guide/) (if you're coming from BCC)
4. [Building BPF applications with libbpf-bootstrap](https://nakryiko.com/posts/libbpf-bootstrap/)
5. [BPF Design FAQ](https://01.org/linuxgraphics/gfx-docs/drm/bpf/bpf_design_QA.html)
6. [eBPF features per kernel version](https://github.com/iovisor/bcc/blob/master/docs/kernel-versions.md#program-types)
7. [BTFHUB code example](https://github.com/aquasecurity/btfhub/tree/main/example)
8. [BCC's libbpf-tools directory](https://github.com/iovisor/bcc/tree/master/libbpf-tools)

Now, some links about formats, eBPF and BTF internals:

1. [ELF - Executable and Linkable Format](https://en.wikipedia.org/wiki/Executable_and_Linkable_Format)
2. [BPF Type Format](https://www.kernel.org/doc/html/latest/bpf/btf.html)
3. [BTF deduplication and Linux Kernel BTF](https://nakryiko.com/posts/btf-dedup/)
4. [BPF LLVM Relocations](https://www.kernel.org/doc/html/latest/bpf/llvm_reloc.html)
