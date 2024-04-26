#!/bin/bash

set -e
set -x # for debugging

# This script installs the dependencies for compiling btfhub and running the
# tests.

# It is intended to be run as root. If not, it will re-run itself as root.
if [[ $EUID -ne 0 ]]; then
    exec sudo -E -- "$0" "$@"
fi

ARCH=$(uname -m)

wait_for_apt_locks() {
    local lock="/var/lib/dpkg/lock"
    local lock_frontend="/var/lib/dpkg/lock-frontend"
    local lock_lists="/var/lib/apt/lists/lock"
    local lock_archives="/var/cache/apt/archives/lock"

    local timeout=20
    local elapsed=0
    local wait_interval=2

    echo "Checking for unattended-upgrades..."
    while pgrep -f unattended-upgrades > /dev/null; do
        if (( elapsed >= timeout )); then
            echo "Timed out waiting for unattended-upgrades to finish. Attempting to kill..."
            pkill -SIGQUIT -f unattended-upgrades || true
            pkill -SIGKILL -f unattended-upgrades || true
            break
        fi

        echo "unattended-upgrades is still running. Waiting..."
        sleep $wait_interval
        ((elapsed += wait_interval))
    done

    timeout=5 # reduce timeout for apt locks
    elapsed=0 # reset timer

    while : ; do
        if ! fuser $lock >/dev/null 2>&1 &&
           ! fuser $lock_frontend >/dev/null 2>&1 &&
           ! fuser $lock_lists >/dev/null 2>&1 &&
           ! fuser $lock_archives >/dev/null 2>&1; then
            echo "All apt locks are free."
            break
        fi

        if (( elapsed >= timeout )); then
            echo "Timed out waiting for apt locks to be released. Attempting to kill locking processes."
            fuser -k -SIGQUIT $lock >/dev/null 2>&1 || true
            fuser -k -SIGQUIT $lock_frontend >/dev/null 2>&1 || true
            fuser -k -SIGQUIT $lock_lists >/dev/null 2>&1 || true
            fuser -k -SIGQUIT $lock_archives >/dev/null 2>&1 || true

            # Give some time for processes to terminate gracefully
            sleep 2

            fuser -k -SIGKILL $lock >/dev/null 2>&1 || true
            fuser -k -SIGKILL $lock_frontend >/dev/null 2>&1 || true
            fuser -k -SIGKILL $lock_lists >/dev/null 2>&1 || true
            fuser -k -SIGKILL $lock_archives >/dev/null 2>&1 || true

            # Delete lock files if they still exist
            rm -f $lock $lock_frontend $lock_lists $lock_archives

            echo "Forced removal of processes locking apt. System may be in an inconsistent state."
            break
        fi

        echo "Waiting for other software managers to finish..."
        sleep $wait_interval
        ((elapsed += wait_interval))
    done
}

disable_unattended_upgrades() {
    # This is a pain point. Make sure to always disable anything touching the
    # dpkg database, otherwise it will fail with locking errors.
    systemctl stop unattended-upgrades || true
    systemctl disable --now unattended-upgrades || true

    wait_for_apt_locks
    apt-get remove -y --purge unattended-upgrades || true
    apt-get remove -y --purge ubuntu-advantage-tools || true
}

remove_alternatives() {
    tools=("$@")

    for tool in "${tools[@]}"; do
        update-alternatives --remove-all "$tool" || true
    done
}

#
# LLVM
#

remove_llvm_alternatives() {
    tools=(
        clang
        clang++
        clangd
        clang-format
        llc
        lld
        llvm-strip
        llvm-config
        ld.lld
        llvm-ar
        llvm-nm
        llvm-objcopy
        llvm-objdump
        llvm-readelf
        opt
        cc
    )

    remove_alternatives "${tools[@]}"
}

