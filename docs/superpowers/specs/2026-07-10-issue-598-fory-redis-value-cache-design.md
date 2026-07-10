# Issue #598 Go-native Apache Fory Redis Value Cache Design

## 배경

Issue #598은 Go 프로세스끼리 사용하는 신뢰된 내부 Redis cache 값에 Apache
Fory payload를 binary bytes로 직접 저장하는 opt-in 경계를 추가한다. 이 기능은
`cache/rediscoord`의 단기 owner-result coordination protocol과 다르다.
`rediscoord`는 codec payload를 JSON/base64 envelope 안에 넣지만, 이 설계는 Redis
value 자체를 versioned binary envelope로 저장한다.

현재 저장소에서 확인한 기반은 다음과 같다.

- `cache/rediscoord/fory`는 Apache Fory Go `v1.3.0`을 사용하며
  `WithXlang(false)`, 명시적 fast/compatible profile, 등록 callback, resource
  limits, mutex 보호, panic sanitization을 제공한다.
- `cache/rediscoord/fory`의 `BTFY` envelope는 coordination codec의 wire format이며
  magic, version, profile, payload length를 포함한다. Cache schema generation은
  포함하지 않는다.
- `redis` package는 caller-owned logical key를 보존하는 `KeyBuilder`, positive
  millisecond TTL 검증, redacted `OpError`를 제공한다.
- `cache.Cache`는 `Clear`를 포함하지만 direct value cache에 이를 구현하면
  namespace 전체 SCAN/삭제 정책이 public API에 유입된다.
- Issue #597 lesson은 모든 allocation layer의 size bound, Fory runtime-owned bytes의
  lock 안 복사, sanitized unwrap cause, rollout tuple 고정을 요구한다.
- `bluetape4k-projects`의 Fory provider는 fixed-schema fast mode를 volatile cache에만
  권장하고 compatible mode와 wire compatibility가 없음을 명시한다.

관련 근거:

- GitHub issues #595, #596, #598
- `cache/rediscoord/fory/{codec,envelope,errors}.go`
- `redis/{key,ttl,errors}.go`
- `docs/lessons/2026-07-10-issue-597-fory-rediscoord-codecs.md`
- Apache Fory Go `v1.3.0` module source의 `fory.go`와 `threadsafe/fory.go`
- `bluetape4k-projects/io/io/.../ForyBinarySerializer.kt`

## 목표

- caller-owned `redis.Cmdable` 위에 `Get`, `Set`, `Delete`만 제공하는 focused generic
  value cache를 추가한다.
- Go-native fast와 Go-native compatible profile을 별도 constructor와 format ID로
  제공한다.
- schema generation을 public configuration, Redis keyspace, binary envelope에 모두
  반영한다.
- malformed, incompatible, oversized payload를 Fory decode 전에 거절한다.
- Redis 실패는 기존 `btredis.OpError`로 redaction하고 provider 실패는 payload-free
  typed error로 반환한다.
- 기존 `cache/rediscoord/fory`의 public API와 `BTFY` wire format을 유지한다.

## 비목표

- xlang 또는 Go/Python/JVM wire interoperability
- durable database/file persistence format
- untrusted object deserialization
- JSON fallback, content sniffing, profile fallback
- 기존 Redis value 자동 migration
- `cache.Cache` 또는 `cache.LoadingCache` 구현
- `Clear`, `KEYS`, namespace-wide `SCAN` cleanup
- JWT/session signing-state storage
- compression provider
- 이 issue 안에서의 benchmark 또는 성능 주장

## 검토한 접근

### 접근 1: `cache/redisfory`에 Fory runtime 복제

새 package가 serialization, synchronization, envelope, Redis IO를 모두 소유한다.
기존 package의 public 동작에는 영향이 적지만 등록, root type validation, limits,
mutex, panic recovery, copy safety가 #597 구현과 중복된다. 두 구현의 security 및
resource limit가 시간이 지나며 달라질 위험이 있어 제외한다.

### 접근 2: 공통 internal runtime과 독립 public envelope

