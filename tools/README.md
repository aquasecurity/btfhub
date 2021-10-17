## DISCLAIMER

This tool was only created thanks to the effort of:

- Mauricio Vasquez Bernal (Kinvolk/Microsoft)
- Rafael David Tinoco (Aqua Security)
- Itay Shakury (Aqua Security)
- Marga Manterola (Kinvolk/Microsoft)
- Lorenzo Fontana (Elastic)

The code has not been upstreamed yet and it is being developed at:

- https://github.com/kinvolk/libbpf/tree/btfgen
- https://github.com/kinvolk/btfgen

## BTFGEN

TL/DR: BTFHUB is a HUB for BTF files of kernels that don't have embedded BTF information available. Since eBPF CO-RE functionality needs BTF information in order to calculate needed relocations during eBPF program load time, it is imperative that an external BTF file, created from existing DWARF information, exists and is available for those kernels that don't support BTF.

More information can be found at:

- [Linux Plumbers 2021 - Towards Truly Portable eBPF](https://linuxplumbersconf.org/event/11/contributions/948/attachments/906/1776/LPC21_Towards_truly_portable_eBPF.pdf)
- Full presentation: https://youtu.be/igJLKyP1lFk?t=724
- Discussions that led to this tool (with parallel work of Mauricio): https://youtu.be/igJLKyP1lFk?t=2872

### PROBLEM 01: SIZE

Unfortunately BTFHUB got quite big and it would be impossible to embbed all existing BTF files into an application, so it can be packaged/delivered and support all different existing non-BTF enabled kernels out there. BTFGEN tool appeared to solve this problem:

- given a specific BPF object, and multiple BTF files (for different kernels), calculate all needed relocations, to each different kernel, and generate a partial BTF file with only needed BTF information. 

This allows us to shrink the need of having all kernel's BTF files into only partial BTF files, containing only the data the application needs.

### PROBLEM 02: NOT ALL TYPES CHANGES IN BETWEEN DIFF KERNEL VERSIONS

Taking the fact that we have generated partial BTF files, and considering they only contain types our eBPF programs will use, it might be the case that those types don't change very often in between all existing kernels. BTFGEN is also "de-duplicating" those partial BTF files, by creating symlinks of the ones that had no changes. 

### SUMMARY

One can opt to use BTFHUB in different ways:

1. To download an ENTIRE BTF file for a specific kernel version and use it as an external BTF file when loading your app using libbpf (example: https://github.com/aquasecurity/tracee/blob/main/tracee-ebpf/main.go#L152). To each different kernel you will have to download the correspondent BTF file available in BTFHUB.

or

2. To clone BTFHUB and use btfgen to generate all BTF files for the kernels you would like to support for your app. Example:

```
[user@host:~/.../aquasec-btfhub/tools][main]$ ./btfgen.sh ~/aquasec-tracee/tracee-ebpf/dist/tracee.bpf.core.o
```

This will generate multiple BTF files and symlinks at `tools/output/{centos,fedora,ubuntu}/` containing ONLY the types being used by `tracee.bpf.core.o`, with the relocations for each specific kernel version already recalculated. Then you can execute your application loading the correspondent - to your running kernel - partial BTF file generated. Example:

```
$ uname -a
Linux bionic 5.4.0-87-generic #98~18.04.1-Ubuntu SMP Wed Sep 22 10:45:04 UTC 2021 x86_64 x86_64 x86_64 GNU/Linux

$ lsb_release -a
No LSB modules are available.
Distributor ID:	Ubuntu
Description:	Ubuntu 18.04.6 LTS
Release:	18.04
Codename:	bionic

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

