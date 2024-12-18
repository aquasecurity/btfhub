#!/bin/sh

die() {
    echo "${@}"
    exit 1
}

if [ -z "$ARCH" ]; then
  ARCH="*"
else
    case ${ARCH} in
    "x86_64")
        ARCH="x86_64"
        ;;
    "aarch64"|"arm64")
        ARCH="arm64"
        ;;
    *)
        die "unsupported architecture: ${ARCH}"
        ;;
    esac
fi

if [ -z "$DISTRO" ]; then
  DISTRO="*"
fi

if [ -z "$VERSION" ]; then
  VERSION="*"
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

# initialize sparse-checkout
git -C "$ARCHIVE_DIR" sparse-checkout init --no-cone

# write patterns for the files we want to pull
echo "$DISTRO/$VERSION/$ARCH/*.btf.tar.xz" > "$ARCHIVE_DIR/.git/info/sparse-checkout"
git -C "$ARCHIVE_DIR" sparse-checkout reapply || die "failed to reapply sparse-checkout"

echo "Sparse checkout completed successfully in $ARCHIVE_DIR"
