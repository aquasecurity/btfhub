#!/bin/bash -e

git submodule update --init --recursive 3rdparty/bpftool
cd ./3rdparty/bpftool
sudo apt-get -y install build-essential libelf-dev libz-dev libcap-dev binutils-dev pkg-config libelf1
make -C src clean
CC=clang make -C src all
sudo cp ./src/bpftool /usr/sbin/bpftool
make -C src clean
