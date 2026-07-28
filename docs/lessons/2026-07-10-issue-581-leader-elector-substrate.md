# 교훈 - Redis Elector Substrate Migration (2026-07-10)

**Related issue:** #581
**Affected module:** `leader/redis`

## L1: Composite Redis value는 shared lease의 compatibility boundary다

### Problem

shared `redis.Lease` script helper는 저장된 정확한 값이 canonical `OwnerToken`이기를
요구한다. single leader Elector는 caller가 elected member를 식별할 수 있도록
의도적으로 `memberID:<random>`을 저장한다.

### Decision

`redis.NewOwnerToken`으로 random suffix만 생성한다. 기존 release/renewal script를
유지한다. 이를 `Lease`로 교체하면 stored value가 바뀌거나 더 넓은 shared
abstraction이 필요하기 때문이다.

### Evidence

- `leader/redis/elector_test.go`: canonical token suffix, redacted provider error
  regression test, 기존 owner-drift와 renewal-loss
  coverage.
- `go test -p 1 -race -count=1 ./leader/redis`
- `make ci` after `golangci-lint cache clean`

### Future Guard

#570 migration에서는 token generation reuse와 whole-value lease reuse를 구분한다.
shared primitive는 정확한 Redis value contract가 package의 persisted format과 맞을 때만
적합하다.
