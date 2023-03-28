## Supported Kernels and Distribution Versions

This is a list of existing distributions and their current status on **eBPF**
and **BTF** supportability over their kernel versions.

> It is **highly recommended** that you update to your latest distribution's
> kernel version in order to use eBPF latest available features.

In the tables bellow you will find BPF, BTF and HUB columns:

* BPF - kernel has support for BPF programs (with or without embedded BTF info)
* BTF - kernel is compiled with DEBUG_INFO_BTF (/sys/kernel/btf/vmlinux avail)
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

### [Alma](https://en.wikipedia.org/wiki/AlmaLinux)

| Alma     | RHEL | Release Date | RHEL Date  | Kernel      | BPF | BTF | HUB |
|----------|------|--------------|------------|-------------|-----|-----|-----|
| 8.3      | 8.3  | 2021-03-30   | 2020-11-03 | 4.18.0-240  |  Y  |  Y  |  -  |
| 8.4      | 8.4  | 2021-05-26   | 2021-05-18 | 4.18.0-305  |  Y  |  Y  |  -  |
| ...      | ...  | ...          | ...        | ...         |  Y  |  Y  |  -  |

> **Note**: **ALL** Alma releases have BPF & BTF support enabled!

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

### [Ubuntu](https://en.wikipedia.org/wiki/Ubuntu_version_history)

| Ubuntu Ver | Num     | Release Date | Kernel  | BPF | BTF | HUB |
|------------|---------|--------------|---------|-----|-----|-----|
| Xenial     | 16.04.2 | 2016-04-21   | 4.4.0   |  Y  |  -  |  Y  |
| Xenial HWE | -       | -            | 4.15.0  |  Y  |  -  |  Y  |
| Bionic     | 18.04.2 | 2018-04-26   | 4.15.0  |  Y  |  -  |  Y  |
| Bionic     | -       | -            | 4.18.0  |  Y  |  -  |  Y  |
| Bionic HWE | -       | -            | 5.4.0   |  Y  |  -  |  Y  |
| Focal      | 20.04.2 | 2020-04-23   | 5.4.0   |  Y  |  -  |  Y  |
| Focal HWE  | -       | -            | 5.8.0   |  Y  |  -  |  Y  |
| Groovy     | 20.10   | 2020-10-22   | 5.8.0   |  Y  |  Y  |  -  |
| Groovy HWE | 20.10   | -            | 5.11.0  |  Y  |  Y  |  -  |
| Hirsute    | 21.04   | 2021-04-22   | 5.11.0  |  Y  |  Y  |  -  |
| ...        | ...     | ...          | ...     |  Y  |  Y  |  -  |

> **Notes**: Bionic HWE, Focal and Focal HWE kernels need this HUB. All other
> future Ubuntu releases will have BPF & BTF support enabled.

### [Oracle Linux](https://en.wikipedia.org/wiki/Oracle_Linux#Software_updates_and_version_history)

| Oracle | Release Date | RH Kernel   | UEK Kernel          | BPF | BTF | HUB |
|--------|--------------|-------------|---------------------|-----|-----|-----|
| 7.8    | 2020-04-08   | 3.10.0-1127 | 4.14.35-1902.300.11 |  Y  |  -  |  Y  |
| 7.9    | 2020-10-07   | 3.10.0-1160 | 5.4.17-2011.6.2     |  Y  |  -  |  Y  |
| 8.2    | 2020-05-06   | 4.18.0-193  | 5.4.17-2011.1.2     |  Y  |  -  |  Y  |
| 8.3    | 2020-11-13   | 4.18.0-240  | 5.4.17-2011.7.4     |  Y  |  -  |  Y  |
| 8.4    | 2021-05-26   | 4.18.0-305  | 5.4.17-2102.201.3   |  Y  |  -  |  Y  |
| ...    | ...          | ...         | ...                 |  Y  |  Y  |  -  |


### [Debian](https://en.wikipedia.org/wiki/Debian_version_history#Release_table)

| Debian        | Release Date | Kernel  | BPF | BTF | HUB |
|---------------|--------------|---------|-----|-----|-----|
| 9 (Stretch)   | 2017-06-17   | 4.9.0   |  Y  |  -  |  Y  |
| 10 (Buster)   | 2019-07-06   | 4.19.0  |  Y  |  -  |  Y  |
| 11 (Bullseye) | 2021-08-14   | 5.10.0  |  Y  |  Y  |  -  |

