#!/usr/bin/env bash

set -euo pipefail

script_dir=$(cd "$(dirname "$0")" && pwd -P)
comparator=$script_dir/compare-gin-adapter-benchmark.py
test_root=$(mktemp -d "${TMPDIR:-/tmp}/compare-gin-adapter-benchmark-test.XXXXXX")
trap 'rm -rf "$test_root"' EXIT

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

write_summary() {
  local destination=$1
  local ns_per_op=$2
  local dirty_tree=${3:-false}
  local goos=${4:-darwin}
  python3 - "$destination" "$ns_per_op" "$dirty_tree" "$goos" <<'PY'
import json
import sys

destination, ns_per_op, dirty_tree, goos = sys.argv[1:]
ns_per_op = float(ns_per_op)
names = [
    "BenchmarkGinAdapter/NoOp/Serial",
    "BenchmarkGinAdapter/NoOp/Parallel",
    "BenchmarkGinAdapter/DirectCore/Serial",
    "BenchmarkGinAdapter/DirectCore/Parallel",
    "BenchmarkGinAdapter/Bridge/Serial",
    "BenchmarkGinAdapter/Bridge/Parallel",
    "BenchmarkGinAdapter/FullAdapter/Serial",
    "BenchmarkGinAdapter/FullAdapter/Parallel",
    "BenchmarkGinAdapter/FullAdapterRetry/Serial",
    "BenchmarkGinAdapter/FullAdapterRetry/Parallel",
    "BenchmarkGinAdapterColdConstruction",
    "BenchmarkGinAdapterColdFirstRequest",
    "BenchmarkGinAdapterWarmRequest/Serial",
    "BenchmarkGinAdapterWarmRequest/Parallel",
]
rows = []
for sample in range(1, 4):
    for index, name in enumerate(names):
        value = ns_per_op if name in {
            "BenchmarkGinAdapter/FullAdapter/Serial",
            "BenchmarkGinAdapter/FullAdapterRetry/Serial",
        } else float(index + 1)
        rows.append({
            "name": name,
            "cpu": 1,
            "iterations": 1000 + sample,
            "ns_per_op": value,
            "bytes_per_op": 100.0 if "FullAdapter" in name else float(index + 1),
            "allocs_per_op": 10.0 if "FullAdapter" in name else float(index + 1),
        })
json.dump({
    "schema_version": 1,
    "benchmark_prefix": "BenchmarkGinAdapter",
    "expected_names": names,
    "metadata": {
        "benchmark_count": "3",
        "cpu": "1",
        "dirty_tree": dirty_tree,
        "capture_eligibility": "N/A" if dirty_tree == "true" else "eligible",
        "no_regression": "N/A",
        "fixture_identity": "gin-v1.12.0-parser-only-local",
        "gin_version": "v1.12.0",
        "go_version": "go1.26.6",
        "goos": goos,
        "goarch": "arm64",
    },
    "rows": rows,
}, open(destination, "w", encoding="utf-8"), indent=2)
PY
}

assert_report_status() {
  local report=$1
  local expected=$2
  python3 - "$report" "$expected" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as stream:
    report = json.load(stream)
if report.get("status") != sys.argv[2]:
    raise SystemExit(f"status={report.get('status')!r}, want {sys.argv[2]!r}")
PY
}

baseline=$test_root/baseline.json
candidate=$test_root/candidate.json
report=$test_root/report.json
write_summary "$baseline" 100
write_summary "$candidate" 105
python3 "$comparator" --baseline "$baseline" --candidate "$candidate" --output "$report" >/dev/null
assert_report_status "$report" passed
printf 'PASS: threshold pass\n'

write_summary "$candidate" 120
if python3 "$comparator" --baseline "$baseline" --candidate "$candidate" --output "$report" >/dev/null 2>&1; then
  fail 'regression was accepted'
else
  status=$?
fi
test "$status" -eq 1 || fail 'regression returned the wrong status'
assert_report_status "$report" failed
grep -q 'FullAdapter/Serial' "$report" || fail 'regression report omitted metric name'
printf 'PASS: threshold failure\n'

write_summary "$candidate" 105 true
if python3 "$comparator" --baseline "$baseline" --candidate "$candidate" --output "$report" >/dev/null 2>&1; then
  fail 'dirty candidate was accepted'
else
  status=$?
fi
test "$status" -eq 2 || fail 'dirty candidate returned the wrong status'
assert_report_status "$report" inconclusive
grep -q 'N/A' "$report" || fail 'dirty candidate did not preserve N/A status'
printf 'PASS: dirty candidate inconclusive\n'

write_summary "$candidate" 105 false linux
if python3 "$comparator" --baseline "$baseline" --candidate "$candidate" --output "$report" >/dev/null 2>&1; then
  fail 'environment mismatch was accepted'
else
  status=$?
fi
test "$status" -eq 2 || fail 'environment mismatch returned the wrong status'
assert_report_status "$report" inconclusive
printf 'PASS: environment mismatch inconclusive\n'

if python3 "$comparator" --baseline "$test_root/missing.json" --candidate "$candidate" --output "$report" >/dev/null 2>&1; then
  fail 'missing baseline was accepted'
else
  status=$?
fi
test "$status" -eq 2 || fail 'missing baseline returned the wrong status'
assert_report_status "$report" inconclusive
printf 'PASS: missing baseline inconclusive\n'

printf 'PASS: benchmark comparator contract\n'
