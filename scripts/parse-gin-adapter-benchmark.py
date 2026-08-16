#!/usr/bin/env python3
"""Parse Gin adapter Go benchmark output into a validated JSON ledger."""

from __future__ import annotations

import argparse
import json
import math
import os
import re
import tempfile
from collections import Counter
from pathlib import Path


EXPECTED_NAMES = (
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
)
EXPECTED = set(EXPECTED_NAMES)
METRIC_RE = re.compile(r"^(?P<value>[0-9]+(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?)$")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    return parser.parse_args()


def finite_number(value: str, field: str) -> float:
    if METRIC_RE.fullmatch(value) is None:
        raise ValueError(f"{field} is not a finite number: {value}")
    number = float(value)
    if not math.isfinite(number):
        raise ValueError(f"{field} is not finite: {value}")
    return number


def parse_benchmark_line(line: str) -> dict[str, object] | None:
    stripped = line.strip()
    if not stripped.startswith("Benchmark"):
        return None
    tokens = stripped.split()
    if len(tokens) < 4:
        raise ValueError(f"malformed benchmark row: {stripped}")
    name, cpu = split_benchmark_name(tokens[0])
    try:
        iterations = int(tokens[1])
    except ValueError as error:
        raise ValueError(f"invalid benchmark iterations: {tokens[1]}") from error
    if iterations <= 0 or tokens[3] != "ns/op":
        raise ValueError(f"invalid benchmark row: {stripped}")
    ns_per_op = finite_number(tokens[2], "ns/op")
    metrics: dict[str, float] = {}
    index = 4
    while index + 1 < len(tokens):
        value = finite_number(tokens[index], tokens[index + 1])
        unit = tokens[index + 1]
        if unit not in ("B/op", "allocs/op"):
            raise ValueError(f"unknown benchmark metric {unit}")
        if unit in metrics:
            raise ValueError(f"duplicate benchmark metric {unit} in {name}")
        metrics[unit] = value
        index += 2
    if set(metrics) != {"B/op", "allocs/op"}:
        raise ValueError(f"benchmark row lacks B/op or allocs/op: {name}")
    if ns_per_op <= 0 or metrics["B/op"] < 0 or metrics["allocs/op"] < 0:
        raise ValueError(f"benchmark row has invalid metric value: {name}")
    return {
        "name": name,
        "cpu": cpu,
        "iterations": iterations,
        "ns_per_op": ns_per_op,
        "bytes_per_op": metrics["B/op"],
        "allocs_per_op": metrics["allocs/op"],
    }


def split_benchmark_name(raw: str) -> tuple[str, int]:
    for expected in sorted(EXPECTED, key=len, reverse=True):
        if raw == expected:
            return expected, 1
        prefix = f"{expected}-"
        if raw.startswith(prefix) and raw[len(prefix) :].isdigit():
            return expected, int(raw[len(prefix) :])
    raise ValueError(f"unknown benchmark row: {raw}")


def parse_input(path: Path) -> tuple[list[dict[str, object]], dict[str, str]]:
    rows: list[dict[str, object]] = []
    metadata: dict[str, str] = {}
    with path.open(encoding="utf-8") as stream:
        for line in stream:
            stripped = line.strip()
            if stripped.startswith(("FAIL", "panic:", "--- FAIL")):
                raise ValueError(f"benchmark command failed: {stripped}")
            if ": " in stripped and not stripped.startswith("Benchmark"):
                key, value = stripped.split(": ", 1)
                if key in {"timestamp_utc", "git_sha", "dirty_tree", "capture_eligibility", "no_regression", "gin_version", "fixture_identity", "go_version", "goos", "goarch", "cpu", "logical_cpus", "benchmark_count", "max_output_bytes", "command"} and key not in metadata:
                    metadata[key] = value
            row = parse_benchmark_line(line)
            if row is None:
                continue
            rows.append(row)
    if not rows:
        raise ValueError("no Gin adapter benchmark rows found")
    missing = sorted(EXPECTED - {str(row["name"]) for row in rows})
    if missing:
        raise ValueError(f"missing benchmark rows: {', '.join(missing)}")
    try:
        expected_samples = int(metadata.get("benchmark_count", "1"))
    except ValueError as error:
        raise ValueError("benchmark_count must be an integer") from error
    if expected_samples <= 0:
        raise ValueError("benchmark_count must be a positive integer")
    raw_cpus = metadata.get("cpu")
    if raw_cpus is None:
        expected_cpus = sorted({int(row["cpu"]) for row in rows})
    else:
        try:
            expected_cpus = [int(value) for value in raw_cpus.split(",")]
        except ValueError as error:
            raise ValueError("cpu metadata must be a comma-separated integer list") from error
        if not expected_cpus or any(value <= 0 for value in expected_cpus) or len(set(expected_cpus)) != len(expected_cpus):
            raise ValueError("cpu metadata must contain unique positive integers")
    if not expected_cpus:
        raise ValueError("no benchmark CPU values found")
    counts = Counter((str(row["name"]), int(row["cpu"])) for row in rows)
    for name in EXPECTED_NAMES:
        for cpu in expected_cpus:
            observed = counts[(name, cpu)]
            if observed != expected_samples:
                raise ValueError(
                    f"benchmark sample count mismatch: {name} cpu={cpu} "
                    f"observed={observed} expected={expected_samples}"
                )
    unexpected_cpus = sorted({int(row["cpu"]) for row in rows} - set(expected_cpus))
    if unexpected_cpus:
        raise ValueError(f"unexpected benchmark CPU values: {unexpected_cpus}")
    if metadata.get("dirty_tree") == "true":
        metadata["no_regression"] = "N/A"
    return rows, metadata


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


def main() -> int:
    args = parse_args()
    try:
        rows, metadata = parse_input(args.input)
        write_json(
            args.output,
            {
                "schema_version": 1,
                "benchmark_prefix": "BenchmarkGinAdapter",
                "expected_names": list(EXPECTED_NAMES),
                "metadata": metadata,
                "rows": rows,
            },
        )
    except (OSError, ValueError) as error:
        print(f"error: {error}")
        return 2
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
