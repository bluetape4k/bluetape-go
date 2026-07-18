# Redis Tiered Value Cache Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `cache/redisvalue`, a bounded serialized Redis L2 provider and a `TieredCache[V]` decorator whose L1 stores `V` directly and whose default configuration can be overridden per cache.

**Architecture:** `ValueCache[V]` owns key construction, TTL normalization, serialization, bounded Redis commands, and namespace clear. `TieredCache[V]` composes one exclusively transferred `cache.Cache[string,V]` L1 with that provider, using an ABA-safe same-key coordinator and a process-local state barrier for invalidation, cleanup, and repair. RESP3 tracking remains outside this package and issue #536 may use only the L1-only invalidation methods.

**Tech Stack:** Go 1.26.3, `github.com/redis/go-redis/v9`, existing `cache` and `serialization` contracts, Redis 7.4 Testcontainers, standard-library synchronization, `testing.AllocsPerRun`, and repository Make targets.

---

## File Map

Create these focused files:

- `cache/redisvalue/doc.go` — package-level behavior and RESP3 boundary.
- `cache/redisvalue/config.go` — public config defaults, validation, and copied constructor options.
- `cache/redisvalue/errors.go` — stable reasons, redacted `CacheError`, clear progress, provider wrapping.
- `cache/redisvalue/ttl.go` — zero/sub-millisecond-safe TTL validation and go-redis wire normalization.
- `cache/redisvalue/value_cache.go` — bounded `Get`, Redis-first `Set`, `SetDefault`, and idempotent `Delete`.
- `cache/redisvalue/clear.go` — streamed `SCAN MATCH` and sequential bounded `UNLINK`.
- `cache/redisvalue/coordination.go` — registry, operation token, load flights, participants, ABA-safe retirement.
- `cache/redisvalue/local_state.go` — healthy/blocking/blocked/repairing barrier, leases, tickets, repair epochs.
- `cache/redisvalue/tiered_cache.go` — public decorator reads, loads, mutations, cleanup, and invalidation.
- `cache/redisvalue/*_test.go` — same-package unit, deterministic concurrency, race, allocation, and integration evidence.
- `cache/redisvalue/documentation_test.go` — executable English/Korean contract-marker parity.
- `cache/redisvalue/example_test.go` — compile-checked public API examples.
- `cache/redisvalue/README.md`, `cache/redisvalue/README.ko.md` — synchronized caller and operator contract.
- `docs/lessons/2026-07-18-issue-535-redis-tiered-value-cache.md` — Type A reusable lesson.

Modify only these existing public indexes:

- `README.md`
- `README.ko.md`
- `CHANGELOG.md`

Do not change `Makefile`, `cache.Cache`, `cache.LoadingCache`, `cache/redisnear`, `cache/rediscoord`, or `redis.ValidateTTL`. Issue #560 owns benchmark-matrix publication and issue #536 owns RESP3 tracking.

## Spec Coverage Matrix

| Acceptance criterion | Implementation and proof |
|---|---|
| 1. Constructor-only, zero-value-safe caches | Tasks 2, 6, and 8 constructor/zero-value tests |
| 2. Caller-owned client and concurrency-safe serializer-backed `ValueCache` | Tasks 2-3 unit tests and Task 9 Redis integration/race evidence |
| 3. Direct-`V` L1 and L2-only serialization | Task 6 reference/serializer-count tests and Task 9 pointer-isolation integration |
| 4. Copied defaults and per-cache overrides | Task 1 config-copy tests and Task 11 examples/docs |
| 5. Allocation-free healthy L1 hit | Task 6 hot-path dependency assertions and Task 10 `AllocsPerRun` proof |
| 6. L1 -> L2 -> loader with ABA-safe collapse | Tasks 4, 6, and 7 coordinator/flight tests |
| 7. Finite/zero TTL and known-write adjustment | Tasks 1, 6, and 9 TTL tests |
| 8. Bounded/redacted reads, writes, clear, inputs, and errors | Tasks 1-3 unit tests plus Task 9 ACL/clear integration |
| 9. Commit-unknown and token-held cleanup | Tasks 2 and 8 mutation/error tests |
| 10. Same-key linearization, tickets, state, and generations | Tasks 4-8 deterministic tests and Task 10 adversarial races |
| 11. One cleanup budget and newest-repair epoch | Tasks 5 and 8 state/repair tests |
| 12. L1-only invalidation and fleet reset boundary | Task 8 multi-decorator tests and Task 11 operator docs |
| 13. Synchronous direct-primary public surface | Task 2 constructor types and Task 11 topology docs |
| 14. Unsupported namespace/L1 sharing | Task 1 namespace validation and Task 11 ownership docs |
| 15. Unit, integration, race, stress, and examples | Tasks 1-10 targeted evidence and Task 12 full gates |
| 16. Synchronized English/Korean docs and RESP3 distinction | Task 11 locale-parity check and root/package docs |

### Task 1: Lock Configuration, Key, TTL, and Error Contracts

**Files:**
- Create: `cache/redisvalue/doc.go`
- Create: `cache/redisvalue/config.go`
- Create: `cache/redisvalue/errors.go`
- Create: `cache/redisvalue/ttl.go`
- Test: `cache/redisvalue/config_test.go`
- Test: `cache/redisvalue/errors_test.go`
- Test: `cache/redisvalue/ttl_test.go`

