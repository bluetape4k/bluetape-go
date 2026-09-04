#!/usr/bin/env bash

set -euo pipefail

script_dir=$(cd "$(dirname "$0")" && pwd -P)
capture_script=$script_dir/capture-gin-adapter-benchmark.sh
parser=$script_dir/parse-gin-adapter-benchmark.py
comparator=$script_dir/compare-gin-adapter-benchmark.py
chart_generator=$(cd "$script_dir/../docs/images/readme-charts" && pwd -P)/generate-gin-adapter-benchmark-summary.mjs
test_root=$(mktemp -d "${TMPDIR:-/tmp}/capture-gin-adapter-benchmark-test.XXXXXX")
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

write_benchmark_rows() {
  local destination=$1
  printf '%s\n' \
    'BenchmarkGinAdapter/NoOp/Serial-1 1 10 ns/op 1 B/op 1 allocs/op' \
    'BenchmarkGinAdapter/NoOp/Parallel-1 1 20 ns/op 2 B/op 1 allocs/op' \
    'BenchmarkGinAdapter/DirectCore/Serial-1 1 30 ns/op 3 B/op 1 allocs/op' \
    'BenchmarkGinAdapter/DirectCore/Parallel-1 1 40 ns/op 4 B/op 1 allocs/op' \
    'BenchmarkGinAdapter/Bridge/Serial-1 1 50 ns/op 5 B/op 1 allocs/op' \
    'BenchmarkGinAdapter/Bridge/Parallel-1 1 60 ns/op 6 B/op 1 allocs/op' \
    'BenchmarkGinAdapter/FullAdapter/Serial-1 1 70 ns/op 7 B/op 1 allocs/op' \
    'BenchmarkGinAdapter/FullAdapter/Parallel-1 1 80 ns/op 8 B/op 1 allocs/op' \
    'BenchmarkGinAdapter/FullAdapterRetry/Serial-1 1 85 ns/op 8 B/op 1 allocs/op' \
    'BenchmarkGinAdapter/FullAdapterRetry/Parallel-1 1 95 ns/op 9 B/op 1 allocs/op' \
    'BenchmarkGinAdapterColdConstruction-1 1 90 ns/op 9 B/op 1 allocs/op' \
    'BenchmarkGinAdapterColdFirstRequest-1 1 100 ns/op 10 B/op 1 allocs/op' \
    'BenchmarkGinAdapterWarmRequest/Serial-1 1 110 ns/op 11 B/op 1 allocs/op' \
    'BenchmarkGinAdapterWarmRequest/Parallel-1 1 120 ns/op 12 B/op 1 allocs/op' \
    >"$destination"
}

setup_fixture() {
  local name=$1
  local fixture=$test_root/$name
  local real_node
  real_node=$(command -v node)
  mkdir -p "$fixture/repo/scripts" "$fixture/repo/docs/images/readme-charts" "$fixture/bin"
  cp "$capture_script" "$fixture/repo/scripts/capture-gin-adapter-benchmark.sh"
  cp "$parser" "$fixture/repo/scripts/parse-gin-adapter-benchmark.py"
  cp "$comparator" "$fixture/repo/scripts/compare-gin-adapter-benchmark.py"
  cp "$chart_generator" "$fixture/repo/docs/images/readme-charts/generate-gin-adapter-benchmark-summary.mjs"
  chmod +x "$fixture/repo/scripts/capture-gin-adapter-benchmark.sh" "$fixture/repo/scripts/parse-gin-adapter-benchmark.py"
  cat >"$fixture/bin/go" <<'FAKE_GO'
#!/usr/bin/env bash
set -eu
case "${1:-}" in
  version)
    printf '%s\n' 'go version go1.26.6 darwin/arm64'
    ;;
  env)
    case "${2:-}" in
      GOOS) printf '%s\n' darwin ;;
      GOARCH) printf '%s\n' arm64 ;;
      *) printf '%s\n' unknown ;;
    esac
    ;;
  list)
    printf '%s\n' v1.12.0
    ;;
  test)
    if [ "${FAKE_GO_SIGNAL:-0}" = 1 ]; then
      kill -TERM "${BLUETAPE_GIN_BENCH_CAPTURE_PID:?}"
      sleep 30
    fi
    if [ "${FAKE_GO_BLOCK:-0}" = 1 ]; then
      sleep 30
    fi
    cat "$FAKE_GO_OUTPUT_FILE"
    exit "${FAKE_GO_EXIT:-0}"
    ;;
  *)
    exit 0
    ;;
