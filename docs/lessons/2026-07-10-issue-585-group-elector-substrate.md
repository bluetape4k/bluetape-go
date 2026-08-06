# 교훈 - Redis GroupElector Substrate Migration (2026-07-10)

**Related issue:** #585
**Affected module:** `leader/redis`

## L1: composite ZSET ownership은 유지하고 canonical suffix만 재사용한다

### Problem

GroupElector는 ZSET member를 `memberID:<random>`으로 저장하고 Lua script는 그 정확한
member value를 Redis server time과 결합한다. shared owner-token/lease helper는
whole-value assumption이 다르다.

### Decision

shared `newElectorToken` helper를 통해 random suffix를 만들 때만
`redis.NewOwnerToken`을 사용한다. 모든 GroupElector acquire/release/renew script와
기존 member-qualified ZSET value를 보존한다.

### Evidence

- `leader/redis/group_test.go`는 stored member prefix와 canonical suffix를 증명하고
  provider failure가 typed, causal, key-redacted 상태로 남는지 검증한다.
- `go test -p 1 -race -count=1 ./leader/redis`
- `make test` and `make race`

### Future Guard

#570 provider migration에서는 reusable token generation과 reusable lease script를
분리한다. script는 정확한 Redis value contract와 time/ownership semantics가 provider의
persisted representation과 일치할 때만 재사용한다.
