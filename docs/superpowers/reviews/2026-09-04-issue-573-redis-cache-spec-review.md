# #573 spec review (Step 2-R)

## 검토 범위와 방법

검토 대상은 `docs/superpowers/specs/2026-09-04-issue-573-redis-cache-design.md`와
현재 `redis/key.go`, `redis/errors.go`, `redis/script.go`, `redis/ttl.go`,
`serialization/serializer.go`, `cache/redisvalue`, `cache/rediscoord`,
`redis/stream` 구현이다. Native lane slot은 기존 완료 agent 기록으로
소진되어 main session이 여섯 관점을 독립 pass로 수행하고 integration을
소유한다.

## 여섯 관점 결과

| 관점 | 확인 내용 | P0 | P1 | P2/P3 |
|---|---|---:|---:|---:|
| Performance | key-per-entry, single-key Lua, bounded payload/round-trip, no hidden scan/goroutine가 명시됨 | 0 | 0 | 0 |
| Stability | context pre/post checkpoint, commit-unknown, no retry worker, fake deep-copy와 race/container 계획이 있음 | 0 | 0 | 0 |
| Security | caller key/payload 보존과 redaction, typed-nil, caller-owned ACL/TLS/credentials, codec 경계가 명시됨 | 0 | 0 | 0 |
| Operator/Ops | Redis persistence/eviction/maxmemory caveat, hash-tag 의미, error runbook, caller lifecycle/rollback이 정의됨 | 0 | 0 | 0 |
| Developer/API | generic `Serializer`, narrow client, sibling package boundary, zero-value and hit bool contract가 명확함 | 0 | 0 | 0 |
| User/Caller | durable Redis와 local/near/stampede 구분, TTL/cancellation/CAS와 compile example 요구가 명시됨 | 0 | 0 | 0 |

## Main-session integration

- `Bucket`/`MapCache` 모두 logical key를 `KeyBuilder.LogicalKey`에 전달해
  trim/case-fold collision을 만들지 않는다.
- MapCache key-per-entry 선택은 entry별 TTL과 Cluster 단일-key atomicity를
  함께 만족하고, cross-key transaction/iteration은 명시적 비목표다.
- Lua result가 `{0}`/`{1,payload}` 또는 `{0,1}` 외이면 malformed로 fail
  closed하며, mutation 오류에는 `btredis.ErrCommitUnknown`을 보존한다.
- `cache.Cache` miss sentinel과 다른 `(value, hit, error)` 계약을 문서와
  acceptance에 반영했다.
- no-live-credential 일반 CI와 Testcontainers readiness/직렬 실행을 분리했다.

## 판정

`P0=0, P1=0`으로 Step 2-R을 통과한다. 구현 시 empty payload, Redis `Nil`,
sub-millisecond TTL, post-dispatch cancellation, typed-nil serializer/client,
그리고 `%+v` redaction을 문서와 동일하게 유지한다.
