# Issue #485 leader/mongo Single Elector Plan

Issue: [#485](https://github.com/bluetape4k/bluetape-go/issues/485)  
Spec: `docs/superpowers/specs/2026-07-09-issue-485-leader-mongo-elector-design.md`  
Date: 2026-07-09

## Tasks

| Task | Complexity | Files | Verification |
|---|---:|---|---|
| T1 Create package API and document model | high | `leader/mongo/*.go` | `go test -count=1 ./leader/mongo` |
| T2 Implement acquire, renew, release, leader read, and local lifecycle | high | `leader/mongo/elector.go` | `go test -count=1 ./leader/mongo` |
| T3 Add integration and lifecycle tests | high | `leader/mongo/*_test.go` | `go test -p 1 -count=1 ./leader/mongo ./testcontainers/mongodb` |
| T4 Add README pair, root package index, and leader backend notes | medium | `leader/mongo/README*.md`, root/leader README pair | `rg -n "leader/mongo|MongoDB" README*.md leader` |
| T5 Add lesson and code-review artifact | medium | `docs/lessons`, `docs/review` | `git diff --check` |
| T6 Run verification and prepare PR | medium | all changed files | targeted tests, race, `make fmt-check`, `make tidy-check`, `make vet`, PR CI |

## Implementation Notes

- Apply `bluetape-go-patterns` for context cancellation, race/stress coverage,
  owner-token semantics, and public API docs.
- Keep Testcontainers-backed verification serial.
- Do not add dependencies; MongoDB driver and Testcontainers MongoDB are already
  present in `go.mod`.
- Use `_id` as the unique leader key and a TTL index on `lease_until` only for
  cleanup.
- Use client-clock `lease_until` in this first slice and document bounded
  clock-skew assumptions.

## Risk Checks

| Risk | Mitigation |
|---|---|
| Duplicate upsert races under contention | Catch `mongo.IsDuplicateKeyError` and retry until context cancellation. |
| Renewal resurrects expired local ownership | Renewal filter includes `lease_until > now`. |
| Resign deletes a new owner | Delete filter includes local `token`. |
| Goroutine leak | `Resign` cancels renewal and waits on `done`; failed renewal closes `done`. |
| TTL overclaim | README states TTL is cleanup only; tests prove expired takeover without TTL deletion. |

## Required Validation

- `go test -count=1 ./leader ./leader/mongo`
- `go test -race -count=1 ./leader ./leader/mongo`
- `go test -p 1 -count=1 ./leader/mongo ./testcontainers/mongodb`
- `make fmt-check`
- `make tidy-check`
- `make vet`
- `git diff --check`

