#!/usr/bin/env bash
# Run inside the frontend-test Compose service. All gray and Podman operations
# are fixtures; only rsync and filesystem operations run against temporary files.
set -Eeuo pipefail
script_directory="$(cd "$(dirname "$0")" && pwd)"
prepare_script="${TEST_PREPARE_SCRIPT:-$script_directory/../gray-prepare-documentation.sh}"
test_root="$(mktemp -d)"
trap 'rm -rf -- "$test_root"' EXIT
passed=0

fail_test() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
assert_file() { [[ -f "$1" ]] || fail_test "Missing file: $1"; }
assert_content() { [[ "$(cat "$1")" == "$2" ]] || fail_test "Unexpected content: $1"; }
assert_cleaned() {
  [[ -z "$(find "$TEST_CASE_DIR/gray" -maxdepth 1 -name '.docs-*' -print -quit)" ]] || fail_test 'Staging directory was not cleaned'
}
case_passed() { passed=$((passed + 1)); printf 'PASS: %s\n' "$1"; }

setup_case() {
  export TEST_CASE_DIR="$test_root/$1"
  export MOCK_STATE_CHANGE='' MOCK_CP_FAIL=0 MOCK_RM_FAIL=0 MOCK_RSYNC_FAIL=0
  mkdir -p "$TEST_CASE_DIR/scripts" "$TEST_CASE_DIR/bin" "$TEST_CASE_DIR/gray/docs-blue/assets" "$TEST_CASE_DIR/gray/docs-green/assets" "$TEST_CASE_DIR/image/assets" "$TEST_CASE_DIR/image/scenarios" "$TEST_CASE_DIR/outside"
  cp "$prepare_script" "$TEST_CASE_DIR/scripts/gray-prepare-documentation.sh"
  cat >"$TEST_CASE_DIR/state.env" <<'STATE'
STABLE_SLOT=blue
CANDIDATE_SLOT=green
CANDIDATE_PERCENT=0
STATE
  cat >"$TEST_CASE_DIR/scripts/gray-common.sh" <<'COMMON'
SCRIPTS_DIR="$TEST_CASE_DIR/scripts"
GRAY_DIR="$TEST_CASE_DIR/gray"
GHCR_IMAGE=ghcr.io/example/sub2api
valid_slot() { [[ "$1" == blue || "$1" == green ]]; }
load_state() { source "$TEST_CASE_DIR/state.env"; }
fail() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }
info() { printf '%s\n' "$*"; }
COMMON
  cat >"$TEST_CASE_DIR/scripts/gray-status.sh" <<'STATUS'
#!/usr/bin/env bash
printf 'status\n' >>"$TEST_CASE_DIR/operations"
STATUS
  cat >"$TEST_CASE_DIR/bin/podman" <<'PODMAN'
#!/usr/bin/env bash
set -euo pipefail
printf 'podman %s\n' "$*" >>"$TEST_CASE_DIR/operations"
case "$1" in
  pull) ;;
  create) printf 'documentation-export-fixture\n' ;;
  cp)
    [[ "$MOCK_CP_FAIL" == 0 ]] || exit 33
    cp -a "$TEST_CASE_DIR/image/." "$3"
    if [[ "$MOCK_STATE_CHANGE" == traffic ]]; then
      printf 'STABLE_SLOT=blue\nCANDIDATE_SLOT=green\nCANDIDATE_PERCENT=10\n' >"$TEST_CASE_DIR/state.env"
    elif [[ "$MOCK_STATE_CHANGE" == promotion ]]; then
      printf 'STABLE_SLOT=green\nCANDIDATE_SLOT=blue\nCANDIDATE_PERCENT=0\n' >"$TEST_CASE_DIR/state.env"
    fi
    ;;
  rm)
    [[ "$2" == documentation-export-fixture ]] || exit 90
    [[ "$MOCK_RM_FAIL" == 0 ]] || exit 34
    ;;
  *) exit 91 ;;
esac
PODMAN
  cat >"$TEST_CASE_DIR/bin/rsync" <<'RSYNC'
#!/usr/bin/env bash
set -euo pipefail
[[ "$MOCK_RSYNC_FAIL" == 0 ]] || exit 35
exec /usr/bin/rsync "$@"
RSYNC
  chmod 755 "$TEST_CASE_DIR/scripts/gray-status.sh" "$TEST_CASE_DIR/bin/podman" "$TEST_CASE_DIR/bin/rsync"
  printf 'new-home' >"$TEST_CASE_DIR/image/index.html"
  printf 'new-404' >"$TEST_CASE_DIR/image/404.html"
  printf 'nested-index' >"$TEST_CASE_DIR/image/scenarios/index.html"
  printf 'new-chunk' >"$TEST_CASE_DIR/image/assets/app.NewHash1.js"
  printf 'new-shared' >"$TEST_CASE_DIR/image/assets/shared.js"
  printf 'candidate-before' >"$TEST_CASE_DIR/gray/docs-green/index.html"
  printf 'candidate-old-chunk' >"$TEST_CASE_DIR/gray/docs-green/assets/page.OldHash1.js"
  printf 'stable-home' >"$TEST_CASE_DIR/gray/docs-blue/index.html"
  printf 'stable-old-chunk' >"$TEST_CASE_DIR/gray/docs-blue/assets/page.OldHash2.js"
  printf 'stable-shared' >"$TEST_CASE_DIR/gray/docs-blue/assets/shared.js"
  printf 'outside-sentinel' >"$TEST_CASE_DIR/outside/sentinel"
}

