# Issue 21 Observability Hooks Inventory

## 범위

- Repository: `bluetape-go`
- Branch: `feat/issue-21-observability-hooks`
- Base: PR #96 branch `origin/feat/issue-19-circuit-breaker-bulkhead` 위에 stacked
- Issue: #21, milestone `0.2.0`
- Package: `resilience`

## Current Source Evidence

- `resilience/events.go`
  - `EventKind`, `Event`, `EventHandler`, `emitEvent`를 정의한다.
  - 현재 success, retry, timeout, circuit transition, circuit rejection,
    bulkhead admission, bulkhead rejection event kind를 노출한다.
  - 현재 payload field는 policy name/type, attempt, delay, error, circuit state,
    previous state, in-flight count다.
- `resilience/retry.go`
  - 다른 attempt 전에 `EventRetry`를 emit한다.
  - attempt가 성공하면 `EventSuccess`를 emit한다.
  - retry exhaustion 또는 predicate-rejected failure에 대한 event는 아직 없다.
- `resilience/timeout.go`
  - success 시 `EventSuccess`를 emit한다.
  - timeout policy가 소유한 child context deadline이 만료될 때만 `EventTimeout`을 emit한다.
  - 현재 event는 `TimeoutError` 외에 timeout duration을 직접 노출하지 않는다.
- `resilience/circuit_breaker.go`
  - mutex 밖에서 circuit state transition event를 emit한다.
  - open 및 half-open overflow state에서 circuit rejection event를 emit한다.
  - ordinary closed-state success에 대한 generic success event는 emit하지 않는다.
- `resilience/bulkhead.go`
  - bulkhead admission, rejection, success를 emit한다.
  - admission 이후 operation error에 대한 failure event는 아직 없다.

## Prior Artifact Evidence

- `docs/superpowers/specs/2026-06-03-issue-18-resilience-core-spec.md`는 event skeleton을
  도입했고 full contract는 의도적으로 #21로 미뤘다.
- `docs/superpowers/specs/2026-06-03-issue-19-circuit-breaker-bulkhead-spec.md`는 circuit 및
  bulkhead event가 #21 expansion과 compatible하게 남아야 한다고 요구했다.
- `docs/lessons/2026-06-03-resilience-core-workflow.md`는 resilience가 first-party이고
  context-aware여야 한다고 기록한다.
- `docs/lessons/2026-06-03-resilience-circuit-breaker-bulkhead.md`는 stateful resilience
  primitive의 deterministic test rule을 기록한다.
- `docs/research/2026-06-01-milestone-0.2.0-resilience-research.md`는 metrics/OpenTelemetry
  integration을 위한 structured event hook을 milestone goal로 둔다. direct telemetry exporter는 #21 밖이다.

## Graph Evidence

- CodeGraph indexed this worktree: 107 files, 927 nodes, 1,647 edges.
- code-review-graph built this worktree: 104 files, 497 nodes, 3,691 edges.
- `Event`, `emitEvent`, policy `Apply` method를 탐색한 결과 `emitEvent`는 retry, timeout,
  circuit breaker, bulkhead path에서 호출된다. structural blast radius는 `resilience`
  package와 README/package docs/tests로 제한된다.

## Adopt / Borrow / Skip

- Adopt:
  - synchronous `EventHandler func(context.Context, Event)` shape를 유지한다.
  - caller가 dependency 없이 log, counter, tracing으로 bridge할 수 있도록 `Event`는 simple struct로 둔다.
  - first-party policy에 안정적이고 유용한 payload field만 추가한다.
- Borrow:
  - resilience4j/failsafe-style event naming은 conceptual reference로만 사용한다.
  - metrics label로 mapping하기 쉬운 event category string을 사용한다.
- Skip:
  - OpenTelemetry dependency 또는 exporter 없음.
  - async event bus 또는 background dispatcher 없음.
  - #21에서 global observer registry 없음.

## Design Constraints

- event emission은 synchronous와 deterministic을 유지해야 한다.
- handler는 user code다. policy mutex를 잡은 채 handler를 실행하면 안 된다.
- 기존 `Event` struct literal은 source-compatible해야 한다.
- event ordering test는 timeout behavior 자체를 검증하는 경우 외에는 arbitrary sleep을 피한다.
- #20 HTTP middleware가 이 contract를 소비하므로 policy type/kind/category 값은 stable constant여야 한다.
