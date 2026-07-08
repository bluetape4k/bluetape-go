# Issue #434 UUID v7 hot-path benchmark environment

- Issue: #434
- Branch: `feat/issue-434-uuid-v7-hotpath`
- Base commit: `001899f`
- Date: 2026-07-08
- OS/arch: `darwin/arm64`
- CPU: `Apple M5`
- Go: `go version go1.26.5 darwin/arm64`
- Benchmark scope: `./id` UUID v7 single-thread and parallel generation
- `benchstat`: `go run golang.org/x/perf/cmd/benchstat@latest`