esac
FAKE_GO
  chmod +x "$fixture/bin/go"
  cat >"$fixture/bin/rsvg-convert" <<'FAKE_RSVG'
#!/usr/bin/env bash
set -eu
output=''
while [ "$#" -gt 0 ]; do
  if [ "$1" = '--output' ]; then
    output=$2
    shift 2
  else
    shift
  fi
done
test -n "$output"
printf '%s\n' 'fixture png' >"$output"
FAKE_RSVG
  chmod +x "$fixture/bin/rsvg-convert"
  cat >"$fixture/bin/node" <<'FAKE_NODE'
#!/usr/bin/env bash
set -eu
if [ "${FAKE_NODE_SIGNAL:-0}" = 1 ]; then
  printf '%s\n' "${FAKE_NODE_STDERR:-chart renderer signal stderr}" >&2
  kill -TERM "$$"
fi
if [ "${FAKE_NODE_MALFORMED:-0}" = 1 ]; then
  printf '%s\n' '{"metadata": {}, "rows": []}' >"${2:?chart input path is required}"
fi
if [ "${FAKE_NODE_BLOCK:-0}" = 1 ]; then
  printf '%s\n' "${FAKE_NODE_STDERR:-chart renderer timeout stderr}" >&2
  sleep 30
fi
if [ "${FAKE_NODE_EXIT:-0}" != 0 ]; then
  printf '%s\n' "${FAKE_NODE_STDERR:-chart renderer exit stderr}" >&2
  exit "$FAKE_NODE_EXIT"
fi
exec "$(cat "$(dirname "$0")/real-node")" "$@"
FAKE_NODE
  printf '%s\n' "$real_node" >"$fixture/bin/real-node"
  chmod +x "$fixture/bin/node"
  write_benchmark_rows "$fixture/bench.txt"
  git -C "$fixture/repo" init -q
  git -C "$fixture/repo" config user.name benchmark-test
  git -C "$fixture/repo" config user.email benchmark-test@example.invalid
  git -C "$fixture/repo" add scripts docs
  git -C "$fixture/repo" commit -qm fixture
  printf '%s\n' "$fixture"
}

run_capture() {
  local fixture=$1
  shift
  (
    cd "$fixture/repo"
    PATH="$fixture/bin:$PATH" \
      FAKE_GO_OUTPUT_FILE="$fixture/bench.txt" \
      BLUETAPE_GIN_BENCH_OUTPUT_DIR=docs/research/outputs/issue-543 \
      "$fixture/repo/scripts/capture-gin-adapter-benchmark.sh" "$@"
  )
}

assert_parser_rejects() {
  local input=$1
  local label=$2
  if python3 "$parser" --input "$input" --output "$test_root/$label.json" >/dev/null 2>&1; then
    fail "parser accepted $label"
  fi
}

