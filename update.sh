#!/bin/bash

##
## This script IS SUPPOSED to be a big monolithic script.
## Thats it: The tree should focus in arranging BTF data.
##

## Syntax: $0 [bionic|focal|centos{7,8}|fedora{29,30,31,32}]

basedir=$(dirname $0)
if [ "$basedir" == "." ]; then
	basedir=$(pwd)
fi

##
## HELPER FUNCTIONS
##

exiterr() {
	echo "ERROR: $@"
	exit 1
}

warn() {
	echo "WARN: $@"
}

info() {
	echo "INFO: $@"
}

###
### 1. UBUNTU
###

### bionic (5.4 hwe kernels)

[[ "$1" == "bionic" ]] && {

origdir=$(pwd)
repository="http://ddebs.ubuntu.com"
regex="linux-image-(4|5).(15|4).0-.*-(generic|aws)-dbgsym"

mkdir -p $basedir/ubuntu/bionic
cd $basedir/ubuntu/bionic/x86_64 || exiterr "no bionic dir found"

wget http://ddebs.ubuntu.com/dists/bionic/main/binary-amd64/Packages -O bionic
wget http://ddebs.ubuntu.com/dists/bionic-updates/main/binary-amd64/Packages -O bionic-updates

[ ! -f bionic ] && exiterr "no bionic packages file found"
[ ! -f bionic-updates ] && exiterr "no bionic-updates packages file found"

cat bionic | grep -E '^(Package|Filename):' | grep --no-group-separator -A1 -E "^Package: $regex" > temp
cat bionic-updates | grep -E '^(Package|Filename):' | grep --no-group-separator -A1 -E "Package: $regex" >> temp
rm bionic; rm bionic-updates; mv temp packages

# for kernel debug symbols packages in ubuntu bionic repository
for package in $(cat packages | grep Package: | sed 's:Package\: ::g' | sort)
do

	# get filename and url to download file from
	filepath=$(cat packages | grep -A1 $package | grep -v "^Package: " | sed 's:Filename\: ::g')
	url=$(echo $repository/$filepath)
	filename=$(basename $filepath)
	version=$(echo $filename | cut -d'-' -f3,4,5)

	echo URL: $url
	echo FILEPATH: $filepath
	echo FILENAME: $filename
	echo VERSION: $version

	# do not download dbg package again (if BTF file already exists)
	if [ -f $version.btf.tar.xz ] || [ -f ./nobpf/$version.btf.tar.xz ]
	then
		info "file $version.btf already exists"
		continue
	fi

	# accelerate download if possible
	if [ ! -f $version.ddeb ]; then
		axel -4 -n 8 $url
		mv $filename $version.ddeb
		if [ ! -f $version.ddeb ]
		then
			warn "$version.ddeb could not be downloaded"
			continue
		fi
	fi

	# extract vmlinux file from ddeb package
	dpkg --fsys-tarfile $version.ddeb | tar xvf - ./usr/lib/debug/boot/vmlinux-$version
	mv ./usr/lib/debug/boot/vmlinux-$version ./$version.vmlinux
	rm -rf ${basedir}/ubuntu/bionic/x86_64/usr

	# generate BTF raw file from DWARF data
	pahole -j $version.btf $version.vmlinux
	# pahole $version.btf > $version.txt
	tar cvfJ ./$version.btf.tar.xz $version.btf

	rm $version.ddeb
	rm $version.btf
	# rm $version.txt
	rm $version.vmlinux

done

rm -f $basedir/ubuntu/bionic/x86_64/packages
cd $origdir >/dev/null

}

### focal (5.4 and 5.8 hwe kernels)

