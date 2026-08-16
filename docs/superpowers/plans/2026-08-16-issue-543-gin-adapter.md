# Gin Adapter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]` syntax) for tracking.

**Goal:** 기존 framework-neutral web/resilience/rate-limit/JWT 기능을 Gin에서 동일한 의미로 사용할 수 있는 web/gin adapter를 추가하고, 보안·취소·재시도·문제 응답 계약을 테스트와 문서로 고정한다.

**Architecture:** Gin 의존성은 web/gin 패키지에만 격리한다. adapter는 기존 web, resilience, ratelimit, jwt core API를 호출하고, Gin request/writer/context 상태를 요청 단위로 보존·복원한다. JWT JWKS 네트워크 provider는 Issue #545 범위로 남기고 이번 작업은 기존 parser 경로만 연결한다.

**Tech Stack:** Go 1.26.3, Gin v1.12.0, 기존 bluetape-go web/resilience/ratelimit/jwt 패키지, httptest, go test -race, Benchmark.

---

## 실행 원칙

- 작업 디렉터리는 /Users/debop/work/bluetape4k/bluetape-go/.worktrees/feat-gin-adapter-543로 고정한다.
- 실행 시작 시 git worktree list --porcelain과 git rev-parse --show-toplevel을 실행해 현재 root가 /Users/debop/work/bluetape4k/bluetape-go/.worktrees/feat-gin-adapter-543인지 확인하고, 다른 경로이면 모든 mutation을 중지한다.
- 구현 전 각 단계의 실패 테스트를 먼저 추가하고, 해당 테스트를 통과시키는 최소 구현만 추가한다.
- exported Go API에는 한국어 Go doc 주석을 작성하고, package README와 README.ko.md를 동일한 범위로 갱신한다.
- web, webtest, root/core 패키지에는 Gin import를 추가하지 않는다. rg -n 'github.com/gin-gonic/gin' web webtest *.go 결과는 비어 있어야 한다.
- 모든 커밋은 Lore 형식의 intent/trailer를 포함한다.
- Merge, branch 삭제, 원격 release와 같은 외부·비가역 작업은 CI 성공과 별도의 최신 승인 전까지 실행하지 않는다.

## Task 1: 재시도 불가 오류를 resilience core에 추가

- [x] resilience/errors.go를 추가하고 기존 retry predicate보다 먼저 검사되는 ErrNonRetryable, NonRetryableError, NonRetryable, IsNonRetryable를 구현한다.

  ~~~go
  var ErrNonRetryable = errors.New("resilience operation is non-retryable")

  type NonRetryableError struct {
      Cause error
  }

  func NonRetryable(err error) error
  func IsNonRetryable(err error) bool
  ~~~

- [x] resilience/policy.go의 retry 판정이 사용자 RetryIf보다 먼저 IsNonRetryable(err)를 확인하도록 수정하고, resilience/circuit_breaker.go의 기본 failure predicate도 IsNonRetryable(err)를 context.Canceled 검사보다 먼저 확인해 NonRetryable(context.Canceled)를 실패로 기록하도록 수정한다.
- [x] resilience/errors_test.go에 nil 입력, errors.Is, errors.As, wrapping chain, custom RetryIf 우선순위, NonRetryable(context.Canceled)의 retry event와 circuit failure 기록 테스트를 작성한다.
- [x] go test ./resilience와 go test -race ./resilience를 실행한다. 예상 결과는 두 명령 모두 PASS이다.
- [x] 커밋한다.

  ~~~text
  Non-retryable errors must stop committed-response retries without hiding failure state

  Constraint: Gin response writers cannot be safely replayed after bytes are committed.
  Rejected: Suppressing committed-handler errors | it would make circuit and caller state falsely successful.
  Confidence: high
  Scope-risk: narrow
  Directive: Preserve the marker before custom RetryIf evaluation.
  Tested: go test ./resilience; go test -race ./resilience
  Not-tested: Gin adapter integration, covered in subsequent tasks.
  ~~~

## Task 2: Gin package 골격과 RFC 9457 문제 응답 bridge

