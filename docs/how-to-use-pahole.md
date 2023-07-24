## Tools that generate BTF information

Typically, BTF information is encoded within the .BTF and .BTF.ext ELF sections, or it can be found in a raw BTF file. There are two primary methods to encode BTF information:

1. **The [pahole](https://lwn.net/Articles/762847/) tool (from the [dwarves](https://github.com/acmel/dwarves)) project):**

	This tool leverages existing non-stripped ELF files that contain [DWARF debug data](https://en.wikipedia.org/wiki/DWARF). The input ELF file can either be the kernel or a standard eBPF ELF object. As a result of the process, two additional ELF sections with BTF encoding are appended to the input ELF file. More recently, this BTF encoding can also be added to an external raw BTF file, which can be used as input for libbpf.

2. **LLVM:**

	When compiling eBPF code, LLVM automatically generates the .BTF and .BTF.ext ELF sections. It's worth noting that unlike pahole, LLVM is capable of generating BTF relocation information. This information is crucial for libbpf to perform CO-RE relocations successfully. 

By choosing either of these methods, you can effectively generate BTF information according to the requirements of your application.

## Reading BTF information

1. **Using `bpftool`**:

	If the kernel you're currently running supports embedded BTF (a feature supported by most modern Linux distributions), you will find the corresponding BTF information in a sysfs file located at `/sys/kernel/btf/vmlinux`. You can inspect the contents of this file by executing:

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

2. **Using `pahole`**:

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

	If the `/sys/kernel/btf/vmlinux` file is not present in your current kernel or in the kernel for which you aim to offer eBPF application support, you will likely need to turn to BTFhub. This necessity arises when the kernel hasn't been compiled with the `DEBUG_INFO_BTF` kconfig option enabled, resulting in the absence of the aforementioned file. This situation is prevalent in most of the kernels represented in the BTFhub-Archive repository. Therefore, in such scenarios, BTFhub might definitely help you.

## How does the Linux kernel generate its own BTF information ?

The central premise of [BTFhub](https://github.com/aquasecurity/btfhub/) and [BTFhub-Archive](https://github.com/aquasecurity/btfhub-archive/) revolves around the automation of kernel debugging package conversion to BTF information, which is vital for libbpf to operate CO-RE capable eBPF applications. This is achieved by the [btfhub](https://github.com/aquasecurity/btfhub/blob/main/cmd/btfhub/main.go) application (compiled through `make`), [which downloads all existing debug kernel packages for the supported distributions and subsequently converts the embedded DWARF information into BTF format.](https://github.com/aquasecurity/btfhub/blob/f37a9cdc160f3add77a24beb6512dbb4557bc728/.github/workflows/cron.yml)

BTFhub operates the `btfhub` application in a manner similar to a cron job, executing it daily. The generated BTF files are then uploaded into the [BTFhub-Archive repository](https://github.com/aquasecurity/btfhub-archive/), where they can be consumed by your project.

1. For hands-on experience, you can add a .BTF ELF section to the non-stripped vmlinuz (uncompressed) kernel file by executing:

	```
	pahole -J vmlinux
	```

2. Alternatively, you can generate an external raw BTF file using the following command:

	```
	pahole --btf_encode_detached external.btf vmlinux
	```

3. If you opt to generate a new .BTF ELF section within the vmlinuz file, you can later extract it into an external raw BTF file (e.g., vmlinux.btf) using this command:

	```
	llvm-objcopy --only-section=.BTF --set-section-flags .BTF=alloc,readonly vmlinux vmlinux.btf
	```

This flexibility ensures a tailored approach to BTF file generation, promoting enhanced compatibility across different kernel versions and distributions.
