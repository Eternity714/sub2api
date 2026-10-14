#!/usr/bin/env bash
set -Eeuo pipefail
source "$(cd "$(dirname "$0")" && pwd)/gray-docsify-common.sh"
tag="${1:-}"
[[ $# == 1 ]] && docsify_valid_tag "$tag" || fail 'Usage: gray-deploy-docsify.sh sha-<commit>; mutable tags are forbidden'
docsify_begin
[[ "$DOCSIFY_CANDIDATE_PERCENT" == 0 ]] || fail 'Cannot replace a Docsify candidate that has traffic; use gray-set-docsify-traffic.sh 0 first'
slot="$DOCSIFY_CANDIDATE_SLOT"
container="sub2api-docsify-$slot-$tag"
content="$DOCSIFY_CONTENT_DIR"
snapshot="$DOCSIFY_DIR/releases/$slot-$tag/content"
old_container="$(docsify_slot_value "$slot" CONTAINER)"
old_tag="$(docsify_slot_value "$slot" TAG)"
if [[ "$old_tag" == "$tag" ]]; then
  [[ "$(docsify_slot_value "$slot" CONTENT)" == "$content" ]] || fail 'Existing immutable Docsify candidate uses a legacy content directory; retained unchanged, not migrated to the fixed directory'
  docsify_validate_content "$content"
  docsify_canonical_mount "$slot" || fail 'Existing immutable Docsify candidate does not bind the fixed content directory read-only; container and evidence retained'
  docsify_health "$slot" || fail 'Existing immutable Docsify candidate is unhealthy; container and evidence retained'
  docsify_apply_proxy || fail 'Nginx configuration failed; candidate retained at zero traffic'
  docsify_report
  exit 0
fi
docsify_podman pull "$DOCSIFY_IMAGE:$tag"
stage="$(mktemp -d "$DOCSIFY_DIR/.stage.XXXXXX")"
export_container=''
cleanup() {
  local exit_status=$?
  trap - EXIT
  [[ -z "$export_container" ]] || docsify_podman rm "$export_container" >/dev/null || true
  [[ "$stage" != "$DOCSIFY_DIR"/.stage.* ]] || rm -rf -- "$stage" || true
  exit "$exit_status"
}
trap cleanup EXIT
mkdir "$stage/content"
export_container="$(docsify_podman create --entrypoint /bin/true "$DOCSIFY_IMAGE:$tag")"
docsify_podman cp "$export_container:/usr/share/nginx/html/docs/." "$stage/content/"
docsify_validate_content "$stage/content"
find "$stage/content" -type d -exec chmod 755 {} +
find "$stage/content" -type f -exec chmod 644 {} +
[[ ! -e "${snapshot%/content}" ]] || fail 'This immutable release already has retained content; inspect it before retrying'
mkdir "${snapshot%/content}"
mv -T -- "$stage/content" "$snapshot"
docsify_prepare_content "$snapshot"
# Only the previously recorded, zero-traffic Docsify slot may be stopped. Never
# remove it: keeping the old container/content preserves logs and rollback evidence.
if [[ -n "$old_container" ]]; then docsify_podman stop "$old_container"; fi
port=18083
[[ "$slot" != green ]] || port=18084
docsify_podman run -d --name "$container" --restart unless-stopped \
  --publish "127.0.0.1:$port:8080" --read-only \
  --security-opt no-new-privileges --cap-drop all \
  --health-cmd 'wget -q -T 4 -O /dev/null http://127.0.0.1:8080/health' \
  --health-interval 10s --health-timeout 5s \
  --health-start-period 5s --health-retries 3 \
  --tmpfs /tmp:rw,noexec,nosuid,size=16m,mode=1777 \
  --volume "$content:/usr/share/nginx/html/docs:ro" "$DOCSIFY_IMAGE:$tag"
printf -v "DOCSIFY_${slot^^}_TAG" '%s' "$tag"
printf -v "DOCSIFY_${slot^^}_CONTAINER" '%s' "$container"
printf -v "DOCSIFY_${slot^^}_CONTENT" '%s' "$content"
docsify_canonical_mount "$slot" || fail 'Docsify candidate did not bind the fixed content directory read-only; Nginx routing was not changed and container/content retained'
docsify_save_state || fail 'Docsify candidate started, but saving release metadata failed; Nginx routing was not changed and container/content retained for inspection'
healthy=0
for ((attempt = 0; attempt < 30; attempt++)); do
  if docsify_health "$slot"; then healthy=1; break; fi
  sleep 2
done
[[ "$healthy" == 1 ]] || fail 'Docsify candidate did not become healthy; zero public entry traffic, container and content retained'
docsify_apply_proxy || fail 'Nginx configuration failed and previous config was restored; Docsify candidate retained at zero traffic'
docsify_podman logs --since 5m --tail 30 "$container"
docsify_report
info "Docsify candidate ready on 127.0.0.1:$port with 0% entry traffic; validate it before increasing traffic"
