# Redis tiered value cache 구현 계획

> 한국어 재작성 범위: 이 계획 문서는 한국어 운영 문서로 읽히도록 제목, 판단, 작업 설명, 위험, 검증, 롤백 문맥을 한국어로 정리한다. 명령, 경로, API 이름, 이슈/PR 번호, 브랜치명, 코드 블록, 테스트 출력 같은 증거 문자열은 정확성을 위해 원문 그대로 보존한다.


> **에이전트 작업자용:** 필수 하위 스킬: 사용 superpowers:subagent-driven-development (권장) 또는 superpowers:executing-plans to 이 계획을 작업 단위로 구현. 단계는 checkbox (`- [ ]`) 추적 문법을 사용.

**목표:** 추가 `cache/redisvalue`, a bounded serialized Redis L2 provider 및 a `TieredCache[V]` decorator whose L1 stores `V` directly 및 whose default configuration can be overridden per cache.

**아키텍처:** `ValueCache[V]` owns key construction, TTL normalization, serialization, bounded Redis commands, 및 namespace clear. `TieredCache[V]` composes one exclusively transferred `cache.Cache[string,V]` L1 함께 that provider, using an ABA-safe same-key coordinator 및 a process-local state barrier for invalidation, cleanup, 및 repair. RESP3 tracking remains outside this 패키지 및 issue #536 may use 만 the L1-만 invalidation methods.

**기술 스택:** Go 1.26.3, `github.com/redis/go-redis/v9`, 기존 `cache` 및 `serialization` contracts, Redis 7.4 Testcontainers, standard-library synchronization, `testing.AllocsPerRun`, 및 repository Make targets.

---

## 파일 지도

생성 these focused files:

- `cache/redisvalue/doc.go` — 패키지-level behavior 및 RESP3 boundary.
- `cache/redisvalue/config.go` — 공개 config defaults, validation, 및 copied constructor options.
- `cache/redisvalue/errors.go` — stable reasons, redacted `CacheError`, clear progress, provider wrapping.
- `cache/redisvalue/ttl.go` — zero/sub-millisecond-safe TTL validation 및 go-redis wire normalization.
- `cache/redisvalue/value_cache.go` — bounded `Get`, Redis-first `Set`, `SetDefault`, 및 idempotent `Delete`.
- `cache/redisvalue/clear.go` — streamed `SCAN MATCH` 및 sequential bounded `UNLINK`.
- `cache/redisvalue/coordination.go` — registry, operation token, load flights, participants, ABA-safe retirement.
- `cache/redisvalue/local_state.go` — healthy/blocking/blocked/repairing barrier, leases, tickets, repair epochs.
- `cache/redisvalue/tiered_cache.go` — 공개 decorator reads, loads, mutations, cleanup, 및 invalidation.
- `cache/redisvalue/*_test.go` — same-패키지 unit, deterministic concurrency, race, allocation, 및 integration evidence.
- `cache/redisvalue/documentation_test.go` — executable 영문/한국어 계약-marker parity.
- `cache/redisvalue/example_test.go` — compile-checked 공개 API example.
- `cache/redisvalue/README.md`, `cache/redisvalue/README.ko.md` — synchronized 호출자 및 운영자 계약.
- `docs/lessons/2026-07-18-issue-535-redis-tiered-value-cache.md` — Type A reusable lesson.

Modify 만 these 기존 공개 indexes:

- `README.md`
- `README.ko.md`
- `CHANGELOG.md`

다음을 하지 않는다: change `Makefile`, `cache.Cache`, `cache.LoadingCache`, `cache/redisnear`, `cache/rediscoord`, 또는 `redis.ValidateTTL`. Issue #560 owns benchmark-matrix publication 및 issue #536 owns RESP3 tracking.

## spec coverage matrix

| Acceptance criterion | Implementation 및 proof |
|---|---|
| 1. Constructor-만, zero-value-safe caches | Tasks 2, 6, 및 8 constructor/zero-value 테스트 |
| 2. Caller-owned client 및 concurrency-safe serializer-backed `ValueCache` | Tasks 2-3 unit 테스트, 작업 9 Redis integration, 및 작업 10 race evidence |
| 3. Direct-`V` L1 및 L2-만 serialization | 작업 6 reference/serializer-count 테스트 및 작업 9 pointer-isolation integration |
| 4. Copied defaults 및 per-cache overrides | 작업 1 config-copy 테스트 및 작업 11 example/docs |
| 5. Allocation-free healthy L1 hit | 작업 6 hot-path dependency assertions 및 작업 10 `AllocsPerRun` proof |
| 6. L1 -> L2 -> loader 함께 ABA-safe collapse | Tasks 4, 6, 및 7 coordinator/flight 테스트 |
| 7. Finite/zero TTL 및 known-write adjustment | Tasks 1, 6, 및 9 TTL 테스트 |
| 8. Bounded/redacted reads, writes, clear, inputs, 및 오류 | Tasks 1-3 unit 테스트 plus 작업 9 ACL/clear integration |
| 9. 커밋-unknown 및 token-held cleanup | Tasks 2 및 8 mutation/오류 테스트 |
| 10. Same-key linearization, tickets, state, 및 generations | Tasks 4-8 deterministic 테스트 및 작업 10 adversarial races |
| 11. One cleanup budget 및 newest-repair epoch | Tasks 5 및 8 state/repair 테스트 |
| 12. L1-만 invalidation 및 fleet reset boundary | 작업 8 multi-decorator 테스트 및 작업 11 운영자 docs |
| 13. Synchronous direct-primary 공개 surface | 작업 2 constructor types 및 작업 11 topology docs |
| 14. Unsupported namespace/L1 sharing | 작업 1 namespace validation 및 작업 11 ownership docs |
| 15. Unit, integration, race, stress, 및 example | Tasks 1-10 targeted evidence 및 작업 12 full gates |
| 16. Synchronized 영문/한국어 docs 및 RESP3 distinction | 작업 11 locale-parity check 및 root/패키지 docs |

### 작업 1: 고정 Configuration, Key, TTL, 및 Error Contracts

**파일:**
- 생성: `cache/redisvalue/doc.go`
- 생성: `cache/redisvalue/config.go`
- 생성: `cache/redisvalue/errors.go`
- 생성: `cache/redisvalue/ttl.go`
- Test: `cache/redisvalue/config_test.go`
- Test: `cache/redisvalue/errors_test.go`
- Test: `cache/redisvalue/ttl_test.go`

- [ ] **단계 1: Write failing configuration, key, TTL, 및 오류 테스트**

추가 table-driven 테스트 that assert independent default copies, every configuration bound, namespace/key limits, TTL normalization, redaction, 및 공유 sentinel identity. This task deliberately defines 없음 cache type 또는 constructor, so its GREEN checkpoint does 아님 depend on later coordinator/local-state work.

```go
func TestDefaultConfigReturnsIndependentValidValues(t *testing.T) {
	first := DefaultConfig()
	second := DefaultConfig()
	if err := first.Validate(); err != nil { t.Fatal(err) }
	if first.Value.RemoteTTL != time.Hour || first.Value.MaxValueBytes != 1<<20 || first.Value.ClearBatchSize != 100 {
		t.Fatalf("value defaults = %+v", first.Value)
	}
	if first.Tiered.LocalTTL != 30*time.Minute || first.Tiered.InvalidationWaitTimeout != 30*time.Second || first.Tiered.LocalCleanupTimeout != time.Second {
		t.Fatalf("tiered defaults = %+v", first.Tiered)
	}
	first.Value.RemoteTTL = 2 * time.Hour
	if second.Value.RemoteTTL != time.Hour { t.Fatalf("defaults share state: %+v", second.Value) }
}
```

추가 explicit `errors.Is` assertions:

```go
func TestInputValidationPreservesRedisSentinels(t *testing.T) {
	if err := validateNamespace("tenant*"); !errors.Is(err, btredis.ErrInvalidKey) { t.Fatalf("namespace = %v", err) }
	if err := validateLogicalKey(" "); !errors.Is(err, btredis.ErrInvalidKey) { t.Fatalf("logical key = %v", err) }
	if err := validateEntryTTL(-time.Nanosecond); !errors.Is(err, btredis.ErrInvalidTTL) { t.Fatalf("ttl = %v", err) }
}
```

- [ ] **단계 2: 실행 the 테스트 및 verify RED**

실행:

```bash
go test -count=1 ./cache/redisvalue -run 'Test(DefaultConfig|Config|InputValidation|TTL|CacheError)'
```

예상: FAIL because 패키지/types are 아님 defined.

- [ ] **단계 3: 구현 exact 공개 configuration**

추가 the exported configuration from the approved spec:

