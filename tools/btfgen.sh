#!/bin/bash

usage() { echo "Usage: $0 [-a <x86_64|arm64> -o <file01.bpf.o> -o <file02.bpf.o>]" 1>&2; exit 1; }

on=0

while getopts ":a:o:" opt; do
    case "${opt}" in
        a)
            a=${OPTARG}
            [[ "${a}" != "x86_64" && "${a}" != "arm64" ]] && usage
        ;;
        o)
            [[ ! -f ${OPTARG} ]] && { echo "error: could not find bpf object: ${OPTARG}"; usage; }
            o+=("${OPTARG}")
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

obj_cmdline=""
for ofile in "${o[@]}"; do
    obj_cmdline+="${ofile} "
done

basedir=$(dirname ${0})/..
if [ "${basedir}" == "." ]; then
    basedir=$(pwd)/..
fi

if [ ! -d ${basedir}/archive ]; then
    echo "error: could not find archive directory"
    exit 1
fi

cd ${basedir}

btfgen=$(which bpftool)
if [ -z "${btfgen}" ]; then
    btfgen=/usr/sbin/bpftool
fi

if [ ! -x "${btfgen}" ]; then
    echo "error: could not find bpftool (w/ btfgen patch) tool"
    exit 1
fi

function ctrlc ()
{
    echo "Exiting due to ctrl-c..."
    rm ${basedir}/*.btf

    exit 2
}

trap ctrlc SIGINT
trap ctrlc SIGTERM

# clean custom-archive directory
find ./custom-archive -mindepth 1 -maxdepth 1 -type d -exec rm -rf {} \;

for dir in $(find ./archive/ -iregex ".*${a}.*" -type d | sed 's:\.\/archive\/::g'| sort -u); do
    # uncompress and process each existing input BTF .tar.xz file
    for file in $(find ./archive/${dir} -name *.tar.xz); do
        dir=$(dirname $file)
        base=$(basename $file)
        extracted=$(tar xvfJ $dir/$base); ret=$?

        dir=${dir/\.\/archive\/}
        out_dir="./custom-archive/${dir}"
        [[ ! -d ${out_dir} ]] && mkdir -p ${out_dir}

        echo "Processing ${extracted}..."

        # generate one output BTF file to each input BTF file given
        $btfgen gen min_core_btf ${extracted} ${out_dir}/${extracted} ${obj_cmdline}
        [[ $ret -eq 0 ]] && [[ -f ./${extracted} ]] && rm ./${extracted}
    done
done
