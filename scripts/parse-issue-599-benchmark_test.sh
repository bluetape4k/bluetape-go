#!/usr/bin/env bash

set -euo pipefail

script_dir=$(cd "$(dirname "$0")" && pwd -P)
parser=$script_dir/parse-issue-599-benchmark.py
test_root=$(mktemp -d "${TMPDIR:-/tmp}/parse-issue-599-benchmark-test.XXXXXX")
trap 'rm -rf "$test_root"' EXIT

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

input=$test_root/benchmark.txt
output=$test_root/summary.json
printf '%s\n' \
  'command: go test -run ^$ -bench ^BenchmarkIssue599 -benchmem -count=3 ./cache/redisfory' \
  'BenchmarkIssue599Codec/JSON/Small/RoundTrip-8 10 100 ns/op 64 B/op 2 allocs/op 32 wire-bytes' \
  'BenchmarkIssue599Codec/JSON/Small/RoundTrip-8 10 120 ns/op 64 B/op 2 allocs/op 32 wire-bytes' \
  'BenchmarkIssue599Codec/JSON/Small/RoundTrip-8 10 110 ns/op 64 B/op 2 allocs/op 32 wire-bytes' \
  'BenchmarkIssue599Codec/NativeFast/Small/RoundTrip-8 10 40 ns/op 48 B/op 1 allocs/op 24 wire-bytes' \
  'BenchmarkIssue599Codec/NativeFast/Small/RoundTrip-8 10 45 ns/op 48 B/op 1 allocs/op 24 wire-bytes' \
  'BenchmarkIssue599Codec/NativeFast/Small/RoundTrip-8 10 42 ns/op 48 B/op 1 allocs/op 24 wire-bytes' \
  'BenchmarkIssue599Codec/NativeCompatible/Small/RoundTrip-8 10 60 ns/op 52 B/op 1 allocs/op 28 wire-bytes' \
  'BenchmarkIssue599Codec/NativeCompatible/Small/RoundTrip-8 10 62 ns/op 52 B/op 1 allocs/op 28 wire-bytes' \
  'BenchmarkIssue599Codec/NativeCompatible/Small/RoundTrip-8 10 61 ns/op 52 B/op 1 allocs/op 28 wire-bytes' \
  'BenchmarkIssue599DirectRedis/JSON/Small/RoundTrip-8 10 1000 ns/op 120 B/op 4 allocs/op 32 wire-bytes' \
  'BenchmarkIssue599DirectRedis/JSON/Small/RoundTrip-8 10 1100 ns/op 120 B/op 4 allocs/op 32 wire-bytes' \
  'BenchmarkIssue599DirectRedis/JSON/Small/RoundTrip-8 10 1050 ns/op 120 B/op 4 allocs/op 32 wire-bytes' \
  'BenchmarkIssue599DirectRedis/NativeFast/Small/RoundTrip-8 10 800 ns/op 100 B/op 3 allocs/op 24 wire-bytes' \
  'BenchmarkIssue599DirectRedis/NativeFast/Small/RoundTrip-8 10 820 ns/op 100 B/op 3 allocs/op 24 wire-bytes' \
  'BenchmarkIssue599DirectRedis/NativeFast/Small/RoundTrip-8 10 810 ns/op 100 B/op 3 allocs/op 24 wire-bytes' \
  'BenchmarkIssue599DirectRedis/NativeCompatible/Small/RoundTrip-8 10 900 ns/op 104 B/op 3 allocs/op 28 wire-bytes' \
  'BenchmarkIssue599DirectRedis/NativeCompatible/Small/RoundTrip-8 10 920 ns/op 104 B/op 3 allocs/op 28 wire-bytes' \
  'BenchmarkIssue599DirectRedis/NativeCompatible/Small/RoundTrip-8 10 910 ns/op 104 B/op 3 allocs/op 28 wire-bytes' \
  'BenchmarkIssue599Coordination/JSON/ColdWinner-8 10 2000 ns/op 160 B/op 5 allocs/op 32 wire-bytes' \
  'BenchmarkIssue599Coordination/JSON/ColdWinner-8 10 2100 ns/op 160 B/op 5 allocs/op 32 wire-bytes' \
  'BenchmarkIssue599Coordination/JSON/ColdWinner-8 10 2050 ns/op 160 B/op 5 allocs/op 32 wire-bytes' \
  'BenchmarkIssue599Coordination/NativeFast/ColdWinner-8 10 1800 ns/op 140 B/op 4 allocs/op 24 wire-bytes' \
  'BenchmarkIssue599Coordination/NativeFast/ColdWinner-8 10 1820 ns/op 140 B/op 4 allocs/op 24 wire-bytes' \
  'BenchmarkIssue599Coordination/NativeFast/ColdWinner-8 10 1810 ns/op 140 B/op 4 allocs/op 24 wire-bytes' \
  'BenchmarkIssue599Coordination/NativeCompatible/ColdWinner-8 10 1900 ns/op 144 B/op 4 allocs/op 28 wire-bytes' \
  'BenchmarkIssue599Coordination/NativeCompatible/ColdWinner-8 10 1920 ns/op 144 B/op 4 allocs/op 28 wire-bytes' \
  'BenchmarkIssue599Coordination/NativeCompatible/ColdWinner-8 10 1910 ns/op 144 B/op 4 allocs/op 28 wire-bytes' \
  'BenchmarkIssue599Contention/NativeFast/Mutex-8 10 300 ns/op 80 B/op 2 allocs/op' \
  'BenchmarkIssue599Contention/NativeFast/Mutex-8 10 310 ns/op 80 B/op 2 allocs/op' \
  'BenchmarkIssue599Contention/NativeFast/Mutex-8 10 290 ns/op 80 B/op 2 allocs/op' \
  'BenchmarkIssue599Contention/NativeFast/Pool-8 10 240 ns/op 72 B/op 2 allocs/op' \
  'BenchmarkIssue599Contention/NativeFast/Pool-8 10 250 ns/op 72 B/op 2 allocs/op' \
  'BenchmarkIssue599Contention/NativeFast/Pool-8 10 230 ns/op 72 B/op 2 allocs/op' \
  >"$input"

python3 "$parser" --input "$input" --output "$output"
test -s "$output" || fail "summary output is empty"

python3 - "$output" <<'PY'
import json
import sys

summary = json.load(open(sys.argv[1], encoding="utf-8"))
assert summary["schema_version"] == 1
assert summary["required_samples"] == 3
assert summary["coverage"]["scenario_coverage"] == 1.0
assert summary["coverage"]["missing_groups"] == []
assert summary["coverage"]["row_count"] == 11
rows = {row["name"]: row for row in summary["rows"]}
assert rows["BenchmarkIssue599Codec/JSON/Small/RoundTrip"]["samples"] == 3
assert rows["BenchmarkIssue599Codec/JSON/Small/RoundTrip"]["ns_per_op"]["median"] == 110.0
assert rows["BenchmarkIssue599Codec/JSON/Small/RoundTrip"]["metrics"]["wire-bytes"]["median"] == 32.0
assert summary["guards"] == {
    "has_round_trip": True,
    "has_wire_bytes": True,
    "has_alloc_metrics": True,
}
PY

missing_input=$test_root/missing.txt
printf '%s\n' \
  'BenchmarkIssue599Codec/JSON/Small/RoundTrip-8 1 100 ns/op 64 B/op 2 allocs/op' \
  >"$missing_input"
if python3 "$parser" --input "$missing_input" --output "$test_root/missing.json" >/dev/null 2>&1; then
  fail "parser accepted incomplete benchmark coverage"
fi
