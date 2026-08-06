# Fory Redis Value Cache 구현 계획

> 한국어 재작성 범위: 이 계획 문서는 한국어 운영 문서로 읽히도록 제목, 판단, 작업 설명, 위험, 검증, 롤백 문맥을 한국어로 정리한다. 명령, 경로, API 이름, 이슈/PR 번호, 브랜치명, 코드 블록, 테스트 출력 같은 증거 문자열은 정확성을 위해 원문 그대로 보존한다.


> **에이전트 작업자용:** 필수 하위 스킬: 사용 superpowers:subagent-driven-development (권장) 또는 superpowers:executing-plans to 이 계획을 작업 단위로 구현. 단계는 checkbox (`- [ ]`) 추적 문법을 사용.

**목표:** 추가 a Go-native Apache Fory direct Redis value cache 함께 explicit fast/compatible profiles, bounded binary envelopes, schema-generation isolation, 및 safe typed 진단.

**아키텍처:** Extract 만 the synchronized Fory runtime lifecycle into `cache/internal/forynative`, while `cache/rediscoord/fory` retains `BTFY v1` 및 its 공개 API. 구성 `cache/redisfory` as an independent 패키지 that owns `BTFV v1`, physical keys, TTL, Redis commands, 및 cache 오류; a 패키지-private codec interface makes dispatch-time cancellation deterministic to 테스트.

**기술 스택:** Go 1.26, Apache Fory Go v1.3.0, go-redis v9, Testcontainers Redis 7.4, standard `testing`, race detector, SVG plus CairoSVG-rendered PNG.

---

## 파일 지도

- 생성 `cache/internal/forynative/runtime.go` 및 `runtime_test.go`: bounded defaults, root validation, registration, synchronized serialization, sanitized internal 오류.
- Modify `cache/rediscoord/fory/codec.go` 및 `codec_test.go`: delegate runtime lifecycle while retaining 공개 API 및 `BTFY v1`.
- 생성 `cache/redisfory/doc.go`, `options.go`, `errors.go`, `envelope.go`, `value_cache.go`: 공개 direct-cache implementation.
- 생성 `cache/redisfory/value_cache_test.go`, `integration_test.go`, `example_test.go`: unit, Redis 7.4, race, 및 compile evidence.
- 생성 `cache/redisfory/README.md`, `README.ko.md`, 및 paired `docs/images/readme-diagrams/redisfory-direct-value-f낮음.svg|png`.
- Modify `README.md`, `README.ko.md`, 및 `CHANGELOG.md`: 패키지 discovery 및 unreleased change.
- 생성 implementation review 및 lesson artifacts under `docs/review` 및 `docs/lessons`.

### 작업 1: Extract The Shared Native Fory Runtime

**파일:**
- 생성: `cache/internal/forynative/runtime.go`
- 생성: `cache/internal/forynative/runtime_test.go`

- [ ] **단계 1: Write failing construction 및 safety 테스트**

사용 registered 테스트 structs 및 the intended API:

```go
func TestNewUsesBoundedDefaultsAndRejectsInvalidInputs(t *testing.T) {
    runtime, err := New[testValue](ProfileNativeFast, Limits{}, registerTestValue)
    if err != nil { t.Fatal(err) }
    if runtime.limits.MaxPayloadBytes != 1<<20 || runtime.limits.MaxDepth != 20 {
        t.Fatalf("limits = %#v", runtime.limits)
    }
    _, err = New[testValue](ProfileNativeFast, Limits{MaxDepth: -1}, registerTestValue)
    var runtimeErr *Error
    if !errors.As(err, &runtimeErr) || runtimeErr.Reason() != ReasonConfiguration {
        t.Fatalf("error = %v", err)
    }
}
```

Also name 테스트 for nil registration, registration 오류/panic redaction, unsupported roots, payload bounds, provider panic sanitization, returned-byte copying, 및 16 workers x 100 exact-value round trips.

- [ ] **단계 2: Confirm the 패키지 is missing**

실행: `go test -count=1 ./cache/internal/forynative`

예상: FAIL because the 패키지/API does 아님 exist.