`cache/internal/forynative`가 Fory runtime lifecycle만 소유하고,
`cache/rediscoord/fory`와 `cache/redisfory`가 각각 public error와 wire envelope를
소유한다. Provider 안전성은 재사용하면서 coordination format과 direct cache format은
분리할 수 있다. 이 접근을 채택한다.

### 접근 3: 기존 `BTFY` codec을 generic Redis cache에 중첩

새 cache envelope 안에 기존 `BTFY` bytes를 넣으면 구현은 빠르지만 envelope가
중첩되고 direct cache가 `rediscoord` import path에 결합된다. 반대로 `BTFY`에 schema
generation을 추가하면 #597의 wire format을 변경한다. 두 변형 모두 경계를 흐리므로
제외한다.

## Architecture

### `cache/internal/forynative`

이 internal package는 다음만 담당한다.

- `WithXlang(false)`를 항상 명시한다.
- fast profile은 `WithCompatible(false)`, compatible profile은
  `WithCompatible(true)`를 사용한다.
- 두 profile 모두 `WithTrackRef(false)`를 사용한다. Profile identity는 이 세 option의
  완전한 tuple이며 일부만 다르게 구성한 runtime을 같은 profile로 취급하지 않는다.
- registration은 constructor 중 단 한 번 수행하며 concurrency 시작 후 변경하지
  않는다.
- root type과 resource limits를 검증한다.
- shared Fory runtime의 serialize/deserialize를 같은 mutex로 보호한다.
- Fory가 반환한 runtime-owned bytes를 mutex 안에서 복사한다.
- registration/provider panic과 원문 오류를 sanitized sentinel로 변환한다.

이 package는 Redis, public envelope, public error text, schema generation을 알지
못한다. 따라서 wire format과 caller API를 소유하지 않는다.

### `cache/rediscoord/fory`

기존 constructor, `Codec[V]`, `CodecError`, reason, `BTFY v1` envelope를 유지한다.
내부 runtime lifecycle만 `cache/internal/forynative`에 위임한다. 기존 tests가 public
behavior와 exact envelope invariants를 회귀 검증한다.

### `cache/redisfory`

새 package는 다음을 소유한다.

- caller-owned `redis.Cmdable`
- namespace와 schema generation 기반 key construction
- direct value TTL
- `BTFV v1` envelope
- `Get`, `Set`, `Delete`
- cache-specific typed errors와 Redis error mapping

Exported concrete type은 constructor 뒤에만 사용할 수 있도록 내부 상태를 숨기고,
zero-value method 호출은 panic 대신 typed `uninitialized` 오류를 반환한다.

## Public API

구체적인 이름은 implementation plan에서 compile-checked test로 고정하되 계약은
다음 shape를 따른다.

```go
type Registration func(*fory.Fory) error

type Options struct {
    Client           redis.Cmdable
    Namespace        string
    SchemaGeneration uint32
    Register         Registration
    MaxPayloadBytes  int
    MaxDepth         int
    MaxTypeFields    int
    MaxTypeMetaBytes int
    MaxSchemaVersionsPerType        int
    MaxAverageSchemaVersionsPerType int
}

func NewNativeFast[V any](options Options) (*ValueCache[V], error)
func NewNativeCompatible[V any](options Options) (*ValueCache[V], error)

func (c *ValueCache[V]) Get(ctx context.Context, key string) (V, error)
func (c *ValueCache[V]) Set(ctx context.Context, key string, value V, ttl time.Duration) error
func (c *ValueCache[V]) Delete(ctx context.Context, key string) error
```

`Registration`은 `cache/redisfory`가 직접 소유한다. 이 type을 공유하기 위해
`cache/rediscoord/fory`를 import하지 않으며 internal package도 public caller API를
노출하지 않는다.

모든 resource limit의 zero 값은 기존 Fory codec과 같은 bounded default를 사용한다.
negative 값은 configuration error다.

| Option | Zero-value default | Additional bound |
|---|---:|---|
| `MaxPayloadBytes` | 1 MiB | raw Fory payload 기준, `math.MaxUint32` 이하 |
| `MaxDepth` | 20 | positive |
| `MaxTypeFields` | 512 | positive |
| `MaxTypeMetaBytes` | 4096 | positive |
| `MaxSchemaVersionsPerType` | 10 | positive |
| `MaxAverageSchemaVersionsPerType` | 3 | positive |

