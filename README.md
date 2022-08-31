While [BTFhub main repository](https://github.com/aquasecurity/btfhub/) contains documentation, tooling and examples on how to use the BTF files, the BTF files exist in the [BTFhub-Archive repository](https://github.com/aquasecurity/btfhub-archive/).

## What is BTF ?

[BTF](https://nakryiko.com/posts/bpf-portability-and-co-re/#btf) is one of the
things that make eBPF portable.

Before [CO-RE](https://nakryiko.com/posts/bpf-portability-and-co-re/) existed,
eBPF developers [had to
compile](https://ebpf.io/what-is-ebpf#how-are-ebpf-programs-written) one eBPF
object per supported kernel. This made eBPF toolkits, such as
[iovisor/bcc](https://github.com/iovisor/bcc), to rely on runtime compilations.

With [CO-RE](https://nakryiko.com/posts/bpf-portability-and-co-re/), the same
eBPF object can be loaded into multiple different kernels. The
[libbpf](https://github.com/libbpf/libbpf)
[loader](https://ebpf.io/what-is-ebpf#loader--verification-architecture) will
allow [CO-RE](https://nakryiko.com/posts/bpf-portability-and-co-re/) by
arranging needed infrastructure for a given eBPF object, such as [eBPF
maps](https://ebpf.io/what-is-ebpf#maps) creation, code relocation, eBPF
probes, links and their attachments, etc.

The [eBPF Type Format (BTF)](https://nakryiko.com/posts/btf-dedup/) is a data
format to store debug information about eBPF objects OR about the kernels they
will be loaded into.

**The idea is this**: Both, the **eBPF object** AND the **target kernel**, have
BTF information available, usually embedded into their ELF files. The
[libbpf](https://github.com/libbpf/libbpf) loader uses the embedded BTF
information to calculate needed changes (relocations, map creations, probe
attachments, ...) for an eBPF object to be loaded and have its programs
executed in any kernel, without modifications to the object.

## What is BTFhub ?

Unfortunately the
[BTF](https://github.com/iovisor/bcc/blob/master/docs/kernel-versions.md#main-features)
format wasn't always available and, because of **missing kernel support**, or
because of the [lack of userland
tools](https://bugs.launchpad.net/ubuntu/+source/linux/+bug/1949286), capable
of understanding the BTF format, distributions release(ed) kernels without the
embedded BTF information.

That is [why BTFhub exists](https://www.youtube.com/watch?v=ZYd0lVRwY80): to
provide BTF information for Linux distributions released kernels that don't
have embedded BTF information. Instead of recompiling your eBPF code to each
existing Linux kernel that does not support BTF information, your code will be
relocated - by libbpf - according to available BTF information from the BTFhub
files.

After libbpf [started supporting external (raw) BTF
files](https://github.com/libbpf/libbpf/commit/4920031c8809696debf43f7b0c8f95ea24b8f61c),
we're able to feed libbpf with this external BTF file for a kernel you want to
run your eBPF code into. Each kernel needs its own BTF file.

**Note**: You won't need BTFhub if you're willing to support your eBPF CO-RE
application only in the latest kernels. Now, if you are willing to support ALL
released kernels, including some Long Term Support Linux distribution versions,
then you may need to use BTFhub.

## Supported Kernels and Distributions

[This is a list](docs/supported-distros.md) of existing distributions and their
current status on **eBPF** and **BTF** support.

## How can I use it ?

1. [This is a code example](example/) of how you should use BTFhub to add
   support to legacy kernels to your eBPF project. The uncompressed full BTF
   files, from the [BTFhub-Archive repository](https://github.com/aquasecurity/btfhub-archive),
   should feed libbpf used by your eBPF project, just like showed in
   [this C example](https://github.com/aquasecurity/btfhub/blob/26ec6014bd7340c3894f486db57a1ef0a712a3b0/example/example.c#L189)
   or [this Go example](https://github.com/aquasecurity/btfhub/blob/26ec6014bd7340c3894f486db57a1ef0a712a3b0/example/example.go#L88).

2. You may use the [BTFgen tool to create smaller BTF
   files](docs/generating-tailored-btfs.md), so you can embed them into your
   eBPF application and make it support all kernels supported by BTFhub.

## Where can I find more information ?

- [How to use Pahole to generate BTF information](https://github.com/aquasecurity/btfhub/blob/main/docs/how-to-use-pahole.md)
- [BTF Generator Internals](https://github.com/aquasecurity/btfhub/blob/main/docs/btfgen-internals.md)
- more references to come...
