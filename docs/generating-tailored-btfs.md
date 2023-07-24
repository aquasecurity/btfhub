## BTFGEN: The BTF generator

[BTFHUB](https://github.com/aquasecurity/btfhub) was created with the aim of enabling eBPF projects like [Tracee](https://github.com/aquasecurity/tracee) to operate on kernels lacking BTF information. 

However, another challenge soon surfaced: the high cost associated with using complete BTF files, given the considerable size of the [BTFHub archive](https://github.com/aquasecurity/btfhub-archive). To address this issue, we collaborated with [Kinvolk (acquired by Microsoft)](https://github.com/kinvolk/) and [Elastic](https://github.com/elastic) to create the [BTFgen](https://github.com/kinvolk/btfgen) tool.

[Watch a presentation about BTFGEN here.](https://www.youtube.com/watch?v=ugzZpP4y25o)

With BTFgen, users can leverage [BTFhub](https://github.com/aquasecurity/btfhub) and [BTFhub-archive](https://github.com/aquasecurity/btfhub-archive) to generate leaner BTF files. These bespoke BTF files are compact enough to be embedded directly within an eBPF-based application, enabling the application to support hundreds of different kernel versions by default, without the need for comprehensive BTF files. 

The BTFGEN was later [incorporated](https://lore.kernel.org/bpf/20220215225856.671072-1-mauricio@kinvolk.io/) in the [bpftool](https://github.com/libbpf/bpftool) as the [min_core_btf](https://man.archlinux.org/man/bpftool-gen.8.en#bpftool~4) sub-function.

## How to Generate Tailored BTF files

1. Clone both [BTFhub](https://github.com/aquasecurity/btfhub) and [BTFhub-Archive](https://github.com/aquasecurity/btfhub-archive) repositories in the same directory:

    ```
    $ git clone git@github.com:aquasecurity/btfhub.git
    
    Cloning into 'btfhub'...
    remote: Enumerating objects: 46, done.
    remote: Counting objects: 100% (22/22), done.
    remote: Compressing objects: 100% (19/19), done.
    remote: Total 46 (delta 5), reused 13 (delta 2), pack-reused 24
    Receiving objects: 100% (46/46), 5.34 MiB | 5.54 MiB/s, done.
    Resolving deltas: 100% (5/5), done.
    
    $ git clone git@github.com:aquasecurity/btfhub-archive.git
    
    Cloning into 'btfhub-archive'...
    remote: Enumerating objects: 943, done.
    remote: Counting objects: 100% (3/3), done.
    remote: Compressing objects: 100% (3/3), done.
    remote: Total 943 (delta 0), reused 2 (delta 0), pack-reused 940
    Receiving objects: 100% (943/943), 942.97 MiB | 13.47 MiB/s, done.
    Resolving deltas: 100% (5/5), done.
    Updating files: 100% (863/863), done.
    ```

2. Enter `btfhub` directory and bring the cloned arquive into the `btfhub` directory:

    ```
    $ cd btfhub ; ls
    3rdparty  archive  cmd  custom-archive  docs  go.mod  go.sum  LICENSE  Makefile  pkg  README.md  tools
    
    $ make bring
    
    WARNING: this will delete all the files in ./archive, press enter to continue ...
    
    sending incremental file list
    ./
    LICENSE
    README.md
    amzn/
    amzn/1 -> 2018
    amzn/2/
    amzn/2/arm64/
    ...
    ```

3. Generate the tailored, to your eBPF object(s), BTF files:

    ```
    $ ./tools/btfgen.sh -a x86_64 -o $HOME/tracee.bpf.core.o
    ...
    OBJ : /home/rafaeldtinoco/tracee.bpf.core.o
    DBTF: ./custom-archive/ubuntu/20.04/x86_64/5.4.0-1047-azure.btf
    SBTF: ./5.4.0-73-generic.btf
    OBJ : /home/rafaeldtinoco/tracee.bpf.core.o
    DBTF: ./custom-archive/ubuntu/20.04/x86_64/5.4.0-73-generic.btf
    SBTF: ./5.11.0-1014-aws.btf
    OBJ : /home/rafaeldtinoco/tracee.bpf.core.o
    DBTF: ./custom-archive/ubuntu/20.04/x86_64/5.11.0-1014-aws.btf
    SBTF: ./5.8.0-1040-azure.btf
    OBJ : /home/rafaeldtinoco/tracee.bpf.core.o
    DBTF: ./custom-archive/ubuntu/20.04/x86_64/5.8.0-1040-azure.btf
    SBTF: ./5.4.0-1025-aws.btf
    OBJ : /home/rafaeldtinoco/tracee.bpf.core.o
    DBTF: ./custom-archive/ubuntu/20.04/x86_64/5.4.0-1025-aws.btf
    ```

4. Check tailored newly generated BTF files and their small size:

    ```
    $ find custom-archive
    ...
    custom-archive/ubuntu/20.04/x86_64/5.4.0-1036-azure.btf
    custom-archive/ubuntu/20.04/x86_64/5.4.0-1026-azure.btf
    custom-archive/ubuntu/20.04/x86_64/5.8.0-49-generic.btf
    custom-archive/ubuntu/20.04/x86_64/5.8.0-1035-aws.btf
    custom-archive/ubuntu/20.04/x86_64/5.4.0-1057-aws.btf
    custom-archive/ubuntu/20.04/x86_64/5.4.0-1043-aws.btf
    custom-archive/ubuntu/20.04/x86_64/5.4.0-1018-aws.btf
    custom-archive/ubuntu/20.04/x86_64/5.4.0-64-generic.btf
    custom-archive/ubuntu/20.04/x86_64/5.8.0-28-generic.btf
    custom-archive/ubuntu/20.04/x86_64/5.8.0-1041-azure.btf
    ...
  
    $ ls -lah custom-archive/ubuntu/20.04/x86_64/5.8.0-1041-azure.btf
    Permissions Size User          Date Modified Name
    .rw-rw-r--  5.6k rafaeldtinoco 22 Nov 22:41  custom-archive/ubuntu/20.04/x86_64/5.8.0-1041-azure.btf
    ```

After the execution of the previous steps, you should possess a `custom-archive` directory brimming with customized BTF files. Each of these tailored files can now be utilized in the same manner as the comprehensive BTF files that are readily available at the [BTFhub-Archive](https://github.com/aquasecurity/btfhub-archive). In other words, these compact, tailored BTF files provide the same functionality and usability as their larger, full BTF counterparts.

> **Note**: The created BTF files are specifically tailored to the given eBPF object and are incompatible with other eBPF objects. If alterations are made to your eBPF source code, it necessitates the re-generation of these files to ensure libbpf's ability to use this smaller, customized BTF file that is solely tailored to suit your needs.

At this stage, it's feasible to incorporate these files into your application. Consequently, whenever your application runs on a specific kernel that's supported by these files, the corresponding BTF file will be loaded through libbpf, among other potential methods.
