#!/usr/bin/env bash
# Run with the frontend-test Compose service; every production command is a fixture.
set -Eeuo pipefail
scripts="$(cd "$(dirname "$0")/.." && pwd)"
test_root="$(mktemp -d)"
trap 'rm -rf -- "$test_root"' EXIT
passed=0
fail_test() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
assert_equal() { [[ "$1" == "$2" ]] || fail_test "$3: expected [$2], got [$1]"; }
pass_case() { passed=$((passed + 1)); printf 'PASS: %s\n' "$1"; }

setup_case() {
  export TEST_CASE_DIR="$test_root/$1"
  export MOCK_UNHEALTHY=0 MOCK_NGINX_FAIL=0 MOCK_RELOAD_FAIL=0 MOCK_BAD_IMAGE=0 MOCK_STATE_SAVE_FAIL=0
  export SUB2API_DOCSIFY_OBSERVATION_SECONDS=0
  mkdir -p "$TEST_CASE_DIR/scripts" "$TEST_CASE_DIR/bin" "$TEST_CASE_DIR/gray" "$TEST_CASE_DIR/nginx" "$TEST_CASE_DIR/image/scenarios" "$TEST_CASE_DIR/containers"
  for file in gray-docsify-common.sh gray-deploy-docsify.sh gray-set-docsify-traffic.sh gray-promote-docsify.sh gray-docsify-status.sh gray-docsify-nginx-install.py; do
    [[ -f "$scripts/$file" ]] || fail_test "Docsify gray implementation missing: $file"
    cp "$scripts/$file" "$TEST_CASE_DIR/scripts/$file"
  done
  export SUB2API_DOCSIFY_NGINX_MAIN="$TEST_CASE_DIR/nginx/main.conf"
  export SUB2API_DOCSIFY_NGINX_POLICY="$TEST_CASE_DIR/nginx/policy.conf"
  export SUB2API_DOCSIFY_NGINX_LOCATIONS="$TEST_CASE_DIR/nginx/locations.conf"
  cat >"$TEST_CASE_DIR/scripts/gray-common.sh" <<'COMMON'
DEPLOY_DIR="$TEST_CASE_DIR"
GRAY_DIR="$TEST_CASE_DIR/gray"
SCRIPTS_DIR="$TEST_CASE_DIR/scripts"
fail() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }
info() { printf '%s\n' "$*"; }
require_sudo() { sudo -n true; }
COMMON
  cat >"$TEST_CASE_DIR/scripts/gray-status.sh" <<'STATUS'
#!/usr/bin/env bash
printf 'gray-status\n' >>"$TEST_CASE_DIR/operations"
STATUS
  cat >"$TEST_CASE_DIR/nginx/main.conf" <<'NGINX'