- [x] go.mod에 github.com/gin-gonic/gin v1.12.0을 추가하고 go mod tidy를 실행한다. go list -m -json github.com/gin-gonic/gin가 v1.12.0과 검증된 sum을 출력해야 한다.
- [x] web/gin/doc.go에 package ginadapter와 package-level 한국어 Go doc을 추가한다.
- [x] web/gin/options.go에 DefaultJWTContextKey, RateLimitKeyFunc, RateLimitOptions, ContextParser, JWTOptions, JWTErrorKind, AuthenticationError, ResilienceOptions를 정의한다. RequestContext는 중복 타입을 만들지 않고 web.RequestContextOptions를 직접 사용한다. JWTOptions는 Parser와 ContextParser 중 정확히 하나를 요구한다.
- [x] AuthenticationError의 value receiver Error()는 고정된 authentication failed: <kind> 문자열만 반환하고 Unwrap을 제공하지 않는다. ProblemDetails() web.Problem은 고정 401만 반환하며, 기존 web.ProblemError 계약으로 RFC 9457 응답에 매핑한다.
- [x] web/gin/problem.go에 AbortWithProblem과 JWTReader를 구현한다.

  ~~~go
  func AbortWithProblem(c *gin.Context, err error) error
  func JWTReader(c *gin.Context, key string) (*jwt.Reader, bool)
  ~~~

- [x] 문제 응답 구현은 writer가 이미 작성된 경우 body를 덮어쓰지 않고 c.Abort()만 수행하며 nil context/error는 web.ErrInvalidProblem으로 반환한다. pre-write marshal 실패 시 generic 500 fallback을 한 번 시도하고 원래 오류를 반환한다.
- [x] web/gin/problem_test.go에 정상 ProblemDetails, generic error, authentication error, writer committed, nil input, marshal failure, context-value reader 테스트를 추가한다. token이나 raw parser error가 응답/로그 문자열에 포함되지 않음을 검증한다.
- [x] web/gin/import_boundary_test.go를 추가해 go list -json ./...의 모든 direct Imports를 검사하고 github.com/gin-gonic/gin이 web/gin package 이외에서 import되면 실패시킨다. var _ ginadapter.ContextParser = (*jwt.DistributedProvider)(nil) compile-time assertion으로 context-only provider 계약도 고정하고, typed-nil parser/ContextParser/limiter/policy도 constructor에서 거부한다.
- [x] go test ./web/gin, go vet ./web/gin, go mod tidy를 실행하고 git diff --exit-code go.mod go.sum으로 tidy parity를 확인한다.
- [x] 커밋한다.

## Task 3: request context middleware

- [x] web/gin/request_context.go에 RequestContext(options web.RequestContextOptions) gin.HandlerFunc를 구현한다. request-scoped context는 기존 web.WithRequestContextOnRequest를 사용하고, middleware 진입 전 request pointer를 defer로 복원한다.
- [x] TrustedProxy가 nil이면 fail-closed로 peer를 untrusted 처리한다. Gin ClientIP, X-Forwarded-For, 임의 client header 값은 trust signal로 사용하지 않는다.
- [x] 정상 반환·panic·downstream request replacement 후 원래 *http.Request 복원, cancellation/deadline 전달, trusted/untrusted peer 동작을 web/gin/request_context_test.go에 추가한다.
- [x] panic 경로는 middleware가 panic을 삼키지 않고 caller-owned Recovery가 처리하도록 검증한다.
- [x] go test ./web/gin -run RequestContext -count=1와 go test -race ./web/gin -run RequestContext -count=1을 실행한다.
- [x] 커밋한다.

## Task 4: Gin-native rate-limit bridge

- [x] web/gin/rate_limit.go에 NewRateLimit(options RateLimitOptions) (gin.HandlerFunc, error)를 구현한다. Gin-native RateLimitKeyFunc를 내부 ratelimit.KeyFunc로 변환하고 shared core handler를 한 번 생성한다. nil 및 typed-nil limiter를 동일하게 거부한다.
- [x] bridge는 요청별 *gin.Context를 context value로 전달하고 downstream c.Next()는 한 번만 호출한다. request context와 Gin context value를 defer로 복원한다.
- [x] 기본 backend/key 오류 응답은 raw error를 노출하지 않는 503 Problem으로 만들고, custom ErrorHandler가 있으면 caller-owned callback을 사용한다. reject 응답은 core가 설정한 Retry-After/X-RateLimit-Remaining 헤더와 429를 보존한다.
- [x] context.Canceled와 context.DeadlineExceeded는 generic backend 503과 구분해 기존 web.ProblemFromError 매핑으로 처리하고, bounded return·downstream 미호출·raw error 미노출을 고정한다.
- [x] web/gin/rate_limit_test.go에 allow, limit reject, backend error, key error, cancellation, custom error handler, concurrent request context isolation, downstream once-only, nil limiter/options validation을 추가한다.
- [x] go test ./web/gin -run RateLimit -count=1와 go test -race ./web/gin -run RateLimit -count=1을 실행한다.
- [x] 커밋한다.

