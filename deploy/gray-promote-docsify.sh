#!/usr/bin/env bash
set -Eeuo pipefail
source "$(cd "$(dirname "$0")" && pwd)/gray-docsify-common.sh"
[[ $# == 0 ]] || fail 'Usage: gray-promote-docsify.sh'
docsify_begin
[[ "$DOCSIFY_CANDIDATE_PERCENT" == 100 && "$DOCSIFY_FULL_TRAFFIC_AT" != 0 ]] || fail 'Docsify promotion requires 100% candidate traffic'
observation="${SUB2API_DOCSIFY_OBSERVATION_SECONDS:-300}"
[[ "$observation" =~ ^(0|[1-9][0-9]*)$ ]] || fail 'Invalid Docsify observation interval'
now="$(date +%s)"
(( now - DOCSIFY_FULL_TRAFFIC_AT >= observation )) || fail "Docsify needs an observation interval of $observation seconds at 100% before promotion"
if ! docsify_health "$DOCSIFY_CANDIDATE_SLOT"; then
  docsify_rollback_entries
  fail 'Unhealthy Docsify candidate cannot be promoted; entries rolled back to stable'
fi
previous_stable="$DOCSIFY_STABLE_SLOT"
previous_candidate="$DOCSIFY_CANDIDATE_SLOT"
previous_promoted_at="$DOCSIFY_PROMOTED_AT"
DOCSIFY_STABLE_SLOT="$previous_candidate"
DOCSIFY_CANDIDATE_SLOT=blue
[[ "$previous_candidate" != blue ]] || DOCSIFY_CANDIDATE_SLOT=green
DOCSIFY_CANDIDATE_PERCENT=0
DOCSIFY_COOKIE_EPOCH=$((DOCSIFY_COOKIE_EPOCH + 1))
DOCSIFY_FULL_TRAFFIC_AT=0
DOCSIFY_PROMOTED_AT="$now"
if ! docsify_apply_proxy; then
  DOCSIFY_STABLE_SLOT="$previous_stable"
  DOCSIFY_CANDIDATE_SLOT="$previous_candidate"
  DOCSIFY_PROMOTED_AT="$previous_promoted_at"
  docsify_rollback_entries
  fail 'Docsify promotion configuration failed; entries rolled back to the previous stable slot'
fi
if ! docsify_health "$DOCSIFY_STABLE_SLOT"; then
  DOCSIFY_STABLE_SLOT="$previous_stable"
  DOCSIFY_CANDIDATE_SLOT="$previous_candidate"
  DOCSIFY_PROMOTED_AT="$previous_promoted_at"
  docsify_rollback_entries
  fail 'Docsify became unhealthy during promotion; entries rolled back to the previous stable slot'
fi
if ! docsify_save_state; then
  DOCSIFY_STABLE_SLOT="$previous_stable"
  DOCSIFY_CANDIDATE_SLOT="$previous_candidate"
  DOCSIFY_PROMOTED_AT="$previous_promoted_at"
  docsify_rollback_entries || fail 'Docsify promotion state save failed and rollback could not be fully recorded; inspect Nginx and state before another deployment'
  fail 'Docsify promotion state save failed; entries rolled back to the previous stable slot at 0%'
fi
docsify_report
info "Docsify promoted to $DOCSIFY_STABLE_SLOT; previous containers and content retained"