```go
type ValueConfig struct {
	RemoteTTL      time.Duration
	MaxValueBytes  int
	ClearBatchSize int64
}

type TieredConfig struct {
	LocalTTL                time.Duration
	InvalidationWaitTimeout time.Duration
	LocalCleanupTimeout     time.Duration
}

type Config struct {
	Value  ValueConfig
	Tiered TieredConfig
}

func DefaultConfig() Config {
	return Config{
		Value: ValueConfig{RemoteTTL: time.Hour, MaxValueBytes: 1 << 20, ClearBatchSize: 100},
		Tiered: TieredConfig{LocalTTL: 30 * time.Minute, InvalidationWaitTimeout: 30 * time.Second, LocalCleanupTimeout: time.Second},
	}
}

func (c Config) Validate() error {
	if err := validateValueConfig(c.Value); err != nil { return err }
	if err := validateTieredConfig(c.Tiered); err != nil { return err }
	if c.Value.RemoteTTL > 0 && c.Tiered.LocalTTL > c.Value.RemoteTTL {
		return newCacheError("validate-config", ReasonConfiguration, "", fmt.Errorf("local ttl exceeds remote ttl: %w", btredis.ErrInvalidTTL))
	}
	return nil
}

```

추가 table rows that call `Config.Validate` directly for every 낮음er/upper bound 및 for `LocalTTL > RemoteTTL`; assert `errors.Is(err, btredis.ErrInvalidTTL)` for TTL relationship failures. Tasks 2 및 6 repeat the relevant validation against copied constructor inputs.

검증 must enforce the approved ranges. 다음을 하지 않는다: call `redis.ValidateTTL`; it rejects the zero 및 positive sub-millisecond TTLs required here. 사용:

```go
func validateEntryTTL(ttl time.Duration) error {
	if ttl < 0 {
		return fmt.Errorf("%w: negative cache ttl", btredis.ErrInvalidTTL)
	}
	return nil
}

func normalizeWireTTL(ttl time.Duration) time.Duration {
	if ttl == 0 {
		return 0
	}
	if ttl < time.Millisecond {
		return time.Millisecond
	}
	if ttl%time.Second == 0 {
		return ttl.Truncate(time.Second)
	}
	return ttl.Truncate(time.Millisecond)
}
```

사용 `reflect.Value.IsNil` 만 for interface dependencies. Validate namespace as exactly one `[A-Za-z0-9._-]+` segment of at most 128 bytes 및 logical keys as non-blank values of at most 1024 bytes 전에 calling `redis.KeyBuilder`.

- [ ] **단계 4: 구현 inspectable redacted 오류**

정의 every approved reason 및 accessor without formatting the causal 오류:

```go
type Reason string

const (
	ReasonConfiguration   Reason = "configuration"
	ReasonUninitialized   Reason = "uninitialized"
	ReasonSerialization   Reason = "serialization"
	ReasonPayloadTooLarge Reason = "payload-too-large"
	ReasonInvalidPayload  Reason = "invalid-payload"
	ReasonLocalFailure    Reason = "local-failure"
	ReasonLocalBlocked    Reason = "local-blocked"
	ReasonProviderFailure Reason = "provider-failure"
	ReasonPartialClear    Reason = "partial-clear"
)

type CacheError struct {
	operation string
	reason    Reason
	keyID     string
	progress  ClearProgress
	hasClear  bool
	cause     error
}

func (e *CacheError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.keyID == "" {
		return fmt.Sprintf("redisvalue %s failed: %s", e.operation, e.reason)
	}
	return fmt.Sprintf("redisvalue %s failed for %s: %s", e.operation, e.keyID, e.reason)
}

func (e *CacheError) Unwrap() error { if e == nil { return nil }; return e.cause }
func (e *CacheError) Operation() string { if e == nil { return "" }; return e.operation }
func (e *CacheError) Reason() Reason { if e == nil { return "" }; return e.reason }
func (e *CacheError) ClearProgress() (ClearProgress, bool) {
	if e == nil || !e.hasClear { return ClearProgress{}, false }
	return e.progress, true
}
```

Provider failures must be `CacheError -> redis.OpError -> original cause`; dispatched mutation failures additionally join `redis.ErrCommitUnknown`. Unit 테스트 must prove raw keys, serialized bytes, provider messages, 및 addresses are absent from `Error()`.

- [ ] **단계 5: 실행 focused 테스트 및 commit**

실행:

```bash
gofmt -w cache/redisvalue/*.go
go test -count=1 ./cache/redisvalue -run 'Test(DefaultConfig|Config|Constructor|Input|TTL|CacheError)'
```

예상: PASS.

커밋 함께 Lore trailers:

```bash
git add cache/redisvalue
git commit -m "Define the redisvalue safety contract" \
  -m "Constraint: zero and sub-millisecond TTLs are valid cache inputs" \
  -m "Rejected: redis.ValidateTTL reuse | its minimum TTL contract is incompatible" \
  -m "Confidence: high" -m "Scope-risk: moderate" \
  -m "Tested: focused redisvalue config, TTL, key, and error tests"
```

### 작업 2: 구현 the Bounded Serialized Redis L2 Provider

**파일:**
- 생성: `cache/redisvalue/value_cache.go`
- Test: `cache/redisvalue/value_cache_test.go`

- [ ] **단계 1: Write failing bounded read/write 테스트 함께 a narrow fake**

추가 `ValueOptions[V]`, `ValueCache[V]`, 및 `NewValueCache` constructor 테스트 here, where the concrete type is first introduced. 검증 nil client, nil/typed-nil serializer, invalid namespace, copied `ValueConfig`, zero-value method safety, 및 공유 input sentinels. The serializer remains 호출자-owned, immutable 후 construction, 및 safe for concurrent `Marshal`/`Unmarshal`; the 패키지 does 아님 add a global codec lock.

정의 one 패키지-internal command interface containing an atomic `ReadBounded` boundary plus `Set`, `Del`, `Scan`, 및 `Unlink`. The production go-redis adapter performs one `GETRANGE` for a non-empty result 및, for an ambiguous empty result, re-runs bounded `GETRANGE` plus `EXISTS` inside one `MULTI`/`EXEC` transaction. The fake keeps read behavior injectable while clear 테스트 make unexpected reads panic. Tests must assert non-empty hit uses one command, zero-length hit/miss returns bytes 및 existence from one atomic recheck, payload length `MaxValueBytes+1` is rejected 전에 unmarshal, marshal happens 전에 Redis, 및 invalid input/cancellation causes 없음 command. Inject a bounded malformed payload 및 a serializer 오류 containing payload/key/address markers; assert exactly one unmarshal, `ReasonInvalidPayload`, inspectable 원인 identity, 및 없음 `DEL`, loader fallback, L1 population, 또는 other mutation. 검증 the outer 오류 string omits the raw payload, logical/physical key, provider address, 및 serializer message while `errors.Is`/`errors.As` can still reach causes. The 기존 oversize-marshal row must prove rejection 전에 Redis dispatch. 작업 8 adds the joined provider/local-cleanup redaction assertion 후 both 오류 paths exist.

추가 `TestValueCacheDifferentKeySerializerCallsProceedConcurrently` 함께 a concurrency-safe latch serializer. Start two `Set` calls for different keys 및 require both `Marshal` invocations to signal entry 전에 releasing either; then preload two bounded payloads, start two different-key `Get` calls, 및 require both `Unmarshal` invocations to enter 전에 release. 검증 모든 four operations succeed 및 exact call counts are two. This deterministic 테스트 fails if `ValueCache` adds a 패키지-global serializer mutex; the 작업 10 race run proves the latch serializer 및 built-in serializer remain race-free.

사용 this exact 공개 constructor shape:

```go
type ValueOptions[V any] struct {
	Client     *redis.Client
	Namespace  string
	Serializer serialization.Serializer[V]
	Config     *ValueConfig
}

func NewValueCache[V any](options ValueOptions[V]) (*ValueCache[V], error)
```

```go
type fakeCommandClient struct {
	getRange func(context.Context, string, int64, int64) *redis.StringCmd
	exists   func(context.Context, ...string) *redis.IntCmd
	set      func(context.Context, string, any, time.Duration) *redis.StatusCmd
	del      func(context.Context, ...string) *redis.IntCmd
	scan     func(context.Context, uint64, string, int64) *redis.ScanCmd
	unlink   func(context.Context, ...string) *redis.IntCmd
}

func TestValueCacheGetUsesBoundedRead(t *testing.T) {
	var end int64
	var existsCalls int
	client := &fakeCommandClient{
		getRange: func(_ context.Context, _ string, _, gotEnd int64) *redis.StringCmd {
			end = gotEnd
			return redis.NewStringResult(`{"name":"keyboard"}`, nil)
		},
		exists: func(context.Context, ...string) *redis.IntCmd {
			existsCalls++
			return redis.NewIntResult(1, nil)
		},
	}
	c := unitValueCache[testValue](client, serialization.NewJSONSerializer[testValue](), ValueConfig{RemoteTTL: time.Hour, MaxValueBytes: 32, ClearBatchSize: 100})
	got, err := c.Get(context.Background(), "sku:42")
	if err != nil || got.Name != "keyboard" || end != 32 || existsCalls != 0 {
		t.Fatalf("get = %+v, %v; end=%d exists=%d", got, err, end, existsCalls)
	}
}
```

