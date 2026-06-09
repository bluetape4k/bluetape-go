# Issue #35 Money Concurrency Notes

- `GoroutineStressTester`: `money/money_concurrency_test.go` repeatedly parses,
  serializes, adds same-currency values, and rejects cross-currency operations
  across goroutines with explicit worker, round, and timeout options.
- Race gate: `go test -race -count=1 ./money`.
- AsyncJobTester N/A: #35 money core has no context-aware async, provider, IO, or cancellation boundary.