- [ ] **단계 3: 구현 the minimal runtime 계약**

```go
type Profile uint8
const (
    ProfileNativeFast Profile = iota + 1
    ProfileNativeCompatible
)
type Registration func(*fory.Fory) error
type Reason string
const (
    ReasonConfiguration Reason = "configuration"
    ReasonUninitialized Reason = "uninitialized"
    ReasonRegistration Reason = "registration"
    ReasonPayloadTooLarge Reason = "payload-too-large"
    ReasonUnsupportedValue Reason = "unsupported-value"
    ReasonForyFailure Reason = "fory-failure"
)
type Error struct { operation string; reason Reason; cause error }
func (e *Error) Operation() string
func (e *Error) Reason() Reason
func (e *Error) Unwrap() error
type Limits struct {
    MaxPayloadBytes int
    MaxDepth int
    MaxTypeFields int
    MaxTypeMetaBytes int
    MaxSchemaVersionsPerType int
    MaxAverageSchemaVersionsPerType int
}
type Runtime[V any] struct { state *state; limits Limits }
type state struct { mu sync.Mutex; runtime *fory.Fory }

func New[V any](profile Profile, limits Limits, register Registration) (*Runtime[V], error)
func (r *Runtime[V]) Serialize(value V) ([]byte, error)
func (r *Runtime[V]) Deserialize(raw []byte) (V, error)
```

Apply defaults `1<<20`, `20`, `512`, `4096`, `10`, `3`; reject negatives 및 payloads over `math.MaxUint32`; validate roots. Construct Fory 함께 `WithXlang(false)`, profile-specific `WithCompatible`, `WithTrackRef(false)`, 및 모든 limits. 고정 만 the Fory call plus returned-byte copy. 교체 registration/provider text 및 panics 함께 sanitized sentinels.
For a struct root, pass `&value` to Fory exactly as the 기존 codec does; primitive 및 `[]byte`
roots pass their value directly. Tests use `errors.As` 및 the accessors above 및 prove that
`Error()` 및 `Unwrap()` contain 만 sanitized 패키지 causes.

- [ ] **단계 4: 실행 focused verification**

실행: `go test -count=1 ./cache/internal/forynative`

예상: PASS.

실행: `go test -race -count=1 ./cache/internal/forynative`

예상: PASS 함께 없음 race report.

- [ ] **단계 5: 커밋**

```bash
git add cache/internal/forynative
git commit -m "Share the constrained Fory runtime without sharing wire formats" \
  -m "Constraint: Fory is not thread-safe and provider details must remain sanitized." \
  -m "Rejected: Export a common codec API | coordination and direct values own different envelopes." \
  -m "Confidence: high" -m "Scope-risk: moderate" \
  -m "Tested: cache/internal/forynative unit and race tests"
```

### 작업 2: 보존 The Existing `BTFY v1` Codec

**파일:**
- Modify: `cache/rediscoord/fory/codec.go`
- Modify: `cache/rediscoord/fory/codec_test.go`

- [ ] **단계 1: 추가 a golden-layout regression 테스트**

```go
func TestBTFYV1LayoutRemainsStable(t *testing.T) {
    codec, err := NewNativeFast[string](Options{Register: func(*fory.Fory) error { return nil }})
    if err != nil { t.Fatal(err) }
    encoded, err := codec.Marshal("value")
    if err != nil { t.Fatal(err) }
    if string(encoded[:4]) != "BTFY" || encoded[4] != 1 || encoded[5] != 1 {
        t.Fatalf("header = %x", encoded[:10])
    }
    if binary.BigEndian.Uint32(encoded[6:10]) != uint32(len(encoded)-10) {
        t.Fatalf("length = %d", binary.BigEndian.Uint32(encoded[6:10]))
    }
}
```

- [ ] **단계 2: Establish the pre-refactor baseline**

실행: `go test -count=1 ./cache/rediscoord/fory`

예상: PASS.

- [ ] **단계 3: Delegate local runtime work to `forynative.Runtime[V]`**

