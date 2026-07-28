# Issue 18 Resilience Core Inventory

## 맥락

Issue #18은 milestone `0.2.0`을 시작하며 first-party `resilience` package를 만든다.
목표는 기존 Go library를 감싸는 것이 아니라 retry, timeout, circuit breaker, bulkhead,
observability hook, HTTP example로 확장 가능한 Go-native policy model을 소유하는 것이다.

## Source Inventory

관찰한 source와 reference input은 다음과 같다.

- `bluetape4k-projects/infra/resilience4j`: service-level resilience concept.
- `bluetape4k-projects/ktor/resilience4j`: 나중에 `net/http`로 대응할 framework integration example.
- `failsafe-go`: composable policy shape와 executor-oriented API idea.
- `cenkalti/backoff`: exponential backoff와 jitter design idea.
- `sony/gobreaker`: #19 circuit breaker state와 half-open behavior reference.
- `golang.org/x/sync/semaphore`: #19 bulkhead/concurrency limiter reference.
- `golang.org/x/time/rate`: future token-bucket rate limiter reference.

## 지금 구현할 것

- `Operation[T]`: `context.Context`를 받는 work unit.
- `Policy[T]`: operation을 감싸는 composable wrapper.
- `Compose`와 `Run`: policy 적용 및 실행 helper.
- Retry policy:
  - max attempts
  - retryable error predicate
  - pluggable backoff
  - deterministic test용 pluggable sleeper
- Backoff policy:
  - no delay
  - constant delay
  - optional jitter가 있는 exponential delay
- Timeout policy:
  - `context.WithTimeout` composition
  - cooperative timeout semantics
- Common errors:
  - retry exhaustion
  - policy-owned timeout
  - `errors.Is`와 `errors.As`를 통한 wrapped cause preservation
- Event skeleton:
  - success
  - retry
  - timeout

## 미룰 것

- Circuit breaker implementation: #19.
- Bulkhead implementation: #19.
- Full event payload matrix와 ordering coverage: #21.
- HTTP middleware와 README copy-paste example: #20.
- OpenTelemetry 또는 metrics exporter dependency: #21 이후.
- Rate limiter implementation: 별도 `0.2.0` child issue가 끌어오지 않는 한 later milestone.

## 결정

작은 public `resilience` package를 first-party implementation으로 만든다. 외부 library는
reference material일 뿐이다. behavior는 deterministic하게, composition은 explicit하게,
error는 standard Go error handling으로 검사 가능하게 유지한다.
