# 교훈 - Redis Cache Coordinator Substrate Migration (2026-07-10)

**Related issue:** #588
**Affected module:** `cache/rediscoord`

## L1: input contract가 호환될 때만 safety primitive를 재사용한다

### Problem

shared `redis.KeyBuilder`는 package-owned structural segment를 검증하고
`redis.OwnerToken`은 canonical value를 받는다. `cache/rediscoord`는 caller
namespace/key를 verbatim 보존하고 short-lived result envelope token을 opaque historical
value로 비교하도록 의도되어 있다.

### Decision

direct provider diagnostic에는 `redis.OpError`만 재사용한다. key layout, duration
normalization, envelope token handling, migration된 `lock/redis` lease boundary는
local로 유지한다.

### Evidence

- `cache/rediscoord/operation_error_test.go`: redacted failure, typed cause,
  late-context cause joining, key-byte preservation, opaque token coverage.
- `TESTCONTAINERS_REUSE_ENABLE=false TESTCONTAINERS_RYUK_DISABLED=false make ci`

### Future Guard

남은 모든 #570 slice에서 shared helper를 채택하기 전에 public key/token/TTL/error
input을 비교한다. 확립된 caller value를 거절하는 helper는 refactoring opportunity가
아니라 compatibility boundary다.

## L2: Local Testcontainers verification은 stale reuse setting을 override해야 한다

### Problem

machine-level Testcontainers configuration이 reuse를 켜고 Ryuk를 꺼 두어 old provider
container가 살아남았고, port-mapped integration run을 간헐적으로 오염시켰다.

### Decision

machine-level setting을 의도적으로 고칠 때까지 repository-wide local verification에는
명시적 non-reuse와 cleanup environment value를 사용한다.

### Future Guard

무관한 Redis, PostgreSQL, NATS test가 connection reset, EOF, timeout이 섞여 실패하면
application code를 바꾸기 전에 labeled stale container를 점검한다. full gate는
`TESTCONTAINERS_REUSE_ENABLE=false TESTCONTAINERS_RYUK_DISABLED=false`로 다시
실행한다.