유지 `Registration`, `Options`, `Codec`, `Profile`, `Reason`, constructors, 오류 accessors, 및 `BTFY` source-compatible. Map options/profile/reasons at the 패키지 boundary. 다음을 하지 않는다: move `wrap`, `unwrap`, 또는 공개 오류 formatting into internal code.

- [ ] **단계 4: 검증 compatibility**

실행: `go test -count=1 ./cache/rediscoord/fory`

예상: PASS including 모든 pre-기존 테스트 및 the golden layout.

실행: `go test -race -count=1 ./cache/internal/forynative ./cache/rediscoord/fory`

예상: PASS.

- [ ] **단계 5: 커밋**

```bash
git add cache/rediscoord/fory
git commit -m "Keep BTFY stable while reusing the native runtime" \
  -m "Constraint: Issue #597 API and BTFY v1 bytes cannot change." \
  -m "Confidence: high" -m "Scope-risk: moderate" \
  -m "Directive: Do not add schema generation to BTFY." \
  -m "Tested: rediscoord/fory unit and race tests"
```

### 작업 3: 정의 The Direct API And `BTFV v1`

**파일:**
- 생성: `cache/redisfory/doc.go`
- 생성: `cache/redisfory/options.go`
- 생성: `cache/redisfory/errors.go`
- 생성: `cache/redisfory/envelope.go`
- 생성: `cache/redisfory/value_cache_test.go`

- [ ] **단계 1: Write failing API, constructor, 오류, 및 envelope 테스트**

Compile-check `Registration`, both constructors, `Profile`, `Reason`, 및 `CacheError` accessors. Test nil/typed-nil `redis.Cmdable`, invalid namespace, zero generation, nil registration, negative limits, uint32 overf낮음, unsupported roots, zero-value cache, 및 redaction. 검증 this layout:

```go
func TestBTFVLayoutAndValidation(t *testing.T) {
    encoded := wrap(ProfileNativeFast, 7, []byte{1, 2, 3})
    if string(encoded[:4]) != "BTFV" || encoded[4] != 1 || encoded[5] != 1 {
        t.Fatalf("header = %x", encoded[:14])
    }
    if binary.BigEndian.Uint32(encoded[6:10]) != 7 ||
       binary.BigEndian.Uint32(encoded[10:14]) != 3 {
        t.Fatalf("metadata = %x", encoded[6:14])
    }
}
```

Mutate magic, version, profile, generation, declared length, trailing bytes, truncation, 및 total size; assert `errors.As` 및 exact `Reason()`.

- [ ] **단계 2: Confirm missing symbols**

실행: `go test -count=1 ./cache/redisfory`

예상: FAIL because the 패키지 API does 아님 exist.

- [ ] **단계 3: 구현 the exact 공개 계약**

```go
type Registration func(*fory.Fory) error
type Profile string
const (
    ProfileNativeFast Profile = "native-fast"
    ProfileNativeCompatible Profile = "native-compatible"
)
type Reason string
const (
    ReasonConfiguration Reason = "configuration"
    ReasonUninitialized Reason = "uninitialized"
    ReasonRegistration Reason = "registration"
    ReasonPayloadTooLarge Reason = "payload-too-large"
    ReasonInvalidMagic Reason = "invalid-magic"
    ReasonUnsupportedVersion Reason = "unsupported-version"
    ReasonFormatMismatch Reason = "format-mismatch"
    ReasonSchemaMismatch Reason = "schema-mismatch"
    ReasonLengthMismatch Reason = "length-mismatch"
    ReasonUnsupportedValue Reason = "unsupported-value"
    ReasonForyFailure Reason = "fory-failure"
)
type CacheError struct { operation string; profile Profile; reason Reason; cause error }
func (e *CacheError) Operation() string
func (e *CacheError) Profile() Profile
func (e *CacheError) Reason() Reason

type Options struct {
    Client redis.Cmdable
    Namespace string
    SchemaGeneration uint32
    Register Registration
    MaxPayloadBytes int
    MaxDepth int
    MaxTypeFields int
    MaxTypeMetaBytes int
    MaxSchemaVersionsPerType int
    MaxAverageSchemaVersionsPerType int
}
func NewNativeFast[V any](Options) (*ValueCache[V], error)
func NewNativeCompatible[V any](Options) (*ValueCache[V], error)
```

