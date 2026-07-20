#!/usr/bin/env bash

set -euo pipefail

source_script=$(cd "$(dirname "$0")" && pwd)/capture-provider-benchmark.sh
test_root=$(mktemp -d "${TMPDIR:-/tmp}/capture-provider-benchmark-test.XXXXXX")
trap 'rm -rf "$test_root"' EXIT

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

assert_file_contains() {
  local file=$1
  local pattern=$2
  grep -Eq "$pattern" "$file" || fail "$file does not contain $pattern"
}

assert_file_excludes() {
  local file=$1
  local pattern=$2
  if grep -Eiq "$pattern" "$file"; then
    fail "$file contains prohibited pattern $pattern"
  fi
}

setup_fixture() {
  local name=$1
  local fixture=$test_root/$name

  mkdir -p "$fixture/repo/scripts" "$fixture/bin"
  cp "$source_script" "$fixture/repo/scripts/capture-provider-benchmark.sh"
  chmod +x "$fixture/repo/scripts/capture-provider-benchmark.sh"
  cat >"$fixture/bin/go" <<'FAKE_GO'
#!/usr/bin/env bash
set -eu
printf '%s\n' "${FAKE_GO_OUTPUT:-BenchmarkProvider-8 1 100 ns/op}"
exit "${FAKE_GO_EXIT:-0}"
FAKE_GO
  chmod +x "$fixture/bin/go"

  git -C "$fixture/repo" init -q
  git -C "$fixture/repo" config user.name benchmark-test
  git -C "$fixture/repo" config user.email benchmark-test@example.invalid
  git -C "$fixture/repo" add scripts/capture-provider-benchmark.sh
  git -C "$fixture/repo" commit -qm fixture

  printf '%s\n' "$fixture"
}

run_capture() {
  local fixture=$1
  shift
  (
    cd "$fixture/repo"
    PATH="$fixture/bin:$PATH" \
      BLUETAPE_PROVIDER_BENCH_OUTPUT_DIR=docs/research/outputs/issue-560 \
      "$fixture/repo/scripts/capture-provider-benchmark.sh" "$@"
  )
}

assert_success_writes_atomic_canonical_output() {
  local fixture
  fixture=$(setup_fixture success)

  run_capture "$fixture" graphio

  local output=$fixture/repo/docs/research/outputs/issue-560/graphio.txt
  test -f "$output" || fail "canonical output was not written"
  assert_file_contains "$output" '^exit_status: 0$'
  assert_file_contains "$output" '^BenchmarkProvider-8 1 100 ns/op$'
  if find "$(dirname "$output")" -maxdepth 1 -name '.*.tmp.*' | grep -q .; then
    fail "artifact-local temporary file was retained"
  fi
}

assert_failure_preserves_previous_success() {
  local fixture
  fixture=$(setup_fixture preserve)
  run_capture "$fixture" graphio

  local output=$fixture/repo/docs/research/outputs/issue-560/graphio.txt
  local before
  before=$(git hash-object "$output")
  if FAKE_GO_EXIT=23 FAKE_GO_OUTPUT='provider failed' run_capture "$fixture" graphio; then
    fail "failed benchmark capture returned success"
  else
    test "$?" -eq 23 || fail "failed benchmark capture changed the command exit status"
  fi
  test "$(git hash-object "$output")" = "$before" || fail "failure replaced canonical output"
}

assert_failure_writes_timestamped_failure_output() {
  local fixture
  fixture=$(setup_fixture failure)

  if FAKE_GO_EXIT=17 FAKE_GO_OUTPUT='provider failed' run_capture "$fixture" graphio; then
    fail "failed benchmark capture returned success"
  else
    test "$?" -eq 17 || fail "failed benchmark capture changed the command exit status"
  fi

  local failed
  failed=$(find "$fixture/repo/docs/research/outputs/issue-560" -maxdepth 1 -name 'graphio-failed-*.txt' -print -quit)
  test -n "$failed" || fail "timestamped failure artifact was not written"
  assert_file_contains "$failed" '^exit_status: 17$'
  test ! -e "$fixture/repo/docs/research/outputs/issue-560/graphio.txt" || fail "failure created canonical output"
}

