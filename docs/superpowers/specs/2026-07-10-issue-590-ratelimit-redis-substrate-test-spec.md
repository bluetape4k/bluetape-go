# Issue #590 Redis Rate Limiter Diagnostic Test Spec

## Target

`ratelimit/redis.Limiter.Allow` must surface only its Redis `Eval` provider
failure as a typed, redacted `redis.OpError`, while retaining the existing
token-bucket and caller-key contracts.

## Regression Cases

| Case | Setup | Required Assertions |
|---|---|---|
| Provider failure | Closed `*redis.Client`; call `Allow` with a marker namespace and key | `errors.Is(redis.ErrClosed)`, `errors.As(*btredis.OpError)`, family `rate limiter`, operation `consume`, expected redacted bucket-key ID, no raw key/namespace/provider marker leak. |
| Late cancellation | Directly exercise the private error helper with a canceled context and stable provider cause | `errors.Is` matches both causes; `errors.As` returns `*btredis.OpError`; formatted error stays redacted. |
| Preflight cancellation | Existing canceled-context test | `context.Canceled` returns before dispatch and is not converted to provider diagnostics. |
| Key parity | Existing exact-key preservation test | `tenant:blue` and ` tenant:blue ` remain distinct exact bucket keys. |
| Behavior parity | Existing burst, refill, TTL, namespace, and stress tests | Script result and concurrent admission remain unchanged. |

## Execution

```bash
go test -p 1 -count=1 ./ratelimit/redis ./redis
go test -p 1 -race -count=1 ./ratelimit/redis
TESTCONTAINERS_REUSE_ENABLE=false TESTCONTAINERS_RYUK_DISABLED=false make ci
```

The package uses Testcontainers, so commands remain serial. No benchmark command
belongs to this test spec because neither the admission algorithm nor the
performance contract changes.
