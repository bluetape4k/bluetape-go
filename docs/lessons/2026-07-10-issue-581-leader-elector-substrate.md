# Lessons Learned - Redis Elector Substrate Migration (2026-07-10)

**Related issue:** #581
**Affected module:** `leader/redis`

## L1: Composite Redis values are a compatibility boundary for shared leases

### Problem

The shared `redis.Lease` script helpers require the exact stored value to be a
canonical `OwnerToken`. The single leader Elector deliberately stores
`memberID:<random>` so callers can identify the elected member.

### Decision

Generate only the random suffix with `redis.NewOwnerToken`. Keep the existing
release and renewal scripts because replacing them with `Lease` would either
change the stored value or require a broader shared abstraction.

### Evidence

- `leader/redis/elector_test.go`: canonical token suffix and redacted provider
  error regression tests, alongside existing owner-drift and renewal-loss
  coverage.
- `go test -p 1 -race -count=1 ./leader/redis`
- `make ci` after `golangci-lint cache clean`

### Future Guard

For #570 migrations, distinguish token generation reuse from whole-value lease
reuse. A shared primitive is suitable only when its exact Redis value contract
matches the package's persisted format.
