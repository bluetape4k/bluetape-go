# Issue #36 Test Log

Date: 2026-06-09
Scope: `probabilistic` package and #36 documentation.

## Commands

```bash
go test -count=1 ./probabilistic
go test -race -count=1 ./probabilistic
git diff --check
rg -n "AsyncJobTester: N/A|probabilistic|#182|Redis-backed Bloom" docs/superpowers README.md README.ko.md WIP.md
golangci-lint cache clean
make ci
```

## Results

- `go test -count=1 ./probabilistic`: PASS
- `go test -race -count=1 ./probabilistic`: PASS
- `go test -count=10 ./probabilistic -run 'TestBloomFilterObservedFalsePositiveRateStaysBounded|TestBloomFilterStressConcurrentOperations|TestBloomFilterStressCustomHasher'`: PASS
- `go test -race -count=3 ./probabilistic -run 'TestBloomFilterStressConcurrentOperations|TestBloomFilterStressCustomHasher'`: PASS
- `git diff --check`: PASS
- `rg -n "AsyncJobTester: N/A|probabilistic|#182|Redis-backed Bloom" docs/superpowers README.md README.ko.md WIP.md`: PASS
- `golangci-lint cache clean`: PASS
- `make ci`: PASS after clearing a stale `golangci-lint` cache entry that
  referenced removed sibling worktree `.worktrees/issue-35-money`; rerun output
  included `0 issues.` and all packages, including `./probabilistic`, passing.
- `make ci`: PASS again after sealed-interface and custom-hasher stress fixes at
  `2026-06-09 15:25:30 KST`.

## Notes

- Deterministic false-positive test inserts 10,000 values into a 1% filter and
  queries 20,000 disjoint missing values with a 3% upper bound.
- Stress test uses `max(32, runtime.GOMAXPROCS(0)*4)` workers and 512 rounds.
- Custom hasher stress test uses `max(16, runtime.GOMAXPROCS(0)*2)` workers and
  256 rounds, and exercises the caller-provided callback path under `-race`.
- Full validation was completed at `2026-06-09 15:25:30 KST`.
