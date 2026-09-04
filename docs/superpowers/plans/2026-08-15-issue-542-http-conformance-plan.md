# Issue #542 HTTP middleware 적합성 구현 계획

## 계획 메타데이터

- 이슈: [#542](https://github.com/bluetape4k/bluetape-go/issues/542)
- 상위 Epic: [#540](https://github.com/bluetape4k/bluetape-go/issues/540)
- 기준 설계: `docs/superpowers/specs/2026-08-15-issue-542-http-conformance-design.md`
- 대상 worktree: `/Users/debop/work/bluetape4k/bluetape-go/.worktrees/feat-web-api-542`
- 브랜치: `feat/web-api-542`
- 실행 방식: TDD, Go 표준 라이브러리 우선, 새 dependency 없음
- 중지 조건: P0/P1 발견, race/timeout/resource cleanup 실패, 또는 기준 계약을
  바꾸는 범위 확장이 생기면 해당 단계에서 멈추고 설계/계획을 갱신한다.

## 작업 순서와 DoD

### 1. 기준선과 변경 범위 고정

Action:

1. worktree가 `develop`의 승인된 기준선인지 확인한다.
2. `git status --short`, `git diff --check`, `go test -count=1 ./...`를 실행한다.
3. `.bluetape` 실행 증거의 `mutation-check`를 대상 worktree에 대해 재확인한다.

Expected DoD:

- dirty source 변경이 없고 기준선 전체 테스트가 통과한다.
- 새 변경은 `webtest`, README parity, 설계/계획/검토/lesson 문서로 한정된다.
- Testcontainers는 새 case에서 사용하지 않으며 기존 저장소 검증은 순차 실행한다.

Rerun/rollback:

- 기준선 실패면 코드 작성 없이 해당 package와 오류를 기록하고 원인을 분리한다.
- worktree가 다른 base이면 변경을 만들지 않고 `develop` 기준 worktree를 복구한다.

### 2. 실패 테스트부터 추가

Action:

1. `webtest/harness_test.go`에 다음 실패/정상 case를 먼저 작성한다.
   - `Adapter`/`Scenario` 입력 검증
   - status/header/body와 next 호출 관찰
   - 사전 취소 및 in-flight 취소의 bounded completion
   - timeout 후 cancel과 늦은 반환을 성공으로 인정하지 않는 경로
   - scenario별 observer/recorder 격리
   - `CloseTracker` read/close/count 계약
2. `webtest/example_test.go`에 `ExampleRun` 또는 동등한 compile-checked
   public API 예제를 추가한다.
3. `go test -count=1 ./webtest`를 실행해 새 테스트가 구현 부재로 실패하는
   것을 확인한다.

Expected DoD:

- 테스트가 설계의 exact API(`Adapter`, `Scenario`, `Observation`, `Run`,
  `CloseTracker`)를 컴파일하며, 구현 전에는 예상한 red 결과를 남긴다.
- 모든 timeout 대기는 context와 bounded timer를 사용하고 unbuffered 결과
  channel을 사용하지 않는다.

Rerun/rollback:

- 테스트가 구현 없이 통과하면 assertion이 계약을 증명하지 못하는 것이므로
  먼저 테스트를 수정한다.
- flaky red가 보이면 sleep을 늘리지 말고 signal channel과 context ownership을
  다시 고정한다.

### 3. `webtest` harness 구현

Action:

1. `webtest/doc.go`에 test support package의 범위, production import 금지,
   `net/http` 우선 경계를 Go doc으로 작성한다.
2. `webtest/harness.go`에 표준 라이브러리만 사용해 다음을 구현한다.
   - `type Adapter func(http.Handler) http.Handler`
   - `type Scenario`와 nil/빈 입력 검증
   - `type Observation`과 header/body defensive copy
   - `func Run(t *testing.T, scenarios ...Scenario)` 및 기본 2초 timeout
   - request context cancel, buffered completion channel, bounded cleanup
   - next 호출/요청 캡처를 동기화한 observer
   - `CloseTracker`와 deterministic close count
3. handler가 취소를 무시하면 timeout 실패를 명확히 보고하고, 임의의
   goroutine을 강제로 종료하거나 global state를 변경하지 않는다.
4. `gofmt`와 `go test -count=1 ./webtest`로 구현을 고정한다.

Expected DoD:

- 모든 harness 테스트가 green이고, exported symbol에 Korean Go doc이 있다.
- `%w`가 필요한 error wrapping은 causal chain을 보존하고, nil input은 조용히
  default handler로 바꾸지 않는다.
- runner가 다음 scenario의 state를 재사용하지 않으며, `Header`/`Body`를
  복사한다.

Rerun/rollback:

- timeout/race 실패 시 handler 실행과 observation snapshot의 owner를 먼저
  확인하고, shared mutable state를 제거한다.
- API를 넓혀야 하면 구현 전에 설계와 Step 3-R 검토를 갱신한다.

### 4. 현재 `net/http` 계약을 harness에 연결

Action:

1. `webtest/nethttp_conformance_test.go`에 `web`, `resilience`, `ratelimit`
   현재 API를 import하는 table-driven case를 추가한다.
2. `web` case는 `WriteProblem` status/media type/body와
   `WithRequestContextOnRequest`의 request/correlation ID, trusted proxy
   필드, 원본 request 보존을 확인한다.
3. `resilience` case는 `NewHandler`의 policy error/timeout status mapping,
   cancellation 전달, `NewRoundTripper` retryable response body close를
   확인한다. transport 경계는 `webtest.CloseTracker`만 공유한다.
4. `ratelimit` case는 custom key와 `RemoteAddr` 기본 key, spoofed
   `X-Forwarded-For` 비신뢰, token 수, 429/`Retry-After`, backend error mapping을
   확인한다.
5. resilience policy의 caller-owned `OnEvent` callback은 case-local recorder로
   동기 호출과 이전 case 이벤트 누출이 없음을 확인한다. global logger/default
   transport를 변경하지 않는다.
6. panic은 현재 resilience policy의 panic/finalization 경계만 검증하고,
   새 recovery middleware를 만들지 않는다.

Expected DoD:

- 각 이슈 계약이 이름 있는 subtest와 구체적인 status/body/context/key/event
  assertion으로 존재한다.
- request cancellation, body close, hook isolation이 정상·실패 경로 모두에서
  증명된다.

Rerun/rollback:

- 현재 API가 설계 계약과 다르면 production API를 넓히지 말고 case 범위를
  실제 behavior로 좁히거나 설계 변경을 먼저 기록한다.
- framework adapter 테스트를 추가하고 싶어지면 #543/#544로 남긴다.

### 5. 문서와 locale parity

Action:

1. `webtest/README.md`와 `webtest/README.ko.md`에 import, 최소 scenario 예제,
   timeout/cancellation, body ownership, test-only/non-goal을 같은 순서로
   작성한다.
2. `README.md`와 `README.ko.md` package table에 `webtest`를 같은 위치와
   의미로 추가한다.
3. 문서의 API 예제가 `go test -run Example -count=1 ./webtest`로 컴파일되는지
   확인하고, Korean technical naturalness checklist로 제목·표·코드 주변
   문장을 다시 읽는다.

Expected DoD:

- English/Korean README의 package scope, 링크, example, non-goal이 일치한다.
- production middleware가 아니라 reusable test support라는 경계가 명확하다.

Rerun/rollback:

- parity drift가 있으면 두 locale을 함께 수정한다.
- public package inventory 변경이 release surface에 영향을 주면 PR 전에
  issue scope를 재검토한다.

### 6. Go 검증과 7-tier review

Action:

1. 최소 검증을 순서대로 실행한다.
   - `git diff --check`
   - `gofmt -w` 대상 Go 파일 후 `make fmt-check`
   - `make tidy-check`, `make vet`, `make lint`
   - `go test -count=1 ./webtest`
   - `go test -race -count=1 ./webtest`
   - `go test -run Example -count=1 ./webtest`
   - `go test -count=1 ./web ./resilience ./ratelimit`
   - `go test -race -count=1 ./web ./resilience ./ratelimit`
   - `go test -count=1 ./...`
   - `make race`, `make ci`
2. Testcontainers/real service가 실행되는 기존 broad command는 한 번에 하나만
   실행하고 connection readiness를 확인한다. 새 conformance는 in-memory라서
   별도 container가 필요 없다.
3. Step 6-R에서 performance, stability, security, operator/Ops, developer/API,
   user/caller 관점을 독립적으로 읽고 main-session에서 통합한다.
4. Go verdict에 `file:line`, P0/P1/P2/P3, exact command/result, 남은 gap을
   기록한다. P0/P1은 다음 단계로 넘기지 않는다.

Expected DoD:

- 모든 configured Go check와 targeted/race/example test가 fresh output으로
  기록된다.
- P0=0, P1=0이며 P2/P3는 수정·후속 이슈·N/A 중 하나로 명시된다.

Rerun/rollback:

- unrelated broad failure면 package/error를 고정하고 영향을 받지 않는
  targeted command를 완료하되 PASS를 과장하지 않는다.
- review가 API/ownership을 바꾸면 Step 3 계획과 테스트를 갱신한 뒤 다시
  review한다.

### 7. lesson, Lore commit, PR 준비

Action:

1. `docs/lessons/2026-08-15-issue-542-http-conformance.md`에 선택한
   `webtest` 경계, timeout/ownership 결정, 실제 검증, 예상 밖의 점, 다음
   adapter에 필요한 guard를 기록한다.
2. spec, plan, review, lesson, source, README를 Lore protocol commit으로
   나눈다. 모든 pushed commit은 한국어 intent line과 `Constraint`, `Rejected`,
   `Confidence`, `Scope-risk`, `Directive`, `Tested`, `Not-tested` trailer를
   갖는다.
3. PR template을 읽고 Korean PR body의 마지막 `## DoD Status`에 issue link,
   변경 파일, 테스트/CI 결과, P0/P1 verdict, known gap을 넣는다.
4. `feat/web-api-542`를 `develop`에 push하고 PR을 만들며 #542의 assignee,
   milestone `0.21.0`, labels를 live read-back으로 확인한다.

Expected DoD:

- lesson이 filler가 아니고 실제 선택·검증·guard를 담는다.
- PR body와 live metadata가 issue와 일치하고, URL/SHA/Closes token이 보존된다.
- merge는 하지 않는다. CI green 이후 최신 review/thread를 다시 읽고, 별도
  명시적 merge 승인을 기다린다.

## 설계-계획 추적표

| 설계 계약 | 계획 단계 | 증거 |
| --- | --- | --- |
| `Adapter`/`Scenario`/`Observation` | 2~3 | red/green harness test, Go doc, example |
| bounded cancellation/timeout | 2~4, 6 | cancellation/timeout test, race output |
| body ownership/close | 2, 4 | `CloseTracker`, retryable response test |
| request context/proxy trust | 4 | ID/trusted proxy/key assertions |
| hook/event isolation/global state | 4 | case-local recorder, no global mutation review |
| README locale parity | 5 | paired README diff and example test |
| P0/P1/CI/PR gate | 6~7 | review artifact, make output, live PR read-back |

## SPW-01~05 기록

- **SPW-01 — PASS:** plan 독자, 목적, Issue/설계 경로, worktree/branch, 명령,
  범위와 중지 조건을 고정했다.
- **SPW-02 — PASS:** 단계별 Action, Expected DoD, 의존 순서, 정확한 파일,
  테스트, rerun/rollback, approval/merge boundary를 포함했다.
- **SPW-03 — PASS:** Korean naturalness checklist를 적용해 구체 동사와
  일관된 `harness`/`scenario`/`소유권` 용어를 사용하고 code token은 보존한다.
- **SPW-04 — PASS:** 승인된 설계의 각 계약을 추적표와 실제 소스/API·Go
  validation command에 연결했다.
- **SPW-05 — PASS:** 저장 후 모든 단계, 파일 경로, 명령, locale parity, PR
  boundary와 expected evidence를 다시 읽는다.
