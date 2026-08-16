# Issue #544 Echo adapter 구현 계획

> **For agentic workers:** 이 계획은 Issue #544의 승인된 Echo adapter 범위를
> task-by-task로 실행한다. 각 단계는 RED/GREEN 증거와 scoped commit을 남긴다.

**목표:** `web/echo`에서 기존 web/rate-limit/JWT/resilience 계약을 Echo native
middleware와 route handler로 연결하고, 문서·CI·PR DoD를 완성한다.

**구조:** Echo 의존성은 `web/echo`에만 격리한다. `problem.go`와
`request_context.go`는 HTTP response/request 경계를 맡고, `rate_limit.go`와
`jwt.go`는 middleware bridge, `resilience.go`는 route-level retry state를 맡는다.
공통 conformance는 `webtest`를 재사용하고 Echo-specific 상태는 별도 테스트에서
증명한다.

**기술 스택:** Go 1.26.3, Echo `v4.15.4`, net/http, existing bluetape-go
`web`, `ratelimit`, `jwt`, `resilience`, Go test/race/vet.

---

## Task 1: worktree와 dependency baseline

**Files:**

- Modify: `go.mod`, `go.sum`
- Create: `web/echo` package files in later tasks
- Test: `web/echo` targeted commands

- [ ] **Step 1: Echo module dependency를 direct require로 고정한다.**
  `go get github.com/labstack/echo/v4@v4.15.4` 후 package import가 생기면
  `go mod tidy`로 direct/transitive 구분을 정리한다.
- [ ] **Step 2: baseline을 기록한다.**
  `go test -count=1 ./web ./web/gin ./resilience ./ratelimit ./jwt`가 현재
  기준선에서 PASS인지 저장하고, 실패하면 기존 실패와 새 실패를 분리한다.
- [ ] **Step 3: dependency boundary RED 검사를 먼저 작성한다.**
  `web/echo/import_boundary_test.go`에서 core package가 Echo를 import하지 않고
  Echo가 Gin을 import하지 않는 compile-time/source boundary를 고정한다.
- [ ] **Step 4: dependency diff를 검토하고 commit한다.**
  `git diff --check`, `go mod tidy`, `go list -m all`을 실행하고
  `feat: Echo adapter dependency boundary를 고정한다` 의도 line과 Lore trailer를
  포함한 commit을 만든다.

## Task 2: Problem과 request context

**Files:**

- Create: `web/echo/doc.go`
- Create: `web/echo/problem.go`
- Create: `web/echo/request_context.go`
- Test: `web/echo/problem_test.go`, `web/echo/request_context_test.go`

- [ ] **Step 1: `AbortWithProblem` RED 테스트를 작성한다.**
  nil context/error, RFC 9457 content type/status/instance, unknown error redaction,
  committed response no-overwrite를 `httptest.NewRecorder`와 Echo context로 고정한다.
- [ ] **Step 2: 최소 Problem 구현을 작성한다.**
  `AbortWithProblem(c echo.Context, err error) error`는 `c.Response().Committed`를
  먼저 확인하고 `web.WriteProblem(c.Response().Writer, c.Request(), err)`를
  호출한다. 실패 시 safe 500 Problem만 시도한다.
- [ ] **Step 3: request context RED/GREEN을 작성한다.**
  `RequestContext(options) echo.MiddlewareFunc`가 request pointer를 복원하고
  trusted restricted header만 전달하며 invalid option은 400 Problem이 되는지
  정상 반환·panic 양쪽에서 검증한다.
- [ ] **Step 4: format와 targeted test를 실행한다.**
  `gofmt -w web/echo/*.go`, `go test -count=1 ./web/echo`.

## Task 3: rate-limit middleware

**Files:**

- Create: `web/echo/rate_limit.go`
- Test: `web/echo/rate_limit_test.go`
- Modify: `web/echo/conformance_test.go`

- [ ] **Step 1: allowed/rejected/backend/canceled RED 케이스를 작성한다.**
  `ratelimit.NewHandler` bridge가 Echo `HandlerFunc`를 한 번만 호출하고, rejected
  응답의 `Retry-After`/`X-RateLimit-Remaining`과 redacted 503을 유지하는지
  검증한다.
- [ ] **Step 2: Echo context carrier를 구현한다.**
  request context에 adapter-owned carrier를 넣고 HTTP downstream handler에서
  `next(c)`의 error를 보존한다. `NewRateLimit` constructor는 nil limiter,
  negative tokens, typed-nil을 거부한다.
- [ ] **Step 3: error path와 cleanup을 구현한다.**
  Echo response가 commit되면 오류 handler를 재호출하지 않고, cancellation은
  `web.WriteProblem` 매핑을 사용하며, custom callback에는 caller-owned redaction
  책임을 문서화한다.
- [ ] **Step 4: package tests/race를 실행한다.**
  `go test -count=1 ./web/echo`, `go test -race -count=1 ./web/echo`.

## Task 4: JWT middleware

**Files:**

- Create: `web/echo/jwt.go`
- Test: `web/echo/jwt_test.go`
- Modify: `web/echo/conformance_test.go`

