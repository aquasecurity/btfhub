#!/bin/bash
# Emit markdown (stdout) for the BTFHUB_HTTP_CACHE job-summary section.
# Args: optional cache directory; else BTFHUB_HTTP_CACHE; else btfhub/.btfhub-http-cache
set -euo pipefail

dir="${1:-${BTFHUB_HTTP_CACHE:-btfhub/.btfhub-http-cache}}"

echo ""
echo "### HTTP metadata cache (\`BTFHUB_HTTP_CACHE\`)"
echo ""
echo "Layout: **\`<sha256-of-URL>.body\`** (bytes as sent on the wire) + **\`<sha256>.meta\`** (JSON: \`etag\`, \`last_modified\`, \`content_type\`, \`content_encoding\`, \`final_url\`). Used by \`pkg/utils\` conditional GET."
echo ""
if [[ ! -d "${dir}" ]]; then
    echo "*No cache directory at \`${dir}\` (nothing downloaded via this cache yet).*"
    echo ""
    exit 0
fi

n="$(find "${dir}" -type f \( -name '*.body' -o -name '*.meta' \) 2> /dev/null | wc -l | tr -d ' ')"
sz="$(du -sh "${dir}" 2> /dev/null | awk '{print $1}')"
echo "| Stat | Value |"
echo "| --- | --- |"
echo "| Path | \`${dir}\` |"
echo "| Cache files (\`*.body\` / \`*.meta\`) | **${n}** |"
echo "| Size (du -sh) | **${sz:-?}** |"
echo ""
echo "<details><summary>File list (temp \`*.part\` / \`*.tmp\` omitted)</summary>"
echo ""
echo '```text'
mapfile -t files < <(find "${dir}" -maxdepth 1 -type f ! -name '*.part' ! -name '*.tmp' 2> /dev/null | sort)
total="${#files[@]}"
max=400
i=0
for f in "${files[@]}"; do
    if [[ "${i}" -ge "${max}" ]]; then
        echo "... and $((total - max)) more"
        break
    fi
    basename "${f}"
    i=$((i + 1))
done
echo '```'
echo ""
echo "</details>"
echo ""
