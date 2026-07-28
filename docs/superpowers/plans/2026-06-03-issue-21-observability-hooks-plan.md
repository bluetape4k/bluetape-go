# Issue 21 Observability Hooks Plan

## 분류

- 작업 유형: Type A - Full Feature.
- 근거: resilience 공용 API에 observer hook, event contracts, examples, README 갱신, 테스트를 추가한다.
- 진행 라인: issue #21 전용 worktree에서 구현한다.

## 목표

`resilience` 정책 실행 흐름에서 caller가 metrics, logs, tracing에 연결할 수 있는 lightweight observer hook을 제공한다. 구현은 외부 observability SDK에 의존하지 않고, 이벤트 구조와 option wiring만 first-party로 유지한다.

## 순서

1. #18, #19에서 확정된 event skeleton과 error contracts를 다시 확인한다.
2. hook 요구사항을 spec과 plan에 기록하고, SDK 비의존 원칙을 명시한다.
3. retry, timeout, circuit breaker, bulkhead 이벤트를 공통 event envelope로 정리한다.
4. observer option과 no-op 기본값을 추가한다.
5. event emission order, error classification, context cancellation을 검증하는 테스트를 추가한다.
6. examples에서 metrics/logging adapter를 caller-owned 형태로 보여준다.
7. README와 package docs에 hook 사용법과 안정성 제약을 반영한다.
8. focused tests, race tests, repo checks, diff checks를 실행한다.

## 리뷰 게이트

다음 항목을 확인한다.

- hook이 OpenTelemetry, Prometheus, slog 같은 특정 SDK를 강제하지 않는지 확인한다.
- observer callback이 policy 동작을 바꾸지 않는지 확인한다.
- panic, blocking, allocation 위험이 문서화되었는지 확인한다.
- event fields가 retry/timeout/circuit breaker/bulkhead에 충분한지 확인한다.
- error classification이 기존 sentinel/typed error와 일치하는지 확인한다.
- examples가 caller-owned instrumentation으로 유지되는지 확인한다.

## 검증 게이트

- `go test -count=1 ./resilience`
- `go test -race -count=1 ./resilience`
- `go test -count=1 ./...`
- `go vet ./...`
- `make fmt-check`
- `git diff --check`
