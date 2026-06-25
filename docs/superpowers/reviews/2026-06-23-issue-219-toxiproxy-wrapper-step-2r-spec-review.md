# Issue #219 Step 2-R Spec Review

Issue: [#219](https://github.com/bluetape4k/bluetape-go/issues/219)
Spec: `docs/superpowers/specs/2026-06-23-issue-219-toxiproxy-wrapper-design.md`
Matrix: `docs/superpowers/research/2026-06-23-issue-219-messaging-http-fixtures-matrix.md`
Date: 2026-06-23

## Review Mode

Six native review lanes were attempted, but every spawn failed with
`collab spawn failed: agent thread limit reached`. Cleanup of existing slots was
interrupted by the user after a long wait. Per user direction, this gate uses
main-session integration fallback for all six perspectives.

## Reviewed Scope

- #219 candidate matrix and roadmap routing.
- Toxiproxy-first wrapper design.
- Public API, validation plan, README expectations, and deferred scope.

## Findings

| Tier | Perspective | P0 | P1 | P2 | P3 | Evidence |
|---|---|---:|---:|---:|---:|---|
| 1 | Performance | 0 | 0 | 0 | 0 | Spec rejects latency timing assertions and requires bounded failure checks instead of sleep-heavy tests. |
| 2 | Stability | 0 | 0 | 0 | 0 | Spec requires context timeouts, `testing.TB.Cleanup`, bounded container cleanup, and construction-failure termination. |
| 3 | Security | 0 | 0 | 0 | 0 | No secrets or production credentials are introduced; network behavior is test-only Docker proxying. |
| 4 | Operator/Ops | 0 | 0 | 0 | 0 | README requirements include Docker, dynamic ports, bounded client timeouts, and normal-CI caveats. |
| 5 | Developer/API | 0 | 0 | 0 | 0 | API uses upstream `testcontainers.ContainerCustomizer` rather than inventing a first-party proxy DSL. |
| 6 | User/Caller | 0 | 0 | 0 | 0 | Caller-facing docs must show Redis proxying, control URI export, failure-injection caveats, and linked deferrals. |

## Resolved During Review

| Severity | Finding | Resolution |
|---|---|---|
| P1 | Matrix first-slice API text still described `Start(ctx, testing.TB)` and `StartServer(ctx, testing.TB)` without the required upstream customizer surface. | Updated the matrix to match the spec: `Start(ctx, testing.TB, opts...)` and `StartServer(ctx, testing.TB, opts...)`. |

## Integrated Verdict

P0=0 P1=0

The spec is narrow enough for one PR, avoids premature broker/HTTP mock catalog
work, and provides testable acceptance criteria for the Toxiproxy slice.