## Task 5: strict JWT authentication middleware

- [x] web/gin/jwt.go에 NewJWT(options JWTOptions) (gin.HandlerFunc, error)를 구현한다. parser와 parse options를 생성 시 검증하고 context key/header/scheme 기본값을 적용한다.
- [x] Authorization header는 정확히 하나만 허용한다. comma-joined 또는 duplicate header, Bearer 이외 scheme, 빈 token, control character, 8 KiB 초과 token을 거부한다.
- [x] constructor에서 ParseOptions slice를 방어적으로 복사하고 nil option을 거부한다. ContextParser가 설정되면 ParseContext(context.Context, string, ...jwt.ParseOption)를 사용하고, strict cancellation 경로에서는 이를 요구한다. 기존 Parser는 ContextParser와 동시에 설정할 수 없다. typed-nil parser interface는 nil로 정규화해 정확히 하나의 실 parser만 선택하며, 둘 다 nil인 경우는 거부한다.
- [x] legacy Parse만 있는 경우 pre/post ctx.Err()를 확인하는 best-effort 경로로 명시하고, blocking parser 취소 뒤 bounded return과 goroutine 잔류 없음은 context-capable parser 경로에서 검증한다. 취소/마감은 JWTErrorCanceled로 분류한다.
- [x] 성공 시 raw token이 아니라 reader만 context key에 저장하고 JWTReader로 읽는다. parser error와 token은 callback/response에 노출하지 않는다. callback에 전달하는 request 복사본에서는 Authorization header를 제거해 callback이 raw token을 읽을 수 없도록 한다.
- [x] web/gin/jwt_test.go에 성공 reader storage, missing/malformed/multiple header, invalid/expired token, cancellation/deadline, context-aware parser, legacy parser, custom ErrorHandler, parser nil/options invalid, option-slice mutation, callback header redaction, downstream isolation 테스트를 추가한다.

  ~~~go
  middleware, err := ginadapter.NewJWT(ginadapter.JWTOptions{
      Parser: parser,
      ErrorHandler: func(_ *gin.Context, err error) {
          callbackErr = err
      },
  })
  ~~~

- [x] go test ./web/gin -run JWT -count=1와 go test -race ./web/gin -run JWT -count=1을 실행한다.
- [x] 커밋한다.

## Task 6: route-level resilience wrapper

- [x] web/gin/resilience.go에 WrapResilience(next gin.HandlerFunc, options ResilienceOptions) gin.HandlerFunc를 구현한다. nil next는 404를 반환하고 Recovery는 호출자가 조합한다.
- [x] constructor에서 Policies slice를 방어적으로 복사하고 nil 및 typed-nil policy 항목을 건너뛴다. 각 policy attempt마다 request/context/errors/keys/index/Params와 response Header snapshot을 만든다. c.Keys는 adapter-owned key의 shallow map snapshot으로 복원하고, nested caller value mutation은 caller-owned 범위로 문서화한다.
- [x] request body가 없거나 GetBody가 있으면 attempt별 request body를 재생성한다. non-empty Body에 GetBody가 없고 첫 시도에서 uncommitted error가 발생하면 두 번째 시도를 시작하지 않고 resilience.NonRetryable(error)로 중단한다. response가 이미 committed된 경우에도 resilience.NonRetryable(err)로 감싸 재시도를 중지하며 body/header를 덮어쓰지 않는다.
- [x] handler 호출 후 새 error와 policyCtx.Err()를 검사하고, uncommitted error는 policy가 재시도할 수 있도록 request/header/keys/index/Params/Errors 상태를 복원한다. 최종 error는 AbortWithProblem을 통해 generic 503 또는 context cancellation/deadline 문제로 매핑한다.
- [x] Recovery는 adapter가 소유하지 않는다. router.Use(gin.Recovery())가 wrapper보다 바깥에 있는 조합에서 panic이 policy success로 기록되지 않고 한 번의 recovery response로 종료되는 것을 고정한다.
- [x] web/gin/resilience_test.go에 성공, 재시도, committed response non-retryable, policy failure, post-operation cancellation, request body replay/non-replay, response header isolation, attempt state isolation, option-slice mutation, panic/Recovery 조합, nil next, concurrent request isolation을 추가한다.
- [x] go test ./web/gin -run Resilience -count=1와 go test -race ./web/gin -run Resilience -count=1을 실행한다.
- [x] 커밋한다.

