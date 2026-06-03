# Issue 21 Observability Hooks Spec Review

## Scope

- Spec: `docs/superpowers/specs/2026-06-03-issue-21-observability-hooks-spec.md`
- Research: `docs/superpowers/research/2026-06-03-issue-21-observability-hooks-inventory.md`
- Issue: #21
- Package: `resilience`
- Review gate: Step 2-R

## Evidence

- Current source inspected:
  - `resilience/events.go`
  - `resilience/retry.go`
  - `resilience/timeout.go`
  - `resilience/circuit_breaker.go`
  - `resilience/bulkhead.go`
  - `resilience/errors.go`
- Prior specs/lessons inspected:
  - `docs/superpowers/specs/2026-06-03-issue-18-resilience-core-spec.md`
  - `docs/superpowers/specs/2026-06-03-issue-19-circuit-breaker-bulkhead-spec.md`
  - `docs/lessons/2026-06-03-resilience-core-workflow.md`
  - `docs/lessons/2026-06-03-resilience-circuit-breaker-bulkhead.md`
- Graph evidence:
  - CodeGraph: 107 files, 927 nodes, 1,647 edges
  - code-review-graph: 104 files, 497 nodes, 3,691 edges

## Findings

- P0: 0
- P1: 0
- P2: 1 found and fixed
  - Bulkhead admission category was initially ambiguous because the spec listed
    success/retry/timeout/rejection/transition/failure categories but described
    accepted bulkhead calls as "success or admission-compatible". The spec now
    includes `admission` as a first-class category and requires accepted
    bulkhead events to use it.
- P3: 0

## Tier Verdicts

| Tier | Scope | Verdict | Evidence |
|---|---|---:|---|
| 1 Security | sensitive data and dependency risk | PASS | Spec adds no exporter, global registry, network IO, auth, secrets, or deserialization. It keeps event payloads caller-supplied and avoids vendor dependencies. |
| 2 Ops/SRE | diagnosis and operational predictability | PASS | Spec requires stable policy type/category/error-category labels, synchronous predictable handlers, and no internal locks while invoking handler code. |
| 3 Structural impact | API and package blast radius | PASS | Existing `EventHandler` and option fields remain; `Event` gains additive fields and constants only. |
| 4 API quality | Go public API shape | PASS | Spec keeps simple structs/constants and avoids a heavy observer interface or global bus. |
| 5 Testability | event ordering and silent failures | PASS | Spec requires ordering tests for retry, timeout, circuit transitions/rejection, and bulkhead admission/rejection/success. |
| 6 Performance/stability | hot path and locking | PASS | Spec preserves synchronous handler behavior and explicitly forbids calling handlers while circuit breaker locks are held. |
| 7 Docs/evidence | README/package docs and issue traceability | PASS | Spec requires package docs, README locale mention, validation commands, and PR metadata for issue #21. |

## Integrated Verdict

P0=0 and P1=0. Step 2-R is closed.
