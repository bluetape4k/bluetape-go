# Issue 19 Circuit Breaker and Bulkhead Plan

## Classification

- Work type: Type A - Full Feature.
- Basis: new public resilience APIs, concurrency behavior, error contracts,
  docs/spec/plan artifacts, deterministic tests.
- Lane: issue worktree `feat/issue-19-circuit-breaker-bulkhead`.

## Sequence

1. Carry the local `.gitignore` cleanup commit in the PR branch because direct
   `develop` push is protected.
2. Add this issue #19 research, spec, and plan before implementation.
3. Extend resilience events and errors for circuit breaker and bulkhead.
4. Implement circuit breaker with mutex-protected state, counters, injected
   time source, and bounded half-open probes.
5. Implement bulkhead with first-party permit accounting, optional waiting, and
   cancellation-safe acquisition.
6. Add deterministic unit tests and examples for circuit breaker and bulkhead.
7. Update README locale package descriptions if the public status changes from
   retry/timeout only to retry/timeout/circuit breaker/bulkhead.
8. Run focused tests, race tests, repo tests, vet, raw golangci-lint, format,
   tidy-check, diff-check, CodeGraph/code-review-graph review, and 7-tier
   review.
9. Open PR with milestone `0.2.0`, assignee `debop`, issue links, problem
   context, solution summary, validation, and final DoD status.

## Review Gate

Review must check:

- no external runtime resilience dependency or wrapper
- composition with existing `Policy[T]`
- state transition correctness under concurrency
- half-open admission determinism
- context cancellation and permit release
- error sentinel and typed error behavior
- event hook compatibility for #21
- test determinism and race-safety

## Validation Gate

Required before completion:

- `go test -count=1 ./resilience`
- `go test -race -count=1 ./resilience`
- `go test -count=1 ./...`
- `go vet ./...`
- `golangci-lint run ./...`
- `make fmt-check`
- `go mod tidy && git diff --exit-code -- go.mod go.sum`
- `git diff --check`
- CodeGraph status and code-review-graph review context
