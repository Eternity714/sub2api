#!/usr/bin/env bash
# Install all gray-docsify-* and gray-*-docsify scripts beside existing gray-common.sh.
set -Eeuo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/gray-common.sh"

DOCSIFY_IMAGE=ghcr.io/eternity714/sub2api-docsify
DOCSIFY_DIR="$GRAY_DIR/docsify"
DOCSIFY_CONTENT_DIR="$DEPLOY_DIR/content"
DOCSIFY_STATE="$DOCSIFY_DIR/state.env"
DOCSIFY_NGINX_MAIN="${SUB2API_DOCSIFY_NGINX_MAIN:-/etc/nginx/conf.d/sub2api.conf}"
DOCSIFY_NGINX_POLICY="${SUB2API_DOCSIFY_NGINX_POLICY:-/etc/nginx/conf.d/sub2api-docsify-policy.conf}"
DOCSIFY_NGINX_LOCATIONS="${SUB2API_DOCSIFY_NGINX_LOCATIONS:-/etc/nginx/snippets/sub2api-docsify-locations.conf}"
DOCSIFY_SERVER_NAME="${SUB2API_DOCSIFY_SERVER_NAME:-www.gkotta.bid}"

docsify_check_directory() {
  local path="$1" link
  [[ ! -L "$path" ]] || fail "Docsify directory must not be a symlink: $path"
  if [[ -e "$path" ]]; then
    [[ -d "$path" ]] || fail "Docsify path must be a directory: $path"
    link="$(find "$path" -type l -print -quit)" || fail "Cannot inspect directory: $path"
    [[ -z "$link" ]] || fail "Docsify directory contains a symlink: $link"
  fi
}

docsify_validate_content() {
  local path="$1" name unsafe_file
  docsify_check_directory "$path"
  [[ -d "$path" ]] || fail "Docsify public content directory is missing: $path"
  for name in index.md _sidebar.md _navbar.md _404.md; do
    [[ -f "$path/$name" ]] || fail "Docsify public content/navigation is missing: $path/$name"
  done
  unsafe_file="$(find "$path" -mindepth 1 \( -name '.*' -o -iname README.md -o \( ! -type d ! -type f \) -o \( -type f ! -iname '*.md' ! -iname '*.png' ! -iname '*.jpg' ! -iname '*.jpeg' ! -iname '*.gif' ! -iname '*.ico' ! -iname '*.svg' ! -iname '*.webp' \) \) -print -quit)" || fail "Cannot inspect Docsify public content: $path"
  [[ -z "$unsafe_file" ]] || fail "Docsify content exposes a maintenance or unsupported file: $unsafe_file"
}

docsify_prepare_content() (
  local source="$1" temporary=''
  [[ -d "$DEPLOY_DIR" && ! -L "$DEPLOY_DIR" ]] || fail "Docsify deployment root must be a real directory: $DEPLOY_DIR"
  if [[ -e "$DOCSIFY_CONTENT_DIR" || -L "$DOCSIFY_CONTENT_DIR" ]]; then
    # Existing maintained content is authoritative; do not chmod or replace it.
    docsify_validate_content "$DOCSIFY_CONTENT_DIR"
    exit 0
  fi
  if [[ "$DOCSIFY_STABLE_SLOT" != legacy ]]; then
    source="$(docsify_slot_value "$DOCSIFY_STABLE_SLOT" CONTENT)"
  fi
  docsify_validate_content "$source"
  temporary="$(mktemp -d "$DEPLOY_DIR/.docsify-content.XXXXXX")" || fail 'Cannot stage the fixed Docsify content directory'
  trap '[[ -z "$temporary" ]] || rm -rf -- "$temporary"' EXIT
  cp -a -- "$source/." "$temporary/"
  docsify_validate_content "$temporary"
  find "$temporary" -type d -exec chmod 755 {} +
  find "$temporary" -type f -exec chmod 644 {} +
  # Never replace an existing directory, including one created during staging.
  mv -T -n -- "$temporary" "$DOCSIFY_CONTENT_DIR"
  [[ ! -e "$temporary" ]] || fail 'The fixed Docsify content directory appeared during initialization; existing content retained'
  temporary=''
)