Give every exported type, option field, constant, constructor, 및 method an 영문 Go doc
comment. Package/type docs state trusted-internal Go-만 storage, 없음 xlang/fallback/`Clear`, 및
mandatory explicit schema generation.

Validate every colon-separated namespace segment against `^[A-Za-z0-9._-]+$`; table 테스트 reject
`*`, `?`, `[`, `]`, backslash, whitespace, control characters, braces, 및 empty segments. 구성
keys 함께 `btredis.NewKeyBuilder("bluetape:cache:fory")`, append
`strings.Split(namespace, ":")...` through `Structural`, then append
`fmt.Sprintf("g%d", generation)` through `Structural`. Detect typed nil clients 전에 runtime
construction. `wrap` uses a 14-byte header; `unwrap` checks total bound 전에 slicing 및 모든
metadata 전에 Fory decode.

- [ ] **단계 4: 실행 focused 테스트**

실행: `go test -count=1 ./cache/redisfory`

예상: PASS for API, constructor, 오류, 및 envelope 테스트.

- [ ] **단계 5: 커밋**

```bash
git add cache/redisfory
git commit -m "Make direct Fory values distinguishable and bounded" \
  -m "Constraint: Direct values need schema generation without changing BTFY." \
  -m "Rejected: Sniff JSON or raw Fory bytes | fallback hides rollout mistakes." \
  -m "Confidence: high" -m "Scope-risk: moderate" \
  -m "Tested: go test -count=1 ./cache/redisfory"
```

### 작업 4: 구현 `Get`, `Set`, And `Delete` Test-First

**파일:**
- 생성: `cache/redisfory/value_cache.go`
- Modify: `cache/redisfory/value_cache_test.go`

- [ ] **단계 1: 추가 failing command 및 cancellation 테스트**

사용 this 패키지-private seam for deterministic cancellation:

```go
type valueCodec[V any] interface {
    Serialize(V) ([]byte, error)
    Deserialize([]byte) (V, error)
}
type cacheState[V any] struct { codec valueCodec[V] }
type commandClient interface {
    Get(context.Context, string) *redis.StringCmd
    Set(context.Context, string, any, time.Duration) *redis.StatusCmd
    Del(context.Context, ...string) *redis.IntCmd
}
```

사용 a Redis command double 또는 go-redis hook to count commands 및 capture key/TTL/bytes. A blocking fake codec waits until the 테스트 cancels context, then returns bytes; assert `Set` returns `context.Canceled` 및 SET count is zero.

Store the validated 공개 `redis.Cmdable` through the narrow `commandClient` field so the 테스트
implement 만 three commands 및 add 없음 mocking dependency. Name 테스트:
`TestValueCacheSetStoresBTFVWithTTL`, `TestValueCacheSetRechecksContextAfterSerialization`,
`TestValueCacheGetMapsRedisNilToCacheMiss`, `TestValueCacheGetValidatesBeforeDecode`,
`TestValueCacheGetRechecksContextAfterRedisReadBeforeDecode`,
`TestValueCacheDeleteValidatesKeyAndIsIdempotent`, `TestValueCacheMethodsNormalizeNilContext`,
`TestValueCacheCommandContextErrorsRemainInspectable`,
`TestValueCacheMethodsSanitizeRedisProviderErrors`, 및 `TestZeroValueCacheReturnsUninitialized`.
Malformed envelope cases inject a fake codec 함께 a deserialize counter 및 assert it stays zero.

- [ ] **단계 2: Confirm missing method behavior**

실행: `go test -count=1 ./cache/redisfory`

예상: FAIL on missing methods 또는 dispatch.

- [ ] **단계 3: 구현 the methods 함께 dispatch boundaries**