- [ ] **Step 1: strict header RED 케이스를 작성한다.**
  missing, duplicate/comma, whitespace/control, over-8KiB, wrong scheme,
  expired/canceled parser, typed-nil parser를 표로 검증한다.
- [ ] **Step 2: `NewJWT`와 `JWTReader`를 구현한다.**
  기존 Gin adapter의 parser validation과 token grammar를 Go-native Echo context에
  적용하고, 성공 시 verified `*jwt.Reader`만 `c.Set`한다.
- [ ] **Step 3: callback redaction을 구현한다.**
  실패 callback에는 Authorization/header가 제거된 request clone과
  `AuthenticationError`만 전달하고, callback이 없으면 safe 401 Problem을 기록한다.
- [ ] **Step 4: examples와 race를 검증한다.**
  `go test -run '^Example$|^Example_migration$' -count=1 ./web/echo`와
  `go test -race -count=1 ./web/echo`를 실행한다.

## Task 5: resilience route wrapper

**Files:**

- Create: `web/echo/resilience.go`
- Test: `web/echo/resilience_test.go`
- Modify: `web/echo/conformance_test.go`

- [ ] **Step 1: success/error/retry boundary RED 케이스를 작성한다.**
  successful route once-only, policy error safe 503, committed response,
  non-replayable body, cancellation/deadline, path/param restoration을 고정한다.
- [ ] **Step 2: Echo `HandlerFunc` error contract를 연결한다.**
  `WrapResilience(next echo.HandlerFunc, options ResilienceOptions) echo.HandlerFunc`가
  policy context를 사용하고 next error를 policy에 전달한다. nil next는 404,
  typed-nil policy는 무시한다.
- [ ] **Step 3: attempt state와 redaction을 구현한다.**
  request/context/body/path/params와 adapter-owned response 상태만 복원한다.
  Echo store enumeration 불가 제약은 retry를 fail-closed하는 조건과 함께
  README에 명시한다. raw route/provider 오류는 observer error로 wrapping한다.
- [ ] **Step 4: targeted/race/vet를 실행한다.**
  `go test -count=1 ./web/echo`, `go test -race -count=1 ./web/echo`,
  `go vet ./web/echo`.

## Task 6: conformance, examples, README parity

**Files:**

- Create: `web/echo/conformance_test.go`, `web/echo/example_test.go`
- Create: `web/echo/README.md`, `web/echo/README.ko.md`
- Modify: `web/README.md`, `web/README.ko.md`, root `README.md`, root `README.ko.md`

- [ ] **Step 1: 공통 conformance를 연결한다.**
  `webtest.Run`으로 problem/context/rate-limit/JWT/resilience 시나리오를 실행하고
  Echo-specific `Committed`, abort, handler-once, `JWTReader`, callback redaction,
  outer error handler를 별도 subtest로 검증한다.
- [ ] **Step 2: compile-checked examples를 추가한다.**
  `Example`은 recovery, request context, rate limit, JWT, route resilience
  조합을 보여 주고 `Example_migration`은 `echo.WrapHandler`와 adapter path를
  compile-check한다.
- [ ] **Step 3: 양국 README를 작성한다.**
  install/import, composition order, error/redaction, retry limitation, runbook,
  migration, verification 명령을 English/Korean source-equivalent로 유지한다.
- [ ] **Step 4: 문서와 source parity를 검증한다.**
  `git diff --check`, 링크/코드 fence 확인, locale diff 검토, examples test를
  다시 실행한다.

## Task 7: final review와 PR delivery

**Files:**

- Modify: `docs/superpowers/lessons/2026-08-16-issue-544-echo-adapter.md`
- Modify: `CHANGELOG.md` only when repository release convention requires it

- [ ] **Step 1: final validation을 순차 실행한다.**
  `go test -count=1 ./web/echo`, `go test -race -count=1 ./web/echo`,
  `go vet ./web/echo`, `make fmt-check`, `make tidy-check`, `make lint`,
  `make ci`, `git diff --check`를 실행한다.
- [ ] **Step 2: 7-Tier review를 통합한다.**
  performance, stability, security, operator/Ops, developer/API, user/caller
  관점에서 P0/P1을 확인하고 main session에서 중복·문서·release evidence를
  통합한다. P0=0/P1=0만 PASS다.
- [ ] **Step 3: reusable lesson을 기록한다.**
  Echo context store의 retry limitation, net/http bridge carrier, callback
  redaction 규칙과 검증 명령을 한국어 lesson에 남긴다.
- [ ] **Step 4: Lore commit을 만든다.**
  모든 commit에 intent, Constraint, Rejected, Confidence, Scope-risk, Directive,
  Tested, Not-tested trailer를 포함한다.
- [ ] **Step 5: stacked PR을 생성하고 live-readback한다.**
  base `develop`, head `feat/echo-adapter-544`, Issue #544 closing token,
  assignee `debop`, issue label/milestone parity, `## DoD Status`를 확인한다.
- [ ] **Step 6: CI/review 후 merge approval을 기다린다.**
  exact head의 required checks와 unresolved thread를 재조회하고 merge-ready
  report를 낸 뒤 fresh approval 이후에만 merge/local sync/cleanup을 수행한다.