assert_unknown_family_fails_before_command() {
  local fixture
  fixture=$(setup_fixture unknown)

  if FAKE_GO_OUTPUT='must-not-run' run_capture "$fixture" unknown-family; then
    fail "unknown family returned success"
  fi
  test ! -e "$fixture/repo/docs" || fail "unknown family created artifacts"

  mkdir -p "$fixture/repo/leader"
  printf 'dirty\n' >"$fixture/repo/leader/dirty.go"
  if (
    cd "$fixture/repo"
    PATH="$fixture/bin:$PATH" BLUETAPE_PROVIDER_BENCH_OUTPUT_DIR=leader \
      "$fixture/repo/scripts/capture-provider-benchmark.sh" graphio
  ); then
    fail "output override was allowed to hide dirty source"
  fi
  test ! -e "$fixture/repo/leader/graphio.txt" || fail "invalid output override created an artifact"
}

assert_secret_pattern_blocks_canonical_output() {
  local fixture
  fixture=$(setup_fixture secret-success)

  FAKE_GO_OUTPUT='authorization=raw-success-secret
endpoint=https://db.internal:8443
HTTPS_PROXY=http://proxy.internal:8080
host_path=/private/var/raw-host-path
container_id=0123456789abcdef0123456789abcdef
AWS_SECRET_ACCESS_KEY=raw-cloud-secret' run_capture "$fixture" graphio

  local output=$fixture/repo/docs/research/outputs/issue-560/graphio.txt
  assert_file_excludes "$output" 'raw-success-secret|raw-cloud-secret|db\.internal|proxy\.internal|raw-host-path|0123456789abcdef|authorization[=:][^[:space:]]+|AWS_SECRET_ACCESS_KEY'
  assert_file_contains "$output" '^\[redacted_output_line\]$'
}

assert_secret_bearing_failure_is_sanitized_before_retention() {
  local fixture
  fixture=$(setup_fixture secret-failure)

  if FAKE_GO_EXIT=29 FAKE_GO_OUTPUT='redis://bench:raw-failure-secret@localhost:6379/0' run_capture "$fixture" graphio; then
    fail "secret-bearing failure returned success"
  else
    test "$?" -eq 29 || fail "secret-bearing failure changed the command exit status"
  fi

  local failed
  failed=$(find "$fixture/repo/docs/research/outputs/issue-560" -maxdepth 1 -name 'graphio-failed-*.txt' -print -quit)
  test -n "$failed" || fail "secret-bearing failure artifact was not written"
  assert_file_excludes "$failed" 'raw-failure-secret|://[^/@[:space:]]+:[^/@[:space:]]+@'
  assert_file_contains "$failed" '^\[redacted_output_line\]$'
}

assert_command_timestamp_sha_and_exit_status_headers_exist() {
  local fixture
  fixture=$(setup_fixture headers)

  run_capture "$fixture" leader-containers

  local output=$fixture/repo/docs/research/outputs/issue-560/leader-containers.txt
  test "$(grep -c '^command:' "$output")" -eq 2 || fail "leader capture does not contain two command headers"
  test "$(grep -c '^timestamp_utc:' "$output")" -eq 2 || fail "leader capture does not contain two timestamps"
  test "$(grep -Ec '^git_sha: [0-9a-f]{40}$' "$output")" -eq 2 || fail "leader capture does not contain two Git SHAs"
  test "$(grep -c '^pre_run_clean: true$' "$output")" -eq 2 || fail "leader capture does not contain clean-state headers"
  test "$(grep -c '^exit_status: 0$' "$output")" -eq 2 || fail "leader capture does not contain two exit statuses"
}

tests=(
  assert_success_writes_atomic_canonical_output
  assert_failure_preserves_previous_success
  assert_failure_writes_timestamped_failure_output
  assert_unknown_family_fails_before_command
  assert_secret_pattern_blocks_canonical_output
  assert_secret_bearing_failure_is_sanitized_before_retention
  assert_command_timestamp_sha_and_exit_status_headers_exist
)

for test_name in "${tests[@]}"; do
  "$test_name"
  printf 'PASS: %s\n' "$test_name"
done