docsify_valid_tag() { [[ "$1" =~ ^sha-[0-9a-f]{7,64}$ ]]; }
# Podman's background monitor must not inherit the operation lock descriptor.
docsify_podman() { command podman "$@" 9>&-; }
docsify_slot_value() { local variable="DOCSIFY_${1^^}_$2"; printf '%s' "${!variable}"; }
docsify_canonical_mount() {
  local slot="$1" container mount
  [[ "$(docsify_slot_value "$slot" CONTENT)" == "$DOCSIFY_CONTENT_DIR" ]] || return 1
  container="$(docsify_slot_value "$slot" CONTAINER)"
  [[ -n "$container" ]] || return 1
  mount="$(docsify_podman inspect --format '{{range .Mounts}}{{if eq .Destination "/usr/share/nginx/html/docs"}}{{.Type}}|{{.Source}}|{{.RW}}{{end}}{{end}}' "$container" 2>/dev/null)" || return 1
  [[ "$mount" == "bind|$DOCSIFY_CONTENT_DIR|false" ]]
}
docsify_slot_target() {
  case "$1" in
    legacy) printf '%s' '$sub2api_gray_target' ;;
    blue) printf 'http://127.0.0.1:18083' ;;
    green) printf 'http://127.0.0.1:18084' ;;
    *) fail "Invalid Docsify slot: $1" ;;
  esac
}

docsify_load_state() {
  DOCSIFY_STABLE_SLOT=legacy
  DOCSIFY_CANDIDATE_SLOT=blue
  DOCSIFY_CANDIDATE_PERCENT=0
  DOCSIFY_COOKIE_EPOCH=1
  DOCSIFY_FULL_TRAFFIC_AT=0
  DOCSIFY_PROMOTED_AT=0
  DOCSIFY_BLUE_TAG='' DOCSIFY_BLUE_CONTAINER='' DOCSIFY_BLUE_CONTENT=''
  DOCSIFY_GREEN_TAG='' DOCSIFY_GREEN_CONTAINER='' DOCSIFY_GREEN_CONTENT=''
  if [[ -e "$DOCSIFY_STATE" ]]; then
    [[ -f "$DOCSIFY_STATE" && ! -L "$DOCSIFY_STATE" ]] || fail 'Docsify state must be a regular file'
    source "$DOCSIFY_STATE"
  fi
  [[ "$DOCSIFY_STABLE_SLOT" =~ ^(legacy|blue|green)$ && "$DOCSIFY_CANDIDATE_SLOT" =~ ^(blue|green)$ && "$DOCSIFY_STABLE_SLOT" != "$DOCSIFY_CANDIDATE_SLOT" ]] || fail 'Invalid Docsify slot state'
  [[ "$DOCSIFY_CANDIDATE_PERCENT" =~ ^(0|[1-9][0-9]?|100)$ && "$DOCSIFY_COOKIE_EPOCH" =~ ^[1-9][0-9]*$ && "$DOCSIFY_FULL_TRAFFIC_AT" =~ ^[0-9]+$ && "$DOCSIFY_PROMOTED_AT" =~ ^[0-9]+$ ]] || fail 'Invalid Docsify traffic state'
  local slot tag container content
  for slot in blue green; do
    tag="$(docsify_slot_value "$slot" TAG)"
    container="$(docsify_slot_value "$slot" CONTAINER)"
    content="$(docsify_slot_value "$slot" CONTENT)"
    if [[ -n "$tag" ]]; then
      docsify_valid_tag "$tag" || fail 'State contains a mutable Docsify image tag'
      [[ "$container" == "sub2api-docsify-$slot-$tag" && ( "$content" == "$DOCSIFY_CONTENT_DIR" || "$content" == "$DOCSIFY_DIR/releases/$slot-$tag/content" ) ]] || fail 'Invalid Docsify container/content state'
    else
      [[ -z "$container" && -z "$content" ]] || fail 'Incomplete Docsify slot metadata'
    fi
  done
  [[ "$DOCSIFY_STABLE_SLOT" == legacy || -n "$(docsify_slot_value "$DOCSIFY_STABLE_SLOT" TAG)" ]] || fail 'Stable Docsify slot has no image'
}