## Task 7: framework-neutral conformance suite

- [x] webtest.Adapter를 사용해 web/gin/conformance_test.go에 request context, rate limit, JWT, Problem, resilience의 공통 동작을 등록한다. production webtest package에는 Gin import를 추가하지 않는다.
- [x] serial conformance 명령 go test -p 1 ./web/gin -run Conformance -count=1을 실행하고 기존 web adapter 의미와 동일한 status/header/context/error 결과를 확인한다.
- [x] go test ./...를 실행해 root/core package가 Gin 의존성 때문에 변경되지 않았음을 확인한다.
- [x] 커밋한다.

## Task 8: README와 운영 문서

- [x] web/gin/README.md와 web/gin/README.ko.md를 작성한다. 설치, production bootstrap 순서(Recovery → request context → auth/rate-limit → route resilience), trusted peer/readiness/observer, migration example, parity matrix, failure/rollback 관찰 지점을 포함한다.
- [x] README 운영 runbook은 다음 실행 순서와 기대 결과를 고정한다: preflight에서 git worktree/root와 go test ExampleBootstrap을 확인하고, canary에서 5분 동안 readiness 200과 정상 요청 2xx를 확인하며, observer 필드(adapter, kind, status, committed, request_id, duration)를 수집한다. raw token/parser error가 관찰되면 즉시 rollback한다. rollback 뒤 readiness 200, 정상 요청 2xx, 5xx/429/401 비율이 canary 기준으로 복귀한 것을 복구 조건으로 기록한다.
- [x] RequestContext에는 별도 startup validation API가 없으므로 readiness 문서에 “header 이름 오류는 첫 request에서 400으로 surface되고 startup preflight는 ExampleBootstrap smoke test로 보완한다”고 명시한다. trusted/untrusted header probe와 cancellation probe의 curl 명령, 기대 status/body/header를 한·영 README에 동일하게 기록한다.
- [x] observer wiring은 rate-limit/JWT/resilience callback이 응답 작성 전후에 호출되는 시점, c.IsAborted(), Writer.Written(), AuthenticationError.Kind, status와 request ID를 기록하는 규칙을 공통 표로 제공한다. callback caller-owned logging에서 Authorization header와 raw error를 기록하지 않는 예시를 포함한다.
- [x] web/README.md, web/README.ko.md, root README.md, root README.ko.md의 Gin 확장 예정 문구를 현재 adapter 링크와 범위 설명으로 갱신한다.
- [x] web/gin/example_test.go에 import와 concrete fixture를 갖춘 ExampleBootstrap 및 ExampleMigration을 추가한다. jwt.NewFixedHMACProvider, ratelimit.New, gin.New, gin.Recovery, web.RequestContextOptions, NewRateLimit, NewJWT, WrapResilience를 실제로 구성하고 constructor error를 확인해 Example 실행이 성공해야 한다. README는 이 example source를 링크하고 미정의 변수나 오류 무시 코드를 사용하지 않는다.

  ~~~go
  provider, err := jwt.NewFixedHMACProvider(jwt.HS256, []byte("0123456789abcdef0123456789abcdef"))
  if err != nil { panic(err) }
  limiter, err := ratelimit.New(ratelimit.Options{RatePerSecond: 10, Burst: 10})
  if err != nil { panic(err) }
  router := gin.New()
  router.Use(gin.Recovery())
  router.Use(ginadapter.RequestContext(web.RequestContextOptions{}))
  limit, err := ginadapter.NewRateLimit(ginadapter.RateLimitOptions{Limiter: limiter})
  if err != nil { panic(err) }
  auth, err := ginadapter.NewJWT(ginadapter.JWTOptions{Parser: provider})
  if err != nil { panic(err) }
  router.Use(limit, auth)
  router.GET("/orders", ginadapter.WrapResilience(func(c *gin.Context) { c.Status(http.StatusNoContent) }, ginadapter.ResilienceOptions{}))
  ~~~

