# Issue 21 Observability Hooks Plan

## Context

- Issue: #21 `Add observability hooks for resilience policies`
- Spec: `docs/superpowers/specs/2026-06-03-issue-21-observability-hooks-spec.md`
- Research: `docs/superpowers/research/2026-06-03-issue-21-observability-hooks-inventory.md`
- Base: stacked on PR #96 because #21 extends #19 circuit/bulkhead events.

## Tasks

| Task | Description | Acceptance Evidence |
|---|---|---|
| T1 | Extend `resilience/events.go` with stable policy type, event category, and error category constants plus additive event payload fields. | Public constants compile; existing tests still compile. |
| T2 | Add small internal event constructors/classification helpers to keep policy event literals consistent. | Policy files use helpers or stable constants consistently; no runtime dependencies. |
| T3 | Update retry emission for retry scheduling, success, retry exhaustion, and predicate-rejected failures. | Retry ordering and exhaustion tests pass; `errors.Is(err, ErrRetryExhausted)` still works. |
| T4 | Update timeout emission with category, timeout duration, and error category while preserving parent-cancellation behavior. | Timeout event tests pass; parent cancellation is not mislabeled. |
| T5 | Update circuit breaker and bulkhead events with stable policy type/category/error category fields. | Circuit transition/rejection and bulkhead admission/rejection/success tests pass. |
| T6 | Add deterministic event ordering/payload tests for retry, timeout, circuit breaker, and bulkhead. | `go test -count=1 ./resilience` and race test pass. |
| T7 | Update package docs and README locale pair to explain synchronous event bridging without vendor dependencies. | README names match source constants/API; docs are source-consistent. |
| T8 | Run full verification and graph-aware 7-tier review. | Required commands pass; review artifact records P0=0/P1=0. |
| T9 | Write lesson, commit, push, create PR, post-PR review, wait for CI, and report DoD. | PR # recorded with milestone `0.2.0`, assignee `debop`, CI success. |

## Validation Commands

- `go test -count=1 ./resilience`
- `go test -race -count=1 ./resilience`
- `go test -count=1 ./...`
- `go vet ./...`
- `golangci-lint run ./...`
- `make fmt-check`
- `go mod tidy && git diff --exit-code -- go.mod go.sum`
- `git diff --check`
- `codegraph sync . && codegraph status .`
- `code-review-graph build --repo . && code-review-graph status --repo .`

## Review Gates

- Step 3-R plan/test review:
  - verify event ordering tests cover each issue acceptance criterion
  - verify no new dependency or async handler is planned
  - verify additive public API shape
- Step 6-R implementation review:
  - Tier 1 security: event data and dependency risk
  - Tier 2 Ops/SRE: logging/metric bridge usability and handler predictability
  - Tier 3 structural: public API and policy blast radius
  - Tier 4 Go quality: constants/helpers, source compatibility, doc comments
  - Tier 5 tests: ordering, payload, parent cancellation, sentinel errors
  - Tier 6 performance/stability: handler lock boundaries and hot path overhead
  - Tier 7 docs/evidence: README locale, package docs, PR DoD, CI evidence

## Risks and Controls

- Risk: adding a typed `PolicyType` field could break callers that assign a
  `string` variable to `Event.PolicyType`.
  - Control: keep `Event.PolicyType` as `string`; add string constants.
- Risk: handler calls while a circuit breaker lock is held could deadlock if
  handler calls `State`.
  - Control: preserve current transition event pattern and review lock scopes.
- Risk: event tests become timing-flaky.
  - Control: use fake sleeper, fake clock, channels, and bounded deadlines only
    when testing timeout behavior.
- Risk: event categories become too broad for metrics.
  - Control: keep a separate `ErrorCategory` field for error label specificity.

## Stop Condition

Stop after PR is created, reviewed, CI is green, and the user receives the
Step DoD report. Do not merge without an explicit merge request.
