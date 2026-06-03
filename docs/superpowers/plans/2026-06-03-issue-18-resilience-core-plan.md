# Issue 18 Resilience Core Plan

## Classification

- Work type: Type A - Full Feature.
- Basis: new public package, new composable API surface, docs/spec/plan
  artifacts, tests, README updates.
- Lane: direct worktree execution with local review-delta and validation gates.

## Sequence

1. Update epic and child issue text to state first-party implementation and
   reference-only external libraries.
2. Add superpowers research inventory, implementation spec, and this plan before
   extending the code further.
3. Add `resilience` package docs and core composition types.
4. Add retry/backoff implementation with fake-sleeper test support.
5. Add timeout implementation with cooperative context semantics.
6. Add tests and examples that lock the #18 contract.
7. Run focused tests, repo-wide tests, `go vet`, formatting, diff checks, and
   local review-delta.
8. Fix review findings before publishing a PR.

## Review Gate

Review must check:

- whether first-party implementation accidentally leaks external-library shape
  or dependencies
- composition order clarity
- error unwrapping and sentinel behavior
- context cancellation and timeout classification
- event skeleton adequacy for #21
- deterministic or bounded test behavior
- README/research consistency

## Validation Gate

Required before completion:

- `go test -count=1 ./resilience`
- `go test -count=1 ./...` or documented infrastructure failure
- `go vet ./...`
- `gofmt`
- `git diff --check`
- local diff review with concrete findings
