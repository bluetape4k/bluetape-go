# #541 web context 및 problem details lesson

## 배경

`#540`의 첫 구현 slice로 `#541`의 framework-neutral `web` package를
`feat/web-api-541`에 추가했다. 상위 Epic의 후속 순서는 `#542` HTTP conformance,
`#543` Gin adapter, `#544` Echo adapter이며 `#545` JWKS provider는 독립 train으로
남긴다.

## 결정과 근거

- 이슈의 `RFC7807` 표현은 현재 표준인 RFC 9457이 RFC 7807을 대체한다는 점을
  반영해 `Problem`과 `application/problem+json` 계약으로 정리했다. media type은
  유지하되 문서의 표준 링크는 RFC 9457을 기준으로 삼았다.
- `ProblemError`만 caller-owned 공개 detail을 제공하고, 일반 오류·nil 입력·잘못된
  status·직렬화 실패는 응답을 시작하기 전에 거부하거나 고정된 내부 오류 문구로
  매핑한다. Extension key는 표준 member와 충돌하거나 빈 값·control character를
  가지면 거부한다.
- Request ID와 correlation ID는 형식 검증 후 읽고, 누락된 request ID는 주입된
  generator 또는 UUID v7로 만든다. Auth subject와 W3C trace 값은 request별
  `TrustedProxy` predicate가 승인할 때만 읽는다. trusted 판단, 인증·인가, response
  header 반영은 helper의 책임이 아니다.
- `WithRequestContextOnRequest`는 `req.WithContext`로 원본 request를 보존하고,
  기존 cancellation을 그대로 연결한다. 전역 상태·goroutine·background refresh는
  추가하지 않았다.

## TDD와 검증 증거

1. Problem RED에서 `web.NewProblem`, `web.Problem`, `web.ProblemFromError`가
   아직 없다는 compile failure를 확인했다.
2. Problem GREEN 후 status mapping, cancellation/deadline, JSON extension 충돌,
   cyclic serialization, request instance, content type, nil 입력을 검증했다.
3. Request context RED에서 `ExtractRequestContext`와 context API 미정의를 확인한
   뒤 trusted/untrusted header, custom name, strict `traceparent`, generator,
   cancellation round-trip 테스트를 GREEN으로 만들었다.
4. 다음 명령이 모두 통과했다.

   ```text
   go test -count=1 ./web
   go test -race -count=1 ./web
   go vet ./web
   make fmt-check
   make tidy-check
   make lint
   go test -count=1 ./...
   make race
   git diff --check
   ```

   `go test -count=1 ./...`와 `make race`는 Testcontainers 포함 전체 package를
   통과했고, `make lint`는 최종 `0 issues`를 반환했다.

## Review와 남은 경계

설계·계획의 architecture, security/API, test, performance, stability/Ops,
user/caller 여섯 관점을 통합해 `P0=0 P1=0 P2=0 P3=0 — PASS`를 확인했다. 독립
native lane 세 개가 bounded window에 결과를 반환하지 않아 종료했고,
`lane timed out; main integration fallback performed`를 review artifact에
기록했다.

첫 PR은 `develop`을 base로 하는 `feat/web-api-541`에 한정한다. 아직 merge하지
않았으며, PR 생성 후 `feat/web-api-542` → `feat/web-api-543` →
`feat/web-api-544` 순으로 base를 연결하는 것이 다음 train 작업이다.

## Writer DoD

- `SPW-01`: PASS — lesson 목적, 대상 이슈, source/RFC 근거를 명시했다.
- `SPW-02`: PASS — 결정, 검증 증거, 남은 경계와 next train을 분리했다.
- `SPW-03`: PASS — API·command·URL·issue 식별자는 보존하고 모호한 benefit claim을
  쓰지 않았다.
- `SPW-04`: PASS — spec/plan, review artifact, live issue hierarchy, test 출력과
  대조했다.
- `SPW-05`: PASS — heading, code fence, checklist, 한국어 문장을 line-readback했다.