- [x] web/gin/conformance_test.go는 webtest.Adapter 기반 framework-neutral 시나리오와 별도로 Gin-specific 시나리오를 둔다. Gin-specific assertion은 c.IsAborted(), c.Writer.Written(), c.Errors, outer gin.Recovery 순서와 downstream once-only를 직접 확인한다.
- [x] go test ./web/gin -run Example -count=1와 git diff --check를 실행한다.
- [x] 커밋한다.

## Task 9: benchmark와 evidence ledger

- [x] web/gin/benchmark_test.go에 deterministic local fixture benchmark를 추가한다. ReportAllocs를 사용하고 no-op, direct-core, bridge, full adapter를 같은 request/writer/context 계약으로 serial과 b.RunParallel로 측정하며 -cpu 1,2,4를 지원한다. parallel fixture는 start gate, worker join, per-iteration request/recorder 격리, 완료 수가 b.N과 일치하는 검사를 갖는다.
- [x] 모든 benchmark의 fixture construction, seed, parser/provider 생성, warm-up, cleanup은 b.StopTimer 구간에 둔다. timer 안에는 지정된 request operation과 required completion만 둔다. serial/direct-core/bridge/full 행의 semantic boundary를 report에 함께 적는다.
- [x] cold-start는 middleware construction과 첫 request를 각각 BenchmarkGinAdapterColdConstruction, BenchmarkGinAdapterColdFirstRequest로 분리하고, warm-request는 동일 fixture에서 10회 warm-up 후 request만 측정한다. JWT는 parser-only fixture로 제한하고 JWKS network/provider benchmark는 Issue #545로 남긴다.
- [x] report 산식을 고정한다: bridge overhead = (bridge ns/op - direct-core ns/op) / direct-core ns/op, full overhead = (full ns/op - no-op ns/op) / no-op ns/op. clean capture는 `capture_eligibility=eligible`로 provenance만 표시하고, baseline SHA와 fixture identity 비교가 없으면 `no_regression=N/A`로 기록하며 숫자를 추정하지 않는다.
- [x] Makefile에 BENCH_COUNT와 BENCH_CPU 변수를 받는 bench-web-gin target을 추가하고 serial 및 CPU matrix를 canonical command로 실행한다.

  ~~~make
  BENCH_COUNT ?= 5
  BENCH_CPU ?= 1,2,4
  bench-web-gin:
      @scripts/capture-gin-adapter-benchmark.sh "$(BENCH_COUNT)" "$(BENCH_CPU)"
  ~~~

- [x] scripts/parse-gin-adapter-benchmark.py를 추가해 raw Go benchmark output을 benchmark name, cpu, ns/op, B/op, allocs/op row의 JSON으로 변환하고 missing, unknown, duplicate, non-finite, failed row, metadata `benchmark_count`별 sample 누락/초과를 거부한다. scripts/capture-gin-adapter-benchmark.sh는 set -euo pipefail, private mktemp directory, signal cleanup, go test -timeout=10m 명령별 raw output, 최대 10 MiB output limit, Go/OS/CPU/Git SHA/dirty-tree/Gin version/fixture identity environment file을 기록하고 이 parser와 chart generator를 호출한다. clean-tree capture는 `capture_eligibility=eligible`로 provenance를 표시하되 baseline 비교가 없는 `no_regression`은 N/A로 기록하고, dirty-tree capture는 두 값을 모두 N/A로 기록한다. 모든 명령과 chart generation이 성공한 경우에만 temp 파일을 canonical 경로로 staging하고 publication 중간 실패 시 기존 canonical 묶음을 rollback하며, non-zero·중단·redaction·publication 실패는 timestamped `-failed-` artifact로 남긴다.
- [x] 다음 canonical capture와 재현 명령을 실행한다.

  ~~~bash
  make bench-web-gin
  BENCH_COUNT=5 BENCH_CPU=1,2,4 make bench-web-gin
  ~~~