- [ ] **Step 1: Write failing configuration, key, TTL, and error tests**

Add table-driven tests that assert independent default copies, every configuration bound, namespace/key limits, TTL normalization, redaction, and shared sentinel identity. This task deliberately defines no cache type or constructor, so its GREEN checkpoint does not depend on later coordinator/local-state work.

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

Add explicit `errors.Is` assertions:

```go
func TestInputValidationPreservesRedisSentinels(t *testing.T) {
	if err := validateNamespace("tenant*"); !errors.Is(err, btredis.ErrInvalidKey) { t.Fatalf("namespace = %v", err) }
	if err := validateLogicalKey(" "); !errors.Is(err, btredis.ErrInvalidKey) { t.Fatalf("logical key = %v", err) }
	if err := validateEntryTTL(-time.Nanosecond); !errors.Is(err, btredis.ErrInvalidTTL) { t.Fatalf("ttl = %v", err) }
}
```

- [ ] **Step 2: Run the tests and verify RED**

Run:

```bash
go test -count=1 ./cache/redisvalue -run 'Test(DefaultConfig|Config|InputValidation|TTL|CacheError)'
```

Expected: FAIL because package/types are not defined.

- [ ] **Step 3: Implement exact public configuration**

Add the exported configuration from the approved spec:

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

Add table rows that call `Config.Validate` directly for every lower/upper bound and for `LocalTTL > RemoteTTL`; assert `errors.Is(err, btredis.ErrInvalidTTL)` for TTL relationship failures. Tasks 2 and 6 repeat the relevant validation against copied constructor inputs.

Validation must enforce the approved ranges. Do not call `redis.ValidateTTL`; it rejects the zero and positive sub-millisecond TTLs required here. Use:

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

Use `reflect.Value.IsNil` only for interface dependencies. Validate namespace as exactly one `[A-Za-z0-9._-]+` segment of at most 128 bytes and logical keys as non-blank values of at most 1024 bytes before calling `redis.KeyBuilder`.

- [ ] **Step 4: Implement inspectable redacted errors**

Define every approved reason and accessor without formatting the causal error:

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

Provider failures must be `CacheError -> redis.OpError -> original cause`; dispatched mutation failures additionally join `redis.ErrCommitUnknown`. Unit tests must prove raw keys, serialized bytes, provider messages, and addresses are absent from `Error()`.

- [ ] **Step 5: Run focused tests and commit**

Run:

```bash
gofmt -w cache/redisvalue/*.go
go test -count=1 ./cache/redisvalue -run 'Test(DefaultConfig|Config|Constructor|Input|TTL|CacheError)'
```

Expected: PASS.

Commit with Lore trailers:

```bash
git add cache/redisvalue
git commit -m "Define the redisvalue safety contract" \
  -m "Constraint: zero and sub-millisecond TTLs are valid cache inputs" \
  -m "Rejected: redis.ValidateTTL reuse | its minimum TTL contract is incompatible" \
  -m "Confidence: high" -m "Scope-risk: moderate" \
  -m "Tested: focused redisvalue config, TTL, key, and error tests"
```

### Task 2: Implement the Bounded Serialized Redis L2 Provider

**Files:**
- Create: `cache/redisvalue/value_cache.go`
- Test: `cache/redisvalue/value_cache_test.go`

- [ ] **Step 1: Write failing bounded read/write tests with a narrow fake**

Add `ValueOptions[V]`, `ValueCache[V]`, and `NewValueCache` constructor tests here, where the concrete type is first introduced. Assert nil client, nil/typed-nil serializer, invalid namespace, copied `ValueConfig`, zero-value method safety, and shared input sentinels. The serializer remains caller-owned, immutable after construction, and safe for concurrent `Marshal`/`Unmarshal`; the package does not add a global codec lock.

Define one package-internal six-command interface containing `GetRange`, `Exists`, `Set`, `Del`, `Scan`, and `Unlink`. The fake implements all six methods; Task 2 configures the first four closures and makes unexpected `Scan`/`Unlink` calls panic, while Task 3 configures the clear closures. Tests must assert non-empty hit uses one command, zero-length hit/miss uses `EXISTS`, payload length `MaxValueBytes+1` is rejected before unmarshal, marshal happens before Redis, and invalid input/cancellation causes no command. Inject a bounded malformed payload and a serializer error containing payload/key/address markers; assert exactly one unmarshal, `ReasonInvalidPayload`, inspectable cause identity, and no `DEL`, loader fallback, L1 population, or other mutation. Assert the outer error string omits the raw payload, logical/physical key, provider address, and serializer message while `errors.Is`/`errors.As` can still reach causes. The existing oversize-marshal row must prove rejection before Redis dispatch. Task 8 adds the joined provider/local-cleanup redaction assertion after both error paths exist.

Use this exact public constructor shape:

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

- [ ] **Step 2: Run focused tests and verify RED**

```bash
go test -count=1 ./cache/redisvalue -run '^TestValueCache'
```

Expected: FAIL because L2 methods are absent.

- [ ] **Step 3: Implement `ValueCache` methods**

Use this internal structure and method order:

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