[[ "$1" == "focal" ]] && {

origdir=$(pwd)
repository="http://ddebs.ubuntu.com"
regex="linux-image-5\.(4|8|11)\..*-(generic|aws)-dbgsym"

mkdir -p $basedir/ubuntu/focal
cd $basedir/ubuntu/focal/x86_64 || exiterr "no focal dir found"

wget http://ddebs.ubuntu.com/dists/focal/main/binary-amd64/Packages -O focal
wget http://ddebs.ubuntu.com/dists/focal-updates/main/binary-amd64/Packages -O focal-updates

[ ! -f focal ] && exiterr "no focal packages file found"
[ ! -f focal-updates ] && exiterr "no focal-updates packages file found"

cat focal | grep -E '^(Package|Filename):' | grep --no-group-separator -A1 -E "^Package: $regex" > temp
cat focal-updates | grep -E '^(Package|Filename):' | grep --no-group-separator -A1 -E "Package: $regex" >> temp
rm focal; rm focal-updates; mv temp packages

# for kernel debug symbols packages in ubuntu focal repository
for package in $(cat packages | grep Package: | sed 's:Package\: ::g' | sort)
do

	# get filename and url to download file from
	filepath=$(cat packages | grep -A1 $package | grep -v "^Package: " | sed 's:Filename\: ::g')
	url=$(echo $repository/$filepath)
	filename=$(basename $filepath)
	version=$(echo $filename | sed 's:linux-image-\(.*\)-dbgsym.*:\1:g')

	echo URL: $url
	echo FILEPATH: $filepath
	echo FILENAME: $filename
	echo VERSION: $version

	# do not download dbg package again (if BTF file already exists)
	if [ -f $version.btf.tar.xz ] || [ -f ./nobpf/$version.btf.tar.xz ]
	then
		info "file $version.btf already exists"
		continue
	fi

	# accelerate download if possible
	if [ ! -f $version.ddeb ]; then
		axel -4 -n 8 $url
		mv $filename $version.ddeb
		if [ ! -f $version.ddeb ]
		then
			warn "$version.ddeb could not be downloaded"
			continue
		fi
	fi

	# extract vmlinux file from ddeb package
	dpkg --fsys-tarfile $version.ddeb | tar xvf - ./usr/lib/debug/boot/vmlinux-$version
	mv ./usr/lib/debug/boot/vmlinux-$version ./$version.vmlinux
	rm -rf ${basedir}/ubuntu/focal/x86_64/usr

	# generate BTF raw file from DWARF data
	pahole -j $version.btf $version.vmlinux
	# pahole $version.btf > $version.txt
	tar cvfJ ./$version.btf.tar.xz $version.btf

	rm $version.ddeb
	rm $version.btf
	# rm $version.txt
	rm $version.vmlinux

done

rm -f $basedir/ubuntu/focal/x86_64/packages
cd $origdir >/dev/null

}

###
### 2. CENTOS
###

### centos7 and centos8

[[ "$1" == centos* ]] && {

centosrel=$1
centosver=${1/centos/}
origdir=$(pwd)

case $centosver in

7)
  # end of life version
  repository="http://mirror.facebook.net/centos-debuginfo/7/x86_64/"
  ;;
8)
  # current version
  repository="http://mirror.facebook.net/centos-debuginfo/8/x86_64/Packages/"
  ;;
*)
  exiterr "only centos7 and centos8 are supported"
esac

regex="kernel-debuginfo-[0-9].*x86_64.rpm"

mkdir -p $basedir/centos/$centosver
cd $basedir/centos/$centosver/x86_64 || exiterr "no $centosver dir found"

info "downloading $repository information"
lynx -dump -listonly $repository | tail -n+4 > $centosrel
[[ ! -f $centosrel ]] && exiterr "no $centosrel packages file found"
cat $centosrel | grep -E $regex | awk '{print $2}' > temp
mv temp packages
rm $centosrel

# for kernel debug symbols packages in $centosrel repository
for line in $(cat packages | sort)
do
	# get filename and url to download file from
	url=$line
	dirname=$(dirname $line)
	filename=$(basename $line)
	version=$(echo $filename | sed 's:kernel-debuginfo-\(.*\).rpm:\1:g')

	echo URL: $url
	echo FILENAME: $filename
	echo VERSION: $version

	# do not download dbg package again (if BTF file already exists)
	if [ -f $version.btf.tar.xz ] || [ -f ./nobpf/$version.btf.tar.xz ]
	then
		info "file $version.btf already exists"
		continue
	fi

	# accelerate download if possible
	axel -4 -n 8 $url
	mv $filename $version.rpm
	if [ ! -f $version.rpm ]
	then
		warn "$version.rpm could not be downloaded"
		continue
	fi

	# extract vmlinux file from rpm package
	vmlinux=".$(rpmquery -qlp $version.rpm 2>&1 | grep vmlinux)"
	echo "INFO: extracting vmlinux from: $version.rpm"
	rpm2cpio $version.rpm | cpio --to-stdout -i $vmlinux > ./$version.vmlinux

	# generate BTF raw file from DWARF data
	echo "INFO: generating BTF file: $version.btf"
	pahole -j $version.btf $version.vmlinux
	# pahole $version.btf > $version.txt
	tar cvfJ ./$version.btf.tar.xz $version.btf

	rm $version.rpm
	rm $version.btf
	# rm $version.txt
	rm $version.vmlinux