docsify_save_state() {
  local temporary variable exit_status
  temporary="$(mktemp "$DOCSIFY_DIR/.state.XXXXXX")" || return $?
  for variable in DOCSIFY_STABLE_SLOT DOCSIFY_CANDIDATE_SLOT DOCSIFY_CANDIDATE_PERCENT DOCSIFY_COOKIE_EPOCH DOCSIFY_FULL_TRAFFIC_AT DOCSIFY_PROMOTED_AT DOCSIFY_BLUE_TAG DOCSIFY_BLUE_CONTAINER DOCSIFY_BLUE_CONTENT DOCSIFY_GREEN_TAG DOCSIFY_GREEN_CONTAINER DOCSIFY_GREEN_CONTENT; do
    printf '%s=%q\n' "$variable" "${!variable}" >>"$temporary" || {
      exit_status=$?; rm -f -- "$temporary" || true; return "$exit_status"
    }
  done
  chmod 600 "$temporary" || { exit_status=$?; rm -f -- "$temporary" || true; return "$exit_status"; }
  mv -T -- "$temporary" "$DOCSIFY_STATE" || { exit_status=$?; rm -f -- "$temporary" || true; return "$exit_status"; }
}

docsify_begin() {
  # Keep this first: the main application remains managed by the existing scripts.
  "$SCRIPTS_DIR/gray-status.sh"
  local command_name path
  for command_name in podman curl nginx systemctl sudo python3 flock find install; do
    command -v "$command_name" >/dev/null || fail "Required command not found: $command_name"
  done
  require_sudo
  for path in "$DOCSIFY_NGINX_MAIN" "$DOCSIFY_NGINX_POLICY" "$DOCSIFY_NGINX_LOCATIONS"; do
    [[ "$path" =~ ^/[A-Za-z0-9_./-]+$ && "$path" != *'/../'* && "$path" != */.. && ! -L "$path" ]] || fail "Unsafe Nginx file path: $path"
    [[ ! -e "$path" || -f "$path" ]] || fail "Nginx configuration must be a regular file: $path"
  done
  [[ -f "$DOCSIFY_NGINX_MAIN" && "$DOCSIFY_NGINX_MAIN" != "$DOCSIFY_NGINX_POLICY" && "$DOCSIFY_NGINX_MAIN" != "$DOCSIFY_NGINX_LOCATIONS" && "$DOCSIFY_NGINX_POLICY" != "$DOCSIFY_NGINX_LOCATIONS" ]] || fail 'Nginx configuration paths are missing or overlap'
  [[ "$DOCSIFY_SERVER_NAME" =~ ^[a-zA-Z0-9.-]+$ ]] || fail 'Invalid Docsify server name'
  docsify_check_directory "$DOCSIFY_DIR"
  mkdir -p "$DOCSIFY_DIR/releases" "$DOCSIFY_DIR/backups"
  exec 9>"$DOCSIFY_DIR/operation.lock"
  flock -n 9 || fail 'Another Docsify gray operation is running'
  docsify_load_state
}

docsify_health() {
  local slot="$1" container target health
  [[ "$slot" != legacy ]] || return 0
  container="$(docsify_slot_value "$slot" CONTAINER)"
  [[ -n "$container" ]] || return 1
  health="$(docsify_podman inspect --format '{{.State.Health.Status}}' "$container" 2>/dev/null)" || return 1
  [[ "$health" == healthy ]] || return 1
  docsify_podman exec "$container" nginx -t >/dev/null 2>&1 || return 1
  target="$(docsify_slot_target "$slot")"
  curl --fail --silent --show-error --max-time 5 "$target/health" >/dev/null
}