`NewValueCache` validates the concrete client, rejects nil and typed-nil serializers with the shared interface-nil helper, copies either `*options.Config` or `DefaultConfig().Value`, validates the namespace, creates `btredis.NewKeyBuilder("bluetape:cache:value")`, appends the namespace with `Structural`, and stores the resulting builder. It retains but never closes or mutates the caller's client or serializer.

`Get` must issue `GETRANGE 0 MaxValueBytes`, conditionally issue `EXISTS` only for a zero-length result, reject length `> MaxValueBytes`, and call `Unmarshal` exactly once. `Set` must validate, marshal once, reject oversized bytes, recheck context, normalize TTL, invoke Redis, and preserve commit ambiguity on every non-nil post-invocation command error. `SetDefault` delegates to `Set` with the copied default. `Delete` validates before invocation, is idempotent, and treats any post-invocation error as commit-unknown.

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

All error helpers accept only `btredis.Key.RedactedID`; raw `Key.Value` is restricted to Redis command arguments.

- [ ] **Step 4: Prove the implemented zero-value methods are safe**

Test the methods introduced so far; Task 3 adds the final interface assertion and `Clear` zero-value row after `Clear` exists:

```go
func TestValueCacheZeroValueReturnsUninitialized(t *testing.T) {
	var c ValueCache[string]
	if _, err := c.Get(context.Background(), "key"); !hasReason(err, ReasonUninitialized) { t.Fatalf("get = %v", err) }
	if err := c.Set(context.Background(), "key", "value", 0); !hasReason(err, ReasonUninitialized) { t.Fatalf("set = %v", err) }
	if err := c.SetDefault(context.Background(), "key", "value"); !hasReason(err, ReasonUninitialized) { t.Fatalf("set default = %v", err) }
	if err := c.Delete(context.Background(), "key"); !hasReason(err, ReasonUninitialized) { t.Fatalf("delete = %v", err) }
}
```

- [ ] **Step 5: Run tests and commit**

```bash
gofmt -w cache/redisvalue/*.go
go test -count=1 ./cache/redisvalue -run '^TestValueCache'
```

Expected: PASS.

```bash
git add cache/redisvalue
git commit -m "Store bounded serialized values in Redis" \
  -m "Constraint: L2 reads must reject oversize before deserialization" \
  -m "Rejected: full GET | it materializes unbounded remote data" \
  -m "Confidence: high" -m "Scope-risk: moderate" \
  -m "Tested: focused ValueCache unit tests"
```

### Task 3: Add Streamed Namespace Clear and Partial Progress

**Files:**
- Create: `cache/redisvalue/clear.go`
- Test: `cache/redisvalue/clear_test.go`

- [ ] **Step 1: Write failing page/chunk/error tests**

Script fake `SCAN` pages larger than `ClearBatchSize`. Assert the exact pattern `bluetape:cache:value:<namespace>:*`, `COUNT` hint, sequential `UNLINK` chunks, cursor-zero retry semantics, cancellation between chunks, and partial progress without raw keys.

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

Use the repository's existing `github.com/google/go-cmp/cmp` dependency for the nested-slice assertion; add no dependency.

- [ ] **Step 2: Verify RED**

```bash
go test -count=1 ./cache/redisvalue -run '^TestValueCacheClear'
```

Expected: FAIL because clear is unimplemented.

- [ ] **Step 3: Implement the cursor loop**

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

Never call `FLUSHDB`, `KEYS`, blocking `DEL`, or pipeline commands.

- [ ] **Step 4: Run clear tests and commit**

Add the final zero-value row and compile-time contract now that every `ValueCache` method exists:

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

Expected: PASS.

```bash
git add cache/redisvalue
git commit -m "Clear value namespaces without blocking Redis" \
  -m "Constraint: SCAN count is a hint and returned pages must be re-chunked" \
  -m "Rejected: FLUSHDB and DEL fallback | both violate the administrative boundary" \
  -m "Confidence: high" -m "Scope-risk: moderate" \
  -m "Tested: streamed clear and partial-progress tests"
```

### Task 4: Build the ABA-Safe Same-Key Coordinator and Load Flights

**Files:**
- Create: `cache/redisvalue/coordination.go`
- Test: `cache/redisvalue/coordination_test.go`

- [ ] **Step 1: Write deterministic token, flight, cancellation, and retirement tests**

Cover one same-key leader, existing followers sharing success/error, follower-only cancellation, leader cancellation before token acquisition, publication/cancellation arbitration, constant-size flight state, active registry returning to zero, and final-retirement ABA against a new lookup.

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

- [ ] **Step 2: Verify RED**

```bash
go test -count=1 ./cache/redisvalue -run '^TestCoordinator'
```

Expected: FAIL because coordinator types do not exist.

- [ ] **Step 3: Implement registry, token, and flight records**

Use a registry mutex to serialize lookup/install/retain and identity-checked retirement. Maintain the lock order `registry -> coordinator`; never request registry retirement while holding the coordinator mutex.

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

Flight publication and follower cancellation must both lock `keyCoordinator.mu`; whichever records first determines the follower result and releases its participant once. The leader retains its participant while waiting for healthy-origin repair, releases its token, then reacquires it and restarts inside the same flight.

- [ ] **Step 4: Run coordinator tests repeatedly and commit**

