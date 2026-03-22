#!/bin/bash
# Run a command inside the Amazon Linux 2023 toolchain image (AL2 debuginfo repos + pahole).
# Examples:
#   ./docker/run-amazon.sh bash
#   ./docker/run-amazon.sh make
#   ./docker/run-amazon.sh ./btfhub -workers 4 -d amzn -r 2
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
IMAGE=${IMAGE:-btfhub-amazon:dev}

if ! docker image inspect "$IMAGE" &> /dev/null; then
    echo "Image $IMAGE not found; building..." >&2
    docker build -f "${ROOT}/docker/Dockerfile.amazon" -t "$IMAGE" "$ROOT"
fi

: "${BTFHUB_HTTP_CACHE:=/btfhub/.btfhub-http-cache}"
: "${BTFHUB_VOLUME_SUFFIX=:z}" # SELinux / Podman on Fedora; clear with BTFHUB_VOLUME_SUFFIX=""

exec docker run --rm -it \
    -v "${ROOT}:/btfhub${BTFHUB_VOLUME_SUFFIX}" \
    -w /btfhub \
    -e BTFHUB_HTTP_CACHE \
    "$IMAGE" "$@"
