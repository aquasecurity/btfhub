#!/bin/bash

die() {
    echo "${@}"
    exit 1
}

if [ -z "$VERSION" ]; then
    VERSION="*"
fi

set -f # disable globbing
DISTROS=${DISTRO:-"*"}
ARCHS=${ARCH:-"*"}

ARCHS="${ARCHS//aarch64/arm64}"
for arch in $ARCHS; do
    case $arch in
        x86_64|arm64|'*')
            ;;
        *)
            die "invalid ARCH: $arch"
            ;;
    esac
done
set +f # enable globbing

IFS=' ' read -r -a DISTROS_ARRAY <<< "$DISTROS"
IFS=' ' read -r -a ARCHS_ARRAY <<< "$ARCHS"

# count the number of elements
NUM_DISTROS=${#DISTROS_ARRAY[@]}
NUM_ARCHS=${#ARCHS_ARRAY[@]}

if [ "$VERSION" != "*" ] && { [ "$NUM_DISTROS" -gt 1 ] || [ "$NUM_ARCHS" -gt 1 ]; }; then
    die "VERSION must be * when DISTRO or ARCH are arrays"
fi

BASE_DIR=$(dirname "$(realpath "$0")")
ARCHIVE_DIR=$(realpath "$BASE_DIR/../archive")
REPO_URL="https://github.com/aquasecurity/btfhub-archive.git"

if [ ! -d "$ARCHIVE_DIR" ]; then
    echo "Directory does not exist. Creating $ARCHIVE_DIR..."
    mkdir -p "$ARCHIVE_DIR"
fi

# if the directory is already a git repository, skip cloning
if [ -d "$ARCHIVE_DIR/.git" ]; then
    echo "Directory is already a Git repository."
else
    # clone into the directory
    git clone --sparse --filter=blob:none "$REPO_URL" "$ARCHIVE_DIR"
fi

SPARSE_CHECKOUT_FILE="$ARCHIVE_DIR/.git/info/sparse-checkout"
true > "$SPARSE_CHECKOUT_FILE" # clear existing file

# initialize sparse-checkout
git -C "$ARCHIVE_DIR" sparse-checkout init --no-cone

echo "Settings patterns to $SPARSE_CHECKOUT_FILE file..."
if [ "$NUM_DISTROS" -eq 1 ] && [ "$NUM_ARCHS" -eq 1 ]; then
    echo "${DISTROS_ARRAY[0]}/$VERSION/${ARCHS_ARRAY[0]}/*.btf.tar.xz" | tee "$SPARSE_CHECKOUT_FILE"
elif [ "$NUM_DISTROS" -gt 1 ] && [ "$NUM_ARCHS" -eq 1 ]; then
    for distro in "${DISTROS_ARRAY[@]}"; do
        echo "$distro/*/${ARCHS_ARRAY[0]}/*.btf.tar.xz" | tee -a "$SPARSE_CHECKOUT_FILE"
    done
elif [ "$NUM_DISTROS" -eq 1 ] && [ "$NUM_ARCHS" -gt 1 ]; then
    for arch in "${ARCHS_ARRAY[@]}"; do
        echo "${DISTROS_ARRAY[0]}/*/$arch/*.btf.tar.xz" | tee -a "$SPARSE_CHECKOUT_FILE"
    done
else
    for distro in "${DISTROS_ARRAY[@]}"; do
        for arch in "${ARCHS_ARRAY[@]}"; do
            echo "$distro/*/$arch/*.btf.tar.xz" | tee -a "$SPARSE_CHECKOUT_FILE"
        done
    done
fi

git -C "$ARCHIVE_DIR" sparse-checkout reapply || die "failed to reapply sparse-checkout"

echo "Sparse checkout completed successfully in $ARCHIVE_DIR"