```bash
go test -count=20 ./cache/redisvalue -run '^TestCoordinator'
go test -race -count=5 ./cache/redisvalue -run '^TestCoordinator'
```

Expected: PASS with registry count zero after every test.

```bash
git add cache/redisvalue
git commit -m "Coordinate same-key loads without retained waiters" \
  -m "Constraint: follower cancellation and registry retirement require exact linearization" \
  -m "Rejected: x/sync/singleflight | its context and participant policy is incompatible" \
  -m "Confidence: high" -m "Scope-risk: broad" \
  -m "Tested: repeated coordinator and race tests"
```

### Task 5: Implement the Local-State Barrier, Tickets, and Repair Epochs

**Files:**
- Create: `cache/redisvalue/local_state.go`
- Test: `cache/redisvalue/local_state_test.go`

- [ ] **Step 1: Write the full state/admission matrix as latch tests**

Test healthy/current admission, healthy/new-generation terminal handling, context-sensitive wait during repairing-from-healthy without a lease/token, fail-closed blocking/blocked/repairing-from-blocked, cancellation leaving state unchanged, one-shot ticket admission, total repair budget, and stale repair owners failing to heal a newer block.

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

- [ ] **Step 2: Verify RED**

```bash
go test -count=1 ./cache/redisvalue -run '^TestLocalState'
```

Expected: FAIL because the barrier is absent.

- [ ] **Step 3: Implement phases, leases, tickets, and epochs**

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

Every phase change closes `changed` and installs a new channel. Healthy lease admission increments `active`; release decrements it and broadcasts when zero. A ticket may be issued only while the lease generation is current and phase is healthy. Repair admission increments generation, denies new leases, waits for `active == 0` within the single caller budget, and records whether admission observed healthy or blocked. Only an explicit repair admitted from blocked may heal; mandatory cleanup admitted from blocked preserves blocked. `block` increments generation and publishes blocking/blocked without waiting for uncooperative old leases.

- [ ] **Step 4: Prove allocation-free healthy admission and commit**

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

Run:

```bash
go test -count=20 ./cache/redisvalue -run '^(TestLocalState|TestHealthyLease)'
go test -race -count=5 ./cache/redisvalue -run '^TestLocalState'
```

Expected: PASS.

```bash
git add cache/redisvalue
git commit -m "Fence local cache state with repair epochs" \
  -m "Constraint: no lease may span Redis I/O or caller loaders" \
  -m "Rejected: check-then-dispatch state checks | transitions can cross the invocation gap" \
  -m "Confidence: high" -m "Scope-risk: broad" \
  -m "Tested: repeated state-machine, allocation, and race tests"
```

### Task 6: Add Tiered Reads, Reference Semantics, and Local TTL Population

**Files:**
- Create: `cache/redisvalue/tiered_cache.go`
- Modify: `cache/redisvalue/ttl.go`
- Test: `cache/redisvalue/tiered_cache_test.go`
- Modify: `cache/redisvalue/ttl_test.go`

- [ ] **Step 1: Write failing L1/L2/reference tests**

Introduce `TieredOptions[V]`, `TieredCache[V]`, and `NewTieredCache` here. Constructor tests reject nil/typed-nil `Local`, nil and uninitialized `Remote`, invalid copied `TieredConfig`, and `LocalTTL > Remote.config.RemoteTTL` when the remote default is positive. Assert healthy L1 hits perform no serializer/Redis call and preserve pointer identity; L2 hits unmarshal once and store the same decoded `V` in L1; separate cold decorators deserialize distinct pointers; only `cache.ErrCacheMiss` falls through; L1 errors and Redis/serialization errors do not become misses.

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

Name the warmed dependency-count test `TestTieredCacheHealthyL1SkipsRemoteAndSerializer`; it asserts pointer identity, zero serializer calls, zero Redis calls, zero local-state allocations, and no coordinator creation. Name the Redis-first mutation reference test in Task 8 `TestTieredCacheSetPreservesReference`.

Add `TestTieredCacheDifferentKeyL1HitsDoNotSerialize`: a channel-latched fake L1 blocks `Get("slow")`, `Get("fast")` must complete before the slow latch is released, both return direct L1 values, and `coordinators.active()` remains zero. This test fails if a package-global or cross-key lock serializes healthy reads. Task 7 adds the corresponding different-key loader proof after `GetOrLoad` exists.

- [ ] **Step 2: Verify RED**

```bash
go test -count=1 ./cache/redisvalue -run '^TestTieredCache(Get|Stores|Pointer|L1|L2|Healthy|DifferentKeyL1)'
```

Expected: FAIL because decorator reads are absent.

- [ ] **Step 3: Implement constructor and `Get`**

Define `TieredCache[V]` with copied config, exclusive L1 reference, remote pointer, coordinator registry, and local state. `NewTieredCache` rejects invalid dependencies before reading the remote TTL or creating coordinator/state objects, copies the selected tiered config, and validates its relationship to the remote's copied default. Final `cache.LoadingCache` compatibility is asserted in Task 8 after every required method exists.

`Get` order is: validate/context/key -> initial healthy lease/L1 -> miss only -> same-key coordinator/token -> healthy recheck and L2 ticket -> release lease -> L2 read -> generation/state post-check -> L1 `Set` using `LocalTTL` -> post-check -> return. A failed `Local.Set` must call token-held mandatory local delete before returning.

