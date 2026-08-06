# Redis Rate Limiter diagnostic substrate migration 구현 계획

> 한국어 재작성 범위: 이 계획 문서는 한국어 운영 문서로 읽히도록 제목, 판단, 작업 설명, 위험, 검증, 롤백 문맥을 한국어로 정리한다. 명령, 경로, API 이름, 이슈/PR 번호, 브랜치명, 코드 블록, 테스트 출력 같은 증거 문자열은 정확성을 위해 원문 그대로 보존한다.


> **에이전트 작업자용:** 필수 하위 스킬: 사용 superpowers:subagent-driven-development (권장) 또는 superpowers:executing-plans to 이 계획을 작업 단위로 구현. 단계는 checkbox (`- [ ]`) 추적 문법을 사용.

**목표:** Make Redis token-bucket provider failures typed 및 redacted 변경하지 않고 rate-limiting behavior 또는 bucket-key compatibility.

**아키텍처:** 유지 `ratelimit/redis` responsible for its bucket key, refill-aware TTL validation, 및 single Lua token-bucket script. 추가 one private 오류-boundary helper that joins a late context 오류 함께 the provider 원인 및 delegates redaction to the 공유 `redis.OpError` implementation.

**기술 스택:** Go, `github.com/redis/go-redis/v9`, 공유 `bluetape-go/redis`, Testcontainers Redis, standard `errors` 패키지.

---

## 파일 지도

| 파일 | 책임 |
|---|---|
| `ratelimit/redis/limiter.go` | 라우팅 만 `Eval` provider failures through the private typed/redacted 오류 helper. |
| `ratelimit/redis/operation_error_test.go` | 고정 provider 원인, late-context, typed-오류, 및 redaction contracts. |
| `ratelimit/redis/README.md` | Describe preserved 원인 inspection 및 redacted provider 진단. |
| `ratelimit/redis/README.ko.md` | 유지 한국어 operational guidance in sync. |
| `docs/lessons/2026-07-10-issue-590-ratelimit-redis-substrate.md` | 기록 why compatibility-incompatible 공유 helpers stay local. |

## 작업 0: 커밋 Approved Design Artifacts

**복잡도:** 낮음

**파일:** 커밋 the two specs, 단계 2-R review, 및 this plan 전에 source 또는 테스트 implementation begins.

- [x] **단계 1:** 실행 `git diff --check` 및 confirm the four tracked workf낮음 artifacts are the 만 staged files.
- [x] **단계 2:** 커밋 함께 Lore trailers: intent is to preserve the approved compatibility boundary; record the 공유-helper rejection, 높음 confidence, narrow scope risk, 및 the plan-review validation gap as `Not-tested`.
- [x] **단계 3:** 검증 `git status --short` is clean 전에 beginning 작업 1.

## 작업 1: RED Provider-Diagnostic Regression Tests

**복잡도:** 보통

**파일:** 생성 `ratelimit/redis/operation_error_test.go`; read `redis/errors.go` 및 `ratelimit/redis/limiter.go`.

**패턴:** Apply `$bluetape-go-patterns`: standard `errors.Is`/`errors.As`, deterministic testing, 호출자-owned client cleanup, 및 없음 parallel Testcontainers execution. Existing `GoroutineStressTester` coverage remains the concurrency guard; this 오류-만 change needs 없음 new stress helper.

- [x] **단계 1:** 추가 a closed-client 테스트 using marker namespace `ns:marker` 및 key ` key:marker `. 검증 `errors.Is(err, redis.ErrClosed)`, `errors.As(err, *btredis.OpError)`, family `rate limiter`, operation `consume`, `btredis.RedactedKeyID(limiter.bucketKey(key))`, 및 absence of raw namespace/key/provider markers in `err.Error()`.
- [x] **단계 2:** 추가 a deterministic late-context 테스트: cancel a context, call `operationError(ctx, "consume", "raw:key", redis.ErrClosed)`, 및 assert both `redis.ErrClosed` 및 `context.Canceled` are discoverable without leaking `raw:key`.
- [x] **단계 3:** 실행 `go test -count=1 ./ratelimit/redis -run 'OperationError'`. 예상 결과: RED because the helper does 아님 yet exist 및 `Eval` still returns a plain wrapped 오류.

