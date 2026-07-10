# Lessons Learned - Redis Lock Substrate Migration (2026-07-10)

**Related issue:** #579
**Affected module:** `lock/redis`

## L1: Shared safety primitives must not narrow legacy option contracts

### Problem

The new `redis.OwnerToken` accepts only canonical 64-character lowercase hex
values, but `lock/redis.Options.Token` has always accepted any non-blank value
after trim normalization. The shared TTL helper also requires millisecond
precision while the existing option accepts every positive duration.

### Decision

Use shared primitives only where their contract is a strict compatibility match:
default-generated tokens and canonical lease unlocks use the substrate; custom
tokens retain a private compatibility script; local TTL validation remains.

### Evidence

- `lock/redis/mutex_test.go`: generated canonical token, custom-token
  normalization, provider-error redaction, existing contention/expiry/context
  tests, and race coverage.
- `go test -p 1 -race -count=1 ./lock/redis`
- `make ci`

### Future Guard

For every #570 migration slice, compare existing public key/token/TTL/error
contracts against the shared primitive before replacing code. A helper that
narrows a caller-visible input domain is a compatibility boundary, not an
automatic refactor target.
