# Issue #694 Echo JWT parser cancellation 경계 설계

## 문서 상태와 근거

- 상태: 로컬 구현·검증 완료, exact-head CI 대기
- 이슈: [#694](https://github.com/bluetape4k/bluetape-go/issues/694)
- 상위 Epic: [#540](https://github.com/bluetape4k/bluetape-go/issues/540)
- 기준 브랜치: `develop`
- 독자: `web/echo` JWT middleware를 구성하는 Go 개발자와 provider 구현자
- 현재 근거: `JWTOptions.Parser` 값이 `ContextParser`도 구현하더라도 기존
  `parseJWT`는 `Parse`만 호출하므로 request cancellation을 전달하지 못했다.

## 목표와 비목표

목표는 기존 `Parser` 설정을 깨지 않으면서 가능한 provider에는 request context를
전달하고, legacy-only provider의 cancellation latency와 goroutine 수명 의미를
명시적으로 고정하는 것이다. pre-canceled request, 호출 중 취소, 정상 완료의
downstream·오류 계약을 회귀 테스트로 보호한다.

provider 내부 작업을 adapter가 강제로 중단하거나, blocking `Parse`를 임의
goroutine으로 감싸서 요청 수명 밖으로 분리하는 방식은 비목표다. root `jwt`
package와 Gin adapter의 public API·dispatch 방식도 이번 변경 범위에 포함하지
않는다.

## 선택지와 결정

| 선택지 | 장점 | 위험 | 결정 |
| --- | --- | --- | --- |
| legacy `Parser`를 goroutine으로 실행하고 cancellation 시 즉시 반환 | middleware latency를 줄인다. | provider가 끝나지 않으면 goroutine과 작업이 request 수명 밖에 남고 결과 회수도 불가능하다. | 기각 |
| `Parser`를 즉시 deprecation하고 `ContextParser`만 허용 | cancellation 계약이 단순하다. | 기존 설정과 local synchronous parser를 불필요하게 깨뜨린다. | 기각 |
| `Parser` 값의 `ContextParser` 구현 여부를 감지하고, legacy-only 값은 동기 호출 | 기존 설정을 유지하면서 context-aware provider를 자동 승격하고 수명 소유권을 보존한다. | legacy-only blocking call의 cancellation latency는 provider 반환 시간에 묶인다. | **채택** |

## dispatch와 cancellation 계약

`parseJWT`는 다음 순서로 parser를 선택한다.

1. `JWTOptions.ContextParser`를 직접 설정했다면 해당 `ParseContext`를 호출한다.
2. `JWTOptions.Parser` 값이 `ContextParser`도 구현하면 request context와 함께
   `ParseContext`를 호출한다.
3. 그 밖의 legacy-only 값은 `Parse`를 현재 goroutine에서 동기 호출한다.

| 경로 | cancellation | goroutine 수명 | 오류 의미 |
| --- | --- | --- | --- |
| explicit `ContextParser` | provider가 `ctx.Done()`을 관찰하고 협력적으로 반환한다. | adapter가 추가 goroutine을 만들지 않으며 호출 반환까지 기다린다. | cancellation은 `JWTErrorCanceled`인 redacted 401이다. |
| auto-upgraded `Parser` | explicit 경로와 동일하게 request context가 전달된다. | explicit 경로와 동일하다. | explicit 경로와 동일하다. |
| legacy-only `Parser` | 호출 전 취소는 `Parse`를 건너뛴다. 호출 중 취소는 `Parse` 반환 뒤 확인한다. | blocking 호출을 분리하지 않고 middleware 호출과 함께 join한다. | 호출 중 취소도 downstream을 호출하지 않고 redacted 401을 반환한다. |

adapter는 provider 작업을 강제 종료하지 않는다. 따라서 I/O 또는 장시간 blocking
가능성이 있는 구현은 `ContextParser`를 구현하고 `ctx.Done()`이 닫히면 자체
resource를 정리한 뒤 반환해야 한다. legacy-only `Parser`는 local CPU parsing처럼
짧고 동기적인 구현에 한해 적합하다.

## 이행과 호환성

- 기존 provider가 `Parse`와 `ParseContext`를 함께 구현하면 설정을 바꾸지 않아도
  Echo adapter가 context-aware 경로로 자동 승격한다.
- 새 Echo 설정은 cancellation 요구를 코드에서 드러내기 위해
  `JWTOptions.ContextParser`를 직접 사용한다.
- local synchronous 구현을 위해 `Parser`는 `0.21.0`에서 유지하며 deprecation하지
  않는다. blocking 구현은 위 이행 경로를 사용해야 한다.
- `AuthenticationError`에는 token과 parser 원인을 넣지 않으며, cancellation도
  기존처럼 `JWTErrorCanceled`와 공개 401 Problem으로 redaction한다.
- Gin adapter는 수정하지 않는다. Echo의 성공·실패 결과는 Gin의 기존 redacted
  인증 결과와 호환되지만, Echo 전용 자동 승격을 Gin에 암묵적으로 확장하지 않는다.

## 회귀 테스트와 검증

- `Parser` 값이 `ContextParser`도 구현할 때 `ParseContext`만 호출하고 request
  context 값을 전달하며, 호출 중 취소를 관찰한 뒤 반환한 late-success reader를
  publish하지 않는지 확인한다.
- pre-canceled request는 legacy provider와 downstream을 모두 호출하지 않는지
  확인한다.
- legacy blocking provider를 호출 중 취소해도 middleware가 먼저 반환하지 않고,
  provider release 뒤 active call이 0이 되며 downstream을 호출하지 않는지 확인한다.
- late cancellation의 custom `ErrorHandler`가 원인을 노출하지 않는
  `AuthenticationError{Kind: JWTErrorCanceled}`만 받는지 확인한다.
- `go test`와 race detector를 `web/echo`에 실행하고, `web/gin` 회귀 테스트로
  기존 호환 계약을 확인한다.

## 완료 조건

- 코드, 회귀 테스트, `README.md`, `README.ko.md`가 같은 dispatch·수명 계약을
  설명한다.
- targeted/full/race/vet/lint/CI가 통과하고 최종 리뷰 P0/P1이 0개다.
- Go pattern guidance에 context-aware migration과 synchronous legacy 수명 규칙이
  반영되어 이후 adapter 구현에서도 재사용된다.

## SPW 기록

- **SPW-01:** PASS — live 이슈, 기준 브랜치, 독자, 현재 구현 근거와 비목표를 고정했다.
- **SPW-02:** PASS — 대안, dispatch 우선순위, cancellation·수명·오류 계약과 DoD를 기록했다.
- **SPW-03:** PASS — 한국어 기술 문체를 사용하고 Go identifier와 명령은 원문을 유지했다.
- **SPW-04:** PASS — blocking, pre-cancel, late-cancel, redaction, Gin 호환 수용 기준을 테스트·문서 항목에 연결했다.
- **SPW-05:** PASS — 저장 후 heading, 표, 링크, 버전, 경로와 code token을 다시 읽었다.

exact-head GitHub CI와 live PR review 증적은 PR 생성 뒤 별도 gate에서 확인하며,
그 전에는 완료로 간주하지 않는다.