map $http_upgrade $connection_upgrade { default upgrade; '' close; }
include /etc/nginx/conf.d/sub2api-gray-policy.conf;
server { listen 80; server_name www.gkotta.bid; return 308 https://www.gkotta.bid$request_uri; }
server {
    listen 443 ssl;
    server_name www.gkotta.bid;
    add_header Strict-Transport-Security "max-age=123456; includeSubDomains" always;
    add_header X-Release-Slot $sub2api_gray_slot always;
    location ~* \.(?:css|js|png|svg)$ { proxy_pass $sub2api_gray_target$request_uri; }
    location / { proxy_pass $sub2api_gray_target$request_uri; }
}
NGINX
  printf 'main-state-sentinel' >"$TEST_CASE_DIR/gray/state.env"
  printf 'main-slots-sentinel' >"$TEST_CASE_DIR/gray/slots.env"
  printf 'main-policy-sentinel' >"$TEST_CASE_DIR/nginx/main-policy.conf"
  printf '# Docs\n' >"$TEST_CASE_DIR/image/index.md"
  printf '# Sidebar\n' >"$TEST_CASE_DIR/image/_sidebar.md"
  printf '# Navbar\n' >"$TEST_CASE_DIR/image/_navbar.md"
  printf '# Missing\n' >"$TEST_CASE_DIR/image/_404.md"
  printf '# Scenarios\n' >"$TEST_CASE_DIR/image/scenarios/index.md"
  cat >"$TEST_CASE_DIR/bin/podman" <<'PODMAN'
#!/usr/bin/env bash
set -euo pipefail
printf 'podman %s\n' "$*" >>"$TEST_CASE_DIR/operations"
case "$1" in
  pull) ;;
  create) printf 'docsify-export-fixture\n' ;;
  cp)
    [[ "$2" == 'docsify-export-fixture:/usr/share/nginx/html/docs/.' ]] || exit 91
    cp -a "$TEST_CASE_DIR/image/." "$3"
    if [[ "$MOCK_BAD_IMAGE" == 1 ]]; then printf 'secret' >"$3/README.md"; fi
    ;;
  rm) [[ "$2" == docsify-export-fixture ]] || exit 92 ;;
  run)
    shift
    while (( $# )); do
      if [[ "$1" == --name ]]; then container="$2"; break; fi
      shift
    done
    printf 'running\n' >"$TEST_CASE_DIR/containers/$container"
    printf '%s\n' "$container"
    ;;
  inspect)
    container="${@: -1}"
    [[ -f "$TEST_CASE_DIR/containers/$container" ]] || exit 1
    case "$*" in
      *Health.Status*) if [[ "$MOCK_UNHEALTHY" == 1 ]]; then printf 'unhealthy\n'; else printf 'healthy\n'; fi ;;
      *State.Running*) cat "$TEST_CASE_DIR/containers/$container" ;;
      *) printf 'fixture\n' ;;
    esac
    ;;
  stop) printf 'stopped\n' >"$TEST_CASE_DIR/containers/$2" ;;
  exec) [[ "$3" == nginx && "$4" == -t ]] ;;
  logs) ;;
  *) exit 93 ;;
esac
PODMAN
  cat >"$TEST_CASE_DIR/bin/sudo" <<'SUDO'
#!/usr/bin/env bash
set -euo pipefail
[[ "${1:-}" != -n ]] || shift
exec "$@"
SUDO
  cat >"$TEST_CASE_DIR/bin/nginx" <<'NGINX_TOOL'
#!/usr/bin/env bash
printf 'nginx %s\n' "$*" >>"$TEST_CASE_DIR/operations"
if [[ "$MOCK_NGINX_FAIL" == 1 && ! -f "$TEST_CASE_DIR/nginx-failed" ]]; then touch "$TEST_CASE_DIR/nginx-failed"; exit 51; fi
NGINX_TOOL
  cat >"$TEST_CASE_DIR/bin/systemctl" <<'SYSTEMCTL'
#!/usr/bin/env bash
printf 'systemctl %s\n' "$*" >>"$TEST_CASE_DIR/operations"
if [[ "$MOCK_RELOAD_FAIL" == 1 && ! -f "$TEST_CASE_DIR/reload-failed" ]]; then touch "$TEST_CASE_DIR/reload-failed"; exit 52; fi
SYSTEMCTL
  cat >"$TEST_CASE_DIR/bin/curl" <<'CURL'
#!/usr/bin/env bash
printf 'curl %s\n' "$*" >>"$TEST_CASE_DIR/operations"
[[ "$MOCK_UNHEALTHY" == 0 ]]
CURL
  cat >"$TEST_CASE_DIR/bin/mv" <<'MV'
#!/usr/bin/env bash
set -euo pipefail
if [[ "${@: -1}" == "$TEST_CASE_DIR/gray/docsify/state.env" && "$MOCK_STATE_SAVE_FAIL" != 0 ]]; then
  if [[ "$MOCK_STATE_SAVE_FAIL" == always || ! -f "$TEST_CASE_DIR/state-save-failed" ]]; then
    touch "$TEST_CASE_DIR/state-save-failed"
    exit 53
  fi
fi
exec /usr/bin/mv "$@"
MV
  chmod 755 "$TEST_CASE_DIR/scripts/gray-status.sh" "$TEST_CASE_DIR/bin/"*
}

