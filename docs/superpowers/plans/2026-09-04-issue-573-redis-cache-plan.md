# Redis Bucket 및 MapCache 구현 계획

> 승인 범위: 사용자가 승인한 0.20.0 Type A 실행의 #573 slice. 구현은
> `feat/issue-573-redis-cache-primitives` worktree에서 수행하며, 기존
> `cache/*`와 `redis/*` 패키지의 동작·key를 변경하지 않는다.

## 목표와 완료 조건

caller-owned Redis client와 generic `serialization.Serializer`를 받는 두
Go-native primitive를 추가한다. Bucket은 single-key operation, MapCache는
key-per-entry와 entry별 TTL을 제공한다. logical key 보존, typed/redacted
error, cancellation/commit-unknown, Lua atomicity를 fake와 Redis
Testcontainers로 증명하고 P0=0/P1=0 exact-head CI를 확보한다.

## 파일 책임

| 경로 | 책임 |
|---|---|
| `redis/bucket/doc.go`, `errors.go`, `bucket.go` | Bucket public contract, validation, serializer, Lua/CAS/context |
| `redis/bucket/bucket_test.go`, `example_test.go` | fake deep copy, unit/race/example |
| `redis/bucket/integration_test.go` | opt-in Redis Testcontainers expiry/atomicity |
| `redis/bucket/README.md`, `README.ko.md` | 사용법과 durability/cancellation/runbook |
| `redis/mapcache/doc.go`, `errors.go`, `mapcache.go` | MapCache key-per-entry contract와 operation 구현 |
| `redis/mapcache/mapcache_test.go`, `example_test.go` | fake/unit/race/example |
| `redis/mapcache/integration_test.go` | opt-in Redis Testcontainers 검증 |
| `redis/mapcache/README.md`, `README.ko.md` | MapCache 운영 경계와 near-cache 구분 |
| `redis/README.md`, `redis/README.ko.md` | sibling index와 primitive comparison |
| `docs/superpowers/specs/2026-09-04-issue-573-redis-cache-design.md` | 승인 설계/source ledger |
| 이 문서 | 실행 task와 검증 증적 |

## Task 1 — spec/plan 및 dependency gate

- [x] live #573/#568 metadata와 local Redis/serialization/cache patterns를 확인한다.
- [x] Bucket/MapCache API, key/TTL/codec/context/atomicity/zero-value/non-goal 계약을 spec에 고정한다.
- [x] source parity를 `keep`(KeyBuilder/OpError/Serializer), `adapt`(redisvalue context/TTL), `split`(near-cache/stampede와 durable primitive), `defer`(hash-wide transaction/clear), `non-goal`(RedisJSON/eviction)으로 기록한다.
- [x] plan review에서 six lenses의 P0/P1과 Redis Cluster/TTL/cancellation 위험을 확인한다. 초기 review의 P1(payload bound)과 후속 review의 mutation-path bypass를 수정하고 Step 6-R 증적에 기록한다.
- [x] fake/client skeleton과 RED test를 먼저 작성하고 missing implementation symbols의 첫 실패를 기록한다.

## Task 2 — shared contract per package (TDD)

- [x] 최소 `Client` interface(`Set`, `SetNX`, `Del`, `Eval`)와 typed-nil reflection 검증을 구현한다. Bounded read는 `Eval`을 통해 `GETRANGE`/`EXISTS`를 호출한다.
- [x] `Options[V]`의 namespace/hash-tag/serializer/logger/`MaxPayloadBytes` 검증과 immutable key builder를 구현한다.
- [x] `ErrInvalidContext`, `ErrInvalidOptions`, `ErrSerialization`, `ErrInvalidPayload`, `ErrMalformedResult`, `ErrPayloadTooLarge`, `ErrUninitialized`를 safe typed `Error`와 함께 정의한다.
- [x] provider error는 `btredis.OpError` 기반으로 raw key/payload/provider text를 숨기고 `errors.Is`/`errors.As`/`btredis.ErrCommitUnknown`을 보존한다.
- [x] TTL `0=persistent`, negative reject, sub-ms→1ms normalization과 exact logical key bytes를 table-driven test한다.

## Task 3 — Bucket GREEN 구현

