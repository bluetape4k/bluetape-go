# Issue 21 Observability Hooks Inventory

## Scope

- Repository: `bluetape-go`
- Branch: `feat/issue-21-observability-hooks`
- Base: stacked on PR #96 branch `origin/feat/issue-19-circuit-breaker-bulkhead`
- Issue: #21, milestone `0.2.0`
- Package: `resilience`

## Current Source Evidence

- `resilience/events.go`
  - defines `EventKind`, `Event`, `EventHandler`, and `emitEvent`
  - currently exposes success, retry, timeout, circuit transition, circuit
    rejection, bulkhead admission, and bulkhead rejection event kinds
  - current payload fields: policy name/type, attempt, delay, error, circuit
    state, previous state, and in-flight count
- `resilience/retry.go`
  - emits `EventRetry` before sleeping for another attempt
  - emits `EventSuccess` when an attempt succeeds
  - does not emit an event for retry exhaustion or predicate-rejected failures
- `resilience/timeout.go`
  - emits `EventSuccess` on success
  - emits `EventTimeout` only when the timeout policy's own child context
    deadline expires
  - current event does not expose timeout duration except through
    `TimeoutError`
- `resilience/circuit_breaker.go`
  - emits circuit state transition events outside the mutex
  - emits circuit rejection events for open and half-open overflow states
  - does not emit a generic success event for ordinary closed-state success
- `resilience/bulkhead.go`
  - emits bulkhead admission, rejection, and success
  - does not emit failure events for operation errors after admission

## Prior Artifact Evidence

- `docs/superpowers/specs/2026-06-03-issue-18-resilience-core-spec.md`
  introduced the event skeleton and intentionally deferred the full contract to
  #21.
- `docs/superpowers/specs/2026-06-03-issue-19-circuit-breaker-bulkhead-spec.md`
  required circuit and bulkhead events to remain compatible with #21 expansion.
- `docs/lessons/2026-06-03-resilience-core-workflow.md`
  records that resilience must remain first-party and context-aware.
- `docs/lessons/2026-06-03-resilience-circuit-breaker-bulkhead.md`
  records deterministic tests as a rule for stateful resilience primitives.
- `docs/research/2026-06-01-milestone-0.2.0-resilience-research.md`
  lists structured event hooks for metrics/OpenTelemetry integration as a
  milestone goal, but direct telemetry exporters are outside #21.

## Graph Evidence

- CodeGraph indexed this worktree: 107 files, 927 nodes, 1,647 edges.
- code-review-graph built this worktree: 104 files, 497 nodes, 3,691 edges.
- CodeGraph exploration of `Event`, `emitEvent`, and policy `Apply` methods
  shows that `emitEvent` is called from retry, timeout, circuit breaker, and
  bulkhead paths. The structural blast radius is therefore limited to the
  `resilience` package plus README/package docs/tests.

## Adopt / Borrow / Skip

- Adopt:
  - Keep the synchronous `EventHandler func(context.Context, Event)` shape.
  - Keep `Event` as a simple struct so callers can bridge it to logs, counters,
    or tracing without a dependency.
  - Add payload fields only when they are stable and useful for first-party
    policies.
- Borrow:
  - Use resilience4j/failsafe-style event naming ideas only as conceptual
    reference; keep Go names explicit and small.
  - Use event category strings that are easy to map to metrics labels.
- Skip:
  - No OpenTelemetry dependency or exporter.
  - No async event bus or background dispatcher.
  - No global observer registry in #21.

## Design Constraints

- Event emission must remain synchronous and deterministic.
- Handlers are user code; policy mutexes must not be held while handlers run.
- Existing `Event` struct literals should remain source-compatible.
- Event ordering tests must avoid arbitrary sleeps except where timeout behavior
  itself is under test.
- HTTP middleware in #20 will consume this contract, so policy type/kind/category
  values should be stable constants.
