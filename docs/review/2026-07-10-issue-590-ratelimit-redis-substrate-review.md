# Issue #590 Redis Rate Limiter Diagnostic Substrate Review

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

## 범위

- Baseline: `08e3c49` (`Preserve rate limiter compatibility before diagnostic migration`)
- Implementation: `ratelimit/redis/{limiter.go,operation_error_test.go,README.md,README.ko.md}`
- Design evidence: issue #590 spec, test spec, Step 2-R, and Step 3-R reviews
- Review mode: local six-perspective equivalent. Native review-lane spawning is
  not exposed in this session; the main session performed independent
  perspective reads and owns the integration verdict.

## 증거

- `make fmt-check`
- `make tidy-check`
- `go vet ./ratelimit/redis ./redis`
- `golangci-lint run --timeout 5m` (`0 issues` after clearing a stale cache
  containing removed worktree paths)
- `go test -p 1 -count=1 ./ratelimit/redis ./redis`
- `go test -p 1 -race -count=1 ./ratelimit/redis`
- `TESTCONTAINERS_REUSE_ENABLE=false TESTCONTAINERS_RYUK_DISABLED=false make ci`
- `git diff --check`
- source scan for `Eval`, `operationError`, `bucketKey`, `IdleTTL`, and shared
  `KeyBuilder`/`NewOpError` contracts

## 6개 관점 발견 사항

| Perspective | P0 | P1 | P2 | P3 | Evidence and Verdict |
|---|---:|---:|---:|---:|---|
| Performance | 0 | 0 | 0 | 0 | `Allow` retains one `Eval`, its existing script, arguments, and atomicity. The change only constructs an error after a failure; benchmark is N/A. |
| Stability | 0 | 0 | 0 | 0 | Preflight context handling is unchanged. Regression tests retain `redis.ErrClosed` and a late `context.Canceled`; serial normal/race tests and full CI pass. |
| Security | 0 | 0 | 0 | 0 | `OpError` receives the exact bucket key only to derive a redacted ID. Tests reject raw namespace, logical key, bucket key, and provider error text in formatted diagnostics. |
| Operator/Ops | 0 | 0 | 0 | 0 | Redis key layout, Lua state, Redis `TIME`, and TTL behavior are unchanged. README pair documents safe cause inspection; rollback is a code revert. |
| Developer/API | 0 | 0 | 0 | 0 | No exported API or `ratelimit.Result` change. `errors.Is` keeps causes and `errors.As` exposes stable low-cardinality labels. |
| User/Caller | 0 | 0 | 0 | 0 | Existing whitespace/delimiter caller-key behavior remains protected by exact-key tests; no unexpected limiter feature or migration is introduced. |

## 호환성 결정

- `redis.KeyBuilder` remains rejected because its structural validation would
  narrow existing namespace/key inputs and alter caller-visible bucket bytes.
- Shared TTL validation remains rejected because `ratelimit/redis` owns a
  refill-aware zero default and a full-refill lower bound.
- Shared compare-delete/extend scripts remain rejected because they cannot
  represent the token-bucket Lua result contract.

## 벤치마크 결정

No measurement was run because the admission algorithm and command count did
not change. Issue #560 owns any provider benchmark matrix and the required
result table, chart, and written analysis.

P0=0 P1=0
