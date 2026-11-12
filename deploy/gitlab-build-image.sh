#!/bin/sh
set -eu

: "${CI_COMMIT_SHA:?}"
: "${CI_PROJECT_URL:?}"
: "${GO_IMAGE:?}"
: "${GHCR_IMAGE:?}"

image="${GHCR_IMAGE}:sha-${CI_COMMIT_SHA}"
diagnostic="$(mktemp)"
trap 'rm -f "$diagnostic"' EXIT

if docker buildx imagetools inspect "$image" >/dev/null 2>"$diagnostic"; then
  echo "Immutable tag already exists: $image; refusing to overwrite it" >&2
  exit 1
fi
if ! grep -Eq 'manifest unknown|name unknown' "$diagnostic" &&
   ! grep -Fq "${image}: not found" "$diagnostic"; then
  cat "$diagnostic" >&2
  echo 'Could not establish registry tag absence' >&2
  exit 1
fi

builder="sub2api-${CI_JOB_ID}"
docker buildx create --name "$builder" --driver docker-container --use
trap 'docker buildx rm "$builder" >/dev/null 2>&1 || true; rm -f "$diagnostic"' EXIT
docker buildx inspect --bootstrap
docker buildx build \
  --platform linux/amd64 \
  --file Dockerfile \
  --build-arg "GOLANG_IMAGE=${GO_IMAGE}" \
  --build-arg "COMMIT=${CI_COMMIT_SHA}" \
  --build-arg "DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --build-arg "GO_CACHE_SCOPE=sub2api-go-1.27.4" \
  --build-arg "APK_MIRROR=${APK_MIRROR:-}" \
  --label "org.opencontainers.image.revision=${CI_COMMIT_SHA}" \
  --label "org.opencontainers.image.source=${CI_PROJECT_URL}" \
  --tag "$image" \
  --metadata-file image-metadata.json \
  --push .

digest="$(jq -er '.["containerimage.digest"] | select(test("^sha256:[0-9a-f]{64}$"))' image-metadata.json)"
docker pull "${GHCR_IMAGE}@${digest}"
revision="$(docker image inspect --format '{{index .Config.Labels "org.opencontainers.image.revision"}}' "${GHCR_IMAGE}@${digest}")"
platform="$(docker image inspect --format '{{.Os}}/{{.Architecture}}' "${GHCR_IMAGE}@${digest}")"
test "$revision" = "$CI_COMMIT_SHA"
test "$platform" = linux/amd64
printf 'IMAGE=%s\nIMAGE_DIGEST=%s\n' "$image" "$digest" > image.env
printf 'Published %s@%s\n' "$GHCR_IMAGE" "$digest"
