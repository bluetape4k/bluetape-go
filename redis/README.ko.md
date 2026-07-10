# redis

`github.com/bluetape4k/bluetape-go/redis`는 bluetape-go의 Redis 기반 package가
공유할 작은 Redis safety primitive를 제공합니다. Package 이름은 `btredis`입니다.

## 범위

이 package는 generic Redis client facade가 아닙니다. Redis connection, retry,
logging, metrics, tenant isolation, package-specific key authorization을 소유하지
않습니다. Caller가 `go-redis` client와 deadline을 소유합니다.

직접 Redis Streams operation은 sibling [`redis/stream`](stream/README.ko.md)
package를 사용하세요. 이 package는 command validation과 sanitized error만
소유하며 stream topology, payload encoding, consumer-group policy, replay,
retention은 caller-owned입니다.

Issue #569는 foundation package만 추가합니다. `lock/redis`, `leader/redis`,
`ratelimit/redis`, `probabilistic/redis` 같은 기존 package는 여기서 migration하지
않습니다. 후속 migration은 package-local helper를 교체하기 전에 old/new key parity
test와 benchmark evidence를 추가해야 합니다.

## Key

`KeyBuilder`는 package-owned structural part와 caller-owned logical key segment를
분리합니다.

- Prefix는 `bluetape:probabilistic:bloom:v1` 같은 colon-delimited package string을
  허용합니다.
- Structural part는 empty value, brace, `:` delimiter를 거부합니다.
- `LogicalKey`는 space, brace, colon을 포함한 caller-owned key byte를 그대로
  보존합니다.
- `WithHashTag`는 empty/braced tag를 거부하고 hash tag를 그대로 보존합니다.
  기존 probabilistic Redis namespace가 colon을 쓰므로 `:`는 허용합니다.
- Hash tag는 Redis Cluster same-slot helper일 뿐 tenant isolation이나
  authorization boundary가 아닙니다.

Diagnostic에는 `Key.RedactedID` 또는 `RedactedKeyID`를 사용하세요. 이 id는
trusted operational log를 위한 stable correlation handle이며 anonymization이
아닙니다. Entropy가 낮은 key는 candidate ID를 다시 계산해 추측될 수 있습니다.
`Key.Value`는 Redis command input이며 caller key material을 포함할 수 있습니다.

## Owner Token

`OwnerToken`은 Redis 비교에 쓰는 256-bit random lowercase-hex credential입니다.
`String`, `GoString`, `slog.LogValuer` formatting은 redacted value를 반환합니다.
`RedisValue()`는 Redis command argument에만 넘기고 log에 남기지 마세요.

## Lease Script

`CompareAndDelete`와 `CompareAndExtend`는 package-level Lua script를
`redis.NewScript`로 실행합니다. Redis dispatch 전에 nil context, canceled context,
nil client, lease, TTL을 검증합니다.

항상 caller-owned timeout을 사용하세요.

```go
ctx, cancel := context.WithTimeout(parent, time.Second)
defer cancel()
ok, err := btredis.CompareAndDelete(ctx, client, lease, "redis lock")
```

`(false, nil)`은 ownership drift를 뜻합니다. Key가 더 이상 lease owner token을
담고 있지 않은 상태이며 infrastructure error가 아닙니다.

Command가 Redis에 dispatch된 뒤 cancellation/deadline error가 발생하면 caller
관점의 commit state는 indeterminate일 수 있습니다. Delete/extend가 commit되지
않았다고 단정하지 말고 Redis state를 확인하거나 idempotent owner workflow로
retry하세요.

## Error 와 Runbook

Redis script/client failure는 `OpError`로 반환합니다. `OpError.Error()`는 sanitized
message입니다. Low-cardinality family/operation label과 redacted key id는 포함하지만
raw key, owner token, provider error text는 포함하지 않습니다. Cause는 `errors.Is`,
`errors.As`, `errors.Unwrap`으로 확인하세요.

운영 체크:

- `false, nil`: owner drift입니다. Reacquire하거나 owner로서의 동작을 중지합니다.
- `context.Canceled` / `context.DeadlineExceeded`: caller timeout path입니다.
  Command가 dispatch되었을 수 있으면 Redis state를 확인합니다.
- `OpError`: Redis script/client path입니다. Unwrapped cause와 redacted key id를
  확인합니다.
- Partial failure: redacted key id는 caller가 안전한 lookup handle을 저장하지
  않는 한 correlation-only 값입니다. Cleanup은 caller-owned namespace를 enumerate한
  뒤 bounded `SCAN` / `MATCH` / `COUNT`로 후보를 좁히고, `RedactedKeyID`를
  local로 다시 계산해 failing id와 matching하세요. Delete 전 candidate set을
  dry-run하고 production에서 broad blocking `KEYS` scan은 피하세요. Raw key/token은
  log에 남기지 않습니다.
- Rollback: #569는 기존 Redis package를 migration하지 않으므로 rollback은 새
  package consumer를 제거하는 것이며 기존 Redis 동작은 바꾸지 않습니다.

## 검증

```bash
go test -count=1 ./redis
go test -p 1 -count=1 ./redis
go test -p 1 -race -count=1 ./redis
go test -count=1 ./redis -run Example
```
