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

assert_file_has_no_blank_line_at_eof() {
  local file=$1
  local suffix
  suffix=$(tail -c 2 "$file" | od -An -tx1 | tr -d '[:space:]')
  test "$suffix" != '0a0a' || fail "$file ends with a blank line"
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
  assert_file_has_no_blank_line_at_eof "$output"
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
  printf '%s\n' "${failed##*/}" | grep -Eq '^graphio-failed-[0-9]{8}T[0-9]{6}Z-[0-9]+\.txt$' || fail "failure artifact has no collision-resistant suffix"
  assert_file_contains "$failed" '^exit_status: 17$'
  test ! -e "$fixture/repo/docs/research/outputs/issue-560/graphio.txt" || fail "failure created canonical output"

  fixture=$(setup_fixture unwritable-output)
  mkdir -p "$fixture/repo/docs/research/outputs/issue-560"
  chmod 500 "$fixture/repo/docs/research/outputs/issue-560"
  if FAKE_GO_EXIT=31 FAKE_GO_OUTPUT='publish failed' run_capture "$fixture" graphio; then
    chmod 700 "$fixture/repo/docs/research/outputs/issue-560"
    fail "unwritable artifact directory returned success"
  else
    local publish_status=$?
    chmod 700 "$fixture/repo/docs/research/outputs/issue-560"
    test "$publish_status" -eq 31 || fail "publish failure changed the benchmark exit status"
  fi
  test ! -e "$fixture/repo/sanitized.txt" || fail "publish failure escaped the artifact directory"
  test -z "$(find "$fixture/repo/docs/research/outputs/issue-560" -maxdepth 1 -type f -print -quit)" || fail "publish failure retained a partial artifact"
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
AWS_SECRET_ACCESS_KEY=raw-cloud-secret
{"password":"raw-json-secret"}
{"authorization":"Bearer raw-json-token"}
{"api-key":"raw-api-key-value"}
{"access-token":"raw-access-token-value"}
{"clientSecret":"raw-client-secret-value"}
-----BEGIN PRIVATE KEY-----
raw-pem-material
-----END PRIVATE KEY-----' run_capture "$fixture" graphio

  local output=$fixture/repo/docs/research/outputs/issue-560/graphio.txt
  assert_file_excludes "$output" 'raw-success-secret|raw-cloud-secret|raw-json-secret|raw-json-token|raw-api-key-value|raw-access-token-value|raw-client-secret-value|raw-pem-material|BEGIN PRIVATE KEY|db\.internal|proxy\.internal|raw-host-path|0123456789abcdef|authorization[=:][^[:space:]]+|AWS_SECRET_ACCESS_KEY'
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

assert_oversized_output_fails_without_canonical_artifact() {
  local fixture
  fixture=$(setup_fixture oversized-output)
  local oversized
  oversized=$(printf '%0256d' 0)

  if BLUETAPE_PROVIDER_BENCH_MAX_OUTPUT_BYTES=64 FAKE_GO_OUTPUT="$oversized" run_capture "$fixture" graphio; then
    fail "oversized benchmark output returned success"
  else
    test "$?" -eq 125 || fail "oversized benchmark output did not fail closed"
  fi

  local output_dir=$fixture/repo/docs/research/outputs/issue-560
  test ! -e "$output_dir/graphio.txt" || fail "oversized output created a canonical artifact"
  local failed
  failed=$(find "$output_dir" -maxdepth 1 -name 'graphio-failed-*.txt' -print -quit)
  test -n "$failed" || fail "oversized output did not retain a failure artifact"
  assert_file_contains "$failed" '^\[output_truncated_at_64_bytes\]$'
}

assert_output_at_exact_limit_succeeds() {
  local fixture
  fixture=$(setup_fixture exact-output-limit)
  local exact
  exact=$(printf '%063d' 0)

  BLUETAPE_PROVIDER_BENCH_MAX_OUTPUT_BYTES=64 FAKE_GO_OUTPUT="$exact" run_capture "$fixture" graphio

  local output=$fixture/repo/docs/research/outputs/issue-560/graphio.txt
  test -f "$output" || fail "output at the exact limit did not create a canonical artifact"
  assert_file_contains "$output" '^exit_status: 0$'
  assert_file_excludes "$output" 'output_truncated_at_64_bytes'
}

assert_output_limit_override_cannot_exceed_default() {
  local fixture
  fixture=$(setup_fixture oversized-limit)

  if BLUETAPE_PROVIDER_BENCH_MAX_OUTPUT_BYTES=16777217 run_capture "$fixture" graphio; then
    fail "output limit override increased the default ceiling"
  else
    test "$?" -eq 2 || fail "oversized output limit did not fail as invalid input"
  fi
  test ! -e "$fixture/repo/docs" || fail "oversized output limit created artifacts"
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
  test "$(grep -c '^max_output_bytes: 16777216$' "$output")" -eq 2 || fail "leader capture does not contain output limits"
  test "$(grep -c '^exit_status: 0$' "$output")" -eq 2 || fail "leader capture does not contain two exit statuses"
}

tests=(
  assert_success_writes_atomic_canonical_output
  assert_failure_preserves_previous_success
  assert_failure_writes_timestamped_failure_output
  assert_unknown_family_fails_before_command
  assert_secret_pattern_blocks_canonical_output
  assert_secret_bearing_failure_is_sanitized_before_retention
  assert_oversized_output_fails_without_canonical_artifact
  assert_output_at_exact_limit_succeeds
  assert_output_limit_override_cannot_exceed_default
  assert_command_timestamp_sha_and_exit_status_headers_exist
)

for test_name in "${tests[@]}"; do
  "$test_name"
  printf 'PASS: %s\n' "$test_name"
done