update_llvm_alternatives() {
    remove_llvm_alternatives

    # Get the major version
    local version
    version=$(echo "${1}" | cut -d. -f1)
    update-alternatives \
        --install /usr/bin/clang clang /usr/bin/clang-"${version}" 0 \
        --slave /usr/bin/clang++ clang++ /usr/bin/clang++-"${version}" \
        --slave /usr/bin/clangd clangd /usr/bin/clangd-"${version}" \
        --slave /usr/bin/clang-format clang-format /usr/bin/clang-format-"${version}" \
        --slave /usr/bin/llc llc /usr/bin/llc-"${version}" \
        --slave /usr/bin/lld lld /usr/bin/lld-"${version}" \
        --slave /usr/bin/llvm-strip llvm-strip /usr/bin/llvm-strip-"${version}" \
        --slave /usr/bin/llvm-config llvm-config /usr/bin/llvm-config-"${version}" \
        --slave /usr/bin/ld.lld ld.lld /usr/bin/ld.lld-"${version}" \
        --slave /usr/bin/llvm-ar llvm-ar /usr/bin/llvm-ar-"${version}" \
        --slave /usr/bin/llvm-nm llvm-nm /usr/bin/llvm-nm-"${version}" \
        --slave /usr/bin/llvm-objcopy llvm-objcopy /usr/bin/llvm-objcopy-"${version}" \
        --slave /usr/bin/llvm-objdump llvm-objdump /usr/bin/llvm-objdump-"${version}" \
        --slave /usr/bin/llvm-readelf llvm-readelf /usr/bin/llvm-readelf-"${version}" \
        --slave /usr/bin/opt opt /usr/bin/opt-"${version}" \
        --slave /usr/bin/cc cc /usr/bin/clang-"${version}"
}

install_llvm_os_packages() {
    # Get the major version
    local version
    version=$(echo "${1}" | cut -d. -f1)

    wait_for_apt_locks
    apt-get install -y \
        llvm-"${version}" \
        clang-"${version}" \
        clangd-"${version}" \
        lld-"${version}"

    update_llvm_alternatives "$version"
}

remove_llvm_os_packages() {
    wait_for_apt_locks
    apt-get remove -y clang-12 clangd-12 lld-12 llvm-12 || true
    apt-get remove -y clang-13 clangd-13 lld-13 llvm-13 || true
    apt-get remove -y clang-14 clangd-14 lld-14 llvm-14 || true
    apt-get --purge autoremove -y
}

remove_llvm_usr_bin_files() {
    rm -f /usr/bin/clang*
    rm -f /usr/bin/clang++*
    rm -f /usr/bin/clangd*
    rm -f /usr/bin/clang-format*

    rm -f /usr/bin/lld*
    rm -f /usr/bin/llc*
    rm -f /usr/bin/llvm-strip*
    rm -f /usr/bin/llvm-config*
    rm -f /usr/bin/ld.lld*
    rm -f /usr/bin/llvm-ar*
    rm -f /usr/bin/llvm-nm*
    rm -f /usr/bin/llvm-objcopy*
    rm -f /usr/bin/llvm-objdump*
    rm -f /usr/bin/llvm-readelf*
    rm -f /usr/bin/opt
    rm -f /usr/bin/cc
}

link_llvm_usr_local_clang() {
    ln -s /usr/local/clang/bin/clang /usr/bin/clang
    ln -s /usr/local/clang/bin/clang++ /usr/bin/clang++
    ln -s /usr/local/clang/bin/clangd /usr/bin/clangd
    ln -s /usr/local/clang/bin/clang-format /usr/bin/clang-format
    ln -s /usr/local/clang/bin/lld /usr/bin/lld
    ln -s /usr/local/clang/bin/llc /usr/bin/llc
    ln -s /usr/local/clang/bin/llvm-strip /usr/bin/llvm-strip
    ln -s /usr/local/clang/bin/llvm-config /usr/bin/llvm-config
    ln -s /usr/local/clang/bin/ld.lld /usr/bin/ld.lld
    ln -s /usr/local/clang/bin/llvm-ar /usr/bin/llvm-ar
    ln -s /usr/local/clang/bin/llvm-nm /usr/bin/llvm-nm
    ln -s /usr/local/clang/bin/llvm-objcopy /usr/bin/llvm-objcopy
    ln -s /usr/local/clang/bin/llvm-objdump /usr/bin/llvm-objdump
    ln -s /usr/local/clang/bin/llvm-readelf /usr/bin/llvm-readelf
    ln -s /usr/local/clang/bin/opt /usr/bin/opt
    ln -s /usr/local/clang/bin/clang /usr/bin/cc
}

