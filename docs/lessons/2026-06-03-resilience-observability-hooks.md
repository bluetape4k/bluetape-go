# Resilience Observability Hooks Lessons (2026-06-03)

Related issue: #21
Affected module: `resilience`

## L1: telemetry adapter보다 low-cardinality label을 먼저 안정화한다

### 문제

Retry, timeout, circuit breaker, bulkhead는 이미 일부 event를 emit했지만 caller는
event kind string과 policy type string을 ad hoc data처럼 다뤄야 했다. HTTP
middleware나 future telemetry bridge는 log, counter, span mapping 전에 stable
label이 필요하므로 이 상태는 brittle하다.

### 교훈

first-party resilience policy는 exporter나 middleware를 추가하기 전에 policy type,
event category, error category contract를 정의한다. exporter dependency는 core
package 밖에 두고, `OnEvent`가 service의 기존 logging/metrics/tracing stack으로
bridge하게 한다.

### Evidence

- `resilience/events.go`는 `EventHandler`를 바꾸지 않고 stable policy type, event
  category, error category constant를 추가했다.
- `README.md`, `README.ko.md`, `resilience/doc.go`는 synchronous handler behavior와
  no-exporter boundary를 문서화했다.
- `go mod tidy && git diff --exit-code -- go.mod go.sum`은 runtime dependency가
  추가되지 않았음을 확인했다.

## L2: 모든 emission path를 전용 regression test와 대조한다

### 문제

첫 구현은 retry predicate-rejected failure emission을 추가했지만 initial test set은
retry ordering과 retry exhaustion만 다뤘다. focused event test가 없으면 future
change가 predicate-rejected failure를 제거하거나 mislabel해도 실패하지 않는다.

### 교훈

policy에 여러 event-producing branch가 있으면 Step 6-R에서 각 branch를 named
regression test에 mapping한다. branch가 구현됐지만 test되지 않았으면 headline
acceptance case가 아니어도 PR creation 전에 test gap을 고친다.

### Evidence

- Step 6-R은 missing predicate-rejected retry test를 P2 finding으로 기록했다.
- `TestRetryPredicateRejectedFailureEmitsFailureEvent`가 해당 behavior를 고정한다.
- fix 이후 `go test -count=1 ./resilience`, `go test -race -count=1 ./resilience`,
  `go test -count=1 ./...`가 통과했다.
