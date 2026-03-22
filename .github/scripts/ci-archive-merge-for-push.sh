#!/bin/bash
#
# Copy collected artifact tree into the archive git checkout
# (excludes .skip-push marker).
#
# Usage: ci-archive-merge-for-push.sh ARTIFACT_DIR ARCHIVE_CHECKOUT
#

set -euo pipefail

src="${1:?usage: ci-archive-merge-for-push.sh ARTIFACT_DIR ARCHIVE_CHECKOUT}"
dst="${2:?usage: ci-archive-merge-for-push.sh ARTIFACT_DIR ARCHIVE_CHECKOUT}"

n=0
while IFS= read -r -d '' f; do
    rel="${f#"${src}/"}"
    rel="${rel#./}"
    mkdir -p "$(dirname "${dst}/${rel}")"
    cp -a "${f}" "${dst}/${rel}"
    ((n++))
done < <(find "${src}" -type f ! -name '.skip-push' -print0 2> /dev/null || true)

echo "Merged ${n} file(s) into ${dst}"
