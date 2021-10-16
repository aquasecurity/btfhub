#!/bin/bash

#
# This script is responsible for generating partial BTF files, specific
# to a given BPF object. This way, instead of relying in a big external
# BTF file, your application can rely in a very minimal external BTF file.
#
# Usage: ./btfgen.sh [path.to.your.bpf.object.file]
#

basedir=$(pwd)

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
