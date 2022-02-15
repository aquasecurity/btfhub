.PHONY: gather update test

gather:
	rm -rf archive
	mkdir archive
	rsync -av ./btfhub-archive-repo/ archive/ --exclude=.git

#
# The following distributions (and versions):
#
# fedora32
# fedora33
# fedora34
# centos8
# bionic
# focal
# debian11
#
# are, now, releasing kernels with BTF support. This means that BTFHUB does not
# need to keep generating BTF files for them, as it won't be needed by eBPF
# CO-RE objects (as they can rely in /sys/kernel/btf/vmlinux file).
#
# (https://github.com/aquasecurity/btfhub/issues/29)
#
# Note: this means that, more and more, BTFHUB will be used for legacy kernels:
# the kernels that were already released, part of older (but, yet, current)
# distributions versions.
#

update:
	for distro in \
		fedora29 \
		fedora30 \
		fedora31 \
		centos7 \
		stretch \
		buster \
		amazon2; \
	do \
		./tools/update.sh $$distro; \
	done
	rsync -av ./archive/ btfhub-archive-repo --exclude=.gitignore

test:
	bats test/update.bats
	bats test/btfgen.bats