run_prepare() {
  PATH="$TEST_CASE_DIR/bin:$PATH" bash "$TEST_CASE_DIR/scripts/gray-prepare-documentation.sh" green sha-abcdef123456 >"$TEST_CASE_DIR/output" 2>&1
}

expect_rejected() {
  if run_prepare; then fail_test 'Unsafe preparation was accepted'; fi
  assert_content "$TEST_CASE_DIR/gray/docs-blue/index.html" stable-home
  assert_content "$TEST_CASE_DIR/gray/docs-green/index.html" candidate-before
  [[ ! -e "$TEST_CASE_DIR/gray/docs-green-release" ]] || fail_test 'A rejected preparation wrote a release marker'
  assert_cleaned
}

setup_case normal
run_prepare || { cat "$TEST_CASE_DIR/output"; fail_test 'Normal preparation failed'; }
assert_content "$TEST_CASE_DIR/gray/docs-green/index.html" new-home
assert_content "$TEST_CASE_DIR/gray/docs-green/scenarios/index.html" nested-index
assert_content "$TEST_CASE_DIR/gray/docs-blue/index.html" stable-home
assert_content "$TEST_CASE_DIR/gray/docs-blue/assets/shared.js" stable-shared
for slot in blue green; do
  for asset in app.NewHash1.js page.OldHash1.js page.OldHash2.js; do
    assert_file "$TEST_CASE_DIR/gray/docs-$slot/assets/$asset"
  done
done
assert_content "$TEST_CASE_DIR/gray/docs-green-release" ghcr.io/example/sub2api:sha-abcdef123456
[[ "$(stat -c %a "$TEST_CASE_DIR/gray/docs-green/index.html")" == 644 ]] || fail_test 'Wrong HTML permissions'
[[ "$(stat -c %a "$TEST_CASE_DIR/gray/docs-green/assets")" == 755 ]] || fail_test 'Wrong directory permissions'
backup_index="$(find "$TEST_CASE_DIR/gray/docs-releases" -name index.html -print -quit)"
assert_content "$backup_index" candidate-before
[[ "$(awk '/^status$/ { count++ } END { print count }' "$TEST_CASE_DIR/operations")" == 2 ]] || fail_test 'Gray state was not rechecked'
assert_cleaned
case_passed 'Complete site, nested index, backup, permissions and both asset generations'

setup_case nonzero
printf 'STABLE_SLOT=blue\nCANDIDATE_SLOT=green\nCANDIDATE_PERCENT=10\n' >"$TEST_CASE_DIR/state.env"
expect_rejected
[[ "$(cat "$TEST_CASE_DIR/operations")" == status ]] || fail_test 'Nonzero candidate triggered Podman'
case_passed 'Reject a candidate with traffic before image extraction'

for change in traffic promotion; do
  setup_case "state-$change"
  export MOCK_STATE_CHANGE="$change"
  expect_rejected
  case_passed "Reject state change during extraction: $change"
done

for linked_path in docs-blue docs-blue/assets docs-blue/assets/nested docs-green docs-green/assets docs-green/assets/nested docs-releases; do
  setup_case "symlink-${linked_path//\//-}"
  path="$TEST_CASE_DIR/gray/$linked_path"
  if [[ -e "$path" ]]; then mv "$path" "$path-original"; fi
  ln -s "$TEST_CASE_DIR/outside" "$path"
  if run_prepare; then fail_test "Accepted symlink: $linked_path"; fi
  assert_content "$TEST_CASE_DIR/outside/sentinel" outside-sentinel
  [[ "$(find "$TEST_CASE_DIR/outside" -type f | wc -l)" == 1 ]] || fail_test 'Wrote outside documentation directories'
  [[ ! -e "$TEST_CASE_DIR/gray/docs-green-release" ]] || fail_test 'Symlink rejection wrote a release marker'
  assert_cleaned
  case_passed "Reject symlink: $linked_path"
done

setup_case image-symlink
ln -s "$TEST_CASE_DIR/outside" "$TEST_CASE_DIR/image/assets/outside"
expect_rejected
case_passed 'Reject symlinks extracted from the image'

setup_case marker-symlink
ln -s "$TEST_CASE_DIR/outside/sentinel" "$TEST_CASE_DIR/gray/docs-green-release"
if run_prepare; then fail_test 'Accepted a symlink release marker'; fi
assert_content "$TEST_CASE_DIR/outside/sentinel" outside-sentinel
assert_content "$TEST_CASE_DIR/gray/docs-green/index.html" candidate-before
assert_cleaned
case_passed 'Reject release marker symlink'

setup_case rsync-failure
export MOCK_RSYNC_FAIL=1
expect_rejected
case_passed 'Resource failure does not publish HTML or a release marker'

setup_case export-and-cleanup-failure
export MOCK_CP_FAIL=1 MOCK_RM_FAIL=1
exit_status=0
run_prepare || exit_status=$?
[[ "$exit_status" == 33 ]] || fail_test "Cleanup masked export failure: $exit_status"
assert_content "$TEST_CASE_DIR/gray/docs-blue/index.html" stable-home
assert_content "$TEST_CASE_DIR/gray/docs-green/index.html" candidate-before
assert_cleaned
case_passed 'Cleanup keeps the original failure and removes staging files'

printf '%s mocked documentation preparation cases passed\n' "$passed"