- [ ] **단계 2: 실행 focused 테스트 및 verify RED**

```bash
go test -count=1 ./cache/redisvalue -run '^TestValueCache'
```

예상: FAIL because L2 methods are absent.

- [ ] **단계 3: 구현 `ValueCache` methods**

사용 this internal structure 및 method order:

```go
type commandClient interface {
	GetRange(context.Context, string, int64, int64) *redis.StringCmd
	Exists(context.Context, ...string) *redis.IntCmd
	Set(context.Context, string, any, time.Duration) *redis.StatusCmd
	Del(context.Context, ...string) *redis.IntCmd
	Scan(context.Context, uint64, string, int64) *redis.ScanCmd
	Unlink(context.Context, ...string) *redis.IntCmd
}

type ValueCache[V any] struct {
	client     commandClient
	serializer serialization.Serializer[V]
	keys       btredis.KeyBuilder
	namespace  string
	config     ValueConfig
}
```

`NewValueCache` validates the concrete client, rejects nil 및 typed-nil serializers 함께 the 공유 interface-nil helper, copies either `*options.Config` 또는 `DefaultConfig().Value`, validates the namespace, creates `btredis.NewKeyBuilder("bluetape:cache:value")`, appends the namespace 함께 `Structural`, 및 stores the resulting builder. It retains but never closes 또는 mutates the 호출자's client 또는 serializer.

`Get` must issue `GETRANGE 0 MaxValueBytes`; for a zero-length result it must atomically re-read `GETRANGE` 및 `EXISTS` inside `MULTI`/`EXEC`, reject length `> MaxValueBytes`, 및 call `Unmarshal` exactly once. `Set` must validate, marshal once, reject oversized bytes, recheck context, normalize TTL, invoke Redis, 및 preserve commit ambiguity on every non-nil post-invocation command 오류. `SetDefault` delegates to `Set` 함께 the copied default. `Delete` validates 전에 invocation, is idempotent, 및 treats any post-invocation 오류 as commit-unknown.

```go
func (c *ValueCache[V]) SetDefault(ctx context.Context, key string, value V) error {
	if err := c.validateInitialized("set-default"); err != nil { return err }
	return c.Set(ctx, key, value, c.config.RemoteTTL)
}

func (c *ValueCache[V]) Get(ctx context.Context, logicalKey string) (V, error) {
	var zero V
	ctx = normalizeContext(ctx)
	if err := c.validateInitialized("get"); err != nil { return zero, err }
	if err := ctx.Err(); err != nil { return zero, err }
	key, err := c.key(logicalKey)
	if err != nil { return zero, err }
	encoded, err := c.client.GetRange(ctx, key.Value, 0, int64(c.config.MaxValueBytes)).Bytes()
	if errors.Is(err, redis.Nil) { return zero, cache.ErrCacheMiss }
	if err != nil { return zero, c.readProviderError("get", key.RedactedID, err) }
	if len(encoded) == 0 {
		exists, existsErr := c.client.Exists(ctx, key.Value).Result()
		if existsErr != nil { return zero, c.readProviderError("get", key.RedactedID, existsErr) }
		if exists == 0 { return zero, cache.ErrCacheMiss }
	}
	if len(encoded) > c.config.MaxValueBytes { return zero, c.cacheError("get", ReasonPayloadTooLarge, key.RedactedID, nil) }
	if err := ctx.Err(); err != nil { return zero, err }
	value, err := c.serializer.Unmarshal(encoded)
	if err != nil { return zero, c.cacheError("get", ReasonInvalidPayload, key.RedactedID, err) }
	return value, nil
}
```

All 오류 helpers accept 만 `btredis.Key.RedactedID`; raw `Key.Value` is restricted to Redis command arguments.

- [ ] **단계 4: 증명 the implemented zero-value methods are safe**

Test the methods introduced so far; 작업 3 adds the final interface assertion 및 `Clear` zero-value row 후 `Clear` exists:

```go
func TestValueCacheZeroValueReturnsUninitialized(t *testing.T) {
	var c ValueCache[string]
	if _, err := c.Get(context.Background(), "key"); !hasReason(err, ReasonUninitialized) { t.Fatalf("get = %v", err) }
	if err := c.Set(context.Background(), "key", "value", 0); !hasReason(err, ReasonUninitialized) { t.Fatalf("set = %v", err) }
	if err := c.SetDefault(context.Background(), "key", "value"); !hasReason(err, ReasonUninitialized) { t.Fatalf("set default = %v", err) }
	if err := c.Delete(context.Background(), "key"); !hasReason(err, ReasonUninitialized) { t.Fatalf("delete = %v", err) }
}
```

- [ ] **단계 5: 실행 테스트 및 commit**

```bash
gofmt -w cache/redisvalue/*.go
go test -count=1 ./cache/redisvalue -run '^TestValueCache'
```

예상: PASS.

```bash
git add cache/redisvalue
git commit -m "Store bounded serialized values in Redis" \
  -m "Constraint: L2 reads must reject oversize before deserialization" \
  -m "Rejected: full GET | it materializes unbounded remote data" \
  -m "Confidence: high" -m "Scope-risk: moderate" \
  -m "Tested: focused ValueCache unit tests"
```

### 작업 3: 추가 Streamed Namespace Clear 및 Partial Progress

**파일:**
- 생성: `cache/redisvalue/clear.go`
- Test: `cache/redisvalue/clear_test.go`

- [ ] **단계 1: Write failing page/chunk/오류 테스트**

Script fake `SCAN` pages larger than `ClearBatchSize`. 검증 the exact pattern `bluetape:cache:value:<namespace>:*`, `COUNT` hint, sequential `UNLINK` chunks, cursor-zero retry semantics, cancellation between chunks, 및 partial progress without raw keys.

```go
func TestValueCacheClearStreamsAndRechunksPages(t *testing.T) {
	client := newClearFake(
		scanPage{cursor: 7, keys: []string{"k1", "k2", "k3", "k4", "k5"}},
		scanPage{cursor: 0, keys: []string{"k6"}},
	)
	c := unitValueCache[string](client, serialization.StringSerializer{}, ValueConfig{RemoteTTL: time.Hour, MaxValueBytes: 32, ClearBatchSize: 2})
	if err := c.Clear(context.Background()); err != nil { t.Fatal(err) }
	if diff := cmp.Diff([][]string{{"k1", "k2"}, {"k3", "k4"}, {"k5"}, {"k6"}}, client.unlinked); diff != "" {
		t.Fatalf("unlink chunks (-want +got):\n%s", diff)
	}
	if client.pattern != "bluetape:cache:value:catalog:*" || client.count != 2 {
		t.Fatalf("scan = %q/%d", client.pattern, client.count)
	}
}
```

사용 the repository's 기존 `github.com/google/go-cmp/cmp` dependency for the nested-slice assertion; add 없음 dependency.

- [ ] **단계 2: 검증 RED**

```bash
go test -count=1 ./cache/redisvalue -run '^TestValueCacheClear'
```

예상: FAIL because clear is unimplemented.

- [ ] **단계 3: 구현 the cursor loop**

```go
func (c *ValueCache[V]) Clear(ctx context.Context) error {
	ctx = normalizeContext(ctx)
	if err := c.validateInitialized("clear"); err != nil { return err }
	if err := ctx.Err(); err != nil { return err }
	pattern := "bluetape:cache:value:" + c.namespace + ":*"
	progress := ClearProgress{}
	var cursor uint64
	for {
		keys, next, err := c.client.Scan(ctx, cursor, pattern, c.config.ClearBatchSize).Result()
		if err != nil { return c.partialClearError(progress, err) }
		progress.ScannedKeys += int64(len(keys))
		for start := 0; start < len(keys); start += int(c.config.ClearBatchSize) {
			if err := ctx.Err(); err != nil { return c.partialClearError(progress, err) }
			end := min(start+int(c.config.ClearBatchSize), len(keys))
			if err := c.client.Unlink(ctx, keys[start:end]...).Err(); err != nil { return c.partialClearError(progress, err) }
			progress.UnlinkedBatches++
		}
		cursor = next
		if cursor == 0 { return nil }
	}
}
```

Never call `FLUSHDB`, `KEYS`, blocking `DEL`, 또는 pipeline commands.

- [ ] **단계 4: 실행 clear 테스트 및 commit**

추가 the final zero-value row 및 compile-time 계약 now that every `ValueCache` method exists:

```go
var _ cache.Cache[string, testValue] = (*ValueCache[testValue])(nil)

func TestValueCacheClearZeroValueReturnsUninitialized(t *testing.T) {
	var c ValueCache[string]
	if err := c.Clear(context.Background()); !hasReason(err, ReasonUninitialized) { t.Fatalf("clear = %v", err) }
}
```

```bash
gofmt -w cache/redisvalue/*.go
go test -count=1 ./cache/redisvalue -run '^TestValueCacheClear'
```

예상: PASS.

```bash
git add cache/redisvalue
git commit -m "Clear value namespaces without blocking Redis" \
  -m "Constraint: SCAN count is a hint and returned pages must be re-chunked" \
  -m "Rejected: FLUSHDB and DEL fallback | both violate the administrative boundary" \
  -m "Confidence: high" -m "Scope-risk: moderate" \
  -m "Tested: streamed clear and partial-progress tests"
```

### 작업 4: 구성 the ABA-Safe Same-Key Coordinator 및 Load Flights

**파일:**
- 생성: `cache/redisvalue/coordination.go`
- Test: `cache/redisvalue/coordination_test.go`

- [ ] **단계 1: Write deterministic token, flight, cancellation, 및 retirement 테스트**

Cover one same-key leader, 기존 fol낮음ers sharing success/오류, fol낮음er-만 cancellation, leader cancellation 전에 token acquisition, publication/cancellation arbitration, constant-size flight state, active registry returning to zero, 및 final-retirement ABA against a new lookup.

```go
func TestCoordinatorRetirementDoesNotCreateTwoTokenDomains(t *testing.T) {
	registry := newCoordinatorRegistry[string]()
	first := registry.acquire("shared")
	retireEntered := make(chan struct{})
	retireRelease := make(chan struct{})
	registry.beforeRetire = func() { close(retireEntered); <-retireRelease }
	done := make(chan struct{})
	go func() { registry.release("shared", first); close(done) }()
	<-retireEntered
	nextReady := make(chan *keyCoordinator[string], 1)
	go func() { nextReady <- registry.acquire("shared") }()
	close(retireRelease)
	<-done
	next := <-nextReady
	if first == next { t.Fatal("retired coordinator was reused") }
	if registry.active() != 1 { t.Fatalf("active coordinators = %d", registry.active()) }
	registry.release("shared", next)
	if registry.active() != 0 { t.Fatalf("registry retained %d coordinators", registry.active()) }
}
```

- [ ] **단계 2: 검증 RED**

```bash
go test -count=1 ./cache/redisvalue -run '^TestCoordinator'
```

예상: FAIL because coordinator types do 아님 exist.

- [ ] **단계 3: 구현 registry, token, 및 flight records**

사용 a registry mutex to serialize lookup/install/retain 및 identity-checked retirement. Maintain the lock order `registry -> coordinator`; never request registry retirement while holding the coordinator mutex.

```go
type loadFlight[V any] struct {
	generation   uint64
	done         chan struct{}
	value        V
	err          error
	published    bool
	participants atomic.Int64
}

type keyCoordinator[V any] struct {
	mu          sync.Mutex
	token       chan struct{}
	flight      *loadFlight[V]
	nextFlight  uint64
	externalRef int64
	tokenUsers  int64
}

type coordinatorRegistry[V any] struct {
	mu           sync.Mutex
	items        map[string]*keyCoordinator[V]
	beforeRetire func()
}

func newKeyCoordinator[V any]() *keyCoordinator[V] {
	c := &keyCoordinator[V]{token: make(chan struct{}, 1)}
	c.token <- struct{}{}
	return c
}

func (c *keyCoordinator[V]) acquireToken(ctx context.Context) error {
	select {
	case <-ctx.Done(): return ctx.Err()
	case <-c.token: return nil
	}
}

func (c *keyCoordinator[V]) releaseToken() { c.token <- struct{}{} }
```

Flight publication 및 fol낮음er cancellation must both lock `keyCoordinator.mu`; whichever records first determines the fol낮음er result 및 releases its participant once. The leader retains its participant while waiting for healthy-origin repair, releases its token, then reacquires it 및 restarts inside the same flight.

- [ ] **단계 4: 실행 coordinator 테스트 repeatedly 및 commit**

```bash
go test -count=20 ./cache/redisvalue -run '^TestCoordinator'
go test -race -count=5 ./cache/redisvalue -run '^TestCoordinator'
```

예상: PASS 함께 registry count zero 후 every 테스트.

```bash
git add cache/redisvalue
git commit -m "Coordinate same-key loads without retained waiters" \
  -m "Constraint: follower cancellation and registry retirement require exact linearization" \
  -m "Rejected: x/sync/singleflight | its context and participant policy is incompatible" \
  -m "Confidence: high" -m "Scope-risk: broad" \
  -m "Tested: repeated coordinator and race tests"
```

### 작업 5: 구현 the Local-State Barrier, Tickets, 및 Repair Epochs

**파일:**
- 생성: `cache/redisvalue/local_state.go`
- Test: `cache/redisvalue/local_state_test.go`

- [ ] **단계 1: Write the full state/admission matrix as latch 테스트**

Test healthy/current admission, healthy/new-generation terminal handling, context-sensitive wait during repairing-from-healthy without a lease/token, fail-closed blocking/blocked/repairing-from-blocked, cancellation leaving state unchanged, one-shot ticket admission, total repair budget, 및 stale repair owners failing to heal a newer block.

```go
func TestLocalStateRepairEpochPreventsStaleHealing(t *testing.T) {
	state := newLocalState()
	first, err := state.beginRepair(context.Background(), repairExplicit)
	if err != nil { t.Fatal(err) }
	state.block()
	if state.finishRepair(first, nil) { t.Fatal("stale repair healed newer block") }
	if state.phaseValue() != phaseBlocked { t.Fatalf("phase = %v", state.phaseValue()) }
	second, err := state.beginRepair(context.Background(), repairExplicit)
	if err != nil { t.Fatal(err) }
	if !state.finishRepair(second, nil) || state.phaseValue() != phaseHealthy {
		t.Fatalf("explicit repair did not heal: %v", state.phaseValue())
	}
}
```

- [ ] **단계 2: 검증 RED**

```bash
go test -count=1 ./cache/redisvalue -run '^TestLocalState'
```

예상: FAIL because the barrier is absent.

- [ ] **단계 3: 구현 phases, leases, tickets, 및 epochs**

```go
type localPhase uint8
const (
	phaseHealthy localPhase = iota
	phaseBlocking
	phaseBlocked
	phaseRepairing
)

type repairOrigin uint8
const (
	repairFromHealthy repairOrigin = iota
	repairFromBlocked
)

type localState struct {
	mu         sync.Mutex
	phase      localPhase
	origin     repairOrigin
	generation uint64
	active     int64
	epoch      uint64
	changed    chan struct{}
}

type localLease struct { state *localState; generation uint64; released bool }
type sideEffectTicket struct { generation uint64 }
type repairLease struct { epoch uint64; origin repairOrigin }
```

Every phase change closes `changed` 및 installs a new channel. Healthy lease admission increments `active`; release decrements it 및 broadcasts when zero. A ticket may be issued 만 while the lease generation is current 및 phase is healthy. Repair admission increments generation, denies new leases, waits for `active == 0` within the single 호출자 budget, 및 records whether admission observed healthy 또는 blocked. Only an explicit repair admitted from blocked may heal; mandatory cleanup admitted from blocked preserves blocked. `block` increments generation 및 publishes blocking/blocked without waiting for uncooperative old leases.

- [ ] **단계 4: 증명 allocation-free healthy admission 및 commit**

```go
func TestHealthyLeaseAdmissionAllocatesNothing(t *testing.T) {
	state := newLocalState()
	allocs := testing.AllocsPerRun(1000, func() {
		lease, err := state.acquireHealthy(context.Background())
		if err != nil { panic(err) }
		lease.release()
	})
	if allocs != 0 { t.Fatalf("allocations = %f", allocs) }
}
```

실행:

```bash
go test -count=20 ./cache/redisvalue -run '^(TestLocalState|TestHealthyLease)'
go test -race -count=5 ./cache/redisvalue -run '^TestLocalState'
```

예상: PASS.

```bash
git add cache/redisvalue
git commit -m "Fence local cache state with repair epochs" \
  -m "Constraint: no lease may span Redis I/O or caller loaders" \
  -m "Rejected: check-then-dispatch state checks | transitions can cross the invocation gap" \
  -m "Confidence: high" -m "Scope-risk: broad" \
  -m "Tested: repeated state-machine, allocation, and race tests"
```

### 작업 6: 추가 Tiered Reads, Reference Semantics, 및 Local TTL Population

**파일:**
- 생성: `cache/redisvalue/tiered_cache.go`
- Modify: `cache/redisvalue/ttl.go`
- Test: `cache/redisvalue/tiered_cache_test.go`
- Modify: `cache/redisvalue/ttl_test.go`

- [ ] **단계 1: Write failing L1/L2/reference 테스트**