```go
func (c *ValueCache[V]) Set(ctx context.Context, logicalKey string, value V, ttl time.Duration) error {
    ctx = normalizeContext(ctx)
    if err := ctx.Err(); err != nil { return err }
    key, err := c.key(logicalKey)
    if err != nil { return err }
    if err := btredis.ValidateTTL("value ttl", ttl); err != nil { return err }
    raw, err := c.state.codec.Serialize(value)
    if err != nil { return c.cacheError("set", err) }
    encoded, err := c.wrap(raw)
    if err != nil { return err }
    if err := ctx.Err(); err != nil { return err }
    if err := c.client.Set(ctx, key.Value, encoded, ttl).Err(); err != nil {
        return c.operationError(ctx, "set", key.Value, err)
    }
    return nil
}
```

구현 `Get` 및 `Delete` 함께 the same normalize/preflight/key sequence. `Get` maps 만
`redis.Nil` to `cache.ErrCacheMiss`, then rechecks `ctx.Err()` 후 bytes return 및 전에
envelope/decode work. Command failures use `operationError`, which replaces the raw Redis 원인
함께 an unexported 패키지 sentinel 및 joins 만 `ctx.Err()` when non-nil 전에 constructing
`btredis.OpError`; 테스트 prove `errors.Is` for cancellation/deadline 및 없음 provider/key/payload
marker through `Error()` 또는 `Unwrap()`. `Delete` treats zero deleted keys as success. Never hold
the Fory mutex over Redis I/O.

- [ ] **단계 4: 실행 unit 및 race 테스트**

실행: `go test -count=1 ./cache/redisfory`

예상: PASS.

실행: `go test -race -count=1 ./cache/internal/forynative ./cache/rediscoord/fory ./cache/redisfory`

예상: PASS.

- [ ] **단계 5: 커밋**

```bash
git add cache/redisfory
git commit -m "Store native Fory values without late canceled writes" \
  -m "Constraint: Commands preserve caller context and redact keys." \
  -m "Rejected: Implement Clear | unbounded namespace deletion is outside contract." \
  -m "Confidence: high" -m "Scope-risk: moderate" \
  -m "Tested: redisfory unit and race tests"
```

### 작업 5: 증명 Redis Storage, TTL, Generation, And Concurrency

**파일:**
- 생성: `cache/redisfory/integration_test.go`

- [ ] **단계 1: Write failing Testcontainers 테스트**

생성 each integration context 함께 `context.WithTimeout(context.Background(), 30*time.Second)`
및 register its cancel function 전에 calling `addr := redistestcontainer.Start(ctx, t)`. 사용
separate failure messages for container startup/readiness 및 Redis operations. Construct
`redis.NewClient(&redis.Options{Addr: addr})`, 및 close that 호출자-owned client 함께 `t.Cleanup`.
Register `integrationValue{Name string; Count int}` as `redisfory.integrationValue`. Test both
profiles, raw Redis bytes beginning `BTFV` 및 아님 JSON/base64, TTL expiry as
`cache.ErrCacheMiss`, explicit miss, idempotent delete, generation 1/2 key isolation, redacted
command failure, 및 16 workers x 100 exact round trips 함께 operation/miss/오류 counts.

- [ ] **단계 2: 실행 integration 테스트**

실행: `go test -count=1 ./cache/redisfory`

예상: FAIL if any direct Redis path is incomplete.

- [ ] **단계 3: Make integration-만 corrections**

Adjust cache code 또는 fixtures without adding retries, ownership, fallback, loading, `Clear`, compression, 또는 migration. Never close the 호출자 client from `ValueCache`.

- [ ] **단계 4: 실행 serial integration 및 race gates**

실행: `go test -count=1 ./cache/redisfory`

예상: PASS against Redis 7.4.

실행: `go test -race -count=1 ./cache/redisfory`

예상: PASS. 다음을 하지 않는다: run another Redis/DB Testcontainers command concurrently.

- [ ] **단계 5: 커밋**

```bash
git add cache/redisfory
git commit -m "Prove direct Fory cache behavior against Redis 7.4" \
  -m "Constraint: Docker-backed tests run serially." \
  -m "Confidence: high" -m "Scope-risk: narrow" \
  -m "Tested: Redis 7.4 integration and race tests"
```

### 작업 6: 추가 Examples And Bilingual Documentation

**파일:**
- 생성: `cache/redisfory/example_test.go`
- 생성: `cache/redisfory/README.md`
- 생성: `cache/redisfory/README.ko.md`
- Modify: `README.md`
- Modify: `README.ko.md`
- Modify: `CHANGELOG.md`