## 작업 2: Minimal `Eval` Error-Boundary Migration

**복잡도:** 보통

**파일:** Modify `ratelimit/redis/limiter.go`; 테스트 `ratelimit/redis/operation_error_test.go`.

**패턴:** Apply `$bluetape-go-patterns`: preserve `context.Context` causes, do 아님 expose sensitive provider values, retain original 오류 through wrapping, 및 keep implementation details unexported.

- [x] **단계 1:** 가져오기 `errors` 및 `github.com/bluetape4k/bluetape-go/redis` as `btredis`. 추가 `operationError`: when `ctx.Err()` is non-nil, set `err = errors.Join(err, ctx.Err())`; return `btredis.NewOpError(btredis.OpLabels{Family: "rate limiter", Operation: operation}, rawKey, err)`.
- [x] **단계 2:** Compute `bucketKey := l.bucketKey(key)` once 전에 `Eval`; pass `[]string{bucketKey}` to the unchanged script call; replace 만 its 오류 return 함께 `operationError(ctx, "consume", bucketKey, err)`.
- [x] **단계 3:** 실행 `gofmt -w ratelimit/redis/limiter.go ratelimit/redis/operation_error_test.go` then `go test -p 1 -count=1 ./ratelimit/redis -run 'OperationError|ContextCancellation|PreservesCallerOwnedKeys'`. 예상 결과: PASS.

## 작업 3: Documentation And Focused 검증

**복잡도:** 낮음

**파일:** Modify `ratelimit/redis/README.md` 및 `ratelimit/redis/README.ko.md`.

**패턴:** Apply `$bluetape-go-patterns`: 공개 behavior documentation stays accurate 및 영문/한국어 패키지 README files remain aligned.

- [x] **단계 1:** 추가 one operational-boundary bullet in both README files: command failures retain their original 원인 for `errors.Is`, expose typed 진단 through `errors.As`, 및 redact raw Redis key/provider details. 다음을 하지 않는다: claim a benchmark result 또는 성능 gain.
- [x] **단계 2:** 실행 sequential focused checks: `make fmt-check`; `make tidy-check`; `go vet ./ratelimit/redis ./redis`; `golangci-lint run --timeout 5m`; `go test -p 1 -count=1 ./ratelimit/redis ./redis`; `go test -p 1 -race -count=1 ./ratelimit/redis`; `git diff --check`. 예상 결과: PASS.

## 작업 4: 리뷰, Lesson, And Publication Readiness

**복잡도:** 보통

**파일:** 생성 `docs/review/2026-07-10-issue-590-ratelimit-redis-substrate-review.md` 및 `docs/lessons/2026-07-10-issue-590-ratelimit-redis-substrate.md`.

- [x] **단계 1:** 실행 the local six-perspective 단계 6-R review. 기록 성능, 안정성, 보안, 운영자/Ops, 개발자/API, 및 사용자/호출자 evidence; require `P0=0 P1=0`; record KeyBuilder/TTL/script-helper rejection, benchmark N/A, 및 #560 ownership.
- [x] **단계 2:** 추가 the focused lesson that an 오류-boundary-만 migration must 아님 adopt helper validation narrowing established key, TTL, 또는 script contracts.
- [x] **단계 3:** 실행 `TESTCONTAINERS_REUSE_ENABLE=false TESTCONTAINERS_RYUK_DISABLED=false make ci`, `git diff --check`, 및 `git status --short`. 예상 결과: PASS. If stale local Testcontainers settings 원인 unrelated provider failures, retain the explicit override 및 do 아님 alter application code.

## 롤백

Revert the migration commit. No key, script, Redis state, 공개 API, 또는 configuration migration occurs, so 없음 data rollback is required.
