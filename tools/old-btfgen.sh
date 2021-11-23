#!/bin/bash

usage() { echo "Usage: $0 [-a <x86_64|arm64> -o <file.bpf.o>]" 1>&2; exit 1; }

while getopts ":a:o:" opt; do
    case "${opt}" in
        a)
            a=${OPTARG}
            [[ "${a}" != "x86_64" && "${a}" != "arm64" ]] && usage
            ;;
	o)
	    o=${OPTARG}
	    [[ ! -f ${o} ]] && { echo "error: could not find bpf object: ${o}"; usage; }
	    ;;
        *)
            usage
            ;;
    esac
done
shift $((OPTIND-1))

if [ -z "${a}" ] || [ -z "${o}" ]; then
    usage
fi

basedir=$(dirname ${0})/..
if [ "${basedir}" == "." ]; then
	basedir=$(pwd)/..
fi

if [ ! -d ${basedir}/archive ]; then
	echo "error: could not find archive directory"
	exit 1
fi

cd ${basedir}

btfgen=./tools/bin/btfgen.$(uname -m)
if [ ! -x "${btfgen}" ]; then
	echo "error: could not find btfgen tool"
	exit 1
fi

for dir in $(find ./archive/ -iregex ".*${a}.*" -type d | sed 's:\.\/archive\/::g'| sort -u); do
	echo $dir
	mkdir -p custom-archive/${dir}
	ls ./archive/${dir}/*.btf > /dev/null 2>&1 || continue
	${btfgen} -p -i ./archive/${dir} -o ./custom-archive/${dir} --object ${o}
done
