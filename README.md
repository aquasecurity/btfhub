The [main BTFhub repository](https://github.com/aquasecurity/btfhub/) serves as a comprehensive resource, housing documentation, tools, and examples to guide users on how to leverage the BTF files effectively. However, the actual BTF files are stored separately in the [BTFhub-Archive repository](https://github.com/aquasecurity/btfhub-archive/). This separation ensures a clean and organized structure, with each repository focusing on its designated role.

## What is BTF ?

The Extended Berkeley Packet Filter (eBPF) is esteemed for its portability, a primary attribute of which is due to the BPF Type Format (BTF). More details about BTF can be discovered in this [comprehensive guide](https://nakryiko.com/posts/bpf-portability-and-co-re/#btf).

Before the advent of [Compile Once-Run Everywhere (CO-RE)](https://nakryiko.com/posts/bpf-portability-and-co-re/), developers working with eBPF had to compile an individual eBPF object for each kernel version they intended to support. This stipulation led toolkits, such as [iovisor/bcc](https://github.com/iovisor/bcc), to depend on runtime compilations to handle different kernel versions.

However, the introduction of [CO-RE](https://nakryiko.com/posts/bpf-portability-and-co-re/) facilitated a significant shift in eBPF portability, allowing a single eBPF object to be loaded into multiple differing kernels. This is achieved by the [libbpf loader](https://github.com/libbpf/libbpf), a component within the eBPF's [loader and verification architecture](https://ebpf.io/what-is-ebpf#loader--verification-architecture). The libbpf loader arranges the necessary infrastructure for an eBPF object, including eBPF map creation, code relocation, setting up eBPF probes, managing links, handling their attachments, among others.

Here's the technical insight: both the eBPF object and the target kernel contain BTF information, generally embedded within their respective ELF (Executable and Linkable Format) files. The libbpf loader leverages this embedded BTF information to calculate the requisite changes such as relocations, map creations, probe attachments, and more for an eBPF object. As a result, this eBPF object can be loaded and have its programs executed across any kernel without the need for object modification, thus enhancing portability.

## BTFHUB

Regrettably, the [BPF Type Format (BTF)](https://github.com/iovisor/bcc/blob/master/docs/kernel-versions.md#main-features) wasn't always readily available. This can be attributed to either a lack of kernel support or the absence of userland tools capable of interpreting the BTF format. As a result, certain Linux distributions ended up releasing kernels without embedded BTF information.

This is the precise [reason behind the existence of BTFhub](https://www.youtube.com/watch?v=ZYd0lVRwY80). BTFhub's primary function is to supply BTF information for those Linux kernels that were released by distributions without this information embedded. Instead of requiring you to recompile your eBPF code for each existing Linux kernel that lacks BTF support, your code will be relocated by libbpf based on the available BTF information fetched from BTFhub's files.

The libbpf's support for external (raw) BTF files, [which started with this commit](https://github.com/libbpf/libbpf/commit/4920031c8809696debf43f7b0c8f95ea24b8f61c), enables us to supply libbpf with an external BTF file corresponding to the kernel you intend your eBPF code to run on. It's important to note that each kernel requires its unique BTF file.

Please note that BTFhub's use is not universally necessary. If your intent is to support your eBPF CO-RE application solely on the most recent kernels, you will not need BTFhub. However, if you aim to support all released kernels, which include versions from some Long Term Support Linux distributions, then BTFhub may prove to be an indispensable resource.

## Supported Kernels and Distributions

[This is a list](docs/supported-distros.md) of existing distributions and their current status on **eBPF** and **BTF** support.

## How to Use

[Tracee](https://github.com/aquasecurity/tracee/), a runtime security and tracing tool for Linux, serves as a leading example of effective utilization of BTFhub. Tracee incorporates a [script](https://github.com/aquasecurity/tracee/blob/6076457ebb95432da3104f358cb9a29a1d8416c4/3rdparty/btfhub.sh#L107-L108) that downloads the contents of both the [BTFhub](https://github.com/aquasecurity/btfhub) and [BTFhub Archive](https://github.com/aquasecurity/btfhub-archive) repositories. 

This script then uses the BTFhub scripts to [generate tailored BTF files](docs/generating-tailored-btfs.md) that are exceptionally small in size. The result is a streamlined integration process and an efficient method for handling BTF files, demonstrating the power of BTFhub and its scripts when used effectively.

## More Information

- [How to use Pahole to generate BTF information](https://github.com/aquasecurity/btfhub/blob/main/docs/how-to-use-pahole.md)
- [BTFGen added to bpftool, how libbpf does relocations](https://github.com/aquasecurity/btfhub/blob/main/docs/btfgen-internals.md)