assert_parser_guards() {
  local fixture
  fixture=$(setup_fixture parser)
  local baseline=$fixture/bench.txt
  python3 "$parser" --input "$baseline" --output "$test_root/parser-valid.json"

  local missing=$test_root/parser-missing.txt
  head -n 1 "$baseline" >"$missing"
  assert_parser_rejects "$missing" missing

  local unknown=$test_root/parser-unknown.txt
  cp "$baseline" "$unknown"
  printf '%s\n' 'BenchmarkGinAdapter/Unknown-1 1 1 ns/op 1 B/op 1 allocs/op' >>"$unknown"
  assert_parser_rejects "$unknown" unknown

  local duplicate=$test_root/parser-duplicate.txt
  cp "$baseline" "$duplicate"
  head -n 1 "$baseline" >>"$duplicate"
  assert_parser_rejects "$duplicate" duplicate

  local incomplete=$test_root/parser-incomplete-samples.txt
  {
    printf '%s\n' 'benchmark_count: 2' 'cpu: 1'
    cat "$baseline"
  } >"$incomplete"
  assert_parser_rejects "$incomplete" incomplete-samples

  local nonfinite=$test_root/parser-nonfinite.txt
  awk 'NR == 1 { sub(/10 ns\/op/, "NaN ns/op") } { print }' "$baseline" >"$nonfinite"
  assert_parser_rejects "$nonfinite" nonfinite

  local failed=$test_root/parser-failed.txt
  {
    printf '%s\n' 'FAIL\tginadapter'
    cat "$baseline"
  } >"$failed"
  assert_parser_rejects "$failed" failed
}

assert_capture_success_and_redaction() {
  local fixture
  fixture=$(setup_fixture success)
  run_capture "$fixture" 1 1
  local output_dir=$fixture/repo/docs/research/outputs/issue-543
  local results=$output_dir/bench-results.json
  test -s "$results" || fail 'successful capture did not publish results'
  test -s "$output_dir/bench-output.txt" || fail 'successful capture did not publish raw output'
  test -s "$output_dir/bench-environment.txt" || fail 'successful capture did not publish environment'
  test -s "$fixture/repo/docs/images/readme-charts/gin-adapter-benchmark-summary.svg" || fail 'SVG was not published'
  test -s "$fixture/repo/docs/images/readme-charts/gin-adapter-benchmark-summary.png" || fail 'PNG was not published'
  assert_file_contains "$results" '\"dirty_tree\": \"false\"'
  assert_file_contains "$results" '\"capture_eligibility\": \"eligible\"'
  assert_file_contains "$results" '\"no_regression\": \"N/A\"'
  assert_file_contains "$results" '\"chart_timeout_seconds\": \"60\"'
  assert_file_contains "$results" '\"chart_max_output_bytes\": \"10485760\"'

  fixture=$(setup_fixture redaction)
  {
    printf '%s\n' 'token=raw-token-must-not-persist'
    printf '%s\n' 'panic: raw-token-must-not-persist'
    printf '%s\n' 'Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.signature'
    cat "$fixture/bench.txt"
  } >"$fixture/secret-bench.txt"
  if (
    cd "$fixture/repo"
    PATH="$fixture/bin:$PATH" FAKE_GO_OUTPUT_FILE="$fixture/secret-bench.txt" \
      BLUETAPE_GIN_BENCH_OUTPUT_DIR=docs/research/outputs/issue-543 \
      "$fixture/repo/scripts/capture-gin-adapter-benchmark.sh" 1 1
  ); then
    :
  else
    fail 'redacted benchmark output unexpectedly failed'
  fi
  assert_file_excludes "$fixture/repo/docs/research/outputs/issue-543/bench-output.txt" 'raw-token-must-not-persist'
  assert_file_excludes "$fixture/repo/docs/research/outputs/issue-543/bench-output.txt" 'eyJhbGciOiJIUzI1NiJ9'
  assert_file_contains "$fixture/repo/docs/research/outputs/issue-543/bench-output.txt" '^\[redacted_output_line\]$'
}

