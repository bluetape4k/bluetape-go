#!/usr/bin/env bash

set -euo pipefail

script_dir=$(cd "$(dirname "$0")" && pwd -P)
capture_script=$script_dir/capture-gin-adapter-benchmark.sh
parser=$script_dir/parse-gin-adapter-benchmark.py
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
    'BenchmarkGinAdapterColdConstruction-1 1 90 ns/op 9 B/op 1 allocs/op' \
    'BenchmarkGinAdapterColdFirstRequest-1 1 100 ns/op 10 B/op 1 allocs/op' \
    'BenchmarkGinAdapterWarmRequest/Serial-1 1 110 ns/op 11 B/op 1 allocs/op' \
    'BenchmarkGinAdapterWarmRequest/Parallel-1 1 120 ns/op 12 B/op 1 allocs/op' \
    >"$destination"
}

setup_fixture() {
  local name=$1
  local fixture=$test_root/$name
  mkdir -p "$fixture/repo/scripts" "$fixture/repo/docs/images/readme-charts" "$fixture/bin"
  cp "$capture_script" "$fixture/repo/scripts/capture-gin-adapter-benchmark.sh"
  cp "$parser" "$fixture/repo/scripts/parse-gin-adapter-benchmark.py"
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
  run_capture "$fixture" 1 1,2
  local output_dir=$fixture/repo/docs/research/outputs/issue-543
  local results=$output_dir/bench-results.json
  test -s "$results" || fail 'successful capture did not publish results'
  test -s "$output_dir/bench-output.txt" || fail 'successful capture did not publish raw output'
  test -s "$output_dir/bench-environment.txt" || fail 'successful capture did not publish environment'
  test -s "$fixture/repo/docs/images/readme-charts/gin-adapter-benchmark-summary.svg" || fail 'SVG was not published'
  test -s "$fixture/repo/docs/images/readme-charts/gin-adapter-benchmark-summary.png" || fail 'PNG was not published'
  assert_file_contains "$results" '\"dirty_tree\": \"false\"'
  assert_file_contains "$results" '\"no_regression\": \"eligible\"'

  fixture=$(setup_fixture redaction)
  {
    printf '%s\n' 'token=raw-token-must-not-persist'
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

assert_dirty_tree_is_not_a_regression_claim() {
  local fixture
  fixture=$(setup_fixture dirty)
  printf '%s\n' dirty >"$fixture/repo/dirty.txt"
  run_capture "$fixture" 1 1
  local results=$fixture/repo/docs/research/outputs/issue-543/bench-results.json
  assert_file_contains "$results" '\"dirty_tree\": \"true\"'
  assert_file_contains "$results" '\"no_regression\": \"N/A\"'
}

assert_publication_failure_preserves_previous_files() {
  local fixture
  fixture=$(setup_fixture publication)
  cat >"$fixture/bin/mv" <<'FAKE_MV'
#!/usr/bin/env bash
set -eu
target=${!#}
case "$target" in
  *bench-output.txt) exit 91 ;;
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
  test ! -e "$fixture/repo/docs/research/outputs/issue-543/bench-output.txt" || fail 'publication failure created canonical output'
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
assert_dirty_tree_is_not_a_regression_claim
printf 'PASS: dirty-tree N/A guard\n'
assert_publication_failure_preserves_previous_files
printf 'PASS: publication failure guard\n'
