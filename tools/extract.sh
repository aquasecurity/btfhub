#!/bin/bash

usage() { echo "Usage: $0 [-a <x86_64|arm64>]" 1>&2; exit 1; }

while getopts ":a:" opt; do
    case "${opt}" in
        a)
            a=${OPTARG}
            [[ "${a}" != "x86_64" && "${a}" != "arm64" ]] && usage
            ;;
        *)
            usage
            ;;
    esac
done
shift $((OPTIND-1))

if [ -z "${a}" ]; then
    usage
fi

basedir=$(dirname $0)/..
if [ "$basedir" == "." ]; then
	basedir=$(pwd)/..
fi

if [ ! -d $basedir/archive ]; then
	echo could not find archive directory
	exit 1
fi

cd $basedir

for file in $(find ./archive/ -type f -iregex ".*${a}.*tar.xz$"); do
	_dir=$(dirname $file)
	_file=$(basename $file)
	cd $_dir
	if [ -f ${_file/\.tar\.xz/} ]; then
		echo $file already uncompressed
		cd - > /dev/null
		continue
	fi
	cp $_file extract.tar.xz
	tar xvfJ extract.tar.xz
	ret=$?
	rm -f extract.tar.xz
	if [ $ret -ne 0 ]; then
		echo file $_file failed to extract
		mv $_file ${_file/\.btf\.tar\.xz/}.failed
	fi
	cd - > /dev/null
done