Introduce `TieredOptions[V]`, `TieredCache[V]`, 및 `NewTieredCache` here. Constructor 테스트 reject nil/typed-nil `Local`, nil 및 uninitialized `Remote`, invalid copied `TieredConfig`, 및 `LocalTTL > Remote.config.RemoteTTL` when the remote default is positive. 검증 healthy L1 hits perform 없음 serializer/Redis call 및 preserve pointer identity; L2 hits unmarshal once 및 store the same decoded `V` in L1; separate cold decorators deserialize distinct pointers; 만 `cache.ErrCacheMiss` falls through; L1 오류 및 Redis/serialization 오류 do 아님 become misses.

```go
type TieredOptions[V any] struct {
	Local  cache.Cache[string, V]
	Remote *ValueCache[V]
	Config *TieredConfig
}

func NewTieredCache[V any](options TieredOptions[V]) (*TieredCache[V], error)
```

```go
func TestTieredCacheL2HitStoresDecodedReference(t *testing.T) {
	local := cache.NewMemory[string, *testValue]()
	remote, serializer := unitRemotePointerCache(t, &testValue{Name: "remote"})
	tiered := mustTieredCache(t, local, remote, nil)

	first, err := tiered.Get(context.Background(), "item")
	if err != nil { t.Fatal(err) }
	second, err := tiered.Get(context.Background(), "item")
	if err != nil { t.Fatal(err) }
	if first != second { t.Fatal("L1 did not preserve pointer identity") }
	if serializer.unmarshalCalls.Load() != 1 { t.Fatalf("unmarshal calls = %d", serializer.unmarshalCalls.Load()) }
}
```

Name the warmed dependency-count 테스트 `TestTieredCacheHealthyL1SkipsRemoteAndSerializer`; it asserts pointer identity, zero serializer calls, zero Redis calls, zero local-state allocations, 및 없음 coordinator creation. Name the Redis-first mutation reference 테스트 in 작업 8 `TestTieredCacheSetPreservesReference`.

추가 `TestTieredCacheDifferentKeyL1HitsDoNotSerialize`: a channel-latched fake L1 blocks `Get("s낮음")`, `Get("fast")` must complete 전에 the s낮음 latch is released, both return direct L1 values, 및 `coordinators.active()` remains zero. This 테스트 fails if a 패키지-global 또는 cross-key lock serializes healthy reads. 작업 7 adds the corresponding different-key loader proof 후 `GetOrLoad` exists.

- [ ] **단계 2: 검증 RED**

```bash
go test -count=1 ./cache/redisvalue -run '^TestTieredCache(Constructor|Get|Stores|Pointer|L1|L2|Healthy|DifferentKeyL1)'
```

예상: FAIL because decorator reads are absent.

- [ ] **단계 3: 구현 constructor 및 `Get`**

정의 `TieredCache[V]` 함께 copied config, exclusive L1 reference, remote pointer, coordinator registry, 및 local state. `NewTieredCache` rejects invalid dependencies 전에 reading the remote TTL 또는 creating coordinator/state objects, copies the selected tiered config, 및 validates its relationship to the remote's copied default. Final `cache.LoadingCache` compatibility is asserted in 작업 8 후 every required method exists.

`Get` order is: validate/context/key -> initial healthy lease/L1 -> miss 만 -> same-key coordinator/token -> healthy recheck 및 L2 ticket -> release lease -> L2 read -> generation/state post-check -> L1 `Set` using `LocalTTL` -> post-check -> return. A failed `Local.Set` must call token-held mandatory local delete 전에 returning.

```go
func (c *TieredCache[V]) Get(ctx context.Context, key string) (V, error) {
	var zero V
	ctx = normalizeContext(ctx)
	if err := c.validateCall("get", ctx, key); err != nil { return zero, err }
	if value, hit, err := c.localGet(ctx, key); err != nil || hit { return value, err }
	return c.getRemoteCoordinated(ctx, key)
}
```

유지 the full sequencing in private helpers named `localGet`, `getRemoteCoordinated`, `populateLocalHeld`, 및 `invalidateLocalHeld`; none may reacquire a token already held by its 호출자.

- [ ] **단계 4: 추가 elapsed known-write TTL helper 테스트**

Test zero, finite, sub-millisecond minimum, fractional truncation, elapsed subtraction, non-positive remainder, 및 delayed `Local.Set`. 사용 an injected monotonic clock private to 테스트; do 아님 expose a clock publicly.

- [ ] **단계 5: 실행 focused 테스트 및 commit**

```bash
gofmt -w cache/redisvalue/*.go
go test -count=1 ./cache/redisvalue -run '^TestTieredCache(Constructor|Get|Stores|Pointer|L1|L2|Healthy|DifferentKeyL1|TTL)'
```

예상: PASS.

```bash
git add cache/redisvalue
git commit -m "Read Redis through a reference-preserving L1" \
  -m "Constraint: only L2 may serialize values" \
  -m "Rejected: cloning L1 values | pointer identity is part of the approved process-local contract" \
  -m "Confidence: high" -m "Scope-risk: broad" \
  -m "Tested: tiered read, reference, error, and TTL tests"
```

### 작업 7: 구현 Collapsed `GetOrLoad` Flights

**파일:**
- Modify: `cache/redisvalue/tiered_cache.go`
- Modify: `cache/redisvalue/coordination.go`
- Test: `cache/redisvalue/tiered_cache_test.go`
- Test: `cache/redisvalue/coordination_test.go`

- [ ] **단계 1: Write failing leader/fol낮음er load 테스트**

Test L1 -> L2 -> loader order, nil loader rejection 전에 coordinator creation, exact one loader call for success 및 오류 waves, first-leader TTL/loader/context ownership, fol낮음er-만 cancellation, leader cancellation publication, healthy-origin repair wait retaining leadership, 없음 write 후 cancellation, 및 coordinator retirement.

```go
func TestGetOrLoadSharesOneLeaderResult(t *testing.T) {
	tiered := newMissTieredCache[string](t)
	entered := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int64
	loader := func(context.Context, string) (string, error) {
		if calls.Add(1) == 1 { close(entered) }
		<-release
		return "loaded", nil
	}
	results := make(chan string, 16)
	for range 16 {
		go func() { value, err := tiered.GetOrLoad(context.Background(), "shared", time.Minute, loader); if err != nil { results <- err.Error(); return }; results <- value }()
	}
	<-entered
	close(release)
	for range 16 { if got := <-results; got != "loaded" { t.Fatalf("result = %q", got) } }
	if calls.Load() != 1 || tiered.coordinators.active() != 0 { t.Fatalf("calls/active = %d/%d", calls.Load(), tiered.coordinators.active()) }
}
```

추가 `TestGetOrLoadDifferentKeysProceedConcurrently`: two loaders for distinct keys both signal entry 전에 either release latch closes, then both complete 함께 one call each 및 the coordinator registry returns to zero. The 테스트 fails if load coordination introduces a 패키지-global 또는 cross-key lock.

- [ ] **단계 2: 검증 RED**

```bash
go test -count=1 ./cache/redisvalue -run '^TestGetOrLoad'
```

예상: FAIL because the load method is absent.

- [ ] **단계 3: 구현 `GetOrLoad` 및 default delegation**

Validate nil loader 및 TTL 전에 registry lookup. The leader owns the active flight 및 operation token, rechecks L1, admits one L2 read, admits the loader 만 후 an exact miss, admits the L2 write 만 후 loader success/current healthy state, then populates L1 함께 the known-write adjusted TTL. Existing fol낮음ers use the leader's loader, TTL, 및 context; their own cancellation detaches 만 them.

```go
func (c *TieredCache[V]) GetOrLoadDefault(ctx context.Context, key string, loader cache.Loader[string, V]) (V, error) {
	if c == nil || c.remote == nil { var zero V; return zero, newCacheError("get-or-load-default", ReasonUninitialized, "", nil) }
	return c.GetOrLoad(ctx, key, c.remote.config.RemoteTTL, loader)
}
```

After result publication, detach the active flight 및 release the leader participant while still holding the operation token, then release the token. Fol낮음ers release participants 후 receiving 또는 canceling. If healthy-origin repair begins while the leader waits, release the key token, retain flight leadership/participant, wait without a local lease, reacquire the token, 및 restart L1/L2 inside the same flight.

- [ ] **단계 4: 실행 repeated 및 race 테스트 및 commit**

```bash
go test -count=20 ./cache/redisvalue -run '^TestGetOrLoad'
go test -race -count=5 ./cache/redisvalue -run '^(TestGetOrLoad|TestCoordinator)'
```

예상: PASS 함께 exact loader counts 및 zero retained coordinators.

```bash
git add cache/redisvalue
git commit -m "Collapse tiered cache loads by key" \
  -m "Constraint: one flight shares the first leader context, TTL, loader, result, and error" \
  -m "Rejected: follower retries inside an active flight | they can multiply failed loader calls" \
  -m "Confidence: high" -m "Scope-risk: broad" \
  -m "Tested: repeated load-flight and race tests"
```

