# Lessons Learned - Redis GroupElector Substrate Migration (2026-07-10)

**Related issue:** #585
**Affected module:** `leader/redis`

## L1: Reuse canonical suffixes without replacing composite ZSET ownership

### Problem

GroupElector stores a ZSET member as `memberID:<random>` and its Lua scripts
combine that exact member value with Redis server time. Shared owner-token and
lease helpers have different whole-value assumptions.

### Decision

Use `redis.NewOwnerToken` only to create the random suffix through the shared
`newElectorToken` helper. Preserve all GroupElector acquire, release, and
renew scripts plus the existing member-qualified ZSET value.

### Evidence

- `leader/redis/group_test.go` proves the stored member prefix and canonical
  suffix, and verifies a provider failure stays typed, causal, and key-redacted.
- `go test -p 1 -race -count=1 ./leader/redis`
- `make test` and `make race`

### Future Guard

For #570 provider migrations, separate reusable token generation from reusable
lease scripts. Reuse a script only when its exact Redis value contract and
time/ownership semantics match the provider's persisted representation.
