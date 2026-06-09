# Issue #35 Money Step 4-T Testlog

## Scope

- Worktree: `.worktrees/issue-35-money`
- Base: `origin/develop`
- Package: `money`
- Issue: #35

## Dependency Evidence

- `go list -m github.com/govalues/money github.com/govalues/decimal`
  - `github.com/govalues/money v0.2.4`
  - `github.com/govalues/decimal v0.1.36`
- `go list -m -json github.com/govalues/money github.com/govalues/decimal`
  - `github.com/govalues/money v0.2.4`, `GoVersion: 1.22`
  - `github.com/govalues/decimal v0.1.36`, `GoVersion: 1.22`
- `gh repo view govalues/money --json nameWithOwner,isArchived,pushedAt,updatedAt,latestRelease,licenseInfo`
  - `govalues/money`, archived false, MIT, latest release `v0.2.4`
- `gh repo view govalues/decimal --json nameWithOwner,isArchived,pushedAt,updatedAt,latestRelease,licenseInfo`
  - `govalues/decimal`, archived false, MIT, latest release `v0.1.36`

## Validation Commands

- `gofmt -w money`: pass
- `go mod tidy`: pass
- `git diff --check`: pass
- `golangci-lint config verify`: pass
- `go test -count=1 ./money`: pass
- `go test -count=1 ./money -run 'Example|Stress|Concurrent'`: pass
- `go test -race -count=1 ./money`: pass
- `go test -count=1 ./money -run 'TestExchangeRate|TestArithmetic|TestConstructors|TestRound|TestCompare|TestParse|TestMarshal'`: pass
- `go test -count=1 ./money -run 'TestExchangeRateValidation|TestExchangeRateConvertOverflow|TestArithmeticErrors|TestParseRejectsAmbiguousOrOversizedInput|ExampleMoney_MarshalText|ExampleMoney_Add_mismatch'`: pass
- `make ci`: pass after implementation commit. This covered `tidy-check`,
  `fmt-check`, `go vet ./...`, `golangci-lint run ./...`, `go test -count=1 ./...`,
  and `go test -race -count=1 ./...`.

## Concurrency Evidence

- `money/money_concurrency_test.go` uses `testing/concurrency.GoroutineStressTester`
  with explicit worker, round, and timeout options.
- `docs/superpowers/reviews/2026-06-09-issue-35-money-concurrency-notes.md`
  records: `AsyncJobTester N/A: #35 money core has no context-aware async, provider, IO, or cancellation boundary.`
