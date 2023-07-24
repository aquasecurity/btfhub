#!/bin/bash -e

git submodule update --init --recursive 3rdparty/dwarves
export DEBIAN_FRONTEND=noninteractive
rm -f *.deb
cd ./3rdparty/dwarves
rm -rf ./debian/
cp -rfp ../debian.dwarves/ ./debian/
sudo apt-get -y install devscripts ubuntu-dev-tools
sudo apt-get -y build-dep .
fakeroot dh binary
rm -rf ./debian/
cd ..
sudo dpkg -i ./pahole*.deb
rm -f *.deb