- [x] docs/images/readme-charts/gin-adapter-benchmark-summary.vl.json와 docs/images/readme-charts/generate-gin-adapter-benchmark-summary.mjs를 추가한다. generator는 raw result rows의 missing, unknown, duplicate, non-finite, failed row를 거부하고 SVG/PNG를 생성한다. self-test와 chart regeneration 명령을 기록한다.

  ~~~bash
  node docs/images/readme-charts/generate-gin-adapter-benchmark-summary.mjs --self-test
  node docs/images/readme-charts/generate-gin-adapter-benchmark-summary.mjs docs/research/outputs/issue-543/bench-results.json
  ~~~

- [x] docs/research/2026-08-16-issue-543-gin-adapter-benchmark.md에 raw command/output, Go/OS/CPU/logical CPUs/date, Git SHA/dirty-tree, Gin/module version, fixture identity, 결과 표, metric direction, overhead 산식, chart source/SVG/PNG, use-case별 해석, caveat, no-regression 또는 N/A 결론을 한국어로 기록한다.
- [x] benchmark가 기능 테스트를 대체하지 않음을 문서에 명시하고, raw result·environment·chart source가 모두 같은 capture SHA를 가리키는지 확인한다.
- [x] make bench-web-gin, node docs/images/readme-charts/generate-gin-adapter-benchmark-summary.mjs --self-test, git diff --check를 실행한다.
- [x] 커밋한다.

## Task 10: 통합 검증, 7-tier code review, lesson, PR 준비

- [x] benchmark를 실행하지 않는 script-contract 검증을 먼저 실행한다. parser missing/unknown/duplicate/non-finite/failed, redaction, output-limit, timeout, dirty-tree, publication-failure fixture를 포함한 scripts/capture-gin-adapter-benchmark_test.sh를 추가하고 make check-bench-web-gin target으로 연결한다. Makefile ci target도 check-bench-web-gin을 포함해 benchmark 자체를 실행하지 않고 capture 계약만 검증한다.

  ~~~bash
  make check-bench-web-gin
  ~~~

- [x] 전체 검증을 순서대로 실행한다.

  ~~~bash
  make fmt-check
  make tidy-check
  make vet
  make lint
  make test
  make race
  make ci
  ~~~

  각 명령은 exit code 0이어야 하며, 실패 시 해당 원인을 수정한 뒤 실패 명령부터 재실행한다.
- [x] Step 6-R code review를 performance, stability, security, operator/Ops, developer/API, user/caller 여섯 lane으로 수행하고 docs/superpowers/reviews/2026-08-16-issue-543-step-6r-code-review.md에 P0/P1/P2와 evidence를 기록한다. main integration verdict에는 0 known P0/P1을 요구한다.
- [x] docs/superpowers/lessons/2026-08-16-issue-543-gin-adapter.md에 재사용 가능한 Go/Gin adapter 교훈, 실패 원인, 다음 수정자가 피해야 할 선택을 한국어로 기록한다.
- [x] gh issue view 543 --json title,body,assignees,milestone,state,parent로 Issue metadata를 재확인하고, PR body에 Closes #543, 설계/계획/리뷰/benchmark/lesson 링크, 테스트 결과, final ## DoD Status를 포함한다.
- [x] PR #687에 대해 `gh pr checks 687 --watch --interval 10`과 `gh pr view 687 --json headRefOid,baseRefName,statusCheckRollup,reviews,reviewDecision,mergeStateStatus,body`를 실행했다. local HEAD `b2ecafc`와 `headRefOid`가 일치하고 CI run `31917992608`이 성공했으며 reviews/comments가 없고 `mergeStateStatus=CLEAN`인 것을 live read-back했다. CI가 성공해도 merge는 별도의 최신 사용자 승인 전까지 보류한다.
- [x] merge 승인 전 stop condition은 “코드·문서·benchmark·review·lesson 완료, 모든 로컬 검증 성공, PR CI/review live evidence 확보, merge approval 미확보”이다.

## 완료 판정

- [x] Issue #543 범위의 Gin adapter와 테스트/문서/benchmark가 존재한다.
- [x] Gin import boundary, strict JWT, redacted errors, trusted peer fail-closed, committed-response retry stop, request cancellation, concurrent isolation이 fresh test output으로 입증된다.
- [x] make ci와 make race가 성공하고 Step 6-R에서 P0/P1이 없다.
- [x] 변경 파일·커밋·PR·CI evidence를 사용자에게 한국어 DoD로 보고한다.
- [x] merge/local sync/cleanup은 별도 최신 승인 전까지 PENDING으로 남긴다.
