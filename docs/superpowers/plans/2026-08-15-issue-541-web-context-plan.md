# #541 web context 및 problem details 구현 계획

> **For agentic workers:** 이 계획은 `feat/web-api-541` worktree에서 작업한다.
> 각 단계는 checkbox로 추적하고, 단계별 RED→GREEN 결과를 남긴다.

**목표:** framework-neutral `web` package를 추가해 `net/http` 요청 컨텍스트와
RFC 9457 problem details 응답을 제공한다.

**구조:** `web/problem.go`는 표준 problem 값, 오류 매핑, JSON 응답을 담당하고,
`web/context.go`는 request/correlation ID와 trusted proxy 전용 auth/trace 값을
context에 연결한다. framework adapter와 middleware conformance는 후속 PR이
이 package를 소비하도록 남긴다.

**기술 스택:** Go 표준 `context`, `errors`, `encoding/json`, `net/http`,
`unicode/utf8`와 repository의 `id.NewUUIDV7`만 사용한다. 새 외부 dependency는
추가하지 않는다.

---

## Task 1: Problem contract RED

**Files:**

- Create: `web/problem_test.go`
- Create: `web/doc.go`

- [x] **Step 1: 공개 계약을 테스트로 고정한다.**

  `web/problem_test.go`에 다음 table-driven 테스트를 추가한다.

  - `NewProblem`이 100 미만 또는 599 초과 status를 거부한다.
  - 빈 type/title을 `about:blank`과 `http.StatusText`로 보정한다.
  - 일반 오류는 500과 `Internal Server Error`로 매핑하고 원문 detail을
    노출하지 않는다.
  - `context.Canceled`는 408, `context.DeadlineExceeded`는 504로 매핑한다.
  - `ProblemError` 구현체의 status/detail/type을 보존하고, nil error와 zero-value
    또는 잘못된 status를 writer가 응답 전에 거부한다.

  최소 public 계약은 다음과 같이 테스트에서 import한다.

  ```go
  problem, err := web.NewProblem(http.StatusBadRequest, "", "invalid input")
  if err != nil {
      t.Fatal(err)
  }
  ```

- [x] **Step 2: RED를 확인한다.**

  Run: `go test -count=1 ./web -run 'Test(NewProblem|ProblemFromError)'`

  Expected: `FAIL` with missing `web` package/API errors.

- [x] **Step 3: package 문서의 최소 골격만 추가한다.**

  `web/doc.go`에는 package 목적과 `#541`의 framework-neutral, no-auth-policy
  경계를 한국어 Go doc으로 적는다. 이 단계에서는 production problem 구현을
  추가하지 않는다.

## Task 2: Problem implementation GREEN

**Files:**

- Create: `web/problem.go`
- Modify: `web/problem_test.go`

- [x] **Step 1: status와 JSON 계약을 최소 구현한다.**

  다음 선언과 동작을 구현한다.

  ```go
  type Problem struct {
      Type       string
      Title      string
      Status     int
      Detail     string
      Instance   string
      Extensions map[string]any
  }

  type ProblemError interface {
      error
      ProblemDetails() Problem
  }

  func NewProblem(status int, title, detail string) (Problem, error)
  func ProblemFromError(err error) Problem
  func WriteProblem(w http.ResponseWriter, req *http.Request, err error) error
  ```

  `NewProblem`은 status를 검증하고 기본 type/title을 채운다. `ProblemFromError`는
  `errors.As`로 `ProblemError`를 찾고, `errors.Is`로 cancellation/deadline을
  먼저 판정한다. 알려지지 않은 오류의 detail은 고정 문구로 설정한다.

  `WriteProblem`은 표준 필드와 extension key 충돌을 검사하고
  `json.Marshal`을 먼저 수행한 뒤에만 header/status/body를 쓴다. `Content-Type`
  은 정확히 `application/problem+json`으로 설정하고, request가 있으면
  `URL.RequestURI()`를 instance에 사용한다. nil writer와 nil request의 오류를
  명확히 반환한다.

- [x] **Step 2: GREEN을 확인한다.**

  Run: `go test -count=1 ./web -run 'Test(NewProblem|ProblemFromError|WriteProblem)'`

  Expected: 모든 Task 1 테스트가 `PASS`.