```go
func (c *TieredCache[V]) Get(ctx context.Context, key string) (V, error) {
	var zero V
	ctx = normalizeContext(ctx)
	if err := c.validateCall("get", ctx, key); err != nil { return zero, err }
	if value, hit, err := c.localGet(ctx, key); err != nil || hit { return value, err }
	return c.getRemoteCoordinated(ctx, key)
}
```

Keep the full sequencing in private helpers named `localGet`, `getRemoteCoordinated`, `populateLocalHeld`, and `invalidateLocalHeld`; none may reacquire a token already held by its caller.

- [ ] **Step 4: Add elapsed known-write TTL helper tests**

Test zero, finite, sub-millisecond minimum, fractional truncation, elapsed subtraction, non-positive remainder, and delayed `Local.Set`. Use an injected monotonic clock private to tests; do not expose a clock publicly.

- [ ] **Step 5: Run focused tests and commit**

```bash
gofmt -w cache/redisvalue/*.go
go test -count=1 ./cache/redisvalue -run '^TestTieredCache(Get|Stores|Pointer|L1|L2|Healthy|DifferentKeyL1|TTL)'
```

Expected: PASS.

```bash
git add cache/redisvalue
git commit -m "Read Redis through a reference-preserving L1" \
  -m "Constraint: only L2 may serialize values" \
  -m "Rejected: cloning L1 values | pointer identity is part of the approved process-local contract" \
  -m "Confidence: high" -m "Scope-risk: broad" \
  -m "Tested: tiered read, reference, error, and TTL tests"
```

### Task 7: Implement Collapsed `GetOrLoad` Flights

**Files:**
- Modify: `cache/redisvalue/tiered_cache.go`
- Modify: `cache/redisvalue/coordination.go`
- Test: `cache/redisvalue/tiered_cache_test.go`
- Test: `cache/redisvalue/coordination_test.go`

- [ ] **Step 1: Write failing leader/follower load tests**

Test L1 -> L2 -> loader order, nil loader rejection before coordinator creation, exact one loader call for success and error waves, first-leader TTL/loader/context ownership, follower-only cancellation, leader cancellation publication, healthy-origin repair wait retaining leadership, no write after cancellation, and coordinator retirement.

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

Add `TestGetOrLoadDifferentKeysProceedConcurrently`: two loaders for distinct keys both signal entry before either release latch closes, then both complete with one call each and the coordinator registry returns to zero. The test fails if load coordination introduces a package-global or cross-key lock.

- [ ] **Step 2: Verify RED**

```bash
go test -count=1 ./cache/redisvalue -run '^TestGetOrLoad'
```

Expected: FAIL because the load method is absent.

- [ ] **Step 3: Implement `GetOrLoad` and default delegation**

Validate nil loader and TTL before registry lookup. The leader owns the active flight and operation token, rechecks L1, admits one L2 read, admits the loader only after an exact miss, admits the L2 write only after loader success/current healthy state, then populates L1 with the known-write adjusted TTL. Existing followers use the leader's loader, TTL, and context; their own cancellation detaches only them.

```go
func (c *TieredCache[V]) GetOrLoadDefault(ctx context.Context, key string, loader cache.Loader[string, V]) (V, error) {
	if c == nil || c.remote == nil { var zero V; return zero, newCacheError("get-or-load-default", ReasonUninitialized, "", nil) }
	return c.GetOrLoad(ctx, key, c.remote.config.RemoteTTL, loader)
}
```

After result publication, detach the active flight and release the leader participant while still holding the operation token, then release the token. Followers release participants after receiving or canceling. If healthy-origin repair begins while the leader waits, release the key token, retain flight leadership/participant, wait without a local lease, reacquire the token, and restart L1/L2 inside the same flight.

- [ ] **Step 4: Run repeated and race tests and commit**

```bash
go test -count=20 ./cache/redisvalue -run '^TestGetOrLoad'
go test -race -count=5 ./cache/redisvalue -run '^(TestGetOrLoad|TestCoordinator)'
```

Expected: PASS with exact loader counts and zero retained coordinators.

```bash
git add cache/redisvalue
git commit -m "Collapse tiered cache loads by key" \
  -m "Constraint: one flight shares the first leader context, TTL, loader, result, and error" \
  -m "Rejected: follower retries inside an active flight | they can multiply failed loader calls" \
  -m "Confidence: high" -m "Scope-risk: broad" \
  -m "Tested: repeated load-flight and race tests"
```

### Task 8: Implement Redis-First Mutations, Invalidation, Cleanup, and Recovery

**Files:**
- Modify: `cache/redisvalue/tiered_cache.go`
- Modify: `cache/redisvalue/local_state.go`
- Test: `cache/redisvalue/tiered_cache_test.go`
- Test: `cache/redisvalue/local_state_test.go`

- [ ] **Step 1: Write failing mutation and blocked-state tests**

