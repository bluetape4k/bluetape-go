# Issue #588 Redis Cache Coordinator Test Spec

> 한국어 요구사항 경계: 이 spec/design/test-spec 문서는 한국어 독자가 요구사항을 추적할 수 있도록 목적과 검증 경계를 한국어로 보강한다. API 이름, command, code identifier, issue/PR 번호, compatibility matrix, acceptance keyword, DoD/test evidence는 요구사항 약화를 막기 위해 원문 그대로 보존한다. 변경자는 아래 literal contract를 삭제하거나 의미를 약하게 바꾸지 않아야 한다.
> 추가 한국어 검증 메모: 영어로 남은 항목은 대부분 code/API/evidence literal이다. 구현 전에는 한국어 경계 문장과 원문 acceptance checklist를 함께 읽고, 검증 gate가 줄어들지 않았는지 확인한다.\n

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
