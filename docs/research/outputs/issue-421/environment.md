# Issue #421 Benchmark Environment

Generated UTC: 2026-07-08T10:37:24Z
Generated local: 2026-07-08 19:37:24 KST

## Host

- OS/arch: darwin/arm64
- Kernel: Darwin 25.5.0 arm64 (hostname redacted)
- CPU: Apple M5
- Go: go version go1.26.5 darwin/arm64

## Package Revision

- Branch: feat/issue-421-rules-bench
- Base commit: d29b86e
- Issue: #421

## Dirty Tree At Capture

```text
 M Makefile
?? docs/research/outputs/issue-421/
?? rules/rules_benchmark_test.go
```

## Command Inventory

| Purpose | Command | Raw output file |
|---|---|---|
| Rules composite/inference benchmark rows | `make bench-rules` (`go test -run '^$' -bench '^BenchmarkRules' -benchmem -count=5 ./rules`) | `rules-benchmark.txt` |

## Metric Direction

| Metric | Direction |
|---|---|
| `ns/op` | Lower is better for the same benchmark row and host. |
| `B/op` | Lower is better for allocation volume. |
| `allocs/op` | Lower is better for allocation count. |

## Interpretation Boundary

This is local benchmark evidence for existing rules composite, sequential engine, and bounded inference paths. Composite rows reuse a steady-state `Facts` map inside each timed loop; inference rows split stable `Count0` and `Count1` workloads. The artifact records the current hot-path shape for follow-up optimization decisions; it does not claim a production ranking or justify API changes by itself.