assert_capture_failures_preserve_canonical_state() {
  local fixture
  fixture=$(setup_fixture output-limit)
  local oversized=$test_root/oversized.txt
  printf '%0256d\n' 0 >"$oversized"
  if (
    cd "$fixture/repo"
    PATH="$fixture/bin:$PATH" FAKE_GO_OUTPUT_FILE="$oversized" \
      BLUETAPE_GIN_BENCH_MAX_OUTPUT_BYTES=64 \
      BLUETAPE_GIN_BENCH_OUTPUT_DIR=docs/research/outputs/issue-543 \
      "$fixture/repo/scripts/capture-gin-adapter-benchmark.sh" 1 1
  ); then
    fail 'output limit failure returned success'
  else
    test "$?" -eq 125 || fail 'output limit failure returned the wrong status'
  fi
  local output_dir=$fixture/repo/docs/research/outputs/issue-543
  test ! -e "$output_dir/bench-results.json" || fail 'output limit replaced canonical results'
  local failed
  failed=$(find "$output_dir" -maxdepth 1 -name 'bench-failed-*.txt' -print -quit)
  test -n "$failed" || fail 'output limit did not retain failure metadata'
  assert_file_contains "$failed" '^failure_phase: benchmark$'
  assert_file_contains "$failed" '^\[output_truncated_at_64_bytes\]$'

  fixture=$(setup_fixture timeout)
  if (
    cd "$fixture/repo"
    PATH="$fixture/bin:$PATH" FAKE_GO_OUTPUT_FILE="$fixture/bench.txt" FAKE_GO_EXIT=124 \
      BLUETAPE_GIN_BENCH_OUTPUT_DIR=docs/research/outputs/issue-543 \
      "$fixture/repo/scripts/capture-gin-adapter-benchmark.sh" 1 1
  ); then
    fail 'timeout fixture returned success'
  else
    test "$?" -eq 124 || fail 'timeout fixture changed the command status'
  fi
  failed=$(find "$fixture/repo/docs/research/outputs/issue-543" -maxdepth 1 -name 'bench-failed-*.txt' -print -quit)
  test -n "$failed" || fail 'timeout fixture did not retain failure metadata'
  assert_file_contains "$failed" '^failure_exit_status: 124$'
}