Cover Redis-first `Set`, original-reference L1 population, same-key mutation order, commit-unknown cleanup, known success plus later cancellation, L1-set failure followed by token-held delete, `Delete`, `InvalidateLocal`, `ClearLocal`, `TieredCache.Clear`, joined errors, cleanup timeout, blocked fail-closed operations, explicit repair, blocked administrative clear, and every method on a zero-value `TieredCache` returning `ReasonUninitialized` without panic. The joined provider/local-cleanup test injects raw payload/key/address/cause markers and asserts every public joined error string remains redacted while `errors.Is`/`errors.As` preserves both causes.

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

- [ ] **Step 2: Verify RED**

```bash
go test -count=1 ./cache/redisvalue -run '^(TestTieredCache(Set|Delete|Clear)|TestInvalidateLocal|TestClearLocal|TestFailedMandatory)'
```

Expected: FAIL because mutation/recovery methods are incomplete.

- [ ] **Step 3: Implement public mutation and invalidation methods**

`Set` and `Delete` acquire the same-key token, obtain a one-shot L2 ticket at the captured generation, invoke Redis, and always run mandatory token-held L1 invalidation after an invoked ambiguous mutation. Known successful `Set` populates the original `V`; known successful `Delete` removes L1. A failed local population invokes `invalidateLocalHeld` without reacquiring the token.

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

Public `InvalidateLocal` passes the child cleanup context to `invalidateLocalHeld`, so token wait plus maintenance admission/delete cannot exceed the caller/`InvalidationWaitTimeout` budget and the local phase uses at most `LocalCleanupTimeout`. Add a latch test with `InvalidationWaitTimeout > LocalCleanupTimeout` that acquires the token immediately, stalls maintenance delete, observes the shorter cleanup deadline, and proves blocked state. Mandatory post-mutation cleanup instead creates `context.WithTimeout(context.Background(), LocalCleanupTimeout)` before calling the same held helper, because safety cleanup must survive caller cancellation. `ClearLocal` uses one `context.WithTimeout(ctx, LocalCleanupTimeout)` budget across explicit repair admission, lease drain, and `Local.Clear`, and heals only if its epoch still owns the state. Neither public invalidation method calls Redis.

- [ ] **Step 4: Implement tiered clear and fleet-scope tests**

`TieredCache.Clear` invokes remote clear first and, once started, always attempts mandatory full local cleanup with the owned timeout. Cleanup origin is captured at local repair admission. A concurrent successful `ClearLocal` during remote scanning wins; an older outer call cannot re-block it. Two decorators sharing L2 must prove that admin/peer clear leaves the other L1 and blocked state untouched until explicit `ClearLocal` or expiry.

After `Get`, `Set`, `Delete`, `Clear`, and `GetOrLoad` all exist, add the final compile-time contract:

```go
var _ cache.LoadingCache[string, testValue] = (*TieredCache[testValue])(nil)
```

- [ ] **Step 5: Run repeated mutation/recovery tests and commit**

```bash
go test -count=20 ./cache/redisvalue -run '^(TestTieredCache(Set|Delete|Clear)|TestInvalidateLocal|TestClearLocal|TestFailedMandatory)'
go test -race -count=5 ./cache/redisvalue -run '^(TestTieredCache|TestInvalidateLocal|TestClearLocal)'
```

Expected: PASS.

```bash
git add cache/redisvalue
git commit -m "Fail closed when local tier cleanup is uncertain" \
  -m "Constraint: Redis mutation outcomes may be unknown after invocation" \
  -m "Rejected: serving stale L1 after cleanup failure | invalidation was not proved" \
  -m "Confidence: high" -m "Scope-risk: broad" \
  -m "Directive: only explicit ClearLocal may heal blocked state" \
  -m "Tested: repeated mutation, cleanup, repair, and race tests"
```

### Task 9: Prove Redis Integration, ACLs, TTL, Clear, and Pointer Isolation

**Files:**
- Create: `cache/redisvalue/integration_test.go`

- [ ] **Step 1: Add one sequential Redis 7.4 Testcontainers suite**

Start one container with `testcontainers/redis.Start`, create caller-owned clients, register cleanup, and prove `PING` readiness with `testing.Eventually`. Add sequential subtests for:

- get/set/delete/miss;
- finite expiration and zero persistence;
- sub-millisecond wire minimum and known-write local TTL adjustment;
- empty payload versus missing key;
- maximum payload and oversize rejection;
- multi-page namespace isolation/clear;
- two decorators sharing L2 bytes but not pointer identity;
- mixed-version `VersionedSerializer` matrix: a version-2 reader accepts version-1 bytes, a version-1 reader rejects version-2 bytes with `serialization.ErrUnsupportedVersion`, and namespace reuse remains prohibited unless the deployment's caller-owned serializer matrix proves both upgrade and rollback readers;
- least-privilege ordinary identity and separate clear-admin identity;
- `+SCAN` foreign-prefix enumeration, denied foreign `GET`/`UNLINK`, and denied `FLUSHDB`/`FLUSHALL`.

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

- [ ] **Step 2: Run serial integration tests**

```bash
TESTCONTAINERS_REUSE_ENABLE=false TESTCONTAINERS_RYUK_DISABLED=false \
  go test -p 1 -count=1 ./cache/redisvalue -run '^TestRedisValueIntegration$'
```

Expected: PASS. If Docker is unavailable, record the exact infrastructure error and do not reinterpret it as a code failure.

- [ ] **Step 3: Run integration tests under the race detector and commit**

