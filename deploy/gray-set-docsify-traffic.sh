#!/usr/bin/env bash
set -Eeuo pipefail
source "$(cd "$(dirname "$0")" && pwd)/gray-docsify-common.sh"
percent="${1:-}"
[[ $# == 1 && "$percent" =~ ^(0|[1-9][0-9]?|100)$ ]] || fail 'Usage: gray-set-docsify-traffic.sh <0..100>'
docsify_begin
previous_percent="$DOCSIFY_CANDIDATE_PERCENT"
if [[ "$percent" != 0 ]] && ! docsify_canonical_mount "$DOCSIFY_CANDIDATE_SLOT"; then
  [[ "$previous_percent" == 0 ]] || docsify_rollback_entries
  fail 'Docsify candidate must bind the fixed content directory read-only before receiving traffic; legacy candidate evidence retained'
fi
if [[ "$percent" != 0 ]] && ! docsify_health "$DOCSIFY_CANDIDATE_SLOT"; then
  [[ "$previous_percent" == 0 ]] || docsify_rollback_entries
  fail 'Docsify candidate is unhealthy; new entries are on the stable slot and candidate evidence is retained'
fi
DOCSIFY_CANDIDATE_PERCENT="$percent"
DOCSIFY_COOKIE_EPOCH=$((DOCSIFY_COOKIE_EPOCH + 1))
if [[ "$percent" == 100 ]]; then
  [[ "$previous_percent" == 100 ]] || DOCSIFY_FULL_TRAFFIC_AT="$(date +%s)"
else
  DOCSIFY_FULL_TRAFFIC_AT=0
fi
if ! docsify_apply_proxy; then
  DOCSIFY_CANDIDATE_PERCENT="$previous_percent"
  [[ "$previous_percent" == 0 ]] || docsify_rollback_entries
  fail 'Docsify Nginx validation/reload failed; previous config restored and active candidate traffic rolled back'
fi
if ! docsify_save_state; then
  docsify_rollback_entries || fail 'Docsify state save failed and rollback could not be fully recorded; inspect Nginx and state before another deployment'
  fail 'Docsify traffic state save failed; entries rolled back to stable at 0% and candidate evidence retained'
fi
if [[ "$percent" != 0 ]] && ! docsify_health "$DOCSIFY_CANDIDATE_SLOT"; then
  docsify_rollback_entries
  fail 'Docsify became unhealthy after switching traffic; entries rolled back to stable'
fi
container="$(docsify_slot_value "$DOCSIFY_CANDIDATE_SLOT" CONTAINER)"
[[ -z "$container" ]] || docsify_podman logs --since 5m --tail 30 "$container"
docsify_report
