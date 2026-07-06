# Issue #401 Raw Benchmark Artifacts

This directory stores accepted raw Go benchmark artifacts for the 0.14.0 SerDe
baseline.

## Files

| File | Purpose |
|---|---|
| `environment.md` | Environment metadata, command inventory, metric direction, fixture versions, and dirty-tree state. |
| `go-serialization-bench.txt` | Full serialization benchmark output. |
| `go-codec-bench.txt` | Full codec benchmark output. |
| `go-compression-bench.txt` | Full compression benchmark output. |

These files are local evidence. They are inputs to #402, not production
rankings by themselves.
