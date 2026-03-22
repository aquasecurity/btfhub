#!/bin/bash
#
# Create empty placeholder files under archive_dir for every path listed.
# btfhub skips kernels when *.btf.tar.xz exists (any size); markers
# .hasbtf / .failed must match the repo layout.
#
# Usage: ci-archive-placeholders.sh LIST_FILE ARCHIVE_DIR
#

set -euo pipefail

list_file="${1:?usage: ci-archive-placeholders.sh LIST_FILE ARCHIVE_DIR}"
archive_dir="${2:?usage: ci-archive-placeholders.sh LIST_FILE ARCHIVE_DIR}"

if [[ ! -f "${list_file}" ]]; then
    echo "WARN: list file missing, skipping placeholders"
    exit 0
fi

# Longest paths first: otherwise a line like ubuntu/xenial processed before
# ubuntu/xenial/amd64/foo can leave ubuntu/xenial as a regular file and break
# btfhub's MkdirAll(archive/ubuntu/xenial/x86_64).
while IFS= read -r rel || [[ -n "${rel:-}" ]]; do
    [[ -z "${rel}" ]] && continue
    [[ "${rel}" =~ ^# ]] && continue
    mkdir -p "$(dirname "${archive_dir}/${rel}")"
    : > "${archive_dir}/${rel}"
done < <(
    awk 'NF && $0 !~ /^#/ { printf "%09d\t%s\n", length($0), $0 }' "${list_file}" \
        | LC_ALL=C sort -r \
        | cut -f2-
)

echo "Placeholders created under ${archive_dir}"