docsify_render_policy() {
  local stable_target candidate_target content_target legacy_assets_target stable_release candidate_release
  stable_target="$(docsify_slot_target "$DOCSIFY_STABLE_SLOT")"
  candidate_target="$(docsify_slot_target "$DOCSIFY_CANDIDATE_SLOT")"
  stable_release=legacy
  [[ "$DOCSIFY_STABLE_SLOT" == legacy ]] || stable_release="$(docsify_slot_value "$DOCSIFY_STABLE_SLOT" TAG)"
  candidate_release="$(docsify_slot_value "$DOCSIFY_CANDIDATE_SLOT" TAG)"
  if [[ -z "$candidate_release" ]]; then candidate_target="$stable_target"; candidate_release="$stable_release"; fi
  content_target='$sub2api_docsify_entry_target'
  [[ "$DOCSIFY_STABLE_SLOT" != legacy ]] || content_target="$candidate_target"
  legacy_assets_target='$sub2api_docsify_content_target'
  [[ "$DOCSIFY_STABLE_SLOT" != legacy ]] || legacy_assets_target='$sub2api_gray_target'
  printf '# Managed by Docsify gray scripts; independent of sub2api-gray-policy.conf.\n'
  if [[ "$DOCSIFY_CANDIDATE_PERCENT" == 0 || "$DOCSIFY_CANDIDATE_PERCENT" == 100 ]]; then
    local route=stable
    [[ "$DOCSIFY_CANDIDATE_PERCENT" != 100 ]] || route=candidate
    printf 'map "" $sub2api_docsify_bucket { default %s; }\n' "$route"
  else
    printf 'split_clients "$remote_addr$http_user_agent" $sub2api_docsify_bucket {\n    %s%% candidate;\n    * stable;\n}\n' "$DOCSIFY_CANDIDATE_PERCENT"
  fi
  if [[ "$DOCSIFY_CANDIDATE_PERCENT" == 0 ]]; then
    printf 'map "" $sub2api_docsify_route { default stable; }\n'
  else
    printf 'map $cookie_sub2api_docsify $sub2api_docsify_route {\n    default $sub2api_docsify_bucket;\n    "%s-candidate" candidate;\n    "%s-stable" stable;\n}\n' "$DOCSIFY_COOKIE_EPOCH" "$DOCSIFY_COOKIE_EPOCH"
  fi
  printf 'map $sub2api_docsify_route $sub2api_docsify_cookie_value {\n    default "%s-stable";\n    candidate "%s-candidate";\n}\n' "$DOCSIFY_COOKIE_EPOCH" "$DOCSIFY_COOKIE_EPOCH"
  printf 'map $sub2api_docsify_route $sub2api_docsify_entry_target {\n    default %s;\n    candidate %s;\n}\n' "$stable_target" "$candidate_target"
  printf 'map "" $sub2api_docsify_content_target { default %s; }\n' "$content_target"
  printf 'map "" $sub2api_docsify_legacy_assets_target { default %s; }\n' "$legacy_assets_target"
  printf 'map $sub2api_docsify_route $sub2api_docsify_release {\n    default "%s";\n    candidate "%s";\n}\n' "$stable_release" "$candidate_release"
}