- [ ] **단계 1: Write compile-checked example**

추가 `ExampleNewNativeFast` 및 `ExampleNewNativeCompatible` 함께 호출자-owned client, explicit registration, `SchemaGeneration: 1`, positive TTL, 및 `Set`/`Get`/`Delete`. Omit `Output:` so 없음 live Redis connection is required.

- [ ] **단계 2: Compile example**

실행: `go test -run Example -count=1 ./cache/redisfory`

예상: PASS.

- [ ] **단계 3: Write synchronized README contracts**

Both locales cover import/usage, profiles, Go-만/없음-xlang, complete option tuple, roots, exact
defaults, 모든 stable reason constants, `BTFV`, schema generation, visible Redis keys/values,
ACL/TLS, 호출자 lifecycle, TTL, typed 오류, 없음 fallback/loading/Clear/compression, Cluster hash
tags, rollout/rollback, bounded `SCAN` cleanup, 낮음-cardinality telemetry, 및 #599 benchmark
ownership. Cleanup distinguishes standalone Redis from Redis Cluster: every primary gets its own
dry-run count, bounded scan/delete, 및 re-scan; neither mode uses `KEYS`.

- [ ] **단계 4: 업데이트 root discovery 및 changelog**

추가 `cache/redisfory` beside cache packages in both root README tables/lists. 추가 one unreleased `Added` bullet without a 성능 claim.

- [ ] **단계 5: 검증 locale consistency**

실행: `rg -n 'redisfory|BTFV|SchemaGeneration|native-compatible' cache/redisfory/README.md cache/redisfory/README.ko.md README.md README.ko.md CHANGELOG.md`

예상: references in both locales, root indexes, 및 changelog.

Compare both 패키지 README heading sets 및 manually check paired import snippets, defaults
table, reason list, non-goals, rollout/rollback, ACL/TLS, hash tags, standalone/Cluster cleanup,
및 benchmark boundary. Compare both root README table/list entries; record parity in the review
artifact.

- [ ] **단계 6: 커밋**

```bash
git add cache/redisfory README.md README.ko.md CHANGELOG.md
git commit -m "Document the boundary of direct Fory cache values" \
  -m "Constraint: Public behavior requires synchronized English and Korean docs." \
  -m "Confidence: high" -m "Scope-risk: narrow" \
  -m "Tested: compile-checked examples and locale reference scan"
```

### 작업 7: 생성 And 감사 The 아키텍처 Diagram

**파일:**
- 생성: `docs/images/readme-diagrams/redisfory-direct-value-f낮음.svg`
- 생성: `docs/images/readme-diagrams/redisfory-direct-value-f낮음.png`
- Modify: `cache/redisfory/README.md`
- Modify: `cache/redisfory/README.ko.md`

- [ ] **단계 1: Load `bluetape4k-diagram` 및 create SVG**

Show 호출자, `ValueCache`, key/schema builder, `BTFV` validation, synchronized Fory runtime, 호출자-owned client, 및 Redis. Mark trust boundaries 및 visible binary storage. Show `rediscoord` as a separate JSON/base64 coordination path.

- [ ] **단계 2: Render PNG**

실행: `cairosvg docs/images/readme-diagrams/redisfory-direct-value-f낮음.svg -o docs/images/readme-diagrams/redisfory-direct-value-f낮음.png`

예상: exit 0 및 non-empty paired files.

- [ ] **단계 3: Embed PNG in both READMEs**

사용 `../../docs/images/readme-diagrams/redisfory-direct-value-f낮음.png`; keep adjacent SVG as source.

- [ ] **단계 4: 감사 및 inspect**

실행 the current diagram skill's XML, endpoint, kind, 및 sequence-style audit commands. Inspect PNG at original resolution 함께 `view_image`. 예상: zero findings, legible labels, 없음 overlap/cropping, correct arrows.

- [ ] **단계 5: 커밋**