assert_chart_failure_diagnostics() {
  local fixture
  fixture=$(setup_fixture chart-exit)
  local output_dir=$fixture/repo/docs/research/outputs/issue-543
  local chart_dir=$fixture/repo/docs/images/readme-charts
  mkdir -p "$output_dir" "$chart_dir"
  printf '%s\n' previous-results >"$output_dir/bench-results.json"
  printf '%s\n' previous-svg >"$chart_dir/gin-adapter-benchmark-summary.svg"
  printf '%s\n' previous-png >"$chart_dir/gin-adapter-benchmark-summary.png"
  if (
    cd "$fixture/repo"
    PATH="$fixture/bin:$PATH" FAKE_GO_OUTPUT_FILE="$fixture/bench.txt" \
      FAKE_NODE_EXIT=42 FAKE_NODE_STDERR='chart renderer exit stderr' \
      BLUETAPE_GIN_BENCH_OUTPUT_DIR=docs/research/outputs/issue-543 \
      "$fixture/repo/scripts/capture-gin-adapter-benchmark.sh" 1 1
  ); then
    fail 'chart failure fixture returned success'
  else
    test "$?" -eq 125 || fail 'chart failure fixture returned the wrong status'
  fi
  local failed
  failed=$(find "$fixture/repo/docs/research/outputs/issue-543" -maxdepth 1 -name 'bench-failed-*.txt' -print -quit)
  test -n "$failed" || fail 'chart failure fixture did not retain failure metadata'
  assert_file_contains "$failed" '^failure_phase: chart$'
  assert_file_contains "$failed" '^chart_failure_reason: exit$'
  assert_file_contains "$failed" '^chart_exit_status: 42$'
  assert_file_contains "$failed" '^chart_stderr_begin$'
  assert_file_contains "$failed" '^chart renderer exit stderr$'
  test "$(cat "$output_dir/bench-results.json")" = previous-results || fail 'chart failure replaced canonical results'
  test "$(cat "$chart_dir/gin-adapter-benchmark-summary.svg")" = previous-svg || fail 'chart failure replaced previous SVG'
  test "$(cat "$chart_dir/gin-adapter-benchmark-summary.png")" = previous-png || fail 'chart failure replaced previous PNG'

  fixture=$(setup_fixture chart-timeout)
  if (
    cd "$fixture/repo"
    PATH="$fixture/bin:$PATH" FAKE_GO_OUTPUT_FILE="$fixture/bench.txt" \
      FAKE_NODE_BLOCK=1 FAKE_NODE_STDERR='chart renderer timeout stderr' \
      BLUETAPE_GIN_BENCH_CHART_TIMEOUT_SECONDS=1 \
      BLUETAPE_GIN_BENCH_OUTPUT_DIR=docs/research/outputs/issue-543 \
      "$fixture/repo/scripts/capture-gin-adapter-benchmark.sh" 1 1
  ); then
    fail 'chart timeout fixture returned success'
  else
    test "$?" -eq 125 || fail 'chart timeout fixture returned the wrong status'
  fi
  failed=$(find "$fixture/repo/docs/research/outputs/issue-543" -maxdepth 1 -name 'bench-failed-*.txt' -print -quit)
  test -n "$failed" || fail 'chart timeout fixture did not retain failure metadata'
  assert_file_contains "$failed" '^failure_phase: chart$'
  assert_file_contains "$failed" '^chart_failure_reason: timeout$'
  assert_file_contains "$failed" '^chart_exit_status: 124$'
  assert_file_contains "$failed" '^chart_timeout_seconds: 1$'
  assert_file_contains "$failed" '^chart renderer timeout stderr$'
  assert_file_contains "$failed" '^\[chart_timeout_after_1_seconds\]$'
  test ! -e "$fixture/repo/docs/research/outputs/issue-543/bench-results.json" || fail 'chart timeout published canonical results'

  fixture=$(setup_fixture chart-signal)
  if (
    cd "$fixture/repo"
    PATH="$fixture/bin:$PATH" FAKE_GO_OUTPUT_FILE="$fixture/bench.txt" \
      FAKE_NODE_SIGNAL=1 FAKE_NODE_STDERR='chart renderer signal stderr' \
      BLUETAPE_GIN_BENCH_OUTPUT_DIR=docs/research/outputs/issue-543 \
      "$fixture/repo/scripts/capture-gin-adapter-benchmark.sh" 1 1
  ); then
    fail 'chart signal fixture returned success'
  else
    test "$?" -eq 125 || fail 'chart signal fixture returned the wrong status'
  fi
  failed=$(find "$fixture/repo/docs/research/outputs/issue-543" -maxdepth 1 -name 'bench-failed-*.txt' -print -quit)
  test -n "$failed" || fail 'chart signal fixture did not retain failure metadata'
  assert_file_contains "$failed" '^failure_phase: chart$'
  assert_file_contains "$failed" '^chart_failure_reason: signal$'
  assert_file_contains "$failed" '^chart_exit_status: 143$'
  assert_file_contains "$failed" '^chart renderer signal stderr$'
  test ! -e "$fixture/repo/docs/research/outputs/issue-543/bench-results.json" || fail 'chart signal published canonical results'

  fixture=$(setup_fixture chart-output-limit)
  local chart_output
  chart_output=$(printf '%256s' '' | tr ' ' x)
  if (
    cd "$fixture/repo"
    PATH="$fixture/bin:$PATH" FAKE_GO_OUTPUT_FILE="$fixture/bench.txt" \
      FAKE_NODE_STDERR="$chart_output" BLUETAPE_GIN_BENCH_CHART_MAX_OUTPUT_BYTES=64 \
      BLUETAPE_GIN_BENCH_OUTPUT_DIR=docs/research/outputs/issue-543 \
      "$fixture/repo/scripts/capture-gin-adapter-benchmark.sh" 1 1
  ); then
    fail 'chart output-limit fixture returned success'
  else
    test "$?" -eq 125 || fail 'chart output-limit fixture returned the wrong status'
  fi
  failed=$(find "$fixture/repo/docs/research/outputs/issue-543" -maxdepth 1 -name 'bench-failed-*.txt' -print -quit)
  test -n "$failed" || fail 'chart output-limit fixture did not retain failure metadata'
  assert_file_contains "$failed" '^failure_phase: chart$'
  assert_file_contains "$failed" '^chart_failure_reason: output_limit$'
  assert_file_contains "$failed" '^chart_exit_status: 125$'
  assert_file_contains "$failed" '^\[chart_output_truncated_at_64_bytes\]$'
  assert_file_excludes "$failed" 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx'
  test ! -e "$fixture/repo/docs/research/outputs/issue-543/bench-results.json" || fail 'chart output-limit published canonical results'

  fixture=$(setup_fixture chart-malformed)
  if (
    cd "$fixture/repo"
    PATH="$fixture/bin:$PATH" FAKE_GO_OUTPUT_FILE="$fixture/bench.txt" \
      FAKE_NODE_MALFORMED=1 \
      BLUETAPE_GIN_BENCH_OUTPUT_DIR=docs/research/outputs/issue-543 \
      "$fixture/repo/scripts/capture-gin-adapter-benchmark.sh" 1 1
  ); then
    fail 'chart malformed-input fixture returned success'
  else
    test "$?" -eq 125 || fail 'chart malformed-input fixture returned the wrong status'
  fi
  failed=$(find "$fixture/repo/docs/research/outputs/issue-543" -maxdepth 1 -name 'bench-failed-*.txt' -print -quit)
  test -n "$failed" || fail 'chart malformed-input fixture did not retain failure metadata'
  assert_file_contains "$failed" '^failure_phase: chart$'
  assert_file_contains "$failed" '^chart_failure_reason: exit$'
  assert_file_contains "$failed" '^chart_exit_status: [1-9][0-9]*$'
  assert_file_contains "$failed" '^chart_stderr_begin$'
  assert_file_contains "$failed" '^chart_stderr_end$'
  test ! -e "$fixture/repo/docs/research/outputs/issue-543/bench-results.json" || fail 'chart malformed-input published canonical results'
}

