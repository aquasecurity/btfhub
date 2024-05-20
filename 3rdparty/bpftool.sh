#!/bin/bash -e

if [ ! -f /etc/os-release ]; then
    echo "Unknown OS"
    exit 1
fi

# shellcheck source=/dev/null
# See SC1091
. /etc/os-release

if [ "$ID" == "alpine" ]; then
    sudo apk add build-base elfutils-dev zlib-dev libcap-dev binutils-dev pkgconf libelf
elif [ "$ID" == "ubuntu" ]; then
    sudo apt-get -y install build-essential libelf-dev libz-dev libcap-dev binutils-dev pkg-config libelf1
else
    echo "Unsupported OS"
    exit 1
fi

git submodule update --init --recursive 3rdparty/bpftool
cd ./3rdparty/bpftool
make -C src clean
CC=clang make -C src all
sudo cp ./src/bpftool /usr/sbin/bpftool
make -C src clean
