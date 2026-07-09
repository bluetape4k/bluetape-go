# Issue 569 Redis Foundation Lessons

## Context

Issue #569 added the public `github.com/bluetape4k/bluetape-go/redis` package
with package name `btredis`. The package is a foundation slice only: Redis key
builders, owner tokens, lease scripts, TTL validation, and redacted operation
errors. Existing Redis-backed packages stay unmigrated until #570 can prove
old/new parity and benchmark impact.

## Lessons

- Public Redis foundation helpers must reject nil contexts, pre-canceled
  contexts, nil clients, typed nil clients, invalid leases, and invalid TTLs
  before dispatching external Redis I/O.
- Owner tokens are credentials, not diagnostic values. Keep `String`,
  `GoString`, and `slog.LogValuer` redacted by default, and expose the raw value
  only through an explicitly sensitive Redis argument method.
- Structural Redis key parts and caller-owned logical keys need separate APIs.
  Structural parts can reject delimiters and braces; logical keys must preserve
  caller bytes verbatim when the caller owns the key namespace.
- Redis script cancellation has a split boundary. Pre-dispatch cancellation can
  be returned as the caller-owned context error; post-dispatch cancellation has
  indeterminate commit state and must be documented as a runbook concern.
- Redis Cluster hash tags are compatibility surfaces. `WithHashTag` must reject
  empty/braced tags but preserve colon-bearing tags because existing
  `probabilistic/redis` namespaces rely on that shape.
- New shared Redis primitives should be introduced before migration. Migration
  PRs need old/new key parity tests and provider benchmark evidence rather than
  silently replacing package-local helpers.

## Verification Notes

- TDD red commands were run before each production slice:
  `go test -count=1 ./redis -run 'OwnerToken|NewOwnerToken|ParseOwnerToken'`,
  `go test -count=1 ./redis -run 'Key|TTL|OpError|Redacted'`, and
  `go test -count=1 ./redis -run 'Lease|CompareAnd'`.
- Real Redis script behavior was verified with the repo-local
  `testcontainers/redis` fixture under serial package execution.
- Final verification for this branch included `go test -p 1 -count=1 ./redis`,
  `go test -p 1 -race -count=1 ./redis`, `go test -count=1 ./redis -run Example`,
  `git diff --check`, no-migration import scan, and `make ci`.
