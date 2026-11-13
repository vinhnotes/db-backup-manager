#!/bin/sh
# Build the Docker image and push it to a registry you specify.
#
# Usage: IMAGE=registry.example.com/your-namespace/db-backup-manager scripts/docker-push.sh [tag]
#   scripts/docker-push.sh [tag] [image]
#   tag        defaults to the git short SHA (with -dirty if there are uncommitted changes)
#
# Environment overrides:
#   IMAGE      required repository without tag (or pass it as the second argument)
#   REGISTRY   optional login server; otherwise inferred from IMAGE (Docker Hub supported)
#   PLATFORMS  default linux/amd64 (e.g. linux/amd64,linux/arm64)
#   PG_MAJOR   PostgreSQL client major version baked into the image (Dockerfile default: 18)
#   LATEST     set to 0 to skip also pushing :latest
#   REGISTRY_USER / REGISTRY_PASSWORD  log in non-interactively before pushing
set -eu
cd "$(dirname "$0")/.."

if [ "$#" -gt 2 ]; then
  echo "Usage: IMAGE=<repository> $0 [tag] [image]" >&2
  exit 1
fi
IMAGE="${2:-${IMAGE:-}}"
if [ -z "$IMAGE" ]; then
  echo "Set IMAGE to your image repository (without tag), or pass it as the second argument." >&2
  exit 1
fi
case "$IMAGE" in
  *://*|*@*|*[[:space:]]*|*/|:*)
    echo "IMAGE must be an image repository without a scheme, tag or digest." >&2
    exit 1 ;;
esac
case "${IMAGE##*/}" in
  *:*) echo "IMAGE must not include a tag; pass the tag as the first argument." >&2; exit 1 ;;
esac
PLATFORMS="${PLATFORMS:-linux/amd64}"
LATEST="${LATEST:-1}"
if [ -z "${REGISTRY:-}" ]; then
  REGISTRY=docker.io
  case "$IMAGE" in
    */*)
      image_host="${IMAGE%%/*}"
      case "$image_host" in
        *.*|*:*|localhost) REGISTRY="$image_host" ;;
      esac ;;
  esac
fi
if { [ -n "${REGISTRY_USER:-}" ] && [ -z "${REGISTRY_PASSWORD:-}" ]; } ||
   { [ -z "${REGISTRY_USER:-}" ] && [ -n "${REGISTRY_PASSWORD:-}" ]; }; then
  echo "Set both REGISTRY_USER and REGISTRY_PASSWORD, or use docker login beforehand." >&2
  exit 1
fi

if [ $# -ge 1 ]; then
  TAG="$1"
else
  TAG=$(git rev-parse --short HEAD)
  if [ -n "$(git status --porcelain)" ]; then TAG="$TAG-dirty"; fi
fi

if [ -n "${REGISTRY_USER:-}" ] && [ -n "${REGISTRY_PASSWORD:-}" ]; then
  printf '%s' "$REGISTRY_PASSWORD" | docker login "$REGISTRY" -u "$REGISTRY_USER" --password-stdin
fi

set -- --platform "$PLATFORMS" -t "$IMAGE:$TAG"
[ "$LATEST" = "1" ] && set -- "$@" -t "$IMAGE:latest"
[ -n "${PG_MAJOR:-}" ] && set -- "$@" --build-arg "PG_MAJOR=$PG_MAJOR"

# A docker-container builder is needed for multi-platform builds and pushes.
if ! docker buildx inspect backupmanager-builder >/dev/null 2>&1; then
  docker buildx create --name backupmanager-builder --driver docker-container >/dev/null
fi

echo "==> Building $IMAGE:$TAG ($PLATFORMS)"
docker buildx build --builder backupmanager-builder "$@" \
  --label "org.opencontainers.image.revision=$(git rev-parse HEAD)" \
  --label "org.opencontainers.image.created=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --push .

echo "==> Pushed $IMAGE:$TAG"
[ "$LATEST" = "1" ] && echo "==> Pushed $IMAGE:latest"
exit 0
