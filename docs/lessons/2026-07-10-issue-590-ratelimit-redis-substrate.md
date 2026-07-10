# Lessons Learned - Redis Rate Limiter Diagnostic Substrate Migration (2026-07-10)

**Related issue:** #590
**Affected module:** `ratelimit/redis`

## L1: Keep shared diagnostics separate from behavior-specific Redis helpers

### Problem

`ratelimit/redis` accepts nonblank caller keys byte-for-byte, derives its own
refill-aware idle TTL, and returns a token-bucket result tuple from a package
owned Lua script. The shared `redis` substrate has useful diagnostics, but its
key, TTL, and ownership-script helpers encode narrower contracts.

### Decision

Reuse only `redis.OpError` for the direct `Eval` failure. Keep bucket-key
formatting, TTL derivation, script execution, and result parsing local. Join a
late context error with the provider cause before redacted error construction.

### Evidence

- `ratelimit/redis/operation_error_test.go` verifies typed error inspection,
  provider/late-context cause retention, deterministic key ID, and marker
  redaction.
- `TESTCONTAINERS_REUSE_ENABLE=false TESTCONTAINERS_RYUK_DISABLED=false make ci`
  passed normal and race repository matrices.

### Future Guard

For each remaining #570 package, compare every shared helper input and output
contract with the established public key, TTL, token, and script behavior.
Adopt a helper only when it preserves the existing contract; diagnostics can be
shared even when key/script/TTL behavior cannot.
