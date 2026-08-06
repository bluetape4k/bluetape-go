# 교훈 - Redis Lock Substrate Migration (2026-07-10)

**Related issue:** #579
**Affected module:** `lock/redis`

## L1: Shared safety primitive는 legacy option contract를 좁히면 안 된다

### Problem

새 `redis.OwnerToken`은 canonical 64-character lowercase hex value만 받지만,
`lock/redis.Options.Token`은 trim normalization 뒤 non-blank value를 항상 허용해
왔다. shared TTL helper도 millisecond precision을 요구하지만 기존 option은 모든
positive duration을 허용한다.

### Decision

contract가 엄격히 compatibility match인 곳에서만 shared primitive를 사용한다.
default-generated token과 canonical lease unlock은 substrate를 사용하고, custom
token은 private compatibility script를 유지하며, local TTL validation은 남긴다.

### Evidence

- `lock/redis/mutex_test.go`: generated canonical token, custom-token
  normalization, provider-error redaction, 기존 contention/expiry/context test, race
  coverage.
- `go test -p 1 -race -count=1 ./lock/redis`
- `make ci`

### Future Guard

모든 #570 migration slice에서 코드를 교체하기 전에 기존 public key/token/TTL/error
contract를 shared primitive와 비교한다. caller-visible input domain을 좁히는 helper는
automatic refactor target이 아니라 compatibility boundary다.
