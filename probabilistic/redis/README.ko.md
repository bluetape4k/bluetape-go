# probabilistic/redis

[English](README.md) | 한국어

`probabilistic/redis`는 Redis-backed shared Bloom filter와 HyperLogLog
cardinality estimate를 제공합니다. Bloom filter는 configuration을 Redis
metadata에 immutable하게 보관하고, 모든 read/mutation 전에 Lua script로 metadata를
검증하며, shared bit는 Redis bitmap string에 저장합니다. HyperLogLog는 core Redis
`PFADD`, `PFCOUNT`, `PFMERGE` command를 사용합니다.

![probabilistic redis bloom runtime](../../docs/images/readme-diagrams/probabilistic-redis-bloom-runtime.png)

## 가져오기

```go
import redisbloom "github.com/bluetape4k/bluetape-go/probabilistic/redis"
```

## 사용 예

```go
cfg, err := probabilistic.NewConfig(1_000_000, 0.01)
if err != nil {
    return err
}

filter, err := redisbloom.NewStringBloomFilter(ctx, redisClient, "auth:tenant-a:login-attempts", cfg)
if err != nil {
    return err
}

changed, err := filter.Put(ctx, "candidate-key")
if err != nil {
    return err
}
if !changed {
    // 모든 hashed bit가 이미 set된 상태입니다. 중복 확정은 아닙니다.
}

mayExist, err := filter.MightContain(ctx, "candidate-key")
```

`[]byte` 값에는 `NewBytesBloomFilter`를 사용하고, custom value type에는 deterministic
`probabilistic.Hasher[T]`를 명시해 `NewBloomFilter`를 사용합니다.

## HyperLogLog

Membership check가 아니라 approximate distinct count가 필요할 때 HyperLogLog를
사용합니다.

```go
hll, err := redisbloom.NewStringHyperLogLog(redisClient, "auth:tenant-a:active-users")
if err != nil {
    return err
}

changed, err := hll.Add(ctx, "user-1", "user-2")
if err != nil {
    return err
}

estimate, err := hll.Count(ctx)
```

`NewBytesHyperLogLog`는 `[]byte` 값을 지원합니다. `NewHyperLogLog`는 custom
deterministic `probabilistic.Hasher[T]`를 받습니다. 값은 hasher를 거친 뒤 SHA-256
hex digest로 저장되므로 Redis에는 raw caller value가 아니라 stable identifier가
전달됩니다.

`Merge(ctx, sourceNamespaces...)`는 `PFMERGE`로 source HLL namespace를 receiver
namespace에 병합하며, receiver의 기존 estimate도 보존합니다.

## Redis 상태

Redis Bloom은 namespace마다 Redis Cluster-safe hash-tag key pair 하나를 사용합니다.

| Key suffix | Type | 목적 |
|---|---|---|
| `:bits` | bitmap string | `GETBIT`, `SETBIT`, `BITCOUNT`, `STRLEN`으로 읽고 쓰는 Bloom bit. |
| `:config` | hash | 모든 shared-state operation 전에 확인하는 immutable metadata. |

Redis HyperLogLog는 namespace마다 Redis Cluster-safe hash-tag key 하나를 사용합니다.

```text
bluetape:probabilistic:hll:v1:{namespace}
```

Namespace는 안정적인 운영 식별자여야 합니다. Raw user ID, email, token, secret,
password, credential, API key를 namespace에 넣지 마세요.

## 동작

- `MightContain(ctx, value) == false`는 값이 확실히 없다는 뜻입니다.
- `MightContain(ctx, value) == true`는 값이 있을 수 있다는 뜻이며 false positive가
  가능합니다.
- `Put(ctx, value) == false`는 모든 hashed bit가 이미 set되어 있었다는 뜻입니다.
  같은 값이 이미 들어 있었다는 확정은 아닙니다.
- `Clear(ctx)`는 config metadata는 보존하고 shared bitmap state만 삭제합니다.
- `BitCount`, `IsEmpty`, `ApproximateElementCount`, `ExpectedFPP`는 shared bitmap에서
  운영 metadata를 읽습니다.
- Config 또는 hasher mismatch는 shared state를 읽거나 변경하기 전에 error로
  처리됩니다.
- HyperLogLog `Count(ctx)`는 approximate cardinality입니다. 값이 삽입되었는지
  확인하는 membership query가 아닙니다.
- HyperLogLog `Add(ctx, values...)`의 반환값은 Redis `PFADD` state change이며,
  중복 확정이 아닙니다.

## 운영 경계

- Redis persistence, eviction policy, TLS, AUTH, ACL, backup policy는 caller가
  책임집니다.
- Shared filter에는 `noeviction` 또는 reserved memory를 권장합니다. `:config`가
  남은 상태에서 `:bits`가 evict되면 namespace를 rebuild하기 전까지 no-false-negative
  기대가 깨질 수 있습니다.
- `Clear`는 administrative action으로 취급하세요. 실수로 삭제했다면 새 namespace로
  rebuild하고 reader를 검증한 뒤, 새 namespace가 승인된 후에만 old key를 정리합니다.
- HyperLogLog key도 ordinary Redis key입니다. Persistence, eviction, ACL, backup
  policy는 caller 책임입니다.

## 테스트

```bash
go test -count=1 ./probabilistic/redis
go test -race -count=1 ./probabilistic/redis
```
