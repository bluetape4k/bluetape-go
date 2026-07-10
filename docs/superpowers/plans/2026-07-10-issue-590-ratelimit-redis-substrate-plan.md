# Redis Rate Limiter Diagnostic Substrate Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Redis token-bucket provider failures typed and redacted without changing rate-limiting behavior or bucket-key compatibility.

**Architecture:** Keep `ratelimit/redis` responsible for its bucket key, refill-aware TTL validation, and single Lua token-bucket script. Add one private error-boundary helper that joins a late context error with the provider cause and delegates redaction to the shared `redis.OpError` implementation.

**Tech Stack:** Go, `github.com/redis/go-redis/v9`, shared `bluetape-go/redis`, Testcontainers Redis, standard `errors` package.

---

## File Map

| File | Responsibility |
|---|---|
| `ratelimit/redis/limiter.go` | Route only `Eval` provider failures through the private typed/redacted error helper. |
| `ratelimit/redis/operation_error_test.go` | Lock provider cause, late-context, typed-error, and redaction contracts. |
| `ratelimit/redis/README.md` | Describe preserved cause inspection and redacted provider diagnostics. |
| `ratelimit/redis/README.ko.md` | Keep Korean operational guidance in sync. |
| `docs/lessons/2026-07-10-issue-590-ratelimit-redis-substrate.md` | Record why compatibility-incompatible shared helpers stay local. |

## Task 0: Commit Approved Design Artifacts

**Complexity:** low

**Files:** Commit the two specs, Step 2-R review, and this plan before source or test implementation begins.

- [x] **Step 1:** Run `git diff --check` and confirm the four tracked workflow artifacts are the only staged files.
- [x] **Step 2:** Commit with Lore trailers: intent is to preserve the approved compatibility boundary; record the shared-helper rejection, high confidence, narrow scope risk, and the plan-review validation gap as `Not-tested`.
- [x] **Step 3:** Verify `git status --short` is clean before beginning Task 1.

## Task 1: RED Provider-Diagnostic Regression Tests

**Complexity:** medium

**Files:** Create `ratelimit/redis/operation_error_test.go`; read `redis/errors.go` and `ratelimit/redis/limiter.go`.

**Patterns:** Apply `$bluetape-go-patterns`: standard `errors.Is`/`errors.As`, deterministic testing, caller-owned client cleanup, and no parallel Testcontainers execution. Existing `GoroutineStressTester` coverage remains the concurrency guard; this error-only change needs no new stress helper.

- [x] **Step 1:** Add a closed-client test using marker namespace `ns:marker` and key ` key:marker `. Assert `errors.Is(err, redis.ErrClosed)`, `errors.As(err, *btredis.OpError)`, family `rate limiter`, operation `consume`, `btredis.RedactedKeyID(limiter.bucketKey(key))`, and absence of raw namespace/key/provider markers in `err.Error()`.
- [x] **Step 2:** Add a deterministic late-context test: cancel a context, call `operationError(ctx, "consume", "raw:key", redis.ErrClosed)`, and assert both `redis.ErrClosed` and `context.Canceled` are discoverable without leaking `raw:key`.
- [x] **Step 3:** Run `go test -count=1 ./ratelimit/redis -run 'OperationError'`. Expected result: RED because the helper does not yet exist and `Eval` still returns a plain wrapped error.

## Task 2: Minimal `Eval` Error-Boundary Migration

**Complexity:** medium

**Files:** Modify `ratelimit/redis/limiter.go`; test `ratelimit/redis/operation_error_test.go`.

**Patterns:** Apply `$bluetape-go-patterns`: preserve `context.Context` causes, do not expose sensitive provider values, retain original errors through wrapping, and keep implementation details unexported.

- [x] **Step 1:** Import `errors` and `github.com/bluetape4k/bluetape-go/redis` as `btredis`. Add `operationError`: when `ctx.Err()` is non-nil, set `err = errors.Join(err, ctx.Err())`; return `btredis.NewOpError(btredis.OpLabels{Family: "rate limiter", Operation: operation}, rawKey, err)`.
- [x] **Step 2:** Compute `bucketKey := l.bucketKey(key)` once before `Eval`; pass `[]string{bucketKey}` to the unchanged script call; replace only its error return with `operationError(ctx, "consume", bucketKey, err)`.
- [x] **Step 3:** Run `gofmt -w ratelimit/redis/limiter.go ratelimit/redis/operation_error_test.go` then `go test -p 1 -count=1 ./ratelimit/redis -run 'OperationError|ContextCancellation|PreservesCallerOwnedKeys'`. Expected result: PASS.

## Task 3: Documentation And Focused Verification

**Complexity:** low

**Files:** Modify `ratelimit/redis/README.md` and `ratelimit/redis/README.ko.md`.

**Patterns:** Apply `$bluetape-go-patterns`: public behavior documentation stays accurate and English/Korean package README files remain aligned.

- [x] **Step 1:** Add one operational-boundary bullet in both README files: command failures retain their original cause for `errors.Is`, expose typed diagnostics through `errors.As`, and redact raw Redis key/provider details. Do not claim a benchmark result or performance gain.
- [x] **Step 2:** Run sequential focused checks: `make fmt-check`; `make tidy-check`; `go vet ./ratelimit/redis ./redis`; `golangci-lint run --timeout 5m`; `go test -p 1 -count=1 ./ratelimit/redis ./redis`; `go test -p 1 -race -count=1 ./ratelimit/redis`; `git diff --check`. Expected result: PASS.

## Task 4: Review, Lesson, And Publication Readiness

**Complexity:** medium

**Files:** Create `docs/review/2026-07-10-issue-590-ratelimit-redis-substrate-review.md` and `docs/lessons/2026-07-10-issue-590-ratelimit-redis-substrate.md`.

- [x] **Step 1:** Run the local six-perspective Step 6-R review. Record performance, stability, security, operator/Ops, developer/API, and user/caller evidence; require `P0=0 P1=0`; record KeyBuilder/TTL/script-helper rejection, benchmark N/A, and #560 ownership.
- [x] **Step 2:** Add the focused lesson that an error-boundary-only migration must not adopt helper validation narrowing established key, TTL, or script contracts.
- [x] **Step 3:** Run `TESTCONTAINERS_REUSE_ENABLE=false TESTCONTAINERS_RYUK_DISABLED=false make ci`, `git diff --check`, and `git status --short`. Expected result: PASS. If stale local Testcontainers settings cause unrelated provider failures, retain the explicit override and do not alter application code.

## Rollback

Revert the migration commit. No key, script, Redis state, public API, or configuration migration occurs, so no data rollback is required.