docsify_apply_proxy() (
  # File writes/reload are transactional. A failed validation restores all three
  # files; backups and containers remain available for investigation.
  set -Eeuo pipefail
  temporary="$(mktemp -d "$DOCSIFY_DIR/.proxy.XXXXXX")" || exit $?
  backup="$(mktemp -d "$DOCSIFY_DIR/backups/nginx-$(date -u +%Y%m%d-%H%M%S).XXXXXX")" || exit $?
  changed=0
  success=0
  restore_proxy() {
    local role path
    for role in main policy locations; do
      case "$role" in main) path="$DOCSIFY_NGINX_MAIN";; policy) path="$DOCSIFY_NGINX_POLICY";; locations) path="$DOCSIFY_NGINX_LOCATIONS";; esac
      if [[ -f "$backup/$role.present" ]]; then
        sudo -n cp -p -- "$backup/$role" "$path" || return $?
      else
        sudo -n rm -f -- "$path" || return $?
      fi
    done
    sudo -n nginx -t && sudo -n systemctl reload nginx
  }
  finish_proxy() {
    local exit_status=$?
    trap - EXIT
    if [[ "$changed" == 1 && "$success" != 1 ]]; then
      printf 'Docsify Nginx operation failed; restoring backup %s\n' "$backup" >&2
      restore_proxy || printf 'CRITICAL: Nginx restore/reload failed; inspect %s immediately\n' "$backup" >&2
    fi
    rm -rf -- "$temporary"
    exit "$exit_status"
  }
  trap finish_proxy EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM
  docsify_render_policy >"$temporary/policy" || exit $?
  sudo -n cat "$DOCSIFY_NGINX_MAIN" >"$temporary/original-main" || exit $?
  python3 "$SCRIPTS_DIR/gray-docsify-nginx-install.py" "$temporary/original-main" "$temporary/main" "$temporary/locations" "$DOCSIFY_NGINX_LOCATIONS" "$DOCSIFY_SERVER_NAME" || exit $?
  for role in main policy locations; do
    case "$role" in main) path="$DOCSIFY_NGINX_MAIN";; policy) path="$DOCSIFY_NGINX_POLICY";; locations) path="$DOCSIFY_NGINX_LOCATIONS";; esac
    if [[ -e "$path" ]]; then
      sudo -n cp -p -- "$path" "$backup/$role" || exit $?
      touch "$backup/$role.present" || exit $?
    fi
  done
  changed=1
  sudo -n mkdir -p -- "$(dirname "$DOCSIFY_NGINX_LOCATIONS")" || exit $?
  for role in policy locations main; do
    case "$role" in main) path="$DOCSIFY_NGINX_MAIN";; policy) path="$DOCSIFY_NGINX_POLICY";; locations) path="$DOCSIFY_NGINX_LOCATIONS";; esac
    sudo -n install -m 644 -- "$temporary/$role" "$path.docsify-$$.tmp" || exit $?
    sudo -n mv -T -- "$path.docsify-$$.tmp" "$path" || exit $?
  done
  sudo -n nginx -t || exit $?
  sudo -n systemctl reload nginx || exit $?
  success=1
)

docsify_rollback_entries() {
  DOCSIFY_CANDIDATE_PERCENT=0
  DOCSIFY_FULL_TRAFFIC_AT=0
  DOCSIFY_COOKIE_EPOCH=$((DOCSIFY_COOKIE_EPOCH + 1))
  if docsify_apply_proxy; then
    if ! docsify_save_state; then
      printf 'CRITICAL: Docsify entry configuration is on stable, but saving rollback state failed; inspect %s before any further deployment\n' "$DOCSIFY_STATE" >&2
      return 1
    fi
    printf 'Docsify entries restored to stable slot %s; candidate container, content and backups retained\n' "$DOCSIFY_STABLE_SLOT" >&2
  else
    printf 'CRITICAL: automatic Docsify traffic rollback failed; inspect Nginx backups in %s/backups\n' "$DOCSIFY_DIR" >&2
    return 1
  fi
}

docsify_report() {
  local slot container tag health
  printf 'docsify_stable_slot=%s\ndocsify_candidate_slot=%s\ndocsify_candidate_percent=%s\n' "$DOCSIFY_STABLE_SLOT" "$DOCSIFY_CANDIDATE_SLOT" "$DOCSIFY_CANDIDATE_PERCENT"
  printf 'docsify_cookie_epoch=%s\ndocsify_full_traffic_at=%s\ndocsify_promoted_at=%s\n' "$DOCSIFY_COOKIE_EPOCH" "$DOCSIFY_FULL_TRAFFIC_AT" "$DOCSIFY_PROMOTED_AT"
  for slot in blue green; do
    container="$(docsify_slot_value "$slot" CONTAINER)"
    tag="$(docsify_slot_value "$slot" TAG)"
    health=absent
    [[ -z "$container" ]] || health="$(docsify_podman inspect --format '{{.State.Health.Status}}' "$container" 2>/dev/null || printf missing)"
    printf 'docsify_%s_image=%s\ndocsify_%s_container=%s\ndocsify_%s_health=%s\ndocsify_%s_content=%s\n' "$slot" "${tag:+$DOCSIFY_IMAGE:$tag}" "$slot" "$container" "$slot" "$health" "$slot" "$(docsify_slot_value "$slot" CONTENT)"
  done
}
