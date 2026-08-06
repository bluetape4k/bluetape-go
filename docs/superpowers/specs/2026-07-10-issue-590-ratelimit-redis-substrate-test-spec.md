# Issue #590 Redis Rate Limiter Diagnostic Test Spec

> 한국어 요구사항 경계: 이 spec/design/test-spec 문서는 한국어 독자가 요구사항을 추적할 수 있도록 목적과 검증 경계를 한국어로 보강한다. API 이름, command, code identifier, issue/PR 번호, compatibility matrix, acceptance keyword, DoD/test evidence는 요구사항 약화를 막기 위해 원문 그대로 보존한다. 변경자는 아래 literal contract를 삭제하거나 의미를 약하게 바꾸지 않아야 한다.
> 추가 한국어 검증 메모: 영어로 남은 항목은 대부분 code/API/evidence literal이다. 구현 전에는 한국어 경계 문장과 원문 acceptance checklist를 함께 읽고, 검증 gate가 줄어들지 않았는지 확인한다.\n

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
