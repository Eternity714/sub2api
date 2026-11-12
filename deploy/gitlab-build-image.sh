#!/bin/sh
set -eu

: "${CI_COMMIT_SHA:?}"
: "${CI_PROJECT_URL:?}"
: "${GO_IMAGE:?}"
: "${GHCR_IMAGE:?}"
: "${BUILDKIT_IMAGE:?}"

if [ "${CANDIDATE_ONLY:-false}" = true ]; then
  : "${VALIDATED_SOURCE_SHA:?Candidate mode requires the accepted source commit}"
  printf '%s\n' "$VALIDATED_SOURCE_SHA" | grep -Eq '^[0-9a-f]{40}$'
  git merge-base --is-ancestor "$VALIDATED_SOURCE_SHA" "$CI_COMMIT_SHA"
  git diff --no-renames --name-only "$VALIDATED_SOURCE_SHA" "$CI_COMMIT_SHA" > candidate-changed-files.txt
  while IFS= read -r changed; do
    case "$changed" in
      .gitlab-ci.yml|deploy/GITLAB_CI.md|deploy/gitlab-build-image.sh|deploy/gitlab-buildkitd.toml) ;;
      *) echo "Candidate mode requires unchanged application and Dockerfile sources: $changed" >&2; exit 1 ;;
    esac
  done < candidate-changed-files.txt
  printf 'Candidate application and Dockerfile sources match accepted commit %s\n' "$VALIDATED_SOURCE_SHA"
fi

verify_release() {
  reference="$1"
  docker pull "$reference"
  revision="$(docker image inspect --format '{{index .Config.Labels "org.opencontainers.image.revision"}}' "$reference")"
  platform="$(docker image inspect --format '{{.Os}}/{{.Architecture}}' "$reference")"
  test "$revision" = "$CI_COMMIT_SHA"
  test "$platform" = linux/amd64
}

write_image_env() {
  printf 'IMAGE=%s\nIMAGE_DIGEST=%s\nCANDIDATE_ONLY=%s\n' "$image" "$digest" "${CANDIDATE_ONLY:-false}" > image.env
}

image="${GHCR_IMAGE}:sha-${CI_COMMIT_SHA}"
diagnostic="$(mktemp)"
trap 'rm -f "$diagnostic"' EXIT

if docker buildx imagetools inspect "$image" >/dev/null 2>"$diagnostic"; then
  verify_release "$image"
  reference="$(docker image inspect --format '{{index .RepoDigests 0}}' "$image")"
  digest="${reference#*@}"
  jq -en --arg digest "$digest" '$digest | test("^sha256:[0-9a-f]{64}$")' >/dev/null
  jq -n --arg digest "$digest" '{"containerimage.digest": $digest, "reused": true}' > image-metadata.json
  write_image_env
  echo "Verified existing immutable image $reference"
  exit 0
fi
if ! grep -Eq 'manifest unknown|name unknown' "$diagnostic" &&
   ! grep -Fq "${image}: not found" "$diagnostic"; then
  cat "$diagnostic" >&2
  echo 'Could not establish registry tag absence' >&2
  exit 1
fi

docker pull "$BUILDKIT_IMAGE"
builder="sub2api-${CI_JOB_ID}"
docker buildx create --name "$builder" --driver docker-container \
  --driver-opt "image=${BUILDKIT_IMAGE}" \
  --buildkitd-config deploy/gitlab-buildkitd.toml --use
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
verify_release "${GHCR_IMAGE}@${digest}"
write_image_env
printf 'Published %s@%s\n' "$GHCR_IMAGE" "$digest"
