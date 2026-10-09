#!/usr/bin/env bash
# Install beside gray-common.sh on the server; prepare files at 0% before deploy.
set -Eeuo pipefail
source "$(cd "$(dirname "$0")" && pwd)/gray-common.sh"

slot="${1:-}"
tag="${2:-}"
valid_slot "$slot" || fail 'Usage: gray-prepare-documentation.sh <blue|green> sha-<commit>'
[[ "$tag" =~ ^sha-[0-9a-f]{7,64}$ ]] || fail 'Only immutable sha-<commit> tags are accepted'
"$SCRIPTS_DIR/gray-status.sh"
load_state
[[ "$slot" == "$CANDIDATE_SLOT" && "$CANDIDATE_PERCENT" == 0 ]] || fail 'Documentation preparation requires the zero-traffic candidate'
stable_slot="$STABLE_SLOT"
for required_command in podman rsync find install; do
  command -v "$required_command" >/dev/null || fail "Required command not found: $required_command"
done

# Neither slot nor an existing child may redirect writes outside the fixed root.
check_directory() {
  local directory="$1"
  local symlink
  [[ ! -L "$directory" ]] || fail "Documentation directory must not be a symlink: $directory"
  if [[ -e "$directory" ]]; then
    [[ -d "$directory" ]] || fail "Documentation path must be a directory: $directory"
    symlink="$(find "$directory" -type l -print -quit)" || fail "Cannot inspect documentation directory: $directory"
    [[ -z "$symlink" ]] || fail "Documentation directory contains a symlink: $directory"
  fi
}

check_candidate_state() {
  "$SCRIPTS_DIR/gray-status.sh"
  load_state
  [[ "$slot" == "$CANDIDATE_SLOT" && "$CANDIDATE_PERCENT" == 0 && "$STABLE_SLOT" == "$stable_slot" ]] || fail 'Gray state changed; documentation preparation requires the same zero-traffic candidate'
}

image="$GHCR_IMAGE:$tag"
podman pull "$image"
stage="$(mktemp -d "$GRAY_DIR/.docs-$slot.XXXXXX")"
export_container=""
cleanup() {
  local exit_status=$?
  trap - EXIT
  if [[ -n "$export_container" ]]; then podman rm "$export_container" >/dev/null || true; fi
  if [[ "$stage" == "$GRAY_DIR"/.docs-* ]]; then rm -rf -- "$stage" || true; fi
  exit "$exit_status"
}
trap cleanup EXIT
export_container="$(podman create --entrypoint /bin/true "$image")"
podman cp "$export_container:/app/docs-default/." "$stage/"
[[ -f "$stage/index.html" && -f "$stage/404.html" && -d "$stage/assets" ]] || fail 'Image has no complete generated documentation site'
check_directory "$stage"
find "$stage" -type d -exec chmod 755 {} +
find "$stage" -type f -exec chmod 644 {} +

target="$GRAY_DIR/docs-$slot"
other_target="$GRAY_DIR/docs-$stable_slot"
release_directory="$GRAY_DIR/docs-releases"
release_marker="$GRAY_DIR/docs-$slot-release"
check_directory "$target"
check_directory "$other_target"
check_directory "$release_directory"
[[ ! -e "$target/index.html" || -f "$target/index.html" ]] || fail 'Candidate index.html must be a regular file'
[[ ! -L "$release_marker" && ( ! -e "$release_marker" || -f "$release_marker" ) ]] || fail 'Documentation release marker must be a regular file'
# Image extraction may take time. Recheck immediately before touching either slot.
# Existing gray scripts have no shared lock; operators must run them serially.
check_candidate_state
mkdir -p "$target/assets" "$other_target/assets" "$release_directory"
chmod 755 "$target" "$target/assets" "$other_target" "$other_target/assets" "$release_directory"
if [[ -f "$target/index.html" ]]; then
  backup_directory="$(mktemp -d "$release_directory/before-$slot-$(date -u +%Y%m%d-%H%M%S).XXXXXX")"
  cp -a "$target/." "$backup_directory/"
fi

# Preserve both generations before publishing HTML. Keep both mount inodes and
# only append missing assets to the stable slot, never replace its existing files.
rsync -a "$stage/assets/" "$target/assets/"
rsync -a --ignore-existing "$target/assets/" "$other_target/assets/"
rsync -a --ignore-existing "$other_target/assets/" "$target/assets/"
# Anchor the exclusion: nested pages such as scenarios/index.html must be copied.
rsync -a --exclude=/assets/ --exclude=/index.html "$stage/" "$target/"
install -m 644 "$stage/index.html" "$target/.index-$tag.tmp"
mv -T -- "$target/.index-$tag.tmp" "$target/index.html"
printf '%s\n' "$image" >"$stage/release"
chmod 644 "$stage/release"
mv -T -- "$stage/release" "$release_marker"
info "Documentation prepared for $slot at $target; stable HTML was not changed"