run_script() {
  PATH="$TEST_CASE_DIR/bin:$PATH" bash "$TEST_CASE_DIR/scripts/$1" "${@:2}" >"$TEST_CASE_DIR/output" 2>&1
}
run_ok() { run_script "$@" || { cat "$TEST_CASE_DIR/output"; fail_test "Command failed: $*"; }; }
reject() { if run_script "$@"; then fail_test "Unsafe command accepted: $*"; fi; }
state_value() { bash -c 'source "$1"; printf "%s" "${!2}"' _ "$TEST_CASE_DIR/gray/docsify/state.env" "$1"; }
assert_main_untouched() {
  assert_equal "$(cat "$TEST_CASE_DIR/gray/state.env")" main-state-sentinel 'Main gray state'
  assert_equal "$(cat "$TEST_CASE_DIR/gray/slots.env")" main-slots-sentinel 'Main slots'
  assert_equal "$(cat "$TEST_CASE_DIR/nginx/main-policy.conf")" main-policy-sentinel 'Main policy'
  [[ "$(head -1 "$TEST_CASE_DIR/operations")" == gray-status ]] || fail_test 'Main gray status was not checked first'
  ! grep -Eq 'podman (rm|stop) sub2api-(blue|green)( |$)' "$TEST_CASE_DIR/operations" || fail_test 'Touched a main application container'
}
validate_real_proxy() {
  [[ -n "${DOCSIFY_REAL_NGINX:-}" ]] || return 0
  sed '/^include \/etc\/nginx\/conf.d\/sub2api-gray-policy.conf;$/d; s/listen 443 ssl;/listen 127.0.0.1:18743;/; s/listen 80;/listen 127.0.0.1:18780;/' "$SUB2API_DOCSIFY_NGINX_MAIN" >"$TEST_CASE_DIR/nginx/server-validation.conf"
  cat >"$TEST_CASE_DIR/nginx/validation.conf" <<CONF
pid $TEST_CASE_DIR/nginx/nginx.pid;
error_log stderr;
events {}
http {
    access_log off;
    map "" \$sub2api_gray_slot { default blue; }
    map "" \$sub2api_gray_target { default http://127.0.0.1:18080; }
    include $SUB2API_DOCSIFY_NGINX_POLICY;
    include $TEST_CASE_DIR/nginx/server-validation.conf;
}
CONF
  "$DOCSIFY_REAL_NGINX" -t -c "$TEST_CASE_DIR/nginx/validation.conf" || fail_test 'Real Nginx rejected generated proxy policy/locations'
  python3 "$scripts/tests/docsify-proxy-test.py" "$DOCSIFY_REAL_NGINX" "$TEST_CASE_DIR/nginx/validation.conf" \
    "$(state_value DOCSIFY_COOKIE_EPOCH)" "$(state_value DOCSIFY_STABLE_SLOT)" "$(state_value DOCSIFY_CANDIDATE_SLOT)" "$(state_value DOCSIFY_CANDIDATE_PERCENT)"
}

setup_case initial
run_ok gray-deploy-docsify.sh sha-abcdef1234567890
assert_equal "$(state_value DOCSIFY_STABLE_SLOT)" legacy 'Initial stable'
assert_equal "$(state_value DOCSIFY_CANDIDATE_PERCENT)" 0 'No public candidate traffic after deploy'
[[ -f "$(state_value DOCSIFY_BLUE_CONTENT)/scenarios/index.md" ]] || fail_test 'Incomplete extracted content'
grep -q -- '127.0.0.1:18083:8080' "$TEST_CASE_DIR/operations" || fail_test 'Candidate port is not loopback-only'
grep -q -- '--read-only' "$TEST_CASE_DIR/operations" || fail_test 'Container filesystem is writable'
grep -qF -- '--health-cmd wget -q -T 4 -O /dev/null http://127.0.0.1:8080/health' "$TEST_CASE_DIR/operations" || fail_test 'Container health command depends on image metadata'
for health_option in '--health-interval 10s' '--health-timeout 5s' '--health-start-period 5s' '--health-retries 3'; do
  grep -qF -- "$health_option" "$TEST_CASE_DIR/operations" || fail_test "Container health timing is missing: $health_option"
done
assert_equal "$(grep -cF "include $SUB2API_DOCSIFY_NGINX_LOCATIONS;" "$SUB2API_DOCSIFY_NGINX_MAIN")" 1 'Server include count'
! grep -qF "$SUB2API_DOCSIFY_NGINX_POLICY" "$SUB2API_DOCSIFY_NGINX_MAIN" || fail_test 'Policy would be included twice'
grep -qF 'proxy_pass $sub2api_docsify_entry_target$request_uri;' "$SUB2API_DOCSIFY_NGINX_LOCATIONS" || fail_test 'Documentation URI was stripped'
grep -qF 'max-age=123456; includeSubDomains' "$SUB2API_DOCSIFY_NGINX_LOCATIONS" || fail_test 'Inherited HSTS was lost'
grep -qF 'location ^~ /docs/assets/' "$SUB2API_DOCSIFY_NGINX_LOCATIONS" || fail_test 'Legacy assets do not have a stable route'
grep -qF 'map "" $sub2api_docsify_legacy_assets_target { default $sub2api_gray_target; }' "$SUB2API_DOCSIFY_NGINX_POLICY" || fail_test 'Initial legacy assets are not routed to the application'
assert_main_untouched
validate_real_proxy
pass_case 'Deploy at zero traffic, preserve app state, URI, headers and legacy assets'

run_ok gray-deploy-docsify.sh sha-abcdef1234567890
assert_equal "$(grep -c '^podman run ' "$TEST_CASE_DIR/operations")" 1 'Idempotent image deployment'
pass_case 'Repeating an immutable release does not replace its running container'
run_ok gray-set-docsify-traffic.sh 10
epoch_before="$(state_value DOCSIFY_COOKIE_EPOCH)"
run_ok gray-set-docsify-traffic.sh 50
assert_equal "$(state_value DOCSIFY_CANDIDATE_PERCENT)" 50 'Traffic percent'
[[ "$(state_value DOCSIFY_COOKIE_EPOCH)" != "$epoch_before" ]] || fail_test 'Traffic change reused sticky cookie epoch'
grep -qF '50% candidate;' "$SUB2API_DOCSIFY_NGINX_POLICY" || fail_test 'Expected split is absent'
reject gray-deploy-docsify.sh sha-fedcba0987654321
assert_main_untouched
validate_real_proxy
pass_case 'Progressive traffic changes invalidate prior cookies and forbid redeploying an active candidate'
reject gray-promote-docsify.sh
run_ok gray-set-docsify-traffic.sh 100
validate_real_proxy
run_ok gray-promote-docsify.sh
assert_equal "$(state_value DOCSIFY_STABLE_SLOT)" blue 'Promoted stable'
assert_equal "$(state_value DOCSIFY_CANDIDATE_SLOT)" green 'Next candidate'
grep -qF 'map "" $sub2api_docsify_legacy_assets_target { default $sub2api_docsify_content_target; }' "$SUB2API_DOCSIFY_NGINX_POLICY" || fail_test 'Promoted legacy asset paths still bypass Docsify'
run_ok gray-deploy-docsify.sh sha-fedcba0987654321
grep -q -- '127.0.0.1:18084:8080' "$TEST_CASE_DIR/operations" || fail_test 'Second slot does not use its independent port'
run_ok gray-set-docsify-traffic.sh 100
run_ok gray-promote-docsify.sh
run_ok gray-deploy-docsify.sh sha-1111111111111111
grep -q '^podman stop sub2api-docsify-blue-sha-abcdef1234567890$' "$TEST_CASE_DIR/operations" || fail_test 'Previous zero-traffic container was not preserved before slot reuse'
[[ -f "$TEST_CASE_DIR/containers/sub2api-docsify-blue-sha-abcdef1234567890" ]] || fail_test 'Old Docsify container was destroyed'
assert_main_untouched
validate_real_proxy
pass_case 'Promotion requires 100%, rotates two slots and retains replaced containers'

for invalid in latest main sha-invalid; do
  setup_case "invalid-$invalid"
  reject gray-deploy-docsify.sh "$invalid"
  [[ ! -f "$TEST_CASE_DIR/operations" ]] || ! grep -q '^podman ' "$TEST_CASE_DIR/operations" || fail_test 'Invalid tag reached Podman'
  pass_case "Reject mutable or invalid image tag: $invalid"
done

setup_case unsafe-content
export MOCK_BAD_IMAGE=1
reject gray-deploy-docsify.sh sha-abcdef1234567890
[[ ! -f "$SUB2API_DOCSIFY_NGINX_POLICY" ]] || fail_test 'Unsafe content was published'
! grep -q '^podman run ' "$TEST_CASE_DIR/operations" || fail_test 'Unsafe image was started'
pass_case 'Reject maintenance files extracted from an image'

setup_case nginx-failure
cp "$SUB2API_DOCSIFY_NGINX_MAIN" "$TEST_CASE_DIR/original.conf"
export MOCK_NGINX_FAIL=1
reject gray-deploy-docsify.sh sha-abcdef1234567890
cmp "$SUB2API_DOCSIFY_NGINX_MAIN" "$TEST_CASE_DIR/original.conf" || fail_test 'Nginx validation failure did not restore the server config'
[[ ! -e "$SUB2API_DOCSIFY_NGINX_POLICY" && ! -e "$SUB2API_DOCSIFY_NGINX_LOCATIONS" ]] || fail_test 'Nginx validation failure left new includes'
assert_equal "$(state_value DOCSIFY_CANDIDATE_PERCENT)" 0 'Nginx failure traffic'
[[ "$(find "$TEST_CASE_DIR/containers" -type f | wc -l)" == 1 ]] || fail_test 'Nginx failure destroyed the candidate'
pass_case 'Nginx validation failure restores config and retains candidate evidence'

setup_case reload-failure
cp "$SUB2API_DOCSIFY_NGINX_MAIN" "$TEST_CASE_DIR/original.conf"
export MOCK_RELOAD_FAIL=1
reject gray-deploy-docsify.sh sha-abcdef1234567890
cmp "$SUB2API_DOCSIFY_NGINX_MAIN" "$TEST_CASE_DIR/original.conf" || fail_test 'Reload failure did not restore the server config'
[[ ! -e "$SUB2API_DOCSIFY_NGINX_POLICY" ]] || fail_test 'Reload failure left the new policy'
pass_case 'Reload failure restores previous config'

setup_case health-failure
run_ok gray-deploy-docsify.sh sha-abcdef1234567890
run_ok gray-set-docsify-traffic.sh 10
export MOCK_UNHEALTHY=1
reject gray-set-docsify-traffic.sh 50
assert_equal "$(state_value DOCSIFY_CANDIDATE_PERCENT)" 0 'Automatic health rollback'
grep -qF 'default stable;' "$SUB2API_DOCSIFY_NGINX_POLICY" || fail_test 'Unhealthy candidate still has public entry traffic'
[[ "$(find "$TEST_CASE_DIR/containers" -type f | wc -l)" == 1 ]] || fail_test 'Health rollback removed candidate'
assert_main_untouched
validate_real_proxy
pass_case 'Health regression sends new documentation entries to stable without destroying candidate'

setup_case bad-percent
run_ok gray-deploy-docsify.sh sha-abcdef1234567890
for invalid in -1 101 10.5 x; do reject gray-set-docsify-traffic.sh "$invalid"; done
pass_case 'Reject invalid traffic percentages'

setup_case symlink-state
mkdir -p "$TEST_CASE_DIR/elsewhere"
ln -s "$TEST_CASE_DIR/elsewhere" "$TEST_CASE_DIR/gray/docsify"
reject gray-deploy-docsify.sh sha-abcdef1234567890
[[ -z "$(find "$TEST_CASE_DIR/elsewhere" -type f -print -quit)" ]] || fail_test 'Writes followed a symlink state root'
pass_case 'Reject a symlink documentation state directory'

setup_case observation
run_ok gray-deploy-docsify.sh sha-abcdef1234567890
run_ok gray-set-docsify-traffic.sh 100
export SUB2API_DOCSIFY_OBSERVATION_SECONDS=300
reject gray-promote-docsify.sh
assert_equal "$(state_value DOCSIFY_STABLE_SLOT)" legacy 'Observation stable'
pass_case 'Promotion enforces the observation interval'

for prior_percent in 0 10; do
  setup_case "traffic-save-failure-$prior_percent"
  run_ok gray-deploy-docsify.sh sha-abcdef1234567890
  [[ "$prior_percent" == 0 ]] || run_ok gray-set-docsify-traffic.sh "$prior_percent"
  export MOCK_STATE_SAVE_FAIL=once
  reject gray-set-docsify-traffic.sh 50
  assert_equal "$(state_value DOCSIFY_CANDIDATE_PERCENT)" 0 'State-save failure traffic rollback'
  grep -qF 'map "" $sub2api_docsify_route { default stable; }' "$SUB2API_DOCSIFY_NGINX_POLICY" || fail_test 'State save failure left candidate entry traffic'
  assert_main_untouched
  pass_case "State save failure rolls traffic back from a previous $prior_percent%"
done

setup_case promote-save-failure
run_ok gray-deploy-docsify.sh sha-abcdef1234567890
run_ok gray-set-docsify-traffic.sh 100
export MOCK_STATE_SAVE_FAIL=once
reject gray-promote-docsify.sh
assert_equal "$(state_value DOCSIFY_STABLE_SLOT)" legacy 'Failed promotion stable slot'
assert_equal "$(state_value DOCSIFY_CANDIDATE_SLOT)" blue 'Failed promotion candidate slot'
assert_equal "$(state_value DOCSIFY_CANDIDATE_PERCENT)" 0 'Failed promotion traffic'
grep -qF 'map "" $sub2api_docsify_route { default stable; }' "$SUB2API_DOCSIFY_NGINX_POLICY" || fail_test 'Promotion save failure left candidate traffic'
assert_main_untouched
pass_case 'Promotion state save failure restores prior stable route and state'

setup_case rollback-save-failure
run_ok gray-deploy-docsify.sh sha-abcdef1234567890
run_ok gray-set-docsify-traffic.sh 10
export MOCK_STATE_SAVE_FAIL=always MOCK_UNHEALTHY=1
reject gray-set-docsify-traffic.sh 50
grep -qF 'saving rollback state failed' "$TEST_CASE_DIR/output" || fail_test 'Rollback state failure was not reported'
! grep -qF 'Docsify entries restored to stable slot' "$TEST_CASE_DIR/output" || fail_test 'Rollback state failure claimed complete success'
grep -qF 'map "" $sub2api_docsify_route { default stable; }' "$SUB2API_DOCSIFY_NGINX_POLICY" || fail_test 'Rollback state failure did not cut traffic'
pass_case 'Rollback save failure cuts entry traffic and reports incomplete state accurately'

setup_case deploy-save-failure
export MOCK_STATE_SAVE_FAIL=once
reject gray-deploy-docsify.sh sha-abcdef1234567890
[[ ! -f "$SUB2API_DOCSIFY_NGINX_POLICY" && ! -f "$TEST_CASE_DIR/gray/docsify/state.env" ]] || fail_test 'Deployment save failure changed routing/state'
[[ "$(find "$TEST_CASE_DIR/containers" -type f | wc -l)" == 1 ]] || fail_test 'Deployment save failure destroyed the candidate'
grep -qF 'saving release metadata failed' "$TEST_CASE_DIR/output" || fail_test 'Deployment save failure was not reported accurately'
pass_case 'Deployment metadata failure retains container without installing proxy routing'

setup_case status-health-failure
run_ok gray-deploy-docsify.sh sha-abcdef1234567890
run_ok gray-set-docsify-traffic.sh 10
export MOCK_UNHEALTHY=1
reject gray-docsify-status.sh
assert_equal "$(state_value DOCSIFY_CANDIDATE_PERCENT)" 0 'Status health rollback'
pass_case 'Status health verification rolls unhealthy active entry traffic back'

printf '%s Docsify gray protection cases passed\n' "$passed"
