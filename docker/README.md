# Local Docker toolchains (mirror CI)

CI runs **Amazon** in an **Amazon Linux 2023** job container and **CentOS / Debian / Fedora / Oracle / Ubuntu** on a **Ubuntu 24.04** self-hosted runner (`tests/install-deps.sh` + `3rdparty/pahole.sh`). These Dockerfiles reproduce those environments on any host.

**Digest-pinned bases:** [`Dockerfile.amazon`](Dockerfile.amazon), [`Dockerfile.distros`](Dockerfile.distros) - bumps via Dependabot (`package-ecosystem: docker`, directory `/docker`). For **BTF / btfhub role** and **`btfhub` flags** overview, see the repo root [README.md](../README.md#running-btfhub-locally).

## Prerequisites

- Docker or **Podman** (`docker` CLI shim). On Fedora, Podman maps the short name `amazonlinux` to `public.ecr.aws`, which does not carry the same digest as Docker Hub - the Amazon image therefore uses **`docker.io/library/amazonlinux`** explicitly.
- **Fedora / SELinux:** bind mounts default to `:z` so the container can read `/btfhub`. If you see `ls: cannot open directory '.': Permission denied` inside the container, ensure you’re on the updated `run-*.sh` scripts, or run with `chcon -Rt container_file_t ~/your/btfhub` once, or `BTFHUB_VOLUME_SUFFIX=""` only if your runtime errors on `:z`.
- BuildKit recommended; images use `syntax=docker/dockerfile:1`.
- **`Dockerfile.*` `RUN` shells** use POSIX `/bin/sh` (Docker default): `set -eu`, `${var}` quoting, `case` where it helps readability. When editing, copy a `RUN` body to a scratch `.sh` file and run `shfmt -p -i 4 -ci -bn -sr -w` and `shellcheck -s sh` (same idea as embedded YAML shell in the Tracee shell style guide).
- Clone with submodules (or run `git submodule update --init --recursive` before `docker build`).

### Makefile shortcuts (from repo root)

```bash
make docker-distros-bash          # shell in distros image
make docker-amazon-bash           # shell in Amazon image
make docker-distros-make          # build btfhub inside distros image
make docker-amazon-make           # build inside Amazon image
make docker-distros-btfhub BTFHUB_ARGS="-workers 4 -d debian -r bullseye"
make docker-amazon-btfhub BTFHUB_ARGS="-workers 4 -d amzn -r 2"
```

Same as `./docker/run-*.sh`; extra env (e.g. `BTFHUB_VOLUME_SUFFIX=""`) still applies when you call the scripts directly or export it before `make`.

## Distros (Ubuntu-based - CentOS, Debian, Fedora, OL, Ubuntu metadata)

```bash
chmod +x docker/run-distros.sh docker/run-amazon.sh   # once

./docker/run-distros.sh bash
# inside the container:
make
./btfhub -workers 4 -d debian -r bullseye
```

Or one-shot:

```bash
./docker/run-distros.sh make
./docker/run-distros.sh ./btfhub -workers 4 -d ubuntu -r focal
```

The repo is bind-mounted at `/btfhub`, so edits on the host apply immediately. Tooling (Go **1.26.1** in the Amazon image, LLVM / `pahole` in the distros image) stays inside the image.

**Note:** The image runs `tests/install-deps.sh pr` so LLVM is installed from apt. Upstream CI `cron` uses `install-deps.sh cron` on runners that already ship LLVM.

## Amazon (Amazon Linux 2023 - `amzn` / AL2 debug RPMs)

```bash
./docker/run-amazon.sh bash
# inside:
make
./btfhub -workers 4 -d amzn -r 2
```

## Optional: match CI preflight / placeholders

Host-side sketch of what [`.github/workflows/cron.yml`](../.github/workflows/cron.yml) does before generate (path list -> placeholders -> **`btfhub -preflight -index ...`** -> optional **`-manifest-out`**). **Flag meanings:** root [README.md](../README.md#running-btfhub-locally) and **`btfhub -h`**.

```bash
# On the host (needs python3 + urllib; script shebang is /usr/bin/python3)
mkdir -p paths
python3 .github/scripts/fetch-archive-paths-github.py paths/existing-archive-paths.txt ubuntu/
bash .github/scripts/ci-archive-placeholders.sh paths/existing-archive-paths.txt archive
```

In the container, point **`-index`** at that path list for **`-preflight`**; use **`-manifest-out`** / **`-manifest`** as in the workflow.

## Environment

- **`BTFHUB_HTTP_CACHE`** - defaults to `/btfhub/.btfhub-http-cache` in the container (same bind mount as the repo, so it persists on the host). Override:  
  `BTFHUB_HTTP_CACHE=/tmp/cache ./docker/run-distros.sh ./btfhub ...`  
  What is cached (metadata vs packages): root [README.md](../README.md#running-btfhub-locally).

## Resource expectations

Full-archive runs are **heavy** (disk, RAM, network). For local use, narrow with `-d`, `-r`, `-workers`, or a manifest from `-preflight`.

## Image tags

Override the image name when building or running:

```bash
docker build -f docker/Dockerfile.distros -t my-btfhub:distros .
IMAGE=my-btfhub:distros ./docker/run-distros.sh bash
```
