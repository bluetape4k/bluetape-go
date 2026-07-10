# Issue #588 Redis Cache Coordinator Test Spec

## Target

`cache/rediscoord.StampedeCache` direct Redis command failures must become
typed, redacted `redis.OpError` values without changing successful or sentinel
paths.

## Regression Cases

| Case | Setup | Required Assertions |
|---|---|---|
| Result read failure | Closed Redis client; call `readOwnerResult` | `errors.Is(redis.ErrClosed)`, `errors.As(*redis.OpError)`, family `cache coordination`, operation `result-get`, result-key ID, no key/token marker leak. |
| Owner read failure | Closed Redis client; call `ownerToken` | Same typed/cause/redaction assertions with operation `owner-get` and lock-key ID. |
| Owner check failure | Closed Redis client; call `ensureOwner` with a valid lease | Same assertions with operation `owner-check`, lease-key ID, and no owner token leak. |
| Result write failure | Closed Redis client; call `storeResult` | Same assertions with operation `result-set`, result-key ID, and no payload/token/key marker leak. |
| Late deadline | Test command dispatch that returns after a deadline | Returned operation error matches the provider cause and `context.DeadlineExceeded`. |
| Key/token parity | Existing contract tests with spaces, delimiters, and opaque owner tokens | Exact lock/result keys and token equality remain unchanged. |

## Execution

```bash
go test -p 1 -count=1 ./cache/rediscoord -run 'OperationError|Key|Token|Deadline'
go test -p 1 -count=1 ./cache/rediscoord
go test -p 1 -race -count=1 ./cache/rediscoord
go test -p 1 -count=1 ./redis ./lock/redis
make ci
```

The Redis Testcontainers package remains serial. No benchmark command belongs
to this test spec because no throughput or algorithm contract changes.