install_llvm_from_github() {
    local version=$1

    remove_llvm_os_packages
    remove_llvm_usr_bin_files

    LLVM_URL="https://github.com/llvm/llvm-project/releases/download/llvmorg-${version}/"

    if [[ $ARCH == x86_64 ]]; then
        LLVM_URL=$LLVM_URL"clang+llvm-${version}-x86_64-linux-gnu-rhel-8.4.tar.xz"
    else
        LLVM_URL=$LLVM_URL"clang+llvm-${version}-aarch64-linux-gnu.tar.xz"
    fi

    LLVM_FILE=$(basename "$LLVM_URL")
    LLVM_DIR="${LLVM_FILE%.tar.xz}"

    # Download
    rm -f "/tmp/$LLVM_FILE"
    curl -L -o "/tmp/$LLVM_FILE" "$LLVM_URL"

    # Install
    cd /usr/local
    rm -rf ./clang
    tar xfJ /tmp/"$LLVM_FILE"
    mv "$LLVM_DIR" ./clang
    cd -

    link_llvm_usr_local_clang
}

#
# Golang
#

remove_golang_alternatives() {
    tools=(
        go
        gofmt
    )

    remove_alternatives "${tools[@]}"
}

update_golang_alternatives() {
    remove_golang_alternatives

    update-alternatives \
        --install /usr/bin/go go /usr/local/go/bin/go 0 \
        --slave /usr/bin/gofmt gofmt /usr/local/go/bin/gofmt
}

remove_golang_os_packages() {
    wait_for_apt_locks
    apt-get remove -y golang golang-go
    apt-get --purge autoremove -y
}

remove_golang_usr_bin_files() {
    rm -f /usr/bin/go
    rm -f /usr/bin/gofmt
}

link_golang_usr_local_go() {
    ln -s /usr/local/go/bin/go /usr/bin/go
    ln -s /usr/local/go/bin/gofmt /usr/bin/gofmt
}

install_golang_from_github() {
    remove_golang_os_packages
    remove_golang_usr_bin_files

    local version=$1
    if [[ $ARCH == x86_64 ]]; then
        GO_URL="https://go.dev/dl/go$version.linux-amd64.tar.gz"
    else
        GO_URL="https://go.dev/dl/go$version.linux-arm64.tar.gz"
    fi

    GO_FILE=$(basename "$GO_URL")

    # Download
    rm -f "/tmp/$GO_FILE"
    curl -L -o "/tmp/$GO_FILE" "$GO_URL"

    # Install
    cd /usr/local
    rm -rf ./go
    tar xfz /tmp/"$GO_FILE"
    cd -

    update_golang_alternatives
}

#
# Main
#

FROM="$1"

# pr: when called from the pr action
# cron: when called from the cron job action
if [[ $FROM != pr ]] && [[ $FROM != cron ]]; then
    echo "Usage: $0 [pr|cron]"
    exit 1
fi

# Common dependencies

export DEBIAN_FRONTEND=noninteractive
disable_unattended_upgrades

wait_for_apt_locks
apt-get update

wait_for_apt_locks
apt-get install -y \
    bsdutils build-essential pkgconf \
    zlib1g-dev libelf-dev \
    software-properties-common \
    devscripts ubuntu-dev-tools

# Install dependencies based on the origin

GO_VERSION="1.22.2"
LLVM_VERSION="14.0.6"

if [[ $FROM == pr ]]; then
    # pr dependencies
    install_golang_from_github "$GO_VERSION"
    install_llvm_os_packages "$LLVM_VERSION"
elif [[ $FROM == cron ]]; then
    # cron dependencies
    update_llvm_alternatives "$LLVM_VERSION"
fi
