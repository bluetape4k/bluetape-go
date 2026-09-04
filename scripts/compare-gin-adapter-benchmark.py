#!/usr/bin/env python3
"""동일한 Gin benchmark fixture의 clean baseline과 candidate를 비교한다."""

from __future__ import annotations

import argparse
import json
import math
import os
import tempfile
from pathlib import Path


TARGETS = (
    "BenchmarkGinAdapter/FullAdapter/Serial",
    "BenchmarkGinAdapter/FullAdapterRetry/Serial",
)
METRICS = (
    ("ns_per_op", 0.15),
    ("bytes_per_op", 0.10),
    ("allocs_per_op", 0.10),
)
ENVIRONMENT_KEYS = (
    "fixture_identity",
    "gin_version",
    "go_version",
    "goos",
    "goarch",
    "cpu",
)


class Inconclusive(Exception):
    """비교할 수 없지만 회귀 없음으로 오판해서는 안 되는 입력이다."""


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--baseline", required=True, type=Path)
    parser.add_argument("--candidate", required=True, type=Path)
    parser.add_argument("--output", type=Path)
    return parser.parse_args()


def write_json(path: Path, value: dict[str, object]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor, temporary = tempfile.mkstemp(prefix=f".{path.name}.", suffix=".tmp", dir=path.parent)
    try:
        with os.fdopen(descriptor, "w", encoding="utf-8") as stream:
            json.dump(value, stream, ensure_ascii=False, indent=2, sort_keys=True)
            stream.write("\n")
        os.replace(temporary, path)
    except BaseException:
        try:
            os.unlink(temporary)
        except FileNotFoundError:
            pass
        raise


def load_summary(path: Path, label: str) -> dict[str, object]:
    try:
        with path.open(encoding="utf-8") as stream:
            summary = json.load(stream)
    except (OSError, json.JSONDecodeError) as error:
        raise Inconclusive(f"{label} benchmark 결과를 읽을 수 없다: {error}") from error
    if not isinstance(summary, dict):
        raise Inconclusive(f"{label} benchmark 결과가 JSON object가 아니다")
    metadata = summary.get("metadata")
    rows = summary.get("rows")
    if not isinstance(metadata, dict) or not isinstance(rows, list) or not rows:
        raise Inconclusive(f"{label} benchmark 결과의 metadata/rows가 없다")
    return summary


def require_metadata(summary: dict[str, object], label: str) -> dict[str, str]:
    raw_metadata = summary["metadata"]
    assert isinstance(raw_metadata, dict)
    metadata: dict[str, str] = {}
    for key, value in raw_metadata.items():
        if isinstance(value, str):
            metadata[key] = value
    required = set(ENVIRONMENT_KEYS) | {"dirty_tree", "capture_eligibility", "benchmark_count"}
    missing = sorted(required - metadata.keys())
    if missing:
        raise Inconclusive(f"{label} metadata가 부족하다: {', '.join(missing)}")
    if metadata["dirty_tree"] != "false" or metadata["capture_eligibility"] != "eligible":
        raise Inconclusive(f"{label} capture가 clean eligible 상태가 아니다")
    try:
        count = int(metadata["benchmark_count"])
    except ValueError as error:
        raise Inconclusive(f"{label} benchmark_count가 정수가 아니다") from error
    if count <= 0:
        raise Inconclusive(f"{label} benchmark_count가 양수가 아니다")
    return metadata


def finite_metric(row: dict[str, object], key: str, label: str) -> float:
    value = row.get(key)
    if not isinstance(value, (int, float)) or isinstance(value, bool) or not math.isfinite(float(value)):
        raise Inconclusive(f"{label} {key}가 유한한 숫자가 아니다")
    number = float(value)
    if key == "ns_per_op" and number <= 0:
        raise Inconclusive(f"{label} ns_per_op가 양수가 아니다")
    if key != "ns_per_op" and number < 0:
        raise Inconclusive(f"{label} {key}가 음수다")
    return number


def target_samples(summary: dict[str, object], metadata: dict[str, str], label: str) -> dict[str, dict[str, list[float]]]:
    raw_rows = summary["rows"]
    assert isinstance(raw_rows, list)
    try:
        raw_cpus = [int(value) for value in metadata["cpu"].split(",")]
    except ValueError as error:
        raise Inconclusive(f"{label} cpu metadata가 올바르지 않다") from error
    if raw_cpus != sorted(set(raw_cpus)) or any(cpu <= 0 for cpu in raw_cpus) or 1 not in raw_cpus:
        raise Inconclusive(f"{label} cpu metadata에 canonical CPU 1이 없다")
    samples: dict[str, dict[str, list[float]]] = {
        name: {metric: [] for metric, _ in METRICS} for name in TARGETS
    }
    for index, raw_row in enumerate(raw_rows):
        if not isinstance(raw_row, dict):
            raise Inconclusive(f"{label} row {index}가 JSON object가 아니다")
        name = raw_row.get("name")
        cpu = raw_row.get("cpu")
        if name not in samples or cpu != 1:
            continue
        for metric, _ in METRICS:
            samples[name][metric].append(finite_metric(raw_row, metric, f"{label} {name}"))
    try:
        count = int(metadata["benchmark_count"])
    except ValueError as error:
        raise Inconclusive(f"{label} benchmark_count가 정수가 아니다") from error
    for name in TARGETS:
        for metric, _ in METRICS:
            observed = len(samples[name][metric])
            if observed != count:
                raise Inconclusive(f"{label} {name} cpu=1 {metric} sample 수가 {observed}개다; 기대값 {count}개")
    return samples


def median(values: list[float]) -> float:
    ordered = sorted(values)
    middle = len(ordered) // 2
    if len(ordered) % 2:
        return ordered[middle]
    return (ordered[middle - 1] + ordered[middle]) / 2


def compare(
    baseline: dict[str, object],
    candidate: dict[str, object],
) -> tuple[str, list[dict[str, object]], list[str]]:
    baseline_metadata = require_metadata(baseline, "baseline")
    candidate_metadata = require_metadata(candidate, "candidate")
    mismatches = [
        key
        for key in ENVIRONMENT_KEYS
        if baseline_metadata[key] != candidate_metadata[key]
    ]
    if mismatches:
        raise Inconclusive(f"baseline/candidate 실행 환경이 다르다: {', '.join(mismatches)}")
    baseline_samples = target_samples(baseline, baseline_metadata, "baseline")
    candidate_samples = target_samples(candidate, candidate_metadata, "candidate")
    metrics: list[dict[str, object]] = []
    failures: list[str] = []
    for name in TARGETS:
        for metric, threshold in METRICS:
            baseline_value = median(baseline_samples[name][metric])
            candidate_value = median(candidate_samples[name][metric])
            if baseline_value == 0:
                delta = None if candidate_value == 0 else math.inf
                regressed = candidate_value > 0
            else:
                delta = (candidate_value - baseline_value) / baseline_value
                regressed = delta > threshold
            entry = {
                "name": name,
                "metric": metric,
                "baseline": baseline_value,
                "candidate": candidate_value,
                "delta_ratio": delta,
                "max_regression": threshold,
                "regressed": regressed,
            }
            metrics.append(entry)
            if regressed:
                failures.append(
                    f"{name} {metric} regression: {candidate_value:g} vs {baseline_value:g} "
                    f"(허용 {threshold * 100:.1f}%)"
                )
    return ("failed" if failures else "passed"), metrics, failures


def build_report(status: str, reason: str | None = None, **extra: object) -> dict[str, object]:
    report: dict[str, object] = {
        "schema_version": 1,
        "status": status,
        "no_regression": "N/A" if status == "inconclusive" else status,
    }
    if reason:
        report["reason"] = reason
    report.update(extra)
    return report


def emit(report: dict[str, object], output: Path | None) -> None:
    if output is None:
        print(json.dumps(report, ensure_ascii=False, indent=2, sort_keys=True))
        return
    write_json(output, report)


def main() -> int:
    args = parse_args()
    try:
        baseline = load_summary(args.baseline, "baseline")
        candidate = load_summary(args.candidate, "candidate")
        status, metrics, failures = compare(baseline, candidate)
        report = build_report(status, metrics=metrics, failures=failures)
        emit(report, args.output)
        if failures:
            print("; ".join(failures), file=os.sys.stderr)
        return 1 if status == "failed" else 0
    except Inconclusive as error:
        report = build_report("inconclusive", str(error))
        emit(report, args.output)
        print(f"inconclusive: {error}", file=os.sys.stderr)
        return 2
    except (OSError, ValueError, TypeError) as error:
        report = build_report("inconclusive", f"비교기 오류: {error}")
        try:
            emit(report, args.output)
        except OSError:
            pass
        print(f"inconclusive: {error}", file=os.sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