assert_signal_preserves_failure_artifact() {
  local fixture
  fixture=$(setup_fixture signal)
  python3 - "$fixture" <<'PY'
import os
import signal
import subprocess
import sys
import time

fixture = sys.argv[1]
repo = os.path.join(fixture, "repo")
environment = os.environ.copy()
environment.update({
    "PATH": os.path.join(fixture, "bin") + os.pathsep + environment["PATH"],
    "FAKE_GO_OUTPUT_FILE": os.path.join(fixture, "bench.txt"),
    "FAKE_GO_BLOCK": "1",
    "BLUETAPE_GIN_BENCH_OUTPUT_DIR": "docs/research/outputs/issue-543",
})
process = subprocess.Popen(
    [os.path.join(repo, "scripts/capture-gin-adapter-benchmark.sh"), "1", "1"],
    cwd=repo,
    env=environment,
)
time.sleep(1.0)
process.send_signal(signal.SIGTERM)
status = process.wait(timeout=5)
if status != 143:
    raise SystemExit(f"signal interruption returned {status}, want 143")
PY
  local output_dir=$fixture/repo/docs/research/outputs/issue-543
  local failed
  failed=$(find "$output_dir" -maxdepth 1 -name 'bench-failed-*.txt' -print -quit)
  test -n "$failed" || fail 'signal interruption did not retain failure metadata'
  assert_file_contains "$failed" '^failure_phase: benchmark$'
  assert_file_contains "$failed" '^failure_exit_status: 143$'
  test ! -e "$output_dir/bench-results.json" || fail 'signal interruption published canonical results'
}

