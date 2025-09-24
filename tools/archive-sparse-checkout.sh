#!/bin/bash

die() {
    echo "${@}"
    exit 1
}

show_usage() {
    cat << EOF
Usage: $0 [--incremental|-i] [options]

Configures sparse checkout for the btfhub-archive repository.

Options:
  --incremental, -i    Append to existing sparse-checkout patterns instead of replacing them
                       This allows multiple calls to incrementally add more patterns
  --exclude, -e        Treat DISTRO, ARCH, and VERSION as exclusion patterns instead of inclusion
  --no-apply, -n       Skip applying sparse-checkout (defer download until final call)
                       Useful for building patterns incrementally without downloading after each call
  --apply-only, -a     Only apply existing patterns without adding new ones
                       Use this for the final call to trigger download after building patterns

Environment Variables:
  DISTRO               Space-separated list of distributions (default: "*")
  ARCH                 Space-separated list of architectures (default: "*")
  VERSION              Version pattern (default: "*")
  EXCLUDE_PATTERNS     Additional custom exclude patterns (optional, only used with --exclude)

Examples:
  # Clear existing patterns and set new ones (default behavior)
  $0

  # Append patterns to existing sparse checkout
  $0 --incremental

  # Set specific distro and arch, clearing existing patterns
  DISTRO="ubuntu" ARCH="x86_64" $0

  # Add additional distro to existing patterns
  DISTRO="centos" ARCH="arm64" $0 --incremental

  # Exclude specific distro (first include everything, then exclude)
  $0  # include everything
  DISTRO="centos" $0 --exclude --incremental

  # Exclude specific version pattern
  VERSION="5.4.0*" $0 --exclude --incremental

  # Include ubuntu x86_64, then exclude debug versions
  DISTRO="ubuntu" ARCH="x86_64" $0
  VERSION="*debug*" $0 --exclude --incremental

  # Custom exclude patterns
  EXCLUDE_PATTERNS="*/debug/* */test/*" $0 --exclude --incremental

  # Build patterns incrementally without downloading until the end
  $0 --no-apply  # include everything without download
  EXCLUDE_PATTERNS="*/*/*/3.*.btf.tar.xz */*/*/4.*.btf.tar.xz" $0 --exclude --incremental --no-apply
  DISTRO="centos" VERSION="7" $0 --exclude --incremental --no-apply
  DISTRO="rhel" VERSION="7" $0 --exclude --incremental --no-apply
  # Final call applies all patterns and downloads
  $0 --apply-only
EOF
}

# Check for help flag
if [ "$1" = "--help" ] || [ "$1" = "-h" ]; then
    show_usage
    exit 0
fi

# Parse flags
INCREMENTAL=false
EXCLUDE_MODE=false
NO_APPLY=false
APPLY_ONLY=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --incremental|-i)
            INCREMENTAL=true
            shift
            ;;
        --exclude|-e)
            EXCLUDE_MODE=true
            shift
            ;;
        --no-apply|-n)
            NO_APPLY=true
            shift
            ;;
        --apply-only|-a)
            APPLY_ONLY=true
            shift
            ;;
        *)
            # Unknown option, stop parsing
            break
            ;;
    esac
done

if [ -z "$VERSION" ]; then
    VERSION="*"
fi

set -f # disable globbing
DISTROS=${DISTRO:-"*"}
ARCHS=${ARCH:-"*"}
EXCLUDE_CUSTOM=${EXCLUDE_PATTERNS:-""}

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
IFS=' ' read -r -a EXCLUDE_CUSTOM_ARRAY <<< "$EXCLUDE_CUSTOM"