따라서 total `BTFV` value bound는 `14 + MaxPayloadBytes`다. Constructor는 nil 및
typed-nil `redis.Cmdable`, nil registration, invalid namespace/generation, negative limit,
payload limit overflow를 Redis I/O 전에 deterministic configuration error로 거절한다.

지원 root shape는 bool, signed/unsigned integer, float, string, struct, `[]byte`다.
pointer, complex, map, array, non-byte slice, interface, func, chan, unsafe pointer는
constructor에서 `unsupported-value`로 거절한다.

`SchemaGeneration`은 0을 허용하지 않는다. 자동 default는 incompatible rollout을
숨길 수 있으므로 제공하지 않는다. `Namespace`는 colon-separated structural segments로
검증하고 각 segment는 cleanup pattern에 안전한 ASCII `[A-Za-z0-9._-]+`만 허용한다.
Caller logical key는 `KeyBuilder.LogicalKey` 규칙대로 보존한다.

Redis key의 conceptual shape는 다음과 같다.

```text
bluetape:cache:fory:<namespace>:g<schema-generation>:<logical-key>
```

Package는 Redis Cluster hash tag를 주입하거나 같은 slot을 보장하지 않는다. 각 public
operation은 single-key command이므로 multi-key co-location이 필요하지 않다. Caller key의
brace는 logical key 일부로 보존되며 caller가 선택한 hash-tag 효과도 그대로 적용된다.
Logical key는 Redis에 평문으로 보이는 material이므로 secret boundary가 아니며 secret을
넣지 않는다.

`Clear`와 loading behavior는 제공하지 않는다. 호출자가 필요하면 기존 memory/near
cache와 명시적으로 조합한다.

## Binary Envelope

`cache/redisfory`는 direct value 전용 `BTFV v1` envelope를 사용한다.

| Offset | Size | Field |
|---:|---:|---|
| 0 | 4 | magic `BTFV` |
| 4 | 1 | envelope version `1` |
| 5 | 1 | format: `1=native-fast`, `2=native-compatible` |
| 6 | 4 | schema generation, big endian |
| 10 | 4 | Fory payload length, big endian |
| 14 | N | raw Go-native Fory payload |

Decode 순서는 total-size bound, minimum header, magic, version, format, schema,
declared payload bound, exact remaining length, Fory decode다. Declared length와 실제
remaining length가 다르면 truncation과 trailing bytes를 모두 `length-mismatch`로
거절한다. JSON 또는 raw Fory bytes를 감지해서 우회 decode하지 않는다.

## Data Flow

### Set

1. nil context는 기존 cache package 관례대로 `context.Background()`로 정규화한다.
2. canceled/deadline context는 serialization 전에 반환한다.
3. logical key와 1ms 이상의 positive TTL을 검증한다.
4. internal runtime mutex 안에서 value를 serialize하고 bytes를 복사한다.
5. payload limit를 검사한 뒤 `BTFV v1` envelope를 할당한다.
6. serialization/envelope 생성 중 cancellation이 발생할 수 있으므로 Redis dispatch
   직전에 `ctx.Err()`를 다시 검사한다.
7. Redis `SET`에 binary bytes와 caller TTL을 전달한다.
8. Redis 실패는 raw key를 노출하지 않는 `btredis.OpError`로 반환한다.

### Get

1. nil context를 정규화하고 pre-canceled/deadline context를 반환한다.
2. logical key를 검증하고 schema generation을 포함한 Redis key를 생성한다.
3. Redis `GET(...).Bytes()`를 수행한다.
4. `redis.Nil`은 `cache.ErrCacheMiss`로 반환한다.
5. 다른 Redis 실패는 `btredis.OpError`로 반환한다.
6. Redis read 이후 cancellation/deadline을 다시 검사해 envelope/Fory work 전에 반환한다.
7. envelope와 size invariants를 모두 검증한다.
8. 검증된 raw payload만 internal runtime mutex 안에서 decode한다.

