# Issue #599 benchmark environment

- Issue: `599`
- Command: `go test -run '^$' -bench '^BenchmarkIssue599' -benchmem -count=3 ./cache/redisfory`
- Parse command: `python3 scripts/parse-issue-599-benchmark.py --input docs/research/outputs/issue-599/benchmark.txt --output docs/research/outputs/issue-599/summary.json`
- Timestamp (UTC): `2026-08-07T04:49:44Z`
- Git SHA at capture: `84ff458a257a9da737856f370c39360300b635b7`
- Source state: approved Issue #599 benchmark/parser changes were uncommitted in the isolated worktree at capture time
- OS/architecture: `darwin/arm64`
- CPU: `Apple M5`
- Go: `go1.26.5`
- Apache Fory: `github.com/apache/fory/go/fory v1.3.0`
- Redis image: `redis:7.4-alpine@sha256:6ab0b6e7381779332f97b8ca76193e45b0756f38d4c0dcda72dbb3c32061ab99`
- Samples: 3 per benchmark row (`-count=3`)
- Redis execution: Testcontainers workloads were run serially
- Raw output: [benchmark.txt](benchmark.txt)
- Parsed summary: [summary.json](summary.json)
- Written report: [Issue #599 report](../../../benchmarks/2026-08-07-issue-599-fory-redis.md)