# count the number of elements
NUM_DISTROS=${#DISTROS_ARRAY[@]}
NUM_ARCHS=${#ARCHS_ARRAY[@]}
NUM_EXCLUDE_CUSTOM=${#EXCLUDE_CUSTOM_ARRAY[@]}

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
    # clone into the directory with minimal data
    git clone --filter=blob:none --no-checkout --single-branch "$REPO_URL" "$ARCHIVE_DIR"
    # Enable sparse-checkout
    git -C "$ARCHIVE_DIR" config core.sparseCheckout true
    # Set up sparse-checkout immediately to avoid downloading everything
    git -C "$ARCHIVE_DIR" sparse-checkout init --no-cone
fi

SPARSE_CHECKOUT_FILE="$ARCHIVE_DIR/.git/info/sparse-checkout"

# Clear existing file only if not in incremental mode AND not in apply-only mode
if [ "$INCREMENTAL" = false ] && [ "$APPLY_ONLY" = false ]; then
    true > "$SPARSE_CHECKOUT_FILE" # clear existing file
    echo "Clearing existing sparse-checkout patterns..."
elif [ "$APPLY_ONLY" = false ]; then
    echo "Appending to existing sparse-checkout patterns..."
fi

# configure sparse-checkout for exclude patterns
git -C "$ARCHIVE_DIR" config core.sparseCheckout true
git -C "$ARCHIVE_DIR" config core.sparseCheckoutCone false

if [ "$APPLY_ONLY" = true ]; then
    echo "Applying existing sparse-checkout patterns..."
else
    if [ "$EXCLUDE_MODE" = true ]; then
        echo "Adding exclude patterns to $SPARSE_CHECKOUT_FILE file..."
    else
        echo "Adding include patterns to $SPARSE_CHECKOUT_FILE file..."
    fi
fi

# Only generate patterns if not in apply-only mode
if [ "$APPLY_ONLY" = false ]; then
    # Determine tee flag based on mode
    TEE_FLAG=""
    if [ "$INCREMENTAL" = true ]; then
        TEE_FLAG="-a"
    fi

    # Helper function to add patterns (include or exclude)
    add_pattern() {
        local pattern="$1"
        if [ "$EXCLUDE_MODE" = true ]; then
            pattern="!$pattern"
        fi
        echo "$pattern" | tee $TEE_FLAG "$SPARSE_CHECKOUT_FILE"
        TEE_FLAG="-a"  # After first iteration, always append
    }

    # Generate patterns based on DISTRO, ARCH, and VERSION
    # Skip pattern generation in exclude mode if all values are wildcards (default state)
    if [ "$EXCLUDE_MODE" = true ] && [ "${DISTROS_ARRAY[0]}" = "*" ] && [ "${ARCHS_ARRAY[0]}" = "*" ] && [ "$VERSION" = "*" ]; then
        echo "Skipping wildcard pattern generation in exclude mode (use EXCLUDE_PATTERNS for custom patterns)"
    else
        if [ "$NUM_DISTROS" -eq 1 ] && [ "$NUM_ARCHS" -eq 1 ]; then
            add_pattern "${DISTROS_ARRAY[0]}/$VERSION/${ARCHS_ARRAY[0]}/*.btf.tar.xz"
        elif [ "$NUM_DISTROS" -gt 1 ] && [ "$NUM_ARCHS" -eq 1 ]; then
            for distro in "${DISTROS_ARRAY[@]}"; do
                add_pattern "$distro/$VERSION/${ARCHS_ARRAY[0]}/*.btf.tar.xz"
            done
        elif [ "$NUM_DISTROS" -eq 1 ] && [ "$NUM_ARCHS" -gt 1 ]; then
            for arch in "${ARCHS_ARRAY[@]}"; do
                add_pattern "${DISTROS_ARRAY[0]}/$VERSION/$arch/*.btf.tar.xz"
            done
        else
            for distro in "${DISTROS_ARRAY[@]}"; do
                for arch in "${ARCHS_ARRAY[@]}"; do
                    add_pattern "$distro/$VERSION/$arch/*.btf.tar.xz"
                done
            done
        fi
    fi

    # Add custom exclude patterns (only when in exclude mode)
    if [ "$EXCLUDE_MODE" = true ] && [ "$NUM_EXCLUDE_CUSTOM" -gt 0 ] && [ "${EXCLUDE_CUSTOM_ARRAY[0]}" != "" ]; then
        for custom_pattern in "${EXCLUDE_CUSTOM_ARRAY[@]}"; do
            echo "!$custom_pattern" | tee -a "$SPARSE_CHECKOUT_FILE"
        done
    fi
fi

if [ "$NO_APPLY" = true ]; then
    echo "Patterns updated but not applied (use without --no-apply to download)"
else
    echo "Applying sparse checkout and downloading files..."
    git -C "$ARCHIVE_DIR" sparse-checkout reapply || die "failed to reapply sparse-checkout"
    # Reset and checkout to force download of matching files
    git -C "$ARCHIVE_DIR" reset --hard HEAD 2>/dev/null || echo "Reset completed"
    echo "Sparse checkout completed successfully in $ARCHIVE_DIR"
fi