Redis `GET` 자체가 response bytes를 materialize하므로 package limit는 Redis client의
최초 allocation을 막지는 못한다. 대신 추가 envelope/Fory allocation 전에 즉시
거절한다. 더 강한 network-level 제한은 caller-owned Redis client와 infrastructure
policy의 책임이다.

### Delete

nil context를 정규화하고 pre-canceled/deadline context와 invalid logical key를 Redis
dispatch 전에 반환한 뒤 Redis `DEL`을 한 번 수행한다. 없는 key 삭제는 idempotent
success다. 실패만 `btredis.OpError`로 반환한다.

## Error Contract

- Miss: `cache.ErrCacheMiss`
- Invalid key: `btredis.ErrInvalidKey`
- Invalid TTL: `btredis.ErrInvalidTTL`
- Redis operation failure: `*btredis.OpError`
- Configuration, envelope, provider failure: `*redisfory.CacheError`

`CacheError`는 operation, profile, stable reason만 formatting한다. Payload, raw key,
registration text, Fory error text, panic value는 `Error()`와 `Unwrap()`에서 모두
노출하지 않는다.

`Profile`과 `Reason`은 exported low-cardinality string types다. `CacheError`는
`Operation() string`, `Profile() Profile`, `Reason() Reason` accessor를 제공한다.
Caller와 tests는 formatted string parsing 대신 `errors.As`와 accessor를 사용한다.

Stable reasons:

- `configuration`
- `uninitialized`
- `registration`
- `payload-too-large`
- `invalid-magic`
- `unsupported-version`
- `format-mismatch`
- `schema-mismatch`
- `length-mismatch`
- `unsupported-value`
- `fory-failure`

Context cancellation/deadline은 Redis 호출 전이면 직접 반환하고, Redis command 중
발생하면 `btredis.OpError`의 sanitized cause에 join해 `errors.Is` 가능하게 유지한다.
그 외 Redis provider error 원문은 key/payload/command text를 포함할 수 있으므로 package
sentinel로 교체한다.

## Concurrency And Ownership

- `ValueCache`는 caller-owned Redis client를 생성하거나 닫지 않는다.
- options와 registration 결과는 construction 이후 immutable하다.
- copied public values가 별도 mutex로 같은 Fory runtime을 보호하는 문제를 피하도록
  mutable runtime과 mutex를 pointer-owned internal state에 함께 둔다.
- serialization lock은 Fory call과 returned bytes copy까지만 유지한다. Redis IO 중에는
  lock을 잡지 않는다.
- 같은 provider를 여러 goroutine이 공유할 수 있으며 race test는 round-trip 값과
  Redis side effect를 함께 검증한다.

## Failure Modes And Mitigations

### Profile 또는 schema 혼합

Keyspace에 generation을 포함해 정상 rollout에서는 이전 값이 miss가 되게 한다.
Envelope에도 profile/schema를 반복 기록해 잘못 복사되거나 수동 주입된 값은 Fory
decode 전에 거절한다.

### Oversized 또는 malformed input

Total bytes와 declared payload를 모두 bound하고 exact length를 요구한다. Fory decode는
모든 envelope validation 이후에만 실행한다.

### Registration drift

Registration callback과 limits를 rollout tuple로 문서화한다. Dynamic registration과
fallback을 금지한다. Compatible profile도 semantic type change를 자동 해결하지 않는다.

### Runtime race와 mutable buffer aliasing

한 runtime과 한 mutex를 같은 internal state에 두고 returned bytes를 lock 안에서
복사한다. `go test -race`와 copy regression test를 실행한다.

### Secret 또는 payload leakage

Redis key는 deterministic redacted ID로만 진단하고 provider cause를 safe sentinel로
교체한다. Tests는 `Error()`와 `Unwrap()` 모두에 marker가 없는지 검증한다.

### TTL 또는 rollout cleanup 오해

