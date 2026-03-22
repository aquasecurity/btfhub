#!/bin/bash
#
# Package new outputs for upload: non-empty *.btf.tar.xz, and
# *.hasbtf / *.failed not in the prior path list.
#
# Usage: ci-archive-collect-new.sh LIST_FILE ARCHIVE_DIR OUT_DIR
#

set -euo pipefail

list_file="${1:?usage: ci-archive-collect-new.sh LIST_FILE ARCHIVE_DIR OUT_DIR}"
archive_dir="${2:?usage: ci-archive-collect-new.sh LIST_FILE ARCHIVE_DIR OUT_DIR}"
out_dir="${3:?usage: ci-archive-collect-new.sh LIST_FILE ARCHIVE_DIR OUT_DIR}"

rm -rf "${out_dir}"
mkdir -p "${out_dir}"

count=0
while IFS= read -r -d '' f; do
    rel="${f#"${archive_dir}/"}"
    rel="${rel#./}"
    case "${rel}" in
        *.btf.tar.xz)
            [[ -s "${f}" ]] || continue
            ;;
        *.hasbtf | *.failed)
            if [[ -f "${list_file}" ]] && grep -Fxq "${rel}" "${list_file}" 2> /dev/null; then
                continue
            fi
            ;;
        *)
            continue
            ;;
    esac
    mkdir -p "${out_dir}/$(dirname "${rel}")"
    cp -a "${f}" "${out_dir}/${rel}"
    ((count++))
done < <(find "${archive_dir}" -type f -print0 2> /dev/null || true)

if ((count == 0)); then
    echo "1" > "${out_dir}/.skip-push"
    echo "No new BTF artifacts; created .skip-push"
else
    echo "Collected ${count} new file(s) under ${out_dir}"
fi
