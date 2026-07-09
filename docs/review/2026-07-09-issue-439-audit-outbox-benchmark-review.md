# Issue #439 Audit Outbox Benchmark Review

Issue: [#439](https://github.com/bluetape4k/bluetape-go/issues/439)
Branch: `feat/issue-439-audit-bench`
Baseline: `cbc5b0a`
Date: 2026-07-09

## Scope

- `audit/benchmark_test.go`
- `audit/sqloutbox/benchmark_test.go`
- `audit/README.md` and `audit/README.ko.md`
- `audit/sqloutbox/README.md` and `audit/sqloutbox/README.ko.md`
- `docs/research/2026-07-09-issue-439-audit-outbox-benchmark.md`
- `docs/research/outputs/issue-439/*`
- `docs/images/readme-charts/audit-outbox-benchmark-summary.*`

## Evidence

- Issue #439 requires benchmark commands/raw output preservation, separate
  in-memory and PostgreSQL/Testcontainers rows, environment metadata, batch and
  payload sizes, and measured evidence for follow-up adapter work.
- The research report includes result tables, the summary chart, commands, raw
  output paths, environment metadata, interpretation, caveats, and follow-up use.
- The chart generator enforces 8 in-memory rows and 4 SQL outbox rows before
  writing SVG and Vega-Lite JSON artifacts.
- The PostgreSQL benchmark is opt-in with
  `BLUETAPE_AUDIT_SQL_OUTBOX_BENCH=1` and uses `postgres:16-alpine` through the
  repository Testcontainers fixture.

## 7-Tier Lanes

| Lane | Verdict | Notes |
|---|---|---|
| Performance | PASS | P0=0 P1=0. In-memory repository, JSON round-trip, enqueue, claim, publish, and dead-letter rows are recorded with `ns/op` or `ms/op`, `B/op`, and `allocs/op`. |
| Stability | PASS | P0=0 P1=0. Test and race gates pass for `./audit` and `./audit/sqloutbox`; PostgreSQL rows are bounded by context timeouts. |
| Security | PASS | P0=0 P1=0. Artifacts contain synthetic payloads and benchmark metadata only; no secrets, tokens, or production audit data are recorded. |
| Operator/Ops | PASS | P0=0 P1=0. Docker-backed benchmark remains explicit opt-in and serial, with image/version and local-runtime caveat documented. |
| Developer/API | PASS | P0=0 P1=0. Public audit and outbox APIs are unchanged; benchmark helpers stay in `_test.go` files. |
| User/Caller | PASS | P0=0 P1=0. README files expose commands and chart links without changing default usage semantics. |
| Integration | PASS | P0=0 P1=0. Main-session review accepted benchmark-only scope and confirmed follow-up adapter claims are evidence-linked, not implied by this branch. |

## Validation

| Command | Status | Evidence |
|---|---|---|
| `go test -count=1 ./audit` | PASS | Audit package tests passed. |
| `go test -count=1 ./audit/sqloutbox` | PASS | SQL outbox package tests passed. |
| `go test -race -count=1 ./audit` | PASS | Audit race gate passed. |
| `go test -race -count=1 ./audit/sqloutbox` | PASS | SQL outbox race gate passed. |
| `go test -run '^$' -bench 'Benchmark(MemoryRepository\|AuditEntryJSONRoundTrip)' -benchmem ./audit` | PASS | Raw output preserved in `docs/research/outputs/issue-439/audit-memory-bench.txt`. |
| `BLUETAPE_AUDIT_SQL_OUTBOX_BENCH=1 go test -p 1 -run '^$' -bench '^BenchmarkAuditSQLOutboxPostgres' -benchtime=100x -benchmem ./audit/sqloutbox` | PASS | Raw output preserved in `docs/research/outputs/issue-439/audit-sqloutbox-postgres-bench.txt`. |
| `go test -run '^$' -bench '^BenchmarkAuditSQLOutboxPostgres' -benchmem ./audit/sqloutbox` | PASS | Default run skips the Testcontainers benchmark without Docker startup. |
| `node docs/images/readme-charts/generate-audit-outbox-benchmark-summary.mjs` | PASS | Chart gate reported `memoryRows=8 sqlRows=4 panels=3 bars=16 canvas=1800x1700`. |
| `xmllint --noout docs/images/readme-charts/audit-outbox-benchmark-summary.svg` | PASS | SVG is well formed. |
| `cairosvg ... -o docs/images/readme-charts/audit-outbox-benchmark-summary.png -s 2` | PASS | PNG rendered at 3600x3400. |
| `git diff --check` | PASS | No whitespace errors. |
| `make fmt-check` | PASS | Go files are formatted. |
| `make tidy-check` | PASS | `go.mod` and `go.sum` stayed unchanged. |
| `make vet` | PASS | Vet completed. |
| `make lint` | PASS | GolangCI-Lint completed with 0 issues after clearing a stale deleted-worktree cache entry. |
| `make ci` | PASS | Full local CI gate completed, including tidy, format, vet, lint, test, and race. |
| Artifact hygiene scan | PASS | No local user paths or secret-like tokens found in issue #439 benchmark artifacts. |

## Findings

P0=0 P1=0

- P2 resolved: PostgreSQL benchmark initially reported impossible operation
  timings when the measured SQL call was outside the timer window. The benchmark
  now starts the timer around `Enqueue`, `Claim`, and `RunOnce` calls.
- P3 documented: local Testcontainers latency is planning evidence only, not a
  production ranking or adapter-selection conclusion.

## Residual Risk

The retained benchmark evidence is a local snapshot on darwin/arm64 with
`postgres:16-alpine`. It does not cover concurrent relay workers, production
PostgreSQL tuning, connection-pool sizing, network latency, or future broker
adapters.