### 작업 8: 구현 Redis-First Mutations, Invalidation, Cleanup, 및 Recovery

**파일:**
- Modify: `cache/redisvalue/tiered_cache.go`
- Modify: `cache/redisvalue/local_state.go`
- Test: `cache/redisvalue/tiered_cache_test.go`
- Test: `cache/redisvalue/local_state_test.go`

- [ ] **단계 1: Write failing mutation 및 blocked-state 테스트**

Cover Redis-first `Set`, original-reference L1 population, same-key mutation order, commit-unknown cleanup, known success plus later cancellation, L1-set failure fol낮음ed by token-held delete, `Delete`, `InvalidateLocal`, `ClearLocal`, `TieredCache.Clear`, joined 오류, cleanup timeout, blocked fail-closed operations, explicit repair, blocked administrative clear, 및 every method on a zero-value `TieredCache` returning `ReasonUninitialized` without panic. The joined provider/local-cleanup 테스트 injects raw payload/key/address/원인 markers 및 asserts every 공개 joined 오류 string remains redacted while `errors.Is`/`errors.As` preserves both causes.

```go
func TestFailedMandatoryCleanupBlocksUntilClearLocal(t *testing.T) {
	local := &faultLocal[string]{deleteErr: errors.New("delete failed")}
	tiered := newTieredWithLocal(t, local)
	err := tiered.Set(context.Background(), "item", "value", time.Minute)
	if !hasReason(err, ReasonLocalBlocked) { t.Fatalf("set error = %v", err) }
	if _, err := tiered.Get(context.Background(), "item"); !hasReason(err, ReasonLocalBlocked) { t.Fatalf("blocked get = %v", err) }
	local.deleteErr = nil
	local.clearErr = nil
	if err := tiered.ClearLocal(context.Background()); err != nil { t.Fatal(err) }
	if tiered.localState.phaseValue() != phaseHealthy { t.Fatalf("phase = %v", tiered.localState.phaseValue()) }
}
```

- [ ] **단계 2: 검증 RED**

```bash
go test -count=1 ./cache/redisvalue -run '^(TestTieredCache(Set|Delete|Clear|ZeroValue)|TestInvalidateLocal|TestClearLocal|TestFailedMandatory)'
```

예상: FAIL because mutation/recovery methods are incomplete.

- [ ] **단계 3: 구현 공개 mutation 및 invalidation methods**

`Set` 및 `Delete` acquire the same-key token, obtain a one-shot L2 ticket at the captured generation, invoke Redis, 및 always run mandatory token-held L1 invalidation 후 an invoked ambiguous mutation. Known successful `Set` populates the original `V`; known successful `Delete` removes L1. A failed local population invokes `invalidateLocalHeld` without reacquiring the token.

```go
func (c *TieredCache[V]) SetDefault(ctx context.Context, key string, value V) error {
	if c == nil || c.remote == nil { return newCacheError("set-default", ReasonUninitialized, "", nil) }
	return c.Set(ctx, key, value, c.remote.config.RemoteTTL)
}

func (c *TieredCache[V]) InvalidateLocal(ctx context.Context, key string) error {
	ctx = normalizeContext(ctx)
	if err := c.validateCall("invalidate-local", ctx, key); err != nil { return err }
	waitCtx, cancel := context.WithTimeout(ctx, c.config.InvalidationWaitTimeout)
	defer cancel()
	coordinator := c.coordinators.acquire(key)
	defer c.coordinators.release(key, coordinator)
	if err := coordinator.acquireToken(waitCtx); err != nil { c.localState.block(); return c.localBlockedError("invalidate-local", key, err) }
	defer coordinator.releaseToken()
	cleanupCtx, cleanupCancel := context.WithTimeout(waitCtx, c.config.LocalCleanupTimeout)
	defer cleanupCancel()
	return c.invalidateLocalHeld(cleanupCtx, key)
}
```

Public `InvalidateLocal` passes the child cleanup context to `invalidateLocalHeld`, so token wait plus maintenance admission/delete cannot exceed the 호출자/`InvalidationWaitTimeout` budget 및 the local phase uses at most `LocalCleanupTimeout`. 추가 a latch 테스트 함께 `InvalidationWaitTimeout > LocalCleanupTimeout` that acquires the token immediately, stalls maintenance delete, observes the shorter cleanup deadline, 및 proves blocked state. Mandatory post-mutation cleanup instead creates `context.WithTimeout(context.Background(), LocalCleanupTimeout)` 전에 calling the same held helper, because safety cleanup must survive 호출자 cancellation. `ClearLocal` uses one `context.WithTimeout(ctx, LocalCleanupTimeout)` budget across explicit repair admission, lease drain, 및 `Local.Clear`, 및 heals 만 if its epoch still owns the state. Neither 공개 invalidation method calls Redis.

- [ ] **단계 4: 구현 tiered clear 및 fleet-scope 테스트**

`TieredCache.Clear` invokes remote clear first 및, once started, always attempts mandatory full local cleanup 함께 the owned timeout. Cleanup origin is captured at local repair admission. A concurrent successful `ClearLocal` during remote scanning wins; an older outer call cannot re-block it. Two decorators sharing L2 must prove that admin/peer clear leaves the other L1 및 blocked state untouched until explicit `ClearLocal` 또는 expiry.

After `Get`, `Set`, `Delete`, `Clear`, 및 `GetOrLoad` 모든 exist, add the final compile-time 계약:

```go
var _ cache.LoadingCache[string, testValue] = (*TieredCache[testValue])(nil)
```

- [ ] **단계 5: 실행 repeated mutation/recovery 테스트 및 commit**

```bash
go test -count=20 ./cache/redisvalue -run '^(TestTieredCache(Set|Delete|Clear|ZeroValue)|TestInvalidateLocal|TestClearLocal|TestFailedMandatory)'
go test -race -count=5 ./cache/redisvalue -run '^(TestTieredCache|TestInvalidateLocal|TestClearLocal)'
```

예상: PASS.

```bash
git add cache/redisvalue
git commit -m "Fail closed when local tier cleanup is uncertain" \
  -m "Constraint: Redis mutation outcomes may be unknown after invocation" \
  -m "Rejected: serving stale L1 after cleanup failure | invalidation was not proved" \
  -m "Confidence: high" -m "Scope-risk: broad" \
  -m "Directive: only explicit ClearLocal may heal blocked state" \
  -m "Tested: repeated mutation, cleanup, repair, and race tests"
```

### 작업 9: 증명 Redis Integration, ACLs, TTL, Clear, 및 Pointer Isolation

**파일:**
- 생성: `cache/redisvalue/integration_test.go`

- [ ] **단계 1: 추가 one sequential Redis 7.4 Testcontainers suite**

Start one container 함께 `testcontainers/redis.Start`, create 호출자-owned clients, register cleanup, 및 prove `PING` readiness 함께 `testing.Eventually`. 추가 sequential subtests for:

- get/set/delete/miss;
- finite expiration 및 zero persistence;
- sub-millisecond wire minimum 및 known-write local TTL adjustment;
- empty payload versus missing key;
- deterministic two-client create interleaving 후 the first empty `GETRANGE`, proving the transactional re-read never fabricates an empty value;
- maximum payload 및 oversize rejection;
- multi-page namespace isolation/clear;
- two decorators sharing L2 bytes but 아님 pointer identity;
- mixed-version `VersionedSerializer` matrix: a version-2 reader accepts version-1 bytes, a version-1 reader rejects version-2 bytes 함께 `serialization.ErrUnsupportedVersion`, 및 namespace reuse remains prohibited unless the deployment's 호출자-owned serializer matrix proves both upgrade 및 rollback readers;
- least-privilege ordinary identity 함께 `GETRANGE`, `EXISTS`, `MULTI`, `EXEC`, `SET`, 및 `DEL`, plus a separate clear-admin identity;
- `+SCAN` foreign-prefix enumeration, denied foreign `GET`/`UNLINK`, 및 denied `FLUSHDB`/`FLUSHALL`.

```go
func TestRedisValueIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	t.Cleanup(cancel)
	addr := redistestcontainer.Start(ctx, t)
	client := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { _ = client.Close() })
	bttesting.Eventually(t, 5*time.Second, func() bool { return client.Ping(ctx).Err() == nil })

	t.Run("ttl-and-zero-persistence", func(t *testing.T) {
		values := mustIntegrationValueCache[testValue](t, client, "ttl")
		if err := values.Set(ctx, "finite", testValue{Name: "finite"}, 20*time.Millisecond); err != nil { t.Fatal(err) }
		bttesting.Eventually(t, 3*time.Second, func() bool { _, err := values.Get(ctx, "finite"); return errors.Is(err, cache.ErrCacheMiss) })
		if err := values.Set(ctx, "persistent", testValue{Name: "persistent"}, 0); err != nil { t.Fatal(err) }
		if ttl, err := client.PTTL(ctx, physicalKey(t, values, "persistent")).Result(); err != nil || ttl != -1 { t.Fatalf("PTTL = %v, %v", ttl, err) }
	})
}
```