TTL은 positive로 강제하고 package는 namespace cleanup을 제공하지 않는다. 이전
generation은 최대 TTL과 safety margin 이후 operator가 별도 bounded tooling으로
정리한다. Runbook은 먼저 dry-run으로
`SCAN MATCH 'bluetape:cache:fory:<namespace>:g<old>:*' COUNT <bounded>`의 후보 수를
기록하고, 같은 bounded batches를 `UNLINK` 또는 `DEL`로 삭제한 뒤 재-scan한다.
Redis Cluster는 모든 primary를 각각 dry-run/scan/delete/re-scan한다. Production cleanup에
`KEYS`를 사용하지 않는다.

### Observability

Package는 metric backend를 소유하지 않는다. Caller는 operation, profile, stable reason,
hit/miss/error status처럼 low-cardinality field만 metric/log에 기록한다. Raw logical/Redis
key, payload, provider text는 기록하지 않으며 correlation이 필요하면
`Key.RedactedID`만 사용한다.

## Testing Strategy

Implementation은 production code보다 failing test를 먼저 작성하고 expected failure를
확인하는 red-green-refactor 순서를 따른다.

### Unit tests

- internal runtime option defaults/invalid values
- nil/typed-nil Redis client와 nil registration 거절
- cleanup glob/control 문자를 포함한 namespace 거절
- missing registration, registration error/panic sanitization
- supported/unsupported generic root shapes
- fast/compatible same-profile round trip
- `BTFY` existing envelope regression
- `BTFV` magic, version, format, schema, length layout
- wrong magic/version/profile/schema
- truncation, trailing bytes, oversized payload
- zero/empty/nil supported values
- malformed Fory bytes와 type mismatch
- zero-value `ValueCache` safety
- all-method nil/pre-canceled context와 invalid-key preflight
- serialization 뒤 cancellation이 Redis `SET`을 dispatch하지 않는 회귀
- Redis read 뒤 cancellation이 envelope/Fory decode를 실행하지 않는 회귀
- command-time cancellation/deadline `errors.Is`와 Redis provider text sanitization
- typed error `errors.As`/accessor와 payload/key/provider text redaction

### Testcontainers Redis integration tests

Redis 7.4 tests는 serially 실행한다.

- stored Redis value가 JSON/base64가 아닌 `BTFV` binary bytes인지 확인
- fast/compatible set/get round trip
- TTL과 expiry miss
- explicit miss와 idempotent delete
- schema generation rotation이 다른 physical key를 사용하는지 확인
- Redis command failure가 redacted `btredis.OpError`인지 확인
- canceled context가 late Redis write를 남기지 않는지 확인

### Race and repository gates

- `go test -count=1 ./cache/internal/forynative ./cache/rediscoord/fory ./cache/redisfory`
- `go test -race -count=1 ./cache/internal/forynative ./cache/rediscoord/fory ./cache/redisfory`
- `go test -run Example -count=1 ./cache/redisfory`
- `go vet ./cache/internal/forynative ./cache/rediscoord/fory ./cache/redisfory`
- `make ci`
- `git diff --check`

Concurrency test는 no-panic smoke가 아니라 worker별 exact value round trip, successful
operation count, unexpected miss/error count를 검증한다. Testcontainers-backed commands는
다른 Redis/DB test invocation과 병렬로 실행하지 않는다.

## Documentation And Diagram

- `cache/redisfory/README.md`와 `README.ko.md`를 함께 추가한다.
- compile-checked `Example...`에서 explicit profile, registration, schema generation,
  TTL을 보여준다.
- root README locale index와 CHANGELOG를 갱신한다.
- trust boundary, ACL/TLS, volatility, profile compatibility, schema rotation, rollback,
  resource limits, no-fallback policy를 설명한다.
- supported/unsupported root shapes, logical key가 Redis-visible material이며 secret을
  포함하면 안 된다는 점, caller-owned low-cardinality observability를 설명한다.
- direct cache와 `rediscoord`의 JSON/base64 coordination path 차이를 보여주는 flow-style
  architecture diagram을 paired SVG/PNG로 추가한다.
- diagram은 `bluetape4k-diagram` architecture rules를 적용하고 CairoSVG로 render한 뒤
  full-size PNG를 검사한다.

