# Issue #411 Redis HyperLogLog Review

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

## 범위

- Issue: #411 `Implement first Redis probabilistic follow-up structure`
- Branch: `issue-411-redis-hll`
- Work type: Type B fast track
- Diff base: `origin/develop`

## 검토한 증거

- New Redis HLL API in `probabilistic/redis/hyperloglog.go`.
- HLL integration, cancellation, error wrapping, raw-value redaction, merge, and
  concurrency tests in `probabilistic/redis/hyperloglog_test.go` and
  `probabilistic/redis/concurrency_test.go`.
- Redis key and error redaction changes in `keys.go` and `errors.go`.
- Public examples and README pairs: root, `probabilistic`, and
  `probabilistic/redis`.
- #410 research decision selecting HLL before Cuckoo.
- code-review-graph diff scan for the 14-file actual changed set.

## 발견 사항

P0/P1 발견 사항 없음.

| Lane | Verdict | Notes |
|---|---|---|
| Performance | PASS | HLL adds one `PFADD`/`PFCOUNT`/`PFMERGE` operation per call and avoids Lua/script hot-path expansion. Values are SHA-256 hex digests, which is bounded and deterministic. P0=0 P1=0. |
| Stability | PASS | Tests cover add/count/merge, invalid options, invalid merge sources, context cancellation, Redis wrong-payload errors, and Testcontainers-backed race execution. P0=0 P1=0. |
| Security | PASS | HLL stores digests instead of raw caller values and `RedisError` reports redacted key IDs. Raw value leakage is hook-tested. P0=0 P1=0. |
| Operator/Ops | PASS | Docs separate Bloom state from HLL state, keep Cuckoo module scope deferred, and state Redis persistence/eviction/ACL ownership. P0=0 P1=0. |
| Developer/API | PASS | API remains narrow: `Add`, `Count`, `Merge`, `HasherKey`, with string/bytes/custom constructors and Go doc comments. P0=0 P1=0. |
| User/Caller | PASS | README and examples state HLL is approximate cardinality only, not membership or duplicate certainty. P0=0 P1=0. |
| Integration | PASS | Existing Bloom key prefix and config metadata remain isolated; cleanup helpers remove HLL keys for shared Redis fixture hygiene. P0=0 P1=0. |

## 검증

- `make fmt-check`: PASS.
- `make tidy-check`: PASS.
- `make vet`: PASS.
- `make lint`: PASS, `0 issues.`
- `go test -count=1 ./probabilistic/redis`: PASS.
- `go test -run 'HyperLogLog|GoroutineStressTesterCoversConcurrentHyperLogLog|AsyncJobTesterCoversHyperLogLog' -count=1 ./probabilistic/redis`: PASS.
- `go test -p 1 -race -count=1 ./probabilistic/redis`: PASS.
- `make test`: PASS.
- `make race`: PASS.
- Serial Testcontainers repair gate after an initial parallel full-gate run:
  `go test -p 1 -count=1 ./probabilistic/redis && go test -p 1 -race -count=1 ./probabilistic/redis`: PASS.
- `git diff --check`: PASS.

## 메모

The first full `make test` and `make race` commands were started concurrently.
Both passed, but the Testcontainers serial rule was repaired by rerunning the
touched Redis package test and race gates sequentially.

P0=0 P1=0.