- [ ] **단계 2: 실행 serial integration 테스트**

```bash
TESTCONTAINERS_REUSE_ENABLE=false TESTCONTAINERS_RYUK_DISABLED=false \
  go test -p 1 -count=1 ./cache/redisvalue -run '^TestRedisValueIntegration$'
```

예상: PASS. If Docker is unavailable, record the exact infrastructure 오류 및 do 아님 reinterpret it as a code failure.

- [ ] **단계 3: 실행 integration 테스트 under the race detector 및 commit**

```bash
TESTCONTAINERS_REUSE_ENABLE=false TESTCONTAINERS_RYUK_DISABLED=false \
  go test -race -p 1 -count=1 ./cache/redisvalue -run '^TestRedisValueIntegration$'
```

예상: PASS.

```bash
git add cache/redisvalue/integration_test.go
git commit -m "Prove redisvalue behavior against Redis 7.4" \
  -m "Constraint: Docker-backed provider tests run serially" \
  -m "Rejected: logical Redis databases as ACL isolation | ACLs are not database-scoped" \
  -m "Confidence: high" -m "Scope-risk: moderate" \
  -m "Tested: Redis 7.4 integration and race tests"
```

### 작업 10: 추가 Adversarial Concurrency, Stress, 및 Allocation 증거

**파일:**
- 생성: `cache/redisvalue/race_test.go`
- Modify: `cache/redisvalue/coordination_test.go`
- Modify: `cache/redisvalue/local_state_test.go`
- Modify: `cache/redisvalue/tiered_cache_test.go`

- [ ] **단계 1: 추가 deterministic pause-point races**

사용 channels, 아님 sleeps, to pause 후 ticket admission 및 전에 loader/`SET`/`DEL`; during L2 read/refill versus `Set`, `Delete`, `InvalidateLocal`, 및 `ClearLocal`; 후 generation capture; during blocked `Clear` remote phase versus explicit repair; 및 during final coordinator retirement. 검증 없음 usable late L1 write 및 document accepted concurrent L2 survivors 후 namespace scan position.

- [ ] **단계 2: 추가 bounded stress 및 registry-zero assertions**

사용 `testing/concurrency.GoroutineStressTester` 함께 explicit workers, rounds, 및 timeout. 구성 the stress cache 함께 the stateless `serialization.JSONSerializer[int]` (optionally wrapped by `VersionedSerializer`) 및 없음 테스트-만 codec mutex, so the full `-race` run exercises concurrent marshal/unmarshal. 실행 mixed `Get`, `GetOrLoad`, `Set`, `Delete`, `ClearLocal`, 및 cancellation; 후 each wave assert loader totals, 없음 data races, `coordinators.active() == 0`, 및 없음 blocked state unless the 테스트 injected cleanup failure.

```go
func TestTieredCacheMixedStressRetiresState(t *testing.T) {
	tiered := newStressTieredCache[int](t)
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{Workers: 16, RoundsPerTask: 100, Timeout: 10 * time.Second})
	var sequence atomic.Uint64
	report, err := tester.Run(context.Background(), func(ctx context.Context) error {
		round := int(sequence.Add(1) - 1)
		key := strconv.Itoa(round % 8)
		switch round % 6 {
		case 0:
			_, err := tiered.Get(ctx, key)
			if err != nil && !errors.Is(err, cache.ErrCacheMiss) { return err }
			return nil
		case 1:
			_, err := tiered.GetOrLoad(ctx, key, time.Minute, func(context.Context, string) (int, error) { return round, nil })
			return err
		case 2:
			return tiered.Set(ctx, key, round, time.Minute)
		case 3:
			return tiered.Delete(ctx, key)
		case 4:
			return tiered.ClearLocal(ctx)
		default:
			canceled, cancel := context.WithCancel(ctx)
			cancel()
			_, err := tiered.Get(canceled, key)
			if !errors.Is(err, context.Canceled) { return fmt.Errorf("canceled get: %w", err) }
			return nil
		}
	})
	if err != nil || report.Completed != 100 { t.Fatalf("stress report = %+v, %v", report, err) }
	if tiered.coordinators.active() != 0 { t.Fatalf("active coordinators = %d", tiered.coordinators.active()) }
}
```

- [ ] **단계 3: 증명 the healthy L1 path allocation claim**

Warm one L1 key, replace Redis 및 serializer 함께 panic-on-call fakes, 및 require `testing.AllocsPerRun(1000, func(){ _, _ = tiered.Get(ctx, "hot") }) == 0` 후 excluding 테스트 closure setup.

- [ ] **단계 4: 실행 repeated/race/stress evidence 및 commit**

```bash
go test -count=20 ./cache/redisvalue -run '^(TestTieredCache.*Race|TestCoordinator.*ABA|TestLocalState.*Epoch)'
go test -race -count=1 ./cache/redisvalue
```

예상: PASS 함께 없음 retained coordinators/flights 및 없음 race reports.

```bash
git add cache/redisvalue
git commit -m "Prove tiered cache linearization under hostile races" \
  -m "Constraint: deterministic latches must replace timing-dependent sleeps" \
  -m "Rejected: namespace clear as a snapshot | concurrent L2 survivors are permitted" \
  -m "Confidence: high" -m "Scope-risk: broad" \
  -m "Tested: repeated latch, stress, allocation, and race tests"
```

### 작업 11: 공개 Examples, Bilingual Documentation, Indexes, 및 the Type A Lesson

**파일:**
- 생성: `cache/redisvalue/example_test.go`
- 생성: `cache/redisvalue/documentation_test.go`
- 생성: `cache/redisvalue/README.md`
- 생성: `cache/redisvalue/README.ko.md`
- Modify: `README.md`
- Modify: `README.ko.md`
- Modify: `CHANGELOG.md`
- 생성: `docs/lessons/2026-07-18-issue-535-redis-tiered-value-cache.md`

- [ ] **단계 1: 추가 compile-checked 공개 example**

Examples must show 호출자-owned direct `*redis.Client`, `DefaultConfig` copy/override, `VersionedSerializer`, `ValueCache`, `TieredCache` 함께 `cache.NewMemory`, default 및 per-entry TTL, immutable pointer snapshot guidance, `ClearLocal` versus namespace `Clear`, 및 fail-closed 오류 handling. 다음을 하지 않는다: instantiate `redis.NewFailoverClient` as supported.

```go
func ExampleNewTieredCache() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379", DialTimeout: 2 * time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second})
	defer func() { _ = client.Close() }()

	serializer, err := serialization.NewVersionedSerializer(serialization.NewJSONSerializer[*exampleValue](), 1)
	if err != nil { return }
	config := redisvalue.DefaultConfig()
	config.Value.RemoteTTL = 10 * time.Minute
	config.Tiered.LocalTTL = time.Minute
	remote, err := redisvalue.NewValueCache(redisvalue.ValueOptions[*exampleValue]{Client: client, Namespace: "catalog", Serializer: serializer, Config: &config.Value})
	if err != nil { return }
	tiered, err := redisvalue.NewTieredCache(redisvalue.TieredOptions[*exampleValue]{Local: cache.NewMemory[string, *exampleValue](), Remote: remote, Config: &config.Tiered})
	if err != nil { return }
	value, err := tiered.GetOrLoadDefault(ctx, "sku:42", func(context.Context, string) (*exampleValue, error) { return &exampleValue{Name: "keyboard"}, nil })
	if err != nil { return }
	_ = value
}
```

- [ ] **단계 2: Write synchronized 영문 및 한국어 READMEs**

Both files must contain the same heading structure 및 facts:

