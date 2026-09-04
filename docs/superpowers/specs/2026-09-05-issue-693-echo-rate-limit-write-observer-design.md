# Issue #693 Echo rate-limit Problem write failure 관측 경계 설계

## 문서 상태와 근거

- 상태: 구현 적용 완료
- 이슈: [#693](https://github.com/bluetape4k/bluetape-go/issues/693)
- 상위 Epic: [#540](https://github.com/bluetape4k/bluetape-go/issues/540)
- 기준 브랜치: `develop`
- 독자: `web/echo` rate-limit middleware를 운영·관측하는 Go 개발자
- 현재 근거: `handleRateLimitError`가 rejection, backend error,
  cancellation 경로의 `web.WriteProblem` 반환 오류를 버리고 있었다.

## 문제와 범위

기본 rate-limit Problem 응답의 body write가 실패해도 adapter가 오류를 조용히
버리면, 이미 commit된 불완전한 응답과 관측 계층의 공백이 동시에 남는다. 이번
변경은 Echo adapter 내부에만 실패 관측 경계를 추가하고, 공통
`web.WriteProblem` API와 custom callback 소유권은 변경하지 않는다.

포함 범위는 rejection, backend error, cancellation의 기본 Problem write
실패를 redacted observer로 저장하는 계약, Echo helper, failing writer 회귀
테스트, 설계 문서와 양국 README다. retry, 두 번째 응답 쓰기, global logger,
Gin adapter, 공통 `web` API 전면 변경은 비목표다.

## 선택지와 결정

| 선택지 | 장점 | 위험 | 결정 |
| --- | --- | --- | --- |
| write 오류를 계속 무시 | 호출 경로가 단순하다. | 불완전한 응답과 관측 공백을 숨긴다. | 기각 |
| write 오류를 Echo outer error handler로 반환 | 일반적인 오류 흐름을 재사용한다. | 이미 commit된 응답을 덮어쓰거나 두 번째 응답을 시도할 수 있다. | 기각 |
| redacted observer를 context에 저장하고 응답은 유지 | commit 경계를 지키면서 caller가 명시적으로 관측할 수 있다. | caller가 helper를 읽어야 한다. | **채택** |

## 계약

1. 기본 rejection/backend/cancellation 경로는 `web.WriteProblem`의 반환
   오류를 `DefaultRateLimitWriteErrorContextKey`에 저장한다.
2. observer의 공개 문자열은
   `rate limit problem response write failed`로 고정하고, `Unwrap`만 원인과
   `errors.Is` 관계를 유지한다. 원인 문자열은 응답이나 observer 문자열에
   포함하지 않는다.
3. write 실패 뒤에는 retry, 두 번째 Problem 쓰기, Echo outer error handler
   덮어쓰기를 수행하지 않고 middleware는 `nil`을 반환한다. 이미 commit된
   status와 header는 그대로 둔다.
4. 정상 writer의 Problem status/body/header와 downstream 호출 규칙은
   유지한다.
5. custom `ErrorHandler`는 기존처럼 terminal·caller-owned다. callback이
   자체 응답을 쓰거나 오류를 관측하는 정책은 caller가 결정하며, adapter는
   callback 경로에 기본 observer를 덧붙이지 않는다.

## 검증 계획과 DoD

- failing `http.ResponseWriter`가 rejection, backend, cancellation 각각에서
  redacted observer와 `errors.Is` 원인 연결을 검증한다.
- 정상 writer의 429와 custom callback의 status가 유지되고 observer가 비어
  있는지 확인한다.
- `RateLimitWriteError(nil)`의 nil-safe 동작을 확인한다.
- `go test -count=1 ./web/echo`, race, vet, lint, `make ci`와 code review를
  실행하고, CI는 exact head에서 확인한다.
- `README.md`와 `README.ko.md`가 동일한 관측·commit 경계를 설명한다.
- 최종 리뷰에서 P0/P1은 0개여야 한다.

## SPW 기록

- **SPW-01:** PASS — 이슈, 기준 브랜치, 독자, 현재 소스 근거와 비목표를 고정했다.
- **SPW-02:** PASS — 세 선택지, commit 경계, redaction, custom 소유권과 DoD를 기록했다.
- **SPW-03:** PASS — 한국어 기술 문체와 `observer`, `redacted`, `commit` 용어를 일관되게 사용했다.
- **SPW-04:** PASS — 세 오류 경로와 정상·custom 회귀 기준을 라이브 이슈와 대조했다.
- **SPW-05:** PASS — 저장 후 heading, 표, 링크, 명령과 code token을 다시 읽었다.
