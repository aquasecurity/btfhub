#!/bin/bash
# Run a command inside the distros (Ubuntu) toolchain image with this repo mounted at /btfhub.
# Examples:
#   ./docker/run-distros.sh bash
#   ./docker/run-distros.sh make
#   ./docker/run-distros.sh ./btfhub -workers 4 -d ubuntu -r focal
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
IMAGE=${IMAGE:-btfhub-distros:dev}

if ! docker image inspect "$IMAGE" &> /dev/null; then
    echo "Image $IMAGE not found; building..." >&2
    docker build -f "${ROOT}/docker/Dockerfile.distros" -t "$IMAGE" "$ROOT"
fi

: "${BTFHUB_HTTP_CACHE:=/btfhub/.btfhub-http-cache}"
# :z = SELinux shared label on the bind mount (Fedora/Podman: fixes "Permission denied" on /btfhub).
# Override with BTFHUB_VOLUME_SUFFIX="" if your runtime rejects the suffix.
: "${BTFHUB_VOLUME_SUFFIX=:z}"

exec docker run --rm -it \
    -v "${ROOT}:/btfhub${BTFHUB_VOLUME_SUFFIX}" \
    -w /btfhub \
    -e BTFHUB_HTTP_CACHE \
    -e HOME=/tmp/root \
    -e GOPATH=/tmp/go \
    -e GOCACHE=/tmp/go-cache \
    "$IMAGE" "$@"