- [x] **Step 3: 직렬화·보안 경계 테스트를 추가하고 유지한다.**

  extension key가 `type`, 빈 문자열, control character일 때 거부되는지,
  순환 map이 writer 상태를 변경하기 전에 오류를 반환하는지, request instance와
  response status/content type이 맞는지 검증한다. `go test -count=1 ./web`로
  전체 package를 다시 실행한다.

## Task 3: Request context RED

**Files:**

- Create: `web/context_test.go`

- [x] **Step 1: trusted-header 계약을 테스트로 고정한다.**

  `httptest.NewRequest` 기반 table-driven 테스트를 추가한다.

  - 기본 header `X-Request-ID`, `X-Correlation-ID`, `X-Auth-Subject`,
    `traceparent`, `tracestate`를 읽는다.
  - request/correlation ID가 없을 때 injected generator를 한 번 호출한다.
  - correlation ID가 없으면 request ID를 재사용한다.
  - `TrustedProxy == nil` 또는 false이면 auth subject와 trace 값을 비운다.
  - trusted proxy true일 때만 auth subject와 trace 값을 보존한다.
  - newline, control character, 256 byte 초과 header를 거부하고, 사용자 지정
    header 이름은 HTTP token 규칙을 통과해야 한다.
  - trusted `traceparent`는 W3C version/trace-id/parent-id/flags 형식을
    통과해야 하며 invalid trace context는 응답 전에 거부한다.
  - `WithRequestContextOnRequest`가 원본 request를 변경하지 않고 context
    cancellation을 보존한다.
  - `WithRequestContext(nil, value)`와 `RequestContextFromContext(nil)`의
    nil 계약을 검증한다.

  희망 API:

  ```go
  value, err := web.ExtractRequestContext(req, web.RequestContextOptions{
      TrustedProxy: func(*http.Request) bool { return true },
      GenerateID:   func() (string, error) { return "generated-1", nil },
  })
  ```

- [x] **Step 2: RED를 확인한다.**

  Run: `go test -count=1 ./web -run 'Test(ExtractRequestContext|RequestContext)'`

  Expected: `FAIL` with missing context types/functions.

## Task 4: Request context implementation GREEN

**Files:**

- Create: `web/context.go`
- Modify: `web/context_test.go`

- [x] **Step 1: context 값과 header 추출을 구현한다.**

  `RequestContext`, `RequestContextOptions`, context key, 기본 header 상수,
  `ExtractRequestContext`, `WithRequestContext`,
  `RequestContextFromContext`, `WithRequestContextOnRequest`를 추가한다.

  ID generator 기본값은 `id.NewUUIDV7`을 호출하는 closure로 둔다. 추출 값은
  trim 후 visible ASCII 단일 line이고 최대 256 byte인지 검사한다. trusted
  `traceparent`는 W3C 형식을 별도로 검증한다. 사용자 지정 header 이름은 HTTP
  token인지 확인한다. trusted proxy predicate는 request별로 한 번만 평가하며
  false일 때 auth/trace header를 보존하지 않는다. global mutable state나
  goroutine은 사용하지 않는다.

- [x] **Step 2: GREEN을 확인한다.**

  Run: `go test -count=1 ./web -run 'Test(ExtractRequestContext|RequestContext)'`

  Expected: Task 3의 모든 테스트가 `PASS`.

- [x] **Step 3: cancellation과 race 범위를 확인한다.**

  Run: `go test -race -count=1 ./web`

  Expected: race report 없이 `PASS`; context 값은 호출 간 공유 mutable state를
  만들지 않는다.

## Task 5: Examples와 locale README

**Files:**

- Create: `web/example_test.go`
- Create: `web/README.md`
- Create: `web/README.ko.md`
- Modify: `README.md`
- Modify: `README.ko.md`

- [x] **Step 1: compile-checked example을 추가한다.**

  `ExampleWriteProblem`은 `ProblemError`를 `httptest.ResponseRecorder`에 쓰고
  status/content type/body를 확인한다. `ExampleWithRequestContextOnRequest`는
  trusted proxy와 generated ID를 보여 준다.

