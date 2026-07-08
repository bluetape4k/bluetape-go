# Issue #421 Rules Benchmark Evidence

## Lesson

Benchmark follow-up issues should make the public Make target reproduce the retained evidence command, not just provide a quick one-shot smoke command. For issue #421, the final `bench-rules` target is phony, listed in `make help`, and runs the same `-count=5` command preserved in `docs/research/outputs/issue-421/rules-benchmark.txt`.

## What Changed

- Added focused rules benchmarks for composite activation, unit, conditional, bounded inference, and sequential engine paths.
- Split bounded inference into stable `Count0` and `Count1` sub-benchmarks instead of mixing workloads inside one row.
- Preserved raw benchmark output and sanitized environment metadata under `docs/research/outputs/issue-421/`.

## Next Time

- Record benchmark command, raw output, metric direction, and interpretation boundary together.
- Avoid host fingerprints in durable benchmark artifacts.
- Treat benchmark rows as evidence for the specific workload shape they measure; add a separate row when a fresh-request workload matters.