done

rm -f $basedir/centos/$centosver/x86_64/packages
cd $origdir >/dev/null

}

###
### 3. Fedora
###

### fedora29-34

[[ "$1" == fedora* ]] && {

fedorarel=$1
fedoraver=${1/fedora/}
origdir=$(pwd)

case $fedoraver in

29 | 30 | 31)
  # end of life versions
  repository01="https://archives.fedoraproject.org/pub/archive/fedora/linux/releases/$fedoraver/Everything/x86_64/debug/tree/Packages/k/"
  repository02="https://archives.fedoraproject.org/pub/archive/fedora/linux/updates/$fedoraver/Everything/x86_64/debug/Packages/k/"
  ;;
32 | 33 | 34)
  repository01="https://dl.fedoraproject.org/pub/fedora/linux/releases/$fedoraver/Everything/x86_64/debug/tree/Packages/k/"
  repository02="https://dl.fedoraproject.org/pub/fedora/linux/releases/$fedoraver/Everything/x86_64/debug/tree/Packages/k/"
  ;;


*)
  exiterr "only fedora29-34 are supported"
esac

regex="kernel-debuginfo-[0-9].*x86_64.rpm"

mkdir -p $basedir/fedora/$fedoraver
cd $basedir/fedora/$fedoraver/x86_64 || exiterr "no $fedoraver dir found"

info "downloading $repository01 information"
lynx -dump -listonly $repository01 | tail -n+4 > $fedorarel
info "downloading $repository02 information"
lynx -dump -listonly $repository02 | tail -n+4 >> $fedorarel
[[ ! -f $fedorarel ]] && exiterr "no $fedorarel packages file found"
cat $fedorarel | grep -E $regex | awk '{print $2}' > temp
mv temp packages
rm $fedorarel

# for kernel debug symbols packages in $fedorarel repository
for line in $(cat packages | sort)
do
	# get filename and url to download file from
	url=$line
	dirname=$(dirname $line)
	filename=$(basename $line)
	version=$(echo $filename | sed 's:kernel-debuginfo-\(.*\).rpm:\1:g')

	echo URL: $url
	echo FILENAME: $filename
	echo VERSION: $version

	# do not download dbg package again (if BTF file already exists)
	if [ -f $version.btf.tar.xz ] || [ -f ./nobpf/$version.btf.tar.xz ]
	then
		info "file $version.btf already exists"
		continue
	fi

	# accelerate download if possible
	axel -4 -n 8 $url
	mv $filename $version.rpm
	if [ ! -f $version.rpm ]
	then
		warn "$version.rpm could not be downloaded"
		continue
	fi

	# extract vmlinux file from rpm package
	vmlinux=".$(rpmquery -qlp $version.rpm 2>&1 | grep vmlinux)"
	echo "INFO: extracting vmlinux from: $version.rpm"
	rpm2cpio $version.rpm | cpio --to-stdout -i $vmlinux > ./$version.vmlinux

	# generate BTF raw file from DWARF data
	echo "INFO: generating BTF file: $version.btf"
	pahole -j $version.btf $version.vmlinux
	# pahole $version.btf > $version.txt
	tar cvfJ ./$version.btf.tar.xz $version.btf

	rm $version.rpm
	rm $version.btf
	# rm $version.txt
	rm $version.vmlinux
done

rm -f $basedir/fedora/$fedoraver/x86_64/packages
cd $origdir >/dev/null

}

exit 0