assert_dirty_tree_is_not_a_regression_claim() {
  local fixture
  fixture=$(setup_fixture dirty)
  printf '%s\n' dirty >"$fixture/repo/dirty.txt"
  run_capture "$fixture" 1 1
  local results=$fixture/repo/docs/research/outputs/issue-543/bench-results.json
  assert_file_contains "$results" '\"dirty_tree\": \"true\"'
  assert_file_contains "$results" '\"capture_eligibility\": \"N/A\"'
  assert_file_contains "$results" '\"no_regression\": \"N/A\"'
}

assert_publication_failure_preserves_previous_files() {
  local fixture
  fixture=$(setup_fixture publication)
  local output_dir=$fixture/repo/docs/research/outputs/issue-543
  local chart_dir=$fixture/repo/docs/images/readme-charts
  mkdir -p "$output_dir" "$chart_dir"
  printf '%s\n' previous-output >"$output_dir/bench-output.txt"
  printf '%s\n' previous-results >"$output_dir/bench-results.json"
  printf '%s\n' previous-environment >"$output_dir/bench-environment.txt"
  printf '%s\n' previous-svg >"$chart_dir/gin-adapter-benchmark-summary.svg"
  printf '%s\n' previous-png >"$chart_dir/gin-adapter-benchmark-summary.png"
  printf '%s\n' previous-chart-source >"$chart_dir/gin-adapter-benchmark-summary.vl.json"
  cat >"$fixture/bin/mv" <<'FAKE_MV'
#!/usr/bin/env bash
set -eu
target=${!#}
case "$target" in
  *bench-results.json) exit 91 ;;
  *) exec /bin/mv "$@" ;;
esac
FAKE_MV
  chmod +x "$fixture/bin/mv"
  if (
    cd "$fixture/repo"
    PATH="$fixture/bin:$PATH" FAKE_GO_OUTPUT_FILE="$fixture/bench.txt" \
      BLUETAPE_GIN_BENCH_OUTPUT_DIR=docs/research/outputs/issue-543 \
      "$fixture/repo/scripts/capture-gin-adapter-benchmark.sh" 1 1
  ); then
    fail 'publication failure returned success'
  else
    test "$?" -eq 125 || fail 'publication failure returned the wrong status'
  fi
  test "$(cat "$output_dir/bench-output.txt")" = previous-output || fail 'publication failure did not roll back raw output'
  test "$(cat "$output_dir/bench-results.json")" = previous-results || fail 'publication failure did not preserve results'
  test "$(cat "$output_dir/bench-environment.txt")" = previous-environment || fail 'publication failure did not preserve environment'
  test "$(cat "$chart_dir/gin-adapter-benchmark-summary.svg")" = previous-svg || fail 'publication failure did not preserve SVG'
  test "$(cat "$chart_dir/gin-adapter-benchmark-summary.png")" = previous-png || fail 'publication failure did not preserve PNG'
  test "$(cat "$chart_dir/gin-adapter-benchmark-summary.vl.json")" = previous-chart-source || fail 'publication failure did not preserve chart source'
  test -n "$(find "$fixture/repo/docs/research/outputs/issue-543" -maxdepth 1 -name 'bench-failed-*.txt' -print -quit)" || fail 'publication failure did not retain failure metadata'
}

assert_chart_self_test() {
  node "$chart_generator" --self-test >/dev/null
}

assert_parser_guards
printf 'PASS: parser guards\n'
assert_chart_self_test
printf 'PASS: chart self-test\n'
assert_capture_success_and_redaction
printf 'PASS: capture success and redaction\n'
assert_capture_failures_preserve_canonical_state
printf 'PASS: capture output-limit and timeout guards\n'
assert_chart_failure_diagnostics
printf 'PASS: chart failure diagnostics guard\n'
assert_signal_preserves_failure_artifact
printf 'PASS: signal failure artifact guard\n'
assert_dirty_tree_is_not_a_regression_claim
printf 'PASS: dirty-tree N/A guard\n'
assert_publication_failure_preserves_previous_files
printf 'PASS: publication failure guard\n'
