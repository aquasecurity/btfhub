#!/bin/bash

#
# This script is responsible for generating partial BTF files, specific
# to a given BPF object. This way, instead of relying in a big external
# BTF file, your application can rely in a very minimal external BTF file.
#
# Usage: ./btfgen.sh [path.to.your.bpf.object.file]
#

basedir=$(pwd)
bpftool=$(which bpftool)
pahole=$(which pahole)

if [ "$1" == "" ]; then
	echo "syntax: $0 [file.bpf.o]"
	exit 1
fi

if [ ! -f $1 ]; then
	echo "could not find bpf object"
	exit 1
fi

[ -d ./input ] && rm -rf ./input/*
[ -d ./output ] && rm -rf ./input/*

finalize() {
	echo -n "previous: $previous ($prev_mid) ($prev_prefix), current: $current ($curr_mid) ($curr_prefix) = "

	if [ ! -f $previous ]; then
		echo "$previous does not exist ?"
		return
	fi

	if [ ! -f $current ]; then
		echo "$current does not exist ?"
		return
	fi

	if [ -L $current ]; then
		echo "$current already a link!"
		return
	fi

	# discover structures in sorted order so we can generate same text to compare
	structs_prev=$(pahole -E $previous | grep ^[a-zA-Z\!\@\#\_\-] | sed -e 's:^struct ::g' -e 's:{::g' -e 's: ::g' | sort | xargs | sed 's: :,:g')
	structs_curr=$(pahole -E $current  | grep ^[a-zA-Z\!\@\#\_\-] | sed -e 's:^struct ::g' -e 's:{::g' -e 's: ::g' | sort | xargs | sed 's: :,:g')

	# kernel changed for sure (number of types)
	if [ "$structs_prev" != "$structs_curr" ]; then
		echo "$previous and $current are different for sure!"
		return
	fi

	# sanity check
	if [ "$structs_prev" == "" ] || [ "$structs_curr" == "" ]; then
		echo "structs is empty, something wrong"
		return
	fi

	pahole -E -C $structs_prev $previous > /tmp/previous.btf || echo 0 > /tmp/previous.btf
	pahole -E -C $structs_curr $current > /tmp/current.btf || echo 1 > /tmp/current.btf

	# compare both btf files: previous and current
	diff -y /tmp/previous.btf /tmp/current.btf 2>&1 > /dev/null
	changed=$?
	echo "CHANGED ? $changed"

	# deduplicate: no difference means current can be a link to previous
	# if previous is a link then link to its readlink
	if [ $changed -eq 0 ]; then
		rm $current
		if [ -L $previous ]; then
			symlink=$(readlink $previous)
			ln -s $symlink $current
		else
			ln -s $previous $current
		fi
	fi
}

#     __  ______    _____   __
#    /  |/  /   |  /  _/ | / /
#   / /|_/ / /| |  / //  |/ /
#  / /  / / ___ |_/ // /|  /
# /_/  /_/_/  |_/___/_/ |_/
#

cd output || die "shit"

# uncompress all btf files from btf hub into output
for dir in $(find ../.. -maxdepth 1 -type d ! -name 'tools' ! -name '.*'); do
	find $dir -type d | sed 's:\.\./\.\./::g' | xargs mkdir -p
	for file in $(find $dir -type f -name '*.tar.xz'); do
		newdir=$(dirname $file | sed 's:\.\./\.\./::g')
		tar xvfJ $file
		mv *.btf $newdir
	done
done

cd $basedir

# move uncompressed btf files into input directory
mv ./output/* ./input/
find ./input/ -name *.btf | xargs dirname | sort -u | sed 's:input:output:g' | xargs mkdir -p

# generate partial btf files into output directory
for dir in $(find ./input/ -name *.btf | xargs dirname | sort -u); do
	./btfgen -i $dir -o ${dir/input/output} --object $1
done

# deduplicate =o) generated BTF files

for dir in $(find ./output/ -name *.btf | xargs dirname | sort -u); do
	previous=""
	current=""
	# centos specific sorting (due to its kernel version numbering)
	if [[ $dir =~ /centos/ ]]; then
		echo == CENTOS: $dir
		for file in $(ls $dir -1 | \
			sed -E 's:([0-9]{2,4})\.el:\1-CHANGE-:g' | \
			sort -V -t'.' -k1 -k2 -k3 | \
			sed -E 's:-CHANGE-:\.el:g'); do

			previous=$current ; prev_prefix=$(echo $previous| cut -d'.' -f1,2,3)
			current=$file ; curr_prefix=$(echo $current | cut -d'.' -f1,2,3)

			[ "$prev_prefix" != "$curr_prefix" ] && continue
			echo "previous: $previous ($prev_prefix), current: $current ($curr_prefix)"
		done

	# fedora does not need deduplication due to low number of BTF files
	elif [[ $dir =~ /fedora/ ]]; then
		continue # no need to deduplicate fedora

	# ubuntu has multiple kernel flavors (-generic, -aws, -azure)
	elif [[ $dir =~ /ubuntu/ ]]; then
		echo -- UBUNTU: $dir
		for file in $(ls -1 $dir | sort -t'-' -k3); do
			previous=$current
			current=$file
			prev_prefix=$(echo $previous| cut -d'.' -f1,2)
			prev_mid=$(echo $previous | cut -d'-' -f3 | sed 's:\.btf::g')
			curr_prefix=$(echo $current | cut -d'.' -f1,2)
			curr_mid=$(echo $current | cut -d'-' -f3 | sed 's:\.btf::g')
			if [ "$prev_prefix" != "$curr_prefix" ] || [ "$prev_mid" != "$curr_mid" ]; then continue; fi
			currdir=$(pwd); cd $dir
			finalize
			cd $currdir
		done
	fi
done

