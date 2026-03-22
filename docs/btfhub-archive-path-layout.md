# btfhub-archive path layout (vs. btfhub `-r` releases)

BTFHub passes an **APT/YUM release id** into `GetKernelPackages` (e.g. Ubuntu
`focal`, Debian `buster`). Published BTFs in
[btfhub-archive](https://github.com/aquasecurity/btfhub-archive) use **different
middle path segments** in many cases. CI builds placeholders from the GitHub
**blob** tree; symlinks or alternate codename dirs are **not** listed as blobs,
so `btfhub` must write under the same segments the tree uses.

Path stats below come from a **recursive tree listing** (no clone / no blob
download), e.g.:

```bash
python3 .github/scripts/fetch-archive-paths-github.py /tmp/paths.txt \
    ubuntu/ debian/ fedora/ centos/ ol/ amzn/
# then inspect unique $distro/$release/ prefixes
```

## Summary

| Distro  | btfhub `-r` values              | Archive dir segment   | Notes                                      |
|---------|----------------------------------|-----------------------|--------------------------------------------|
| ubuntu  | xenial, bionic, focal            | 16.04, 18.04, 20.04   | Blobs under numeric dirs; codename symlinks|
| debian  | stretch, buster, bullseye        | 9, 10, bullseye       | stretch/buster dirs barely used vs 9/10    |
| fedora  | 24 ... 31                          | same                  | OK                                         |
| centos  | 7, 8                             | same                  | OK                                         |
| ol      | 7, 8                             | same                  | OK                                         |
| amzn    | 1, 2                             | same (+ `2018`, ...)    | AL2 uses `2`; other tags exist in archive  |

Mapping is implemented in `cmd/btfhub/main.go` (`archiveLayoutDir`).