- [x] `Get`은 `GETRANGE`/`EXISTS` bounded Lua로 Redis miss를 `(zero,false,nil)`로 매핑하고 response 뒤 cancellation 시 unmarshal하지 않는다.
- [x] `Set`/`SetIfAbsent`는 marshal·size/ctx preflight 뒤 exact key와 normalized TTL을 한 번만 dispatch한다.
- [x] `GetAndDelete` Lua result `{0}`/`{1,payload}`/`{2}` parser와 malformed/partial result를 구현한다. `{2}`는 기존 key를 보존한다.
- [x] `CompareAndSet` Lua가 expected/replacement bytes를 preflight하고 현재 값을 bounded read로 확인한 뒤 persistent/PX branch를 atomic하게 선택한다.
- [x] `Delete`와 모든 mutation의 provider error/output-plus-error/post-dispatch cancellation을 commit-unknown으로 매핑한다.

## Task 4 — MapCache GREEN 구현

- [x] MapCache는 `map` structural segment와 logical key-per-entry만 사용하고 전체 map hash/iteration/clear를 추가하지 않는다.
- [x] Bucket과 동일한 typed value/TTL/context/error contract를 유지하며 `Get`, `Set`, `SetIfAbsent`, `GetAndDelete`, `CompareAndSet`, `Delete`를 구현한다.
- [x] 같은 namespace의 서로 다른 key가 collision 없이 보존되고 entry별 TTL 만료가 독립적인지 fake/integration으로 확인한다.

## Task 5 — tests/examples/docs

- [x] mutex-safe fake는 input/payload deep-copy, call count/sequence/context, configured errors, output-plus-error를 기록한다.
- [x] codec failure, malformed Lua result, no-dispatch cancellation, late cancellation, redaction, concurrent CAS exact winner와 payload bound를 normal/race로 검증한다.
- [x] Redis Testcontainers는 다른 Docker suite와 직렬 실행하고 readiness/cleanup, expiry, empty/oversized legacy value 보존, concurrent CAS를 증명한다. 환경 미충족은 명시적 PENDING 증적으로 남긴다.
- [x] package README locale pair와 compile-checked example에서 durable Redis, process-local `cache.Memory`, `cache/redisnear`, `cache/rediscoord`를 구분한다.
- [x] root `redis` README 두 locale에 sibling link, API comparison, payload bound, persistence/eviction/ACL/TLS/maxmemory와 caller-owned retry/timeout/client/codec를 추가한다.

## Task 6 — 검증 명령

Package와 container suite는 순차 실행한다.

```bash
gofmt -w redis/bucket/*.go redis/mapcache/*.go
go test -count=1 ./redis/bucket ./redis/mapcache
go test -race -count=1 ./redis/bucket ./redis/mapcache
go test -run Example -count=1 ./redis/bucket ./redis/mapcache
go vet ./redis/bucket ./redis/mapcache
make fmt-check
make tidy-check
make vet
make lint
git diff --check
```

가능하면 `go test -p 1 -count=1 ./redis/... ./cache/...`를 실행한다. baseline의
`cache/rediscoord` expiry failure는 변경 전 증적으로 분리하고 새 package
결과에 섞지 않는다. Testcontainers는 `go test -p 1`과 명시적인 Colima/
Docker readiness 확인 뒤 실행한다.

## Task 7 — review/PR gate

- [x] Step 6-R 7-Tier review를 performance, stability, security, operator/Ops,
  developer/API, user/caller six lenses와 main integration으로 수행하고
  `docs/superpowers/reviews/2026-09-04-issue-573-redis-cache-primitives-step-6r-code-review.md`
  에 exact base/head와 fallback 경계를 기록한다.
- [x] P0/P1 finding은 수정 후 동일 exact head에서 normal/race/문서 검증을 반복한다.
  최종 `P0=0`, `P1=0`이며 architect 독립 lane unavailable은 PASS가 아닌
  main-session fallback과 `PENDING` 조건부 gate로 기록했다.
- [ ] Korean PR body 끝에 `## DoD Status`를 두고 tests/static/review/docs/
  integration/gaps를 기록하며 #573/milestone/assignee live read-back을 한다.
- [ ] PR 생성은 승인된 target repo/base/head에서만 수행하고, merge는 fresh
  exact-head CI와 별도 승인을 거친다.

## 롤백과 후속

PR revert가 rollback이며 기존 Redis key layout/cache behavior는 그대로다.
cross-key transaction, namespace scan/clear, RedisJSON, local eviction,
near-cache invalidation/stampede integration과 0.24.0 follow-up issues는
후속 범위로 남긴다.
