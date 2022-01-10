.PHONY: gather update test

gather:
	rm -rf archive
	mkdir archive
	rsync -av ./btfhub-archive-repo/ archive/ --exclude=.git

update:
	for distro in \
		fedora29 \
		fedora30 \
		fedora31 \
		fedora32 \
		fedora33 \
		fedora34 \
		centos7 \
		centos8 \
		bionic \
		focal \
		stretch \
		buster \
		bullseye \
		amazon2; \
	do \
		./tools/update.sh $$distro; \
	done
	rsync -av ./archive/ btfhub-archive-repo --exclude=.gitignore

test:
	bats test/update.bats
	bats test/btfgen.bats