```bash
TESTCONTAINERS_REUSE_ENABLE=false TESTCONTAINERS_RYUK_DISABLED=false \
  go test -race -p 1 -count=1 ./cache/redisvalue -run '^TestRedisValueIntegration$'
```

Expected: PASS.

```bash
git add cache/redisvalue/integration_test.go
git commit -m "Prove redisvalue behavior against Redis 7.4" \
  -m "Constraint: Docker-backed provider tests run serially" \
  -m "Rejected: logical Redis databases as ACL isolation | ACLs are not database-scoped" \
  -m "Confidence: high" -m "Scope-risk: moderate" \
  -m "Tested: Redis 7.4 integration and race tests"
```

### Task 10: Add Adversarial Concurrency, Stress, and Allocation Evidence

**Files:**
- Create: `cache/redisvalue/race_test.go`
- Modify: `cache/redisvalue/coordination_test.go`
- Modify: `cache/redisvalue/local_state_test.go`
- Modify: `cache/redisvalue/tiered_cache_test.go`

- [ ] **Step 1: Add deterministic pause-point races**

Use channels, not sleeps, to pause after ticket admission and before loader/`SET`/`DEL`; during L2 read/refill versus `Set`, `Delete`, `InvalidateLocal`, and `ClearLocal`; after generation capture; during blocked `Clear` remote phase versus explicit repair; and during final coordinator retirement. Assert no usable late L1 write and document accepted concurrent L2 survivors after namespace scan position.

- [ ] **Step 2: Add bounded stress and registry-zero assertions**

Use `testing/concurrency.GoroutineStressTester` with explicit workers, rounds, and timeout. Build the stress cache with the stateless `serialization.JSONSerializer[int]` (optionally wrapped by `VersionedSerializer`) and no test-only codec mutex, so the full `-race` run exercises concurrent marshal/unmarshal. Run mixed `Get`, `GetOrLoad`, `Set`, `Delete`, `ClearLocal`, and cancellation; after each wave assert loader totals, no data races, `coordinators.active() == 0`, and no blocked state unless the test injected cleanup failure.

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

- [ ] **Step 3: Prove the healthy L1 path allocation claim**

Warm one L1 key, replace Redis and serializer with panic-on-call fakes, and require `testing.AllocsPerRun(1000, func(){ _, _ = tiered.Get(ctx, "hot") }) == 0` after excluding test closure setup.

- [ ] **Step 4: Run repeated/race/stress evidence and commit**

```bash
go test -count=20 ./cache/redisvalue -run '^(TestTieredCache.*Race|TestCoordinator.*ABA|TestLocalState.*Epoch)'
go test -race -count=1 ./cache/redisvalue
```

Expected: PASS with no retained coordinators/flights and no race reports.

```bash
git add cache/redisvalue
git commit -m "Prove tiered cache linearization under hostile races" \
  -m "Constraint: deterministic latches must replace timing-dependent sleeps" \
  -m "Rejected: namespace clear as a snapshot | concurrent L2 survivors are permitted" \
  -m "Confidence: high" -m "Scope-risk: broad" \
  -m "Tested: repeated latch, stress, allocation, and race tests"
```

### Task 11: Publish Examples, Bilingual Documentation, Indexes, and the Type A Lesson

**Files:**
- Create: `cache/redisvalue/example_test.go`
- Create: `cache/redisvalue/documentation_test.go`
- Create: `cache/redisvalue/README.md`
- Create: `cache/redisvalue/README.ko.md`
- Modify: `README.md`
- Modify: `README.ko.md`
- Modify: `CHANGELOG.md`
- Create: `docs/lessons/2026-07-18-issue-535-redis-tiered-value-cache.md`

- [ ] **Step 1: Add compile-checked public examples**

Examples must show caller-owned direct `*redis.Client`, `DefaultConfig` copy/override, `VersionedSerializer`, `ValueCache`, `TieredCache` with `cache.NewMemory`, default and per-entry TTL, immutable pointer snapshot guidance, `ClearLocal` versus namespace `Clear`, and fail-closed error handling. Do not instantiate `redis.NewFailoverClient` as supported.

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

- [ ] **Step 2: Write synchronized English and Korean READMEs**

Both files must contain the same heading structure and facts:

