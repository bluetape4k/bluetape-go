# probabilistic/redis

[English](README.md) | 한국어

`probabilistic/redis`는 Redis-backed shared Bloom filter와 HyperLogLog
cardinality estimate, 모듈 기반의 삭제 가능한 Cuckoo filter를 제공합니다.
Bloom filter는 configuration을 Redis
metadata에 immutable하게 보관하고, 모든 read/mutation 전에 Lua script로 metadata를
검증하며, shared bit는 Redis bitmap string에 저장합니다. HyperLogLog는 core Redis
`PFADD`, `PFCOUNT`, `PFMERGE` command를 사용합니다.

![probabilistic redis runtime map](../../docs/images/readme-diagrams/probabilistic-redis-bloom-runtime.png)

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

## Workshop Adoption

Application-level 사용 예제는 이 package 밖의 workshop에 둡니다.
[`probabilistic-dedupe-admission`](https://github.com/bluetape4k/bluetape-go-workshop/tree/develop/examples/probabilistic-dedupe-admission)은
probabilistic admission control을 보여주고,
[`shared-redis-bloom-admission`](https://github.com/bluetape4k/bluetape-go-workshop/tree/develop/examples/shared-redis-bloom-admission)은
이 Redis-backed Bloom surface를 사용합니다. Redis HyperLogLog adoption은
workshop issue
[#151](https://github.com/bluetape4k/bluetape-go-workshop/issues/151)에
추적합니다.

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

## Redis 가정

- 현재 Bloom과 HyperLogLog surface는 ordinary Redis command만 사용합니다.
  `NewStringBloomFilter`, `NewBytesBloomFilter`, `NewHyperLogLog`,
  `NewStringHyperLogLog`, `NewBytesHyperLogLog`에는 RedisBloom module이
  필요하지 않습니다.
- Testcontainers suite는 Redis `redis:7.4-alpine`에서 실행합니다. Production
  Redis도 여기서 쓰는 core command family를 제공해야 합니다. Lua script 실행,
  hash/string bitmap command, HyperLogLog command가 필요합니다.
- `CF.ADD`, `CF.EXISTS` 같은 RedisBloom module `CF*` Cuckoo command는 아직
  이 package 범위가 아닙니다. Module availability, ACL, persistence,
  Testcontainers coverage가 명확해질 때까지 후속 범위로 둡니다.
- Bloom capacity는 `probabilistic.Config`가 고정합니다.
  `ExpectedInsertions`, `FalsePositiveProbability`, bit size, hash count,
  hasher key는 namespace의 immutable Redis metadata가 됩니다. 값을 바꾸려면
  새 namespace와 rebuild가 필요합니다.
- HyperLogLog capacity와 error rate는 Redis가 소유합니다. Membership check나
  duplicate certainty가 아니라 approximate distinct count가 필요할 때 선택하세요.

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

Diagnostic check는 보통 관련 key family에서 시작합니다.

```text
HGETALL bluetape:probabilistic:bloom:v1:{namespace}:config
STRLEN  bluetape:probabilistic:bloom:v1:{namespace}:bits
BITCOUNT bluetape:probabilistic:bloom:v1:{namespace}:bits
PFCOUNT bluetape:probabilistic:hll:v1:{namespace}
EXISTS  bluetape:probabilistic:hll:v1:{namespace}
PTTL    bluetape:probabilistic:hll:v1:{namespace}
```

## Cuckoo 필터

`NewCuckoo`는 별도 `bluetape:probabilistic:cuckoo:v1` key를 사용합니다.
CF를 지원하는 Redis 서버가 필요합니다. 일반 Redis 7.4 client나 go-redis CF 메서드의
존재만으로 서버 지원을 판단할 수 없습니다. 새 Go 의존성은 필요하지 않습니다.

```go
client := redis.NewClient(&redis.Options{
    Addr: "localhost:6379", MaxRetries: -1,
    DialTimeout: 3 * time.Second, ReadTimeout: 3 * time.Second,
    WriteTimeout: 3 * time.Second, ContextTimeoutEnabled: true,
})
defer client.Close()
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
filter, err := redisbloom.NewCuckoo(redisbloom.CuckooOptions{
    Client: client, Namespace: "tenant-a:members",
})
if err != nil { return err }
if err = filter.Reserve(ctx, redisbloom.CuckooReserveOptions{
    Capacity: 1024, Expansion: 0,
}); err != nil { return err }
if err = filter.Add(ctx, "item"); err != nil { return err }
mayExist, err := filter.Exists(ctx, "item")
```

- `Reserve`는 기존 key를 덮어쓰지 않습니다. Capacity는 4..2^30이며 BucketSize의
  두 배 이상입니다. BucketSize 0은 2, MaxIterations 0은 20입니다.
  Expansion 0은 **확장 금지**이며 최대값은 32768입니다. 서버가 capacity와 양수
  expansion을 2의 거듭제곱으로 반올림하며 capacity 이전에 포화될 수도 있습니다.
  이는 신뢰하는 운영 설정이지 안전한 tenant quota가 아닙니다. 메모리·CPU·namespace
  수를 별도로 제한하고 신뢰하지 않는 요청 옵션을 그대로 전달하지 마세요.
- `Add`는 단일 item의 `CF.INSERT NOCREATE`를 사용합니다. 자동 생성·batch·hashing·
  adapter 재시도는 없습니다. item은 빈 문자열·NUL을 포함한 최대64 KiB byte열을
  보존합니다. Namespace에는 기존 1..128 byte ASCII 검증 규칙을 적용합니다.
- `Exists`는 근사값이며 false는 missing과 wrong-type도 포함합니다. 건강 상태 검사로
  사용하지 마세요. 작은 필터에서는 같은 item만 반복해도 `Count`가 과대 추정할 수
  있습니다. 두 결과 모두 정확한 소유권 장부가 아닙니다.
- `Delete`는 알려진 성공 삽입을 한 번만 제거합니다. `Exists`로 삭제 권한을 추론하거나,
  unknown mutation을 성공으로 세거나, 알려진 삽입 수를 초과해 삭제하지 마세요.
  never-added item 삭제는 다른 fingerprint를 제거해 false negative를 만들 수 있습니다.
  이 필터를 인증이나 fencing에 사용하지 마세요.
- `ErrCuckooUnsupported`는 확정적인 CF unknown-command 응답이며 ACL·transport 오류가
  아닙니다. middleware가 감싸거나 결합한 오류는 보수적으로 미지원으로 분류하지
  않으며 mutation에는 commit-unknown을 보존합니다.
  `ErrCuckooReply`는 잘못된 응답, `ErrCuckooFull`은 공간 또는 확장 자원
  부족에 따른 삽입 거부입니다. 서버 응답만으로 두 원인을 구분하지 못합니다.
  전송된 mutation 실패·취소에는 `redis.ErrCommitUnknown`이 포함될 수 있으므로 자동 재실행하지 마세요.
- 모든 mutation 경로에서 client·middleware·호출자 재시도를 꺼야 합니다. Cluster는
  redirect replay도 제한하세요(`MaxRedirects: -1`). 좁은 client 인터페이스는 설정을
  강제하지 못하며 이번 검증에는 Cluster transport가 포함되지 않습니다.
- client 생성·Close, credential·TLS·deadline·IO timeout·quota·관측은 호출자가 소유합니다.
  adapter 내부 logger나 goroutine은 없습니다. 최상위 오류는 payload·provider 문자열을
  숨기지만 unwrap한 원인은 민감정보를 포함할 수 있으므로 직접 로깅하지 마세요.
  endpoint는 신뢰 경계 안에 있으며 적대적인 서버의 RESP 디코딩 할당까지 제한하지 못합니다.
- nil context는 오류입니다. 전송 전 취소는 명령을 보내지 않고 응답 후 취소는 성공을
  반환하지 않습니다. 비협조적인 client를 강제 종료하거나 goroutine으로 분리하지 않습니다.

네트워크 없는 `ExampleNewCuckoo`가 성공 삽입 장부와 unknown 응답 처리를 검증합니다.
다른 Docker suite와 분리해 digest로 고정한 Redis 8.8.1 양성 fixture를 실행할 수 있습니다.

```bash
BLUETAPE_CUCKOO_INTEGRATION=1 go test -p 1 -count=1 -timeout=5m ./probabilistic/redis -run '^TestCuckoo'
BLUETAPE_CUCKOO_INTEGRATION=1 go test -race -p 1 -count=1 -timeout=5m ./probabilistic/redis -run '^TestCuckoo'
```

기본 테스트는 fake CF 계약과 일반 Redis 미지원을 검증합니다. opt-in을 실행하지 않은
결과는 서버 지원을 입증하지 않습니다.

## 기존 Bloom·HyperLogLog 테스트

Package 테스트는 Testcontainers for Go로 Redis `redis:7.4-alpine`을 시작합니다.
Container startup은 90초로 제한하고, readiness ping은 10초 window 안에서 짧은
bounded context를 사용하며, live Redis operation과 cleanup도 package-local operation
timeout을 사용합니다.

Coverage 범위:

- Bloom filter configuration reuse, mismatch/corrupt metadata handling, clear,
  false-negative protection, Lua command shape, cancellation, concurrent call.
- HyperLogLog add/count/merge, byte/custom hasher value, invalid option,
  cancelled context, redacted Redis error, raw value non-disclosure, concurrent
  call.
- Concurrent Redis operation과 cancellation behavior에는 `GoroutineStressTester`,
  `AsyncJobTester`를 사용합니다.

```bash
go test -count=1 ./probabilistic/redis
go test -race -count=1 ./probabilistic/redis
```

다른 Testcontainers package와 함께 실행할 때는 package를 직렬로 실행하세요.
Repository의 `make test`, `make race` target은 이 이유로 이미 `-p 1`을 사용합니다.
