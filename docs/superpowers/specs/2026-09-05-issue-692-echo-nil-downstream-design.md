# Issue #692 Echo nil downstream handler 계약 설계

## 문서 상태와 근거

- 상태: 구현 승인 후 적용 중
- 이슈: [#692](https://github.com/bluetape4k/bluetape-go/issues/692)
- 상위 Epic: [#540](https://github.com/bluetape4k/bluetape-go/issues/540)
- 기준 브랜치: `develop`
- 독자: `web/echo` middleware를 조합하는 Go 개발자
- 현재 근거: `request_context.go`와 `jwt.go`는 `next == nil`일 때 `nil`을
  반환하고, `rate_limit.go`와 `resilience.go`는 404를 기록한다.

## 문제와 범위

Echo middleware 네 개가 같은 nil downstream 입력을 서로 다르게 처리한다.
호출자는 chain이 종료되는지, 404 응답이 확정되는지 middleware마다 다시
확인해야 한다. 이번 변경은 기존 `web/echo` 모듈 안에서 이 계약을 단일화하고
회귀 테스트와 양국 README에 고정한다.

포함 범위는 `RequestContext`, `NewJWT`, `NewRateLimit`, `WrapResilience`의
request-time nil downstream 처리, Echo 전용 테스트, 설계 문서와 README다.
새 framework abstraction, 공통 `web` API, Gin 동작, non-nil handler 경로는
변경하지 않는다.

## 선택지와 결정

| 선택지 | 장점 | 위험 | 결정 |
| --- | --- | --- | --- |
| nil을 `nil`로 반환 | 응답을 caller에게 맡길 수 있다. | Echo의 기본 200이 남아 chain 종료와 성공을 구분하지 못한다. | 기각 |
| middleware별 차이를 문서화 | 구현 변경이 없다. | 같은 adapter의 조합 규칙이 분산되고 conformance가 약하다. | 기각 |
| 네 middleware를 404로 fail-closed | 이미 404를 사용하는 두 경로와 일치하고, 누락된 downstream을 성공 응답으로 오인하지 않는다. | 명시적으로 nil을 허용하던 호출자는 404를 관찰한다. | **채택** |

## 계약

1. `c`가 nil이면 기존처럼 `nil`을 반환한다.
2. `next == nil`이면 request, parser, limiter, resilience policy를 만지기
   전에 `c.NoContent(http.StatusNotFound)`를 호출하고 반환한다.
3. 이 경로에서는 downstream handler와 caller-owned parser/limiter/policy를
   호출하지 않는다.
4. non-nil downstream의 status, request 복원, cancellation, error handler,
   response commit 동작은 기존 계약을 유지한다.

404는 Echo chain의 명시적인 terminal response다. outer error handler가 이를
다시 덮어쓰지 않도록 middleware는 별도 오류를 반환하지 않는다.

## 검증 계획과 DoD

- `web/echo/nil_downstream_test.go`의 table-driven 테스트가 네 middleware의
  status 404와 committed response를 확인한다.
- 같은 테스트가 nil downstream에서 limiter와 resilience policy가 호출되지
  않는지 확인하고, JWT 경로가 인증 검사를 시작하기 전에 종료되는지 고정한다.
- 기존 `go test -count=1 ./web/echo`, `go test -race -count=1 ./web/echo`,
  `go vet ./web/echo`, `make lint`, `make ci`를 순서대로 실행한다.
- `README.md`와 `README.ko.md`가 동일한 404·non-nil 회귀 계약을 설명한다.
- 최종 리뷰에서 P0/P1은 0개여야 한다.

## SPW 기록

- **SPW-01:** PASS — 이슈, 기준 브랜치, 독자, 현재 소스 근거와 비목표를 고정했다.
- **SPW-02:** PASS — 선택지, 계약, 실패 의미, 호환성, 검증과 DoD를 기록했다.
- **SPW-03:** PASS — 한국어 기술 문체와 `nil downstream`, `fail-closed`,
  `committed` 용어를 일관되게 사용했다.
- **SPW-04:** PASS — 네 구현 파일과 테스트/README 수용 기준을 라이브 이슈와
  대조했다.
- **SPW-05:** PASS — 저장 후 Markdown heading, 표, 링크, 명령과 code token을
  다시 읽었다.