1. L1 stores `V`/references; only L2 serializes.
2. Defaults and per-cache override table.
3. L1 ownership and pointer immutability.
4. L1 -> L2 -> first-leader loader policy.
5. TTL semantics, unknown existing L2 expiry window, and zero TTL.
6. Strict Redis-first errors, commit-unknown, cancellation, blocked recovery.
7. `ClearLocal`, `InvalidateLocal`, L2-only admin clear, non-atomic `SCAN`/`UNLINK` cost, and the fleet reset runbook.
8. Stable direct-primary requirement; failover/proxy/cluster/ring unsupported.
9. ACL/TLS/readiness/memory/timeout/telemetry/partial-clear operations.
10. `VersionedSerializer`, namespace rotation, rollout, rollback, and zero-TTL cleanup.
11. Why current Pub/Sub `redisnear` must not wrap this decorator, `rediscoord` ownership, and #536 RESP3 proof gate.
12. Serial normal/race test commands.
13. Redis bytes are untrusted; serializers must avoid executable deserialization, return errors rather than panic for malformed input, and own temporary allocations, nesting/recursion, decompression, and CPU limits because `MaxValueBytes` bounds Redis admission bytes rather than decoder work.
14. Tamper-sensitive deployments wrap payloads in an authenticated envelope in addition to `VersionedSerializer`; built-in versioning detects compatibility mismatches, not malicious modification.
15. A namespace is one exclusive tenant/schema/clear trust domain and not an authorization boundary; incompatible tenants or wire formats require separate namespaces and Redis ACL/network isolation.
16. `SCAN COUNT` is a hint: the client retains one Redis-controlled page and one bounded `UNLINK` argument chunk, but cannot bound the byte size of a returned page or external keys.
17. The serializer is caller-owned, immutable after construction, and safe for concurrent marshal/unmarshal; the package does not clone it or serialize calls behind a global lock.
18. The built-in versioned envelope is backward-readable only (`v2` reads `v1`, `v1` rejects `v2`); namespace reuse across an upgrade/rollback window is prohibited until the application proves its exact bidirectional serializer matrix.

Place ordered `<!-- redisvalue-contract: <id> -->` markers before the 18 synchronized facts in both READMEs. Add an executable parity test:

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

- [ ] **Step 3: Update root indexes and changelog**

Add `cache/redisvalue` next to `cache/redisfory` in both package tables and cache-category lists. Add one `Unreleased / Added` changelog bullet describing the generic Redis L2 plus reference-preserving tiered decorator and explicitly excluding RESP3 coherence.

- [ ] **Step 4: Write the Type A lesson**

Use this structure with evidence from the final implementation:

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

Keep these exact test names when adding the tests in Tasks 6, 9, and 10. Do not claim benchmark results owned by #560.

- [ ] **Step 5: Compile examples, verify locale parity, and commit**

```bash
go test -count=1 ./cache/redisvalue -run '^(Example|TestReadmeContractMarkersStayInParity)'
git diff --check
```

Expected: PASS.

```bash
git add cache/redisvalue README.md README.ko.md CHANGELOG.md docs/lessons/2026-07-18-issue-535-redis-tiered-value-cache.md
git commit -m "Document the redisvalue ownership boundary" \
  -m "Constraint: public behavior requires synchronized English and Korean documentation" \
  -m "Rejected: marketing the decorator as RESP3-coherent | issue #536 remains the proof gate" \
  -m "Confidence: high" -m "Scope-risk: moderate" \
  -m "Tested: compile-checked examples, locale structure scan, and diff check"
```

### Task 12: Run Full Verification and Complete the Pre-PR Review Gate

**Files:**
- Modify only if verification finds a defect: files already introduced by Tasks 1-11
- Create: `docs/superpowers/reviews/2026-07-18-issue-535-redis-tiered-value-cache-step-6r-code-review.md`

- [ ] **Step 1: Run focused verification first**

```bash
TESTCONTAINERS_REUSE_ENABLE=false TESTCONTAINERS_RYUK_DISABLED=false \
  go test -p 1 -count=1 ./cache/redisvalue
TESTCONTAINERS_REUSE_ENABLE=false TESTCONTAINERS_RYUK_DISABLED=false \
  go test -race -p 1 -count=1 ./cache/redisvalue
```

Expected: PASS.

- [ ] **Step 2: Run static and repository gates**

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

Expected: every command exits 0. `make ci` repeats earlier gates intentionally as the repository completion proof.

- [ ] **Step 3: Run Step 6-R six-lane review and main integration**

Review the exact implementation HEAD independently from performance, stability, security, operator/Ops, developer/API, and user/caller perspectives. The main session integrates findings and requires `P0=0/P1=0`. Record exact SHA, commands, lane counts, accepted P2s, and any main-session timeout fallback in the review artifact.

- [ ] **Step 4: Fix every P0/P1 and rerun affected plus full gates**

For each repair, add or tighten a regression test first, prove RED against the defect, implement the smallest fix, prove GREEN, commit with Lore trailers, then rerun all six perspectives against the new exact HEAD. Do not declare convergence from reviews of different substantive commits.

- [ ] **Step 5: Commit review evidence and stop at the PR authority gate**

```bash
git add docs/superpowers/reviews/2026-07-18-issue-535-redis-tiered-value-cache-step-6r-code-review.md
git commit -m "Record redisvalue implementation convergence" \
  -m "Constraint: pre-PR review requires six perspectives on one exact implementation head" \
  -m "Confidence: high" -m "Scope-risk: narrow" \
  -m "Tested: targeted, race, stress, Testcontainers, examples, static gates, and make ci"
```

Report the exact branch/HEAD and merge-ready local evidence. Do not push, create a PR, enable auto-merge, merge, tag, publish, or delete the worktree without the next explicit authority gate.

## Rollback Strategy

Before publication, revert the implementation commits in reverse task order; no existing package, Redis key family, interface, or dependency is changed. After adoption, roll back callers to their prior cache path while retaining the old namespace for at least the maximum finite TTL plus margin. Incompatible serializer changes rotate the namespace; zero-TTL namespaces require an explicit stable-primary admin clear. A rollback never treats Redis logical databases as tenant isolation and never connects the unsupported failover/proxy topology to `ValueCache.Clear`.
