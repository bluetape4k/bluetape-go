Resolves #223.

## Summary

- Documented the Go-native utility parity boundary for logging, time, math,
  measured values, money, geo, and science candidates.
- Kept the current public API unchanged because `core`, `measure`, and `money`
  already cover the small repeated helper demand found for this milestone.
- Filed follow-up research issues for larger domains that exceed #223's closure
  milestone scope.

## Follow-ups

- #275 evaluates `slog`/observability hook boundaries.
- #276 evaluates geo and coordinate utility scope.
- #277 evaluates focused statistics and math utility scope.

## Review

- Step 6-R 7-tier review artifact is included under `docs/superpowers/reviews/`.
- Step 6-R verdict: P0=0, P1=0.
- Go stress: package-specific stress helpers are not applicable because this PR
  adds no Go code, goroutines, shared state, or public concurrency claim.

## Validation

- PASS `git diff --check`
- PASS `make fmt-check`
- PASS `make tidy-check`
- PASS `make vet`
- PASS `golangci-lint cache clean && make lint`
- PASS `make test`
- PASS `make race`
- PENDING GitHub CI

## DoD Status

- [x] Worktree created from current `origin/develop`.
- [x] #223 candidate inventory completed against sibling bluetape4k modules.
- [x] Existing Go package coverage evaluated.
- [x] Kotlin/JVM-specific non-goals documented.
- [x] Follow-up issues filed for oversized observability, geo/science, and math scopes.
- [x] Step 6-R 7-tier review completed with P0=0 P1=0.
- [x] Local validation gates completed.
- [ ] GitHub CI completed.
