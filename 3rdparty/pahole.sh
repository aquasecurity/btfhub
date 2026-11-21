#!/bin/bash -e

# Builds pahole from the 3rdparty/dwarves submodule as a Debian package, and
# installs it. With PAHOLE_DEB_CACHE set to a directory, the built package is
# kept there, and a package already there is installed without building.

export DEBIAN_FRONTEND=noninteractive

cache_dir=""
if [ -n "${PAHOLE_DEB_CACHE}" ]; then
    cache_dir="$(realpath -m "${PAHOLE_DEB_CACHE}")"
fi

if [ -n "${cache_dir}" ] && ls "${cache_dir}"/pahole_*.deb > /dev/null 2>&1; then
    # apt-get, unlike dpkg, also installs the libraries the package needs
    sudo DEBIAN_FRONTEND=noninteractive apt-get -y install "${cache_dir}"/pahole_*.deb
    exit 0
fi

git submodule update --init --recursive 3rdparty/dwarves
rm -f *.deb
cd ./3rdparty/dwarves
rm -rf ./debian/
cp -rfp ../debian.dwarves/ ./debian/
sudo DEBIAN_FRONTEND=noninteractive apt-get -y install devscripts ubuntu-dev-tools
sudo DEBIAN_FRONTEND=noninteractive apt-get -y build-dep .
fakeroot dh binary
rm -rf ./debian/
cd ..
sudo dpkg -i ./pahole*.deb
if [ -n "${cache_dir}" ]; then
    mkdir -p "${cache_dir}"
    cp ./pahole_*.deb "${cache_dir}"/
fi
rm -f *.deb
