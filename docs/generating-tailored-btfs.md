## The BTF generator

The reason for [BTFhub to exist](https://www.youtube.com/watch?v=ZYd0lVRwY80)
is to allow [Tracee](https://github.com/aquasecurity/tracee) and other similar
eBPF projects to be able to run in kernels that do not provide BTF information.

Now, there is a second need: relying in full BTF files is expensive as the
entire [BTFHub archive](https://github.com/aquasecurity/btfhub-archive) is big.
Because of that, together with [Kinvolk](https://github.com/kinvolk/) and
[Elastic](https://github.com/elastic), we have created the
[BTFgen](https://github.com/kinvolk/btfgen) tool.

You may use [BTFhub](https://github.com/aquasecurity/btfhub) and
[BTFhub-archive](https://github.com/aquasecurity/btfhub-archive) to generate
smaller BTF files. The tailored BTf files are so small they can be embedded in
an eBPF based application, allowing it to support several hundreds of kernels
by default.

## How to Generate Tailored BTF files

Clone both [BTFhub](https://github.com/aquasecurity/btfhub) and
[BTFhub-Archive](https://github.com/aquasecurity/btfhub-archive) repositories:

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

Enter `btfhub` directory:

```
$ cd btfhub ; ls
archive  custom-archive  docs  example  LICENSE  README.md  tools
```

Bring the cloned archive into the `btfhub` directory:

```
$ rsync -avz ../btfhub-archive/ --exclude=.git* --exclude=README.md ./archive/

sending incremental file list
./
centos/
centos/7/
centos/7/arm64/
centos/7/arm64/3.18.9-200.el7.aarch64.btf.tar.xz
centos/7/arm64/3.19.0-0.80.aa7a.aarch64.btf.tar.xz
centos/7/arm64/4.0.0-0.rc7.git1.1.el7.aarch64.btf.tar.xz
centos/7/arm64/4.0.0-1.el7.aarch64.btf.tar.xz
centos/7/arm64/4.11.0-22.el7.2.aarch64.btf.tar.xz
centos/7/arm64/4.11.0-22.el7a.aarch64.btf.tar.xz
centos/7/arm64/4.11.0-45.4.1.el7a.aarch64.btf.tar.xz
...
ubuntu/20.04/x86_64/5.8.0-55-generic.btf.tar.xz
ubuntu/20.04/x86_64/5.8.0-59-generic.btf.tar.xz
ubuntu/20.04/x86_64/5.8.0-63-generic.btf.tar.xz
```

Generate the tailored, to your eBPF object(s), BTF files:

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

Check tailored newly generated BTF files and their small size:


```
$ find custom-archive | grep ubuntu | tail -10
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

$ ls -lah custom-archive/ubuntu/20.04/x86_64/5.8.0-1041-azure.btf
Permissions Size User          Date Modified Name
.rw-rw-r--  5.6k rafaeldtinoco 22 Nov 22:41  custom-archive/ubuntu/20.04/x86_64/5.8.0-1041-azure.btf
```

At this point you have a `custom-archive` directory full of tailored BTF files.
You may now use EACH tailored BTF file just like the full BTF files, available
at the [BTFhub-Archive](https://github.com/aquasecurity/btfhub-archive), as
showed in the [BTFhub example](../example/).

> **Pay attention**: the generated BTF files are tailored for the specific
> given eBPF object and won't work with another eBPF objects. If you change
> your eBPF source code you will need to re-generate them to make sure
> libbpf will able to use this smaller BTF file (tailored to your needs only).

You may, now, embed those files in your application so, each time you run in a
specific kernel, supported by those files, you load the correspondent BTF file
through libbpf, for example.

