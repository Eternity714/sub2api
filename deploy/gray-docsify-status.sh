#!/usr/bin/env bash
set -Eeuo pipefail
source "$(cd "$(dirname "$0")" && pwd)/gray-docsify-common.sh"
[[ $# == 0 ]] || fail 'Usage: gray-docsify-status.sh'
docsify_begin
docsify_report
if ! sudo -n nginx -t; then
  [[ "$DOCSIFY_CANDIDATE_PERCENT" == 0 ]] || docsify_rollback_entries
  fail 'Nginx validation failed; active Docsify entry traffic rolled back and evidence retained'
fi
if [[ "$DOCSIFY_CANDIDATE_PERCENT" != 0 ]] && ! docsify_health "$DOCSIFY_CANDIDATE_SLOT"; then
  docsify_rollback_entries
  fail 'Unhealthy active Docsify candidate detected; entries rolled back to stable'
fi
