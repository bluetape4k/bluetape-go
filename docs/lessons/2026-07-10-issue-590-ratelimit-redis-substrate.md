# 교훈 - Redis Rate Limiter Diagnostic Substrate Migration (2026-07-10)

**Related issue:** #590
**Affected module:** `ratelimit/redis`

## L1: shared diagnostic과 behavior-specific Redis helper를 분리한다

### Problem

`ratelimit/redis`는 nonblank caller key를 byte-for-byte로 받고, 자체 refill-aware
idle TTL을 계산하며, package-owned Lua script에서 token-bucket result tuple을
반환한다. shared `redis` substrate는 유용한 diagnostic을 갖지만 key, TTL,
ownership-script helper는 더 좁은 contract를 encode한다.

### Decision

direct `Eval` failure에는 `redis.OpError`만 재사용한다. bucket-key formatting, TTL
derivation, script execution, result parsing은 local로 둔다. redacted error를 만들기
전에 late context error를 provider cause와 join한다.

### Evidence

- `ratelimit/redis/operation_error_test.go`는 typed error inspection,
  provider/late-context cause retention, deterministic key ID, marker
  redaction.
- `TESTCONTAINERS_REUSE_ENABLE=false TESTCONTAINERS_RYUK_DISABLED=false make ci`
  가 normal/race repository matrix를 통과했다.

### Future Guard

남은 각 #570 package에서 모든 shared helper input/output contract를 확립된 public
key, TTL, token, script behavior와 비교한다. 기존 contract를 보존할 때만 helper를
채택한다. key/script/TTL behavior가 공유될 수 없어도 diagnostic은 공유할 수 있다.