```bash
git add docs/images/readme-diagrams/redisfory-direct-value-flow.svg \
  docs/images/readme-diagrams/redisfory-direct-value-flow.png \
  cache/redisfory/README.md cache/redisfory/README.ko.md
git commit -m "Make the direct Fory cache trust boundary inspectable" \
  -m "Constraint: README visuals require paired editable and rendered assets." \
  -m "Confidence: high" -m "Scope-risk: narrow" \
  -m "Tested: diagram audits and original-size PNG inspection"
```

### 작업 8: 실행 검증 And 리뷰 Gates

**파일:**
- 생성: `docs/review/2026-07-10-issue-598-fory-redis-value-cache-implementation-review.md`
- 생성: `docs/lessons/2026-07-10-issue-598-fory-redis-value-cache.md`
- Modify 만 on findings: files from Tasks 1-7

- [ ] **단계 1: 실행 targeted gates sequentially**

```bash
go test -p 1 -count=1 ./cache/internal/forynative ./cache/rediscoord/fory ./cache/redisfory
go test -race -p 1 -count=1 ./cache/internal/forynative ./cache/rediscoord/fory ./cache/redisfory
go test -run Example -count=1 ./cache/redisfory
go vet ./cache/internal/forynative ./cache/rediscoord/fory ./cache/redisfory
git diff --check
```

예상: every command exits 0; Docker-backed commands are 아님 parallelized.

- [ ] **단계 2: 실행 full repository verification**

실행: `make ci`

예상: PASS for formatting, tidy, vet, lint, 테스트, 및 configured checks.

- [ ] **단계 3: 실행 단계 6-R**

실행 six independent 성능, 안정성, 보안, 운영자/Ops, 개발자/API, 및 사용자/호출자 read-만 reviews plus main integration. 기록 P0/P1/P2/P3 및 evidence. Fix P0/P1 테스트-first 및 rerun affected lanes to P0=0/P1=0.

- [ ] **단계 4: 실행 단계 7-R**

리뷰 the complete branch diff against spec 및 plan 함께 the same lanes plus main integration. Re-run the smallest proof 후 late changes, then `make ci` 및 `git diff --check`. Close 만 at P0=0/P1=0.

- [ ] **단계 5: 기록 lessons 및 evidence**

문서화 runtime boundaries, dispatch-time cancellation, envelope-first validation, key visibility, 및 benchmark separation. 다음을 하지 않는다: invent benchmark results; link #599.

- [ ] **단계 6: 커밋 evidence**

```bash
git add docs/review/2026-07-10-issue-598-fory-redis-value-cache-implementation-review.md \
  docs/lessons/2026-07-10-issue-598-fory-redis-value-cache.md
git commit -m "Retain the evidence behind the direct Fory cache decision" \
  -m "Constraint: Type A delivery requires review and lesson artifacts." \
  -m "Confidence: high" -m "Scope-risk: narrow" \
  -m "Tested: targeted gates; make ci; Step 6-R and Step 7-R P0=0 P1=0"
```

### 작업 9: 공개 The Pull Request Without Merging

**파일:**
- No production file changes expected.

- [ ] **단계 1: 검증 branch hygiene**

실행: `git status --short && git log --oneline origin/develop..HEAD && git diff --check origin/develop...HEAD`

예상: clean status, intentional Lore commits, 없음 whitespace 오류.

실행: `gh issue view 598 --json state,assignees,milestone,labels && gh issue view 599 --json state,assignees,milestone,labels`

예상: both issues have the intended assignee, milestone, labels, 및 state 전에 publication.

- [ ] **단계 2: Push 및 create the issue-linked PR**

The PR body summarizes API/wire/operational boundaries, exact verification, 및 #599 benchmark ownership, 및 ends 함께 `## DoD Status`.

- [ ] **단계 3: 검증 live metadata 및 CI**

사용 `gh pr view` 및 `gh pr checks` to verify closing reference, assignee, milestone, final body
section, head SHA, 및 checks. Fix failures within the approved plan automatically 및 rerun the
affected gate.

- [ ] **단계 4: Stop at merge approval**

Report PR URL, CI, 단계 6-R/7-R counts, 및 테스트. 다음을 하지 않는다: merge without explicit 사용자 approval.
