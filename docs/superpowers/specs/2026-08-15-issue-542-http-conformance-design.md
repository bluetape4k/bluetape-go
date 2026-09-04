# Issue #542 HTTP middleware 적합성 스위트 설계

## 상태와 독자

- 상태: 사용자 설계 승인 완료
- 이슈: [#542](https://github.com/bluetape4k/bluetape-go/issues/542)
- 상위 조사: [#508](https://github.com/bluetape4k/bluetape-go/issues/508)
- Epic: [#540](https://github.com/bluetape4k/bluetape-go/issues/540)
- 기준 문서: [HTTP middleware conformance research gate](https://github.com/bluetape4k/bluetape4k-wiki/blob/develop/research/2026-07-09-bluetape-go-ecosystem-parity-research-gate.md)
- 대상 브랜치: `feat/web-api-542`
- 독자: `net/http` middleware를 구현하거나 이후 Gin/Echo adapter를 추가하는 Go 개발자
- 언어: 한국어 기술 문서. 코드 토큰, 명령, URL, API 이름은 원문을 유지한다.

## 문제와 현재 증거

이슈 #542는 서로 다른 HTTP middleware가 같은 호출 경계에서 상태 코드,
취소, 리소스 수명, 요청 식별자, 오류, 관찰 이벤트를 일관되게 처리하는지
검증할 수 있는 재사용 가능한 스위트를 요구한다. 이 스위트가 없으면
framework adapter마다 같은 계약을 다시 작성하고, `net/http`에서 확인한
동작과 adapter 동작이 어긋나도 늦게 발견한다.

현재 `develop`에는 다음 경계가 있다.

| 경계 | 현재 API와 근거 | 스위트에서 확인할 관찰값 |
| --- | --- | --- |
| 오류 응답 | `web.WriteProblem`, `web.ProblemError` (`web/problem.go`) | status, `application/problem+json`, body, cancellation/deadline 매핑 |
| 요청 컨텍스트 | `web.WithRequestContextOnRequest`, `web.RequestContextFromContext` (`web/context.go`) | request/correlation ID, trusted proxy 필드, 원본 request 보존 |
| resilience handler | `resilience.NewHandler` (`resilience/http.go`) | policy 오류의 status/body, timeout, caller-owned error handler |
| rate-limit handler | `ratelimit.NewHandler` (`ratelimit/http.go`) | key, token 수, 거부 status, `Retry-After`, proxy header 비신뢰 |
| resilience hook | `Policy`의 `OnEvent` (`resilience/retry.go`, `bulkhead.go`, `circuit_breaker.go`) | 동기 이벤트 전달, case 간 이벤트 누출 없음 |
| response body 수명 | `resilience.NewRoundTripper` (`resilience/http.go`) | retryable response body를 재시도 전에 닫음 |
| 기존 harness | `ratelimit/ratelimittest` | table-driven case, bounded timeout, cancellation, race 패턴 |

기준선은 새 worktree의 `go test -count=1 ./...` 전체 통과다. 현재 구현에
없는 Gin/Echo/Fiber middleware나 recovery 정책을 이 이슈에서 새로 만들지는
않는다.

## 범위

### 포함

1. 루트 `webtest` 패키지에 framework-neutral conformance harness와 HTTP
   fixture를 추가한다.
2. harness는 `http.Handler` adapter를 같은 입력과 관찰 계약으로 실행한다.
   각 case는 독립된 request, recorder, observer, timeout을 사용한다.
3. `net/http` 기준 case를 `web`, `resilience`, `ratelimit`의 현재 API와
   연결해 다음 동작을 고정한다.
   - status/error body 매핑
   - 사전 취소와 in-flight cancellation의 bounded completion
   - 명시된 소유 경계의 request/response body close
   - request ID와 correlation context 전달 및 원본 request 불변
   - resilience policy panic/error의 복구·전파 경계
   - rate-limit key, token, trusted proxy header 기본값
   - caller-owned hook/event 관찰과 global state 비누출
4. 향후 adapter가 같은 case를 연결할 수 있도록 `Adapter`, `Scenario`,
   `Observation` 및 close-tracking fixture를 작고 명시적인 API로 둔다.
5. Go README와 `README.ko.md`의 package inventory 및 `webtest` 사용 범위를
   동기화한다.

### 제외

- Gin, Echo, Fiber adapter 구현과 해당 framework dependency 추가
- 새 production middleware, 인증/인가, JWT/JWKS 정책 구현
- 전역 logger/default transport를 변경하는 테스트 설계
- benchmark 실행과 성능 수치 주장. benchmark fixture 재사용 지점만 문서화하며
  측정 산출물은 #560에서 다룬다.
- Testcontainers 의존 테스트. 현재 HTTP harness는 in-memory
  `httptest`와 fake transport만 사용한다.

## 선택안과 결정

| 선택안 | 장점 | 비용과 위험 | 결정 |
| --- | --- | --- | --- |
| 패키지 공용 `webtest` harness | #543/#544 adapter가 같은 case와 observer를 재사용하고, net/http 계약을 한 곳에서 읽을 수 있다. | 작은 API를 설계하고 각 case의 소유 경계를 명확히 해야 한다. | **채택** |
| 패키지별 테스트만 추가 | 구현이 빠르고 공개 surface가 없다. | adapter마다 fixture와 timeout이 복제되어 계약 drift가 생긴다. | 기각 |
| 내부 fixture table만 공유 | 공개 API가 가장 작다. | adapter가 response/context/event 관찰을 연결하기 어렵고 case 실패 설명이 약하다. | 기각 |

공용 harness는 테스트 지원 패키지로만 사용한다. production package가
`webtest`를 import하지 않으며, 새 외부 dependency도 추가하지 않는다.

## 설계 계약

### Adapter와 scenario

- `Adapter`는 `http.Handler`를 받아 감싼 handler를 반환하는 함수형 경계다.
  구현 시 `type Adapter func(http.Handler) http.Handler`로 고정한다.
  framework adapter는 이 경계로 변환한 뒤 같은 scenario를 실행한다.
- `Scenario`는 `Name string`, `Adapter Adapter`,
  `NewRequest func(context.Context) *http.Request`, `Next http.Handler`,
  `Timeout time.Duration`, `Assert func(*testing.T, Observation)` 필드를
  가진다. `Name`, `Adapter`, `NewRequest`, `Next`, `Assert`가 nil/빈 값이면
  harness contract 오류로 즉시 실패한다.
- `Run(t *testing.T, scenarios ...Scenario)`는 scenario마다 새
  `httptest.ResponseRecorder`, request, observer를 만들고, timeout 안에
  handler가 반환하지 않으면 request context를 취소한 뒤 bounded drain을
  시도하고 실패를 보고한다. result channel은 buffered로 만들어 timeout 뒤
  늦게 반환한 goroutine이 runner를 막지 않게 한다. middleware가 취소를
  무시하면 이를 성공으로 바꾸지 않고 `cancellation/timeout` 실패로 남긴다.
  한 scenario의 mutable state를 다음 scenario와 공유하지 않는다.
- 입력 누락(adapter, request, next, assertion)을 조용히 보정하지 않는다.
  harness 자체의 contract 오류는 테스트를 즉시 실패시킨다.

### Observation과 fixture

`Observation`은 다음 읽기 전용 필드로 고정한다.

```go
type Observation struct {
    StatusCode  int
    Header      http.Header
    Body        []byte
    NextCalls   int
    NextRequest *http.Request
}
```

`Header`와 `Body`는 runner가 복사해 assertion이 recorder 내부를 변경하지
못하게 한다. `NextRequest`가 nil이면 middleware가 next에 도달하지 않은
것이다. next에 도달한 request의 context는 `NextRequest.Context()`에서 읽는다.
assertion은 관찰값을 통해 검증하고, handler 내부의 private field나 global
state를 직접 조작하지 않는다.

`CloseTracker`는 `io.ReadCloser`의 close 호출과 close 횟수를 기록한다.
소유자가 명확한 response body(예: retryable `RoundTripper` 응답)는 close를
요구한다. 일반 server handler의 incoming request body처럼 `net/http` server가
소유하는 수명은 middleware contract로 잘못 강제하지 않는다.

`RoundTripper`는 `Handler` adapter와 호출 형태가 다르므로 `Run`에 억지로
합치지 않는다. `resilience`의 package-local transport case가
`webtest.CloseTracker`를 사용해 retry 전 response body close를 확인하고,
`webtest`는 그 소유권 fixture만 제공한다. 이후 transport conformance가
독립적으로 커지면 별도의 `TransportScenario`를 추가한다.

`EventRecorder`는 caller-owned resilience event를 case-local하게 저장한다.
`OnEvent` callback을 대체하거나 global logger를 변경하지 않으며, 각 case의
초기 길이와 종료 길이를 비교해 이전 case 이벤트가 섞이지 않았음을 확인한다.

### 계약별 기준

- 오류 응답은 실제 `web.WriteProblem` 결과를 비교한다. 상태 코드만 보고 body
  형식을 통과시키지 않는다.
- cancellation은 `context.Canceled`와 `context.DeadlineExceeded`를 구분하고,
  goroutine 종료를 bounded timeout으로 기다린다. timeout을 늘려 flaky test를
  숨기지 않는다. `Scenario.Timeout <= 0`이면 harness 기본 timeout
  `2 * time.Second`를 사용하며, timeout 뒤 cleanup 대기는 같은 상한을 넘지
  않는다.
- request context는 `WithRequestContextOnRequest`가 복사본을 반환하고 원본
  request의 context와 header를 바꾸지 않는지 확인한다. trusted proxy predicate가
  false이면 auth/trace 값을 읽지 않는다.
- rate-limit key는 `RemoteAddr`를 기본 원천으로 확인하고 `X-Forwarded-For`
  같은 proxy header를 자동 신뢰하지 않는다. custom `KeyFunc`와 token 수는
  observer가 받은 실제 인자를 비교한다.
- panic/recovery는 resilience policy가 panic을 오류/이벤트 계약으로 처리하는
  현재 경계만 검증한다. 일반 `http.Handler` panic을 임의로 복구하는 새
  middleware는 추가하지 않는다.
- hook/log는 `OnEvent` 호출의 동기성·context 전달·case isolation만 고정한다.
  log output 형식이나 전역 logger 설정은 contract에 포함하지 않는다.

## 실패 모드와 완화

| 실패 모드 | 원인 | 완화 |
| --- | --- | --- |
| case가 반환하지 않음 | middleware가 취소를 전달하지 않거나 goroutine이 누수됨 | case timeout, 취소 후 bounded completion, race 실행 |
| body close 누락 | retry 경로가 response body 소유권을 놓침 | `CloseTracker`와 retry 전 close assertion |
| proxy 신뢰 상승 | 기본 key가 forwarding header를 사용함 | `RemoteAddr`와 spoofed header를 함께 넣는 고정 case |
| 이벤트 누출 | package/global callback 또는 공유 recorder 사용 | case별 recorder 생성과 종료 후 isolation 확인 |
| framework 의미 과장 | net/http만 지원하면서 adapter 동작을 가정함 | `Adapter` seam만 제공하고 #543/#544에서 실제 adapter를 검증 |
| timing flaky | `Sleep`과 무제한 channel 대기 | context timeout, buffered result channel, 단일 owner cleanup |

## 호환성과 운영

`webtest`는 새 production import path를 요구하지 않으며 기존 public API를
변경하지 않는다. README에는 테스트 전용 성격과 향후 adapter 연결 경계를
명시한다. 테스트는 기본적으로 `go test -count=1`, `go test -race`에서
실행되며, Docker나 외부 서비스 없이 반복 가능해야 한다.

## 승인 기준과 DoD

- [ ] `webtest` harness가 `http.Handler` adapter와 scenario/observation
  contract를 문서화한다.
- [ ] `web`, `resilience`, `ratelimit`의 위 계약 case가 green이다.
- [ ] cancellation, body close, event isolation case가 bounded timeout 안에
  완료되고 `go test -race`에서 통과한다.
- [ ] package README와 루트 English/Korean README가 같은 범위와 링크를
  설명한다.
- [ ] `go test -count=1 ./...`, `make fmt-check`, `make tidy-check`,
  `make vet`, `make lint`, `make race`, `make ci` 결과를 PR에 남긴다.
- [ ] Issue #542 링크, milestone, assignee, labels와 PR metadata가 일치한다.
- [ ] PR 생성 후 CI green과 최신 review/thread를 다시 읽고, 별도 merge
  승인을 받기 전에는 merge하지 않는다.

## SPW-01~05 기록

- **SPW-01 — PASS:** 독자·목적·언어·Issue/parent/research URL·현재 API 파일과
  미지원 영역을 위에 고정했다.
- **SPW-02 — PASS:** 문제, 범위, 비목표, 선택안, 계약, 실패 모드, 호환성,
  acceptance/DoD를 포함했다.
- **SPW-03 — PASS:** `korean-naturalness-checklist.md`를 읽고 사실을 먼저
  고정한 뒤 concrete verb, 일관된 `harness`/`adapter`/`scenario` 용어,
  불확실성 경계를 적용했다.
- **SPW-04 — PASS:** `web/problem.go`, `web/context.go`,
  `resilience/http.go`, `ratelimit/http.go`, `resilience/* OnEvent`,
  `ratelimit/ratelimittest`와 live Issue #542/#508/wiki gate를 대조했다.
- **SPW-05 — PASS:** 저장 직후 제목, 링크, 명령, API 이름, scope/비목표,
  acceptance, locale/register를 다시 읽고 의미 변경이 없음을 확인한다.