- [x] **Step 2: README 두 언어를 같은 계약으로 작성한다.**

  package import, problem response, header trust rule, auth policy non-goal,
  cancellation/context ownership, 후속 `#542` conformance 경계를 모두 설명한다.
  Korean 기술 문장은 `korean-naturalness-checklist.md`의 KO-01~KO-06을 통과시키고,
  API 이름·commands·URLs는 그대로 보존한다.

- [x] **Step 3: root package table을 갱신한다.**

  `README.md`와 `README.ko.md`의 package inventory에 `web` 항목을 같은 위치와
  의미로 추가한다. 다이어그램은 새로 만들지 않는다. 범위가 package docs와
  목록에 한정되므로 `bluetape-diagram`은 N/A로 기록한다.

## Task 6: 검증과 train handoff

**Files:**

- Modify: `docs/superpowers/specs/2026-08-15-issue-541-web-context-design.md`
- Modify: `docs/superpowers/plans/2026-08-15-issue-541-web-context-plan.md`
- Create: `docs/lessons/2026-08-15-issue-541-web-context.md`

- [x] **Step 1: format, static, targeted 검증을 실행한다.**

  ```bash
  gofmt -w web/*.go
  git diff --check
  go test -count=1 ./web
  go test -race -count=1 ./web
  go vet ./web
  make fmt-check
  make tidy-check
  make lint
  ```

- [x] **Step 2: proportional repository 검증을 실행한다.**

  `go test -count=1 ./...`를 실행하고, Testcontainers가 포함된 전체 race는
  repository helper인 `make race`를 사용해 직렬 실행한다. 실패하면 원인을
  분리해 `#541` 범위의 오류만 수정하고 targeted 검증부터 반복한다.

- [x] **Step 3: 계획·spec·diff를 대조한다.**

  `git diff --stat`, `git diff --check`, `git status --short`, public symbol
  목록, README locale parity를 읽어 acceptance criterion 누락을 확인한다.

- [x] **Step 4: 한국어 lesson을 작성한다.**

  RFC 7807 명칭과 RFC 9457 현재 표준의 차이, trusted proxy를 helper가 소유하지
  않는 이유, test evidence와 남은 `#542` 경계를 기록한다. SPW-01~SPW-05를
  통과시킨 뒤에만 commit한다.

- [x] **Step 5: Lore 형식으로 첫 PR commit을 만든다.**

  ```bash
  git add web README.md README.ko.md docs/superpowers docs/lessons
  git commit -m "web API 오류와 요청 컨텍스트 경계를 정의한다"
  ```

  commit body에는 `Constraint`, `Rejected`, `Confidence`, `Scope-risk`,
  `Directive`, `Tested`, `Not-tested` trailer를 한국어 문장으로 채운다.

- [ ] **Step 6: stacked train handoff를 검증한다.**

  `feat/web-api-542`는 `feat/web-api-541`을 base로 만들고 #542 conformance만
  추가한다. 이후 `feat/web-api-543` → `feat/web-api-544`를 순차적으로 base
  한다. `#545`는 `develop` base의 독립 branch로 유지한다. 이 slice에서는 PR을
  생성하되 merge하지 않고, live PR body의 `## DoD Status`, assignee `debop`,
  milestone `0.21.0`, labels, base/head SHA를 `gh pr view`로 읽어 확인한다.

## 롤백·재실행

- 구현 실패 시 `feat/web-api-541` worktree의 uncommitted diff만 되돌리고
  `develop`에는 side effect가 없도록 유지한다.
- public API 변경이 필요하면 spec/plan을 먼저 수정하고 영향받는 RED 테스트와
  plan review를 다시 실행한다.
- `go test ./...`의 unrelated Testcontainers 실패는 원문 오류와 package를
  기록하고 `./web` targeted/race/vet 검증을 유지한다. 이를 PASS로 둔갑시키지
  않는다.

## 계획 self-review

- spec의 두 책임(problem/context), five failure modes, acceptance, docs, PR
  boundary가 Task 1~6에 모두 매핑된다.
- placeholder(`TBD`, `TODO`, 미정 단계)를 사용하지 않았다.
- `NewProblem`/`WriteProblem`의 error 반환과 테스트 명칭이 spec과 일치한다.
- `#542` 이후 순서는 HTTP conformance → Gin → Echo이며, JWKS `#545`는 별도
  branch라는 의존성 제약을 보존한다.