1. L1 stores `V`/references; 만 L2 serializes.
2. Defaults 및 per-cache override table.
3. L1 ownership 및 pointer immutability.
4. L1 -> L2 -> first-leader loader policy.
5. TTL semantics, unknown 기존 L2 expiry window, 및 zero TTL.
6. Strict Redis-first 오류, commit-unknown, cancellation, blocked recovery.
7. `ClearLocal`, `InvalidateLocal`, L2-만 admin clear, non-atomic `SCAN`/`UNLINK` cost, 및 the fleet reset runbook.
8. Stable direct-primary requirement; failover/proxy/cluster/ring unsupported.
9. ACL/TLS/readiness/memory/timeout/telemetry/partial-clear operations.
10. `VersionedSerializer`, namespace rotation, rollout, rollback, 및 zero-TTL cleanup.
11. Why current Pub/Sub `redisnear` must 아님 wrap this decorator, `rediscoord` ownership, 및 #536 RESP3 proof gate.
12. Serial normal/race 테스트 commands.
13. Redis bytes are untrusted; serializers must avoid executable deserialization, return 오류 rather than panic for malformed input, 및 own temporary allocations, nesting/recursion, decompression, 및 CPU limits because `MaxValueBytes` bounds Redis admission bytes rather than decoder work.
14. Tamper-sensitive deployments wrap payloads in an authenticated envelope in addition to `VersionedSerializer`; built-in versioning detects compatibility mismatches, 아님 malicious modification.
15. A namespace is one exclusive tenant/schema/clear trust domain 및 아님 an authorization boundary; incompatible tenants 또는 wire formats require separate namespaces 및 Redis ACL/network isolation.
16. `SCAN COUNT` is a hint: the client retains one Redis-controlled page 및 one bounded `UNLINK` argument chunk, but cannot bound the byte size of a returned page 또는 external keys.
17. The serializer is 호출자-owned, immutable 후 construction, 및 safe for concurrent marshal/unmarshal; the 패키지 does 아님 clone it 또는 serialize calls behind a global lock.
18. The built-in versioned envelope is backward-readable 만 (`v2` reads `v1`, `v1` rejects `v2`); namespace reuse across an upgrade/rollback window is prohibited until the application proves its exact bidirectional serializer matrix.

Place ordered `<!-- redisvalue-contract: <id> -->` markers 전에 the 18 synchronized facts in both READMEs. 추가 an executable parity 테스트:

```go
func TestReadmeContractMarkersStayInParity(t *testing.T) {
	expected := []string{
		"l1-boundary", "config", "ownership", "load-policy", "ttl", "errors",
		"clear", "topology", "operations", "versioning", "resp3", "tests",
		"untrusted-payload", "authentication", "namespace", "scan-bounds",
		"serializer-concurrency", "compatibility-matrix",
	}
	marker := regexp.MustCompile(`<!-- redisvalue-contract: ([a-z0-9-]+) -->`)
	heading := regexp.MustCompile(`(?m)^(#{1,3}) `)
	var headingLevels []int
	for _, path := range []string{"README.md", "README.ko.md"} {
		data, err := os.ReadFile(path)
		if err != nil { t.Fatal(err) }
		matches := marker.FindAllStringSubmatch(string(data), -1)
		got := make([]string, 0, len(matches))
		for _, match := range matches { got = append(got, match[1]) }
		if !slices.Equal(got, expected) { t.Fatalf("%s markers = %v, want %v", path, got, expected) }
		headings := heading.FindAllStringSubmatch(string(data), -1)
		levels := make([]int, 0, len(headings))
		for _, match := range headings { levels = append(levels, len(match[1])) }
		if headingLevels == nil { headingLevels = levels } else if !slices.Equal(levels, headingLevels) { t.Fatalf("%s heading levels = %v, want %v", path, levels, headingLevels) }
	}
}
```

- [ ] **단계 3: 업데이트 root indexes 및 changelog**

추가 `cache/redisvalue` next to `cache/redisfory` in both 패키지 tables 및 cache-category lists. 추가 one `Unreleased / Added` changelog bullet describing the generic Redis L2 plus reference-preserving tiered decorator 및 explicitly excluding RESP3 coherence.

- [ ] **단계 4: Write the Type A lesson**

사용 this structure 함께 evidence from the final implementation:

```markdown
# Lessons Learned - Redis Tiered Value Cache (#535)

**Related issue:** #535
**Affected package:** `cache/redisvalue`

## L1: A local reference cache and a serialized remote cache need different boundaries

### Problem
Serializing or cloning values before every L1 write would make the local tier a second serialization boundary. Pointer-valued callers would receive a different object after refill, and healthy L1 hits would pay work that belongs only to the remote tier.

### Decision
`TieredCache[V]` stores `V` directly in its exclusively owned L1. Only `ValueCache[V]` invokes `serialization.Serializer[V]`. Callers that choose pointer-valued `V` treat cached objects as immutable snapshots while cached.

### Evidence
`TestTieredCacheSetPreservesReference`, `TestTieredCacheHealthyL1SkipsRemoteAndSerializer`, `TestTieredCacheL2HitStoresDecodedReference`, `TestTieredCacheMixedStressRetiresState`, and `TestRedisValueIntegration/pointer-isolation` prove the boundary at unit, stress, race, and real-Redis levels.

### Future Guard
Future RESP3 work calls only `InvalidateLocal` or `ClearLocal`; it never routes invalidation events through `Set`, `Delete`, or `Clear`, because those methods mutate L2.
```

유지 these exact 테스트 names when adding the 테스트 in Tasks 6, 9, 및 10. 다음을 하지 않는다: claim benchmark results owned by #560.

- [ ] **단계 5: Compile example, verify locale parity, 및 commit**

```bash
go test -count=1 ./cache/redisvalue -run '^(Example|TestReadmeContractMarkersStayInParity)'
git diff --check
```

예상: PASS.

```bash
git add cache/redisvalue README.md README.ko.md CHANGELOG.md docs/lessons/2026-07-18-issue-535-redis-tiered-value-cache.md
git commit -m "Document the redisvalue ownership boundary" \
  -m "Constraint: public behavior requires synchronized English and Korean documentation" \
  -m "Rejected: marketing the decorator as RESP3-coherent | issue #536 remains the proof gate" \
  -m "Confidence: high" -m "Scope-risk: moderate" \
  -m "Tested: compile-checked examples, locale structure scan, and diff check"
```

### 작업 12: 실행 Full 검증 및 Complete the Pre-PR 리뷰 Gate

**파일:**
- Modify 만 if verification finds a defect: files already introduced by Tasks 1-11
- 생성: `docs/superpowers/reviews/2026-07-18-issue-535-redis-tiered-value-cache-step-6r-code-review.md`

- [ ] **단계 1: 실행 focused verification first**

```bash
TESTCONTAINERS_REUSE_ENABLE=false TESTCONTAINERS_RYUK_DISABLED=false \
  go test -p 1 -count=1 ./cache/redisvalue
TESTCONTAINERS_REUSE_ENABLE=false TESTCONTAINERS_RYUK_DISABLED=false \
  go test -race -p 1 -count=1 ./cache/redisvalue
```

예상: PASS.

- [ ] **단계 2: 실행 static 및 repository gates**

```bash
make fmt-check
make tidy-check
make vet
make lint
TESTCONTAINERS_REUSE_ENABLE=false TESTCONTAINERS_RYUK_DISABLED=false make test
TESTCONTAINERS_REUSE_ENABLE=false TESTCONTAINERS_RYUK_DISABLED=false make race
TESTCONTAINERS_REUSE_ENABLE=false TESTCONTAINERS_RYUK_DISABLED=false make ci
git diff --check origin/develop...HEAD
```

예상: every command exits 0. `make ci` repeats earlier gates intentionally as the repository completion proof.

- [ ] **단계 3: 실행 단계 6-R six-lane review 및 main integration**

리뷰 the exact implementation HEAD independently from 성능, 안정성, 보안, 운영자/Ops, 개발자/API, 및 사용자/호출자 perspectives. The main session integrates findings 및 requires `P0=0/P1=0`. 기록 exact SHA, commands, lane counts, accepted P2s, 및 any main-session timeout fallback in the review artifact.

- [ ] **단계 4: Fix every P0/P1 및 rerun affected plus full gates**

For each repair, add 또는 tighten a regression 테스트 first, prove RED against the defect, implement the smallest fix, prove GREEN, commit 함께 Lore trailers, then rerun 모든 six perspectives against the new exact HEAD. 다음을 하지 않는다: declare convergence from reviews of different substantive commits.

- [ ] **단계 5: 커밋 review evidence 및 stop at the PR authority gate**

```bash
git add docs/superpowers/reviews/2026-07-18-issue-535-redis-tiered-value-cache-step-6r-code-review.md
git commit -m "Record redisvalue implementation convergence" \
  -m "Constraint: pre-PR review requires six perspectives on one exact implementation head" \
  -m "Confidence: high" -m "Scope-risk: narrow" \
  -m "Tested: targeted, race, stress, Testcontainers, examples, static gates, and make ci"
```

Report the exact branch/HEAD 및 merge-ready local evidence. 다음을 하지 않는다: push, create a PR, enable auto-merge, merge, tag, publish, 또는 delete the worktree without the next explicit authority gate.

## 롤백 Strategy

Before publication, revert the implementation commits in reverse task order; 없음 기존 패키지, Redis key family, interface, 또는 dependency is changed. After adoption, roll back callers to their prior cache path while retaining the old namespace for at least the maximum finite TTL plus margin. Incompatible serializer changes rotate the namespace; zero-TTL namespaces require an explicit stable-primary admin clear. A rollback never treats Redis logical databases as tenant isolation 및 never connects the unsupported failover/proxy topology to `ValueCache.Clear`.
