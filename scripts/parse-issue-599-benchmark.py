#!/usr/bin/env python3
"""Parse Issue #599 Go benchmark output into a reviewable JSON summary."""

from __future__ import annotations

import argparse
import json
import os
import re
import statistics
import tempfile
from collections import defaultdict
from pathlib import Path
from typing import Iterable


BENCHMARK_LINE = re.compile(
    r"^(?P<name>BenchmarkIssue599.+)-(?P<workers>\d+)\s+"
    r"(?P<iterations>\d+)\s+"
    r"(?P<ns_per_op>[0-9]+(?:\.[0-9]+)?)\s+ns/op(?P<metrics>.*)$"
)
METRIC = re.compile(r"(?P<value>[0-9]+(?:\.[0-9]+)?)\s+(?P<unit>[A-Za-z][A-Za-z0-9_./-]*)")

REQUIRED_GROUPS = (
    "codec/JSON",
    "codec/NativeFast",
    "codec/NativeCompatible",
    "direct-redis/JSON",
    "direct-redis/NativeFast",
    "direct-redis/NativeCompatible",
    "coordination/JSON",
    "coordination/NativeFast",
    "coordination/NativeCompatible",
    "contention/NativeFast",
)
REQUIRED_SAMPLES = 3


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    return parser.parse_args()


def parse_rows(lines: Iterable[str]) -> dict[str, list[dict[str, object]]]:
    rows: dict[str, list[dict[str, object]]] = defaultdict(list)
    for line in lines:
        match = BENCHMARK_LINE.match(line.strip())
        if match is None:
            continue

        metrics: dict[str, float] = {}
        for metric in METRIC.finditer(match.group("metrics")):
            metrics[metric.group("unit")] = float(metric.group("value"))
        row = {
            "workers": int(match.group("workers")),
            "iterations": int(match.group("iterations")),
            "ns_per_op": float(match.group("ns_per_op")),
            "metrics": metrics,
        }
        rows[match.group("name")].append(row)
    return rows


def stats(values: Iterable[float]) -> dict[str, float]:
    numbers = list(values)
    return {
        "median": float(statistics.median(numbers)),
        "min": float(min(numbers)),
        "max": float(max(numbers)),
    }


def summarize(name: str, samples: list[dict[str, object]]) -> dict[str, object]:
    metric_names = sorted(
        {
            metric_name
            for sample in samples
            for metric_name in sample["metrics"]  # type: ignore[index]
        }
    )
    metrics = {
        metric_name: stats(
            sample["metrics"][metric_name]  # type: ignore[index]
            for sample in samples
            if metric_name in sample["metrics"]  # type: ignore[index]
        )
        for metric_name in metric_names
    }
    return {
        "name": name,
        "samples": len(samples),
        "iterations": sorted({sample["iterations"] for sample in samples}),
        "workers": sorted({sample["workers"] for sample in samples}),
        "ns_per_op": stats(sample["ns_per_op"] for sample in samples),  # type: ignore[index]
        "bytes_per_op": metrics.pop("B/op", None),
        "allocs_per_op": metrics.pop("allocs/op", None),
        "metrics": metrics,
    }


def group_key(name: str) -> str:
    """Normalize benchmark path segments used for required coverage checks."""
    key = name.removeprefix("BenchmarkIssue599").lower()
    return key.replace("directredis", "direct-redis")


def build_summary(parsed: dict[str, list[dict[str, object]]]) -> dict[str, object]:
    if not parsed:
        raise ValueError("no BenchmarkIssue599 rows found")

    names = sorted(parsed)
    rows = [summarize(name, parsed[name]) for name in names]
    under_sampled = [row["name"] for row in rows if row["samples"] < REQUIRED_SAMPLES]
    if under_sampled:
        raise ValueError(
            f"benchmark rows need at least {REQUIRED_SAMPLES} samples: {', '.join(under_sampled)}"
        )
    present_groups = [
        group
        for group in REQUIRED_GROUPS
        if any(group_key(name).startswith(group.lower()) for name in names)
    ]
    missing_groups = [group for group in REQUIRED_GROUPS if group not in present_groups]
    has_round_trip = any("/RoundTrip" in name for name in names)
    has_wire_bytes = any("wire-bytes" in row["metrics"] for row in rows)  # type: ignore[index]
    has_alloc_metrics = all(row["allocs_per_op"] is not None for row in rows)
    if missing_groups:
        raise ValueError(f"missing required benchmark groups: {', '.join(missing_groups)}")
    if not has_round_trip:
        raise ValueError("no RoundTrip benchmark rows found")
    if not has_wire_bytes:
        raise ValueError("no wire-bytes metric found")
    if not has_alloc_metrics:
        raise ValueError("one or more benchmark rows lack allocs/op")

    return {
        "schema_version": 1,
        "benchmark_prefix": "BenchmarkIssue599",
        "required_samples": REQUIRED_SAMPLES,
        "rows": rows,
        "coverage": {
            "required_groups": list(REQUIRED_GROUPS),
            "present_groups": present_groups,
            "missing_groups": missing_groups,
            "scenario_coverage": len(present_groups) / len(REQUIRED_GROUPS),
            "row_count": len(rows),
        },
        "guards": {
            "has_round_trip": has_round_trip,
            "has_wire_bytes": has_wire_bytes,
            "has_alloc_metrics": has_alloc_metrics,
        },
    }


def write_json(path: Path, summary: dict[str, object]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, temporary = tempfile.mkstemp(prefix=f".{path.name}.", suffix=".tmp", dir=path.parent)
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as stream:
            json.dump(summary, stream, ensure_ascii=False, indent=2, sort_keys=True)
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
        with args.input.open(encoding="utf-8") as stream:
            summary = build_summary(parse_rows(stream))
        write_json(args.output, summary)
    except (OSError, ValueError) as error:
        print(f"error: {error}", flush=True)
        return 2
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