## Rollout And Rollback

한 deployment의 rollout tuple은 다음 전체다.

- namespace
- profile
- complete Fory option tuple: xlang, compatible, track-ref
- schema generation
- registration set와 registered names
- max payload/depth/type metadata/schema history limits
- Fory dependency version

Incompatible 변경은 schema generation을 증가시켜 새 keyspace로 배포한다. Reader와
writer는 같은 tuple로 전환한다. Rollback은 이전 generation/configuration으로 되돌린다.
이전 generation은 최대 value TTL과 safety margin 동안 유지한다. Package는 old
generation을 자동 탐색, fallback, migration, cleanup하지 않는다.

## Benchmark Boundary

Issue #598은 성능 주장을 하지 않는다. 후속 issue #599에서 JSON, native-fast,
native-compatible을 같은 조건으로 측정한다. Benchmark artifact에는 다음이 모두 있어야
한다.

- exact command와 OS, CPU, Go version, git SHA, dirty-tree state
- raw output path
- result table
- Chart
- written analysis와 use-case recommendation
- serialize/decode-only local benchmark와 opt-in Redis-backed parallel benchmark
- shared mutex와 pool/thread-safe runtime의 contention 비교 및 runtime 전략 결론

짧은 local run은 production ranking이 아니라 comparable snapshot으로 표현한다.

## Acceptance Criteria

- [ ] `cache/redisfory`가 `Get`, `Set`, `Delete`만 제공한다.
- [ ] fast/compatible constructor가 `WithXlang(false)`를 명시한다.
- [ ] complete profile tuple이 `WithXlang(false)`, profile별 `WithCompatible`,
      `WithTrackRef(false)`로 고정된다.
- [ ] resource limit zero defaults와 payload/envelope upper bound가 고정되고 invalid
      configuration이 Redis I/O 전에 거절된다.
- [ ] schema generation이 public configuration, keyspace, envelope에 포함된다.
- [ ] `BTFV v1`이 malformed/profile/schema/size/trailing input을 Fory decode 전에 거절한다.
- [ ] miss, Redis operation error, cache/provider error가 각 typed contract를 따른다.
- [ ] `Registration`, `Profile`, `Reason`, `CacheError` accessor가 독립 public API로
      compile-checked된다.
- [ ] caller-owned Redis client lifecycle, ACL/TLS, retries, deadlines를 변경하지 않는다.
- [ ] 기존 `cache/rediscoord/fory` public API와 `BTFY v1` behavior가 유지된다.
- [ ] Testcontainers integration과 race tests가 direct storage와 shared provider safety를 증명한다.
- [ ] English/Korean README, compile-checked example, paired SVG/PNG diagram이 존재한다.
- [ ] implementation PR에 benchmark 성능 주장이 없고 #599 경계를 링크한다.

## Definition Of Done

- [ ] 승인된 spec과 implementation plan이 feature branch에 commit된다.
- [ ] 모든 production behavior가 failing test 이후 구현된다.
- [ ] targeted unit/integration/race/example/vet checks가 통과한다.
- [ ] `make ci`와 `git diff --check`가 통과한다.
- [ ] diagram XML/render/kind audit/full-size inspection evidence가 기록된다.
- [ ] Step 2-R, Step 3-R, Step 6-R, Step 7-R 7-Tier review가 P0/P1=0으로 닫힌다.
- [ ] PR body의 마지막 section이 `## DoD Status`이고 live body를 재검증한다.
- [ ] merge는 별도 사용자 요청 전에는 수행하지 않는다.

## Draft Task List

1. Shared internal Fory runtime을 test-first로 추출하고 #597 regression을 유지한다.
2. `BTFV v1` envelope와 typed cache error를 test-first로 구현한다.
3. direct Redis `ValueCache` methods와 key/schema/TTL contract를 test-first로 구현한다.
4. Testcontainers, race, examples를 추가한다.
5. bilingual README, root index, CHANGELOG, architecture diagram을 추가한다.
6. 전체 validation과 7-Tier review를 수행하고 PR을 생성한다.
