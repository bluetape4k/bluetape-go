# Fory Redis Value Cache Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a Go-native Apache Fory direct Redis value cache with explicit fast/compatible profiles, bounded binary envelopes, schema-generation isolation, and safe typed diagnostics.

**Architecture:** Extract only the synchronized Fory runtime lifecycle into `cache/internal/forynative`, while `cache/rediscoord/fory` retains `BTFY v1` and its public API. Build `cache/redisfory` as an independent package that owns `BTFV v1`, physical keys, TTL, Redis commands, and cache errors; a package-private codec interface makes dispatch-time cancellation deterministic to test.

**Tech Stack:** Go 1.26, Apache Fory Go v1.3.0, go-redis v9, Testcontainers Redis 7.4, standard `testing`, race detector, SVG plus CairoSVG-rendered PNG.

---

## File Map

- Create `cache/internal/forynative/runtime.go` and `runtime_test.go`: bounded defaults, root validation, registration, synchronized serialization, sanitized internal errors.
- Modify `cache/rediscoord/fory/codec.go` and `codec_test.go`: delegate runtime lifecycle while retaining public API and `BTFY v1`.
- Create `cache/redisfory/doc.go`, `options.go`, `errors.go`, `envelope.go`, `value_cache.go`: public direct-cache implementation.
- Create `cache/redisfory/value_cache_test.go`, `integration_test.go`, `example_test.go`: unit, Redis 7.4, race, and compile evidence.
- Create `cache/redisfory/README.md`, `README.ko.md`, and paired `docs/images/readme-diagrams/redisfory-direct-value-flow.svg|png`.
- Modify `README.md`, `README.ko.md`, and `CHANGELOG.md`: package discovery and unreleased change.
- Create implementation review and lesson artifacts under `docs/review` and `docs/lessons`.

### Task 1: Extract The Shared Native Fory Runtime

**Files:**
- Create: `cache/internal/forynative/runtime.go`
- Create: `cache/internal/forynative/runtime_test.go`

- [ ] **Step 1: Write failing construction and safety tests**

Use registered test structs and the intended API:

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

Also name tests for nil registration, registration error/panic redaction, unsupported roots, payload bounds, provider panic sanitization, returned-byte copying, and 16 workers x 100 exact-value round trips.

- [ ] **Step 2: Confirm the package is missing**

Run: `go test -count=1 ./cache/internal/forynative`

Expected: FAIL because the package/API does not exist.

- [ ] **Step 3: Implement the minimal runtime contract**

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

Apply defaults `1<<20`, `20`, `512`, `4096`, `10`, `3`; reject negatives and payloads over `math.MaxUint32`; validate roots. Construct Fory with `WithXlang(false)`, profile-specific `WithCompatible`, `WithTrackRef(false)`, and all limits. Lock only the Fory call plus returned-byte copy. Replace registration/provider text and panics with sanitized sentinels.
For a struct root, pass `&value` to Fory exactly as the existing codec does; primitive and `[]byte`
roots pass their value directly. Tests use `errors.As` and the accessors above and prove that
`Error()` and `Unwrap()` contain only sanitized package causes.

- [ ] **Step 4: Run focused verification**

Run: `go test -count=1 ./cache/internal/forynative`

Expected: PASS.

Run: `go test -race -count=1 ./cache/internal/forynative`

Expected: PASS with no race report.

- [ ] **Step 5: Commit**

```bash
git add cache/internal/forynative
git commit -m "Share the constrained Fory runtime without sharing wire formats" \
  -m "Constraint: Fory is not thread-safe and provider details must remain sanitized." \
  -m "Rejected: Export a common codec API | coordination and direct values own different envelopes." \
  -m "Confidence: high" -m "Scope-risk: moderate" \
  -m "Tested: cache/internal/forynative unit and race tests"
```

### Task 2: Preserve The Existing `BTFY v1` Codec

**Files:**
- Modify: `cache/rediscoord/fory/codec.go`
- Modify: `cache/rediscoord/fory/codec_test.go`

- [ ] **Step 1: Add a golden-layout regression test**

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

- [ ] **Step 2: Establish the pre-refactor baseline**

Run: `go test -count=1 ./cache/rediscoord/fory`

Expected: PASS.

- [ ] **Step 3: Delegate local runtime work to `forynative.Runtime[V]`**

Keep `Registration`, `Options`, `Codec`, `Profile`, `Reason`, constructors, error accessors, and `BTFY` source-compatible. Map options/profile/reasons at the package boundary. Do not move `wrap`, `unwrap`, or public error formatting into internal code.

- [ ] **Step 4: Verify compatibility**

Run: `go test -count=1 ./cache/rediscoord/fory`

Expected: PASS including all pre-existing tests and the golden layout.

Run: `go test -race -count=1 ./cache/internal/forynative ./cache/rediscoord/fory`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cache/rediscoord/fory
git commit -m "Keep BTFY stable while reusing the native runtime" \
  -m "Constraint: Issue #597 API and BTFY v1 bytes cannot change." \
  -m "Confidence: high" -m "Scope-risk: moderate" \
  -m "Directive: Do not add schema generation to BTFY." \
  -m "Tested: rediscoord/fory unit and race tests"
```

### Task 3: Define The Direct API And `BTFV v1`

**Files:**
- Create: `cache/redisfory/doc.go`
- Create: `cache/redisfory/options.go`
- Create: `cache/redisfory/errors.go`
- Create: `cache/redisfory/envelope.go`
- Create: `cache/redisfory/value_cache_test.go`

- [ ] **Step 1: Write failing API, constructor, error, and envelope tests**

Compile-check `Registration`, both constructors, `Profile`, `Reason`, and `CacheError` accessors. Test nil/typed-nil `redis.Cmdable`, invalid namespace, zero generation, nil registration, negative limits, uint32 overflow, unsupported roots, zero-value cache, and redaction. Assert this layout:

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

Mutate magic, version, profile, generation, declared length, trailing bytes, truncation, and total size; assert `errors.As` and exact `Reason()`.

- [ ] **Step 2: Confirm missing symbols**

Run: `go test -count=1 ./cache/redisfory`

Expected: FAIL because the package API does not exist.

- [ ] **Step 3: Implement the exact public contract**

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

Give every exported type, option field, constant, constructor, and method an English Go doc
comment. Package/type docs state trusted-internal Go-only storage, no xlang/fallback/`Clear`, and
mandatory explicit schema generation.

Validate every colon-separated namespace segment against `^[A-Za-z0-9._-]+$`; table tests reject
`*`, `?`, `[`, `]`, backslash, whitespace, control characters, braces, and empty segments. Build
keys with `btredis.NewKeyBuilder("bluetape:cache:fory")`, append
`strings.Split(namespace, ":")...` through `Structural`, then append
`fmt.Sprintf("g%d", generation)` through `Structural`. Detect typed nil clients before runtime
construction. `wrap` uses a 14-byte header; `unwrap` checks total bound before slicing and all
metadata before Fory decode.

- [ ] **Step 4: Run focused tests**

Run: `go test -count=1 ./cache/redisfory`

Expected: PASS for API, constructor, error, and envelope tests.

- [ ] **Step 5: Commit**

```bash
git add cache/redisfory
git commit -m "Make direct Fory values distinguishable and bounded" \
  -m "Constraint: Direct values need schema generation without changing BTFY." \
  -m "Rejected: Sniff JSON or raw Fory bytes | fallback hides rollout mistakes." \
  -m "Confidence: high" -m "Scope-risk: moderate" \
  -m "Tested: go test -count=1 ./cache/redisfory"
```

### Task 4: Implement `Get`, `Set`, And `Delete` Test-First

**Files:**
- Create: `cache/redisfory/value_cache.go`
- Modify: `cache/redisfory/value_cache_test.go`

- [ ] **Step 1: Add failing command and cancellation tests**

Use this package-private seam for deterministic cancellation:

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

Use a Redis command double or go-redis hook to count commands and capture key/TTL/bytes. A blocking fake codec waits until the test cancels context, then returns bytes; assert `Set` returns `context.Canceled` and SET count is zero.

Store the validated public `redis.Cmdable` through the narrow `commandClient` field so the tests
implement only three commands and add no mocking dependency. Name tests:
`TestValueCacheSetStoresBTFVWithTTL`, `TestValueCacheSetRechecksContextAfterSerialization`,
`TestValueCacheGetMapsRedisNilToCacheMiss`, `TestValueCacheGetValidatesBeforeDecode`,
`TestValueCacheGetRechecksContextAfterRedisReadBeforeDecode`,
`TestValueCacheDeleteValidatesKeyAndIsIdempotent`, `TestValueCacheMethodsNormalizeNilContext`,
`TestValueCacheCommandContextErrorsRemainInspectable`,
`TestValueCacheMethodsSanitizeRedisProviderErrors`, and `TestZeroValueCacheReturnsUninitialized`.
Malformed envelope cases inject a fake codec with a deserialize counter and assert it stays zero.

- [ ] **Step 2: Confirm missing method behavior**

Run: `go test -count=1 ./cache/redisfory`

Expected: FAIL on missing methods or dispatch.

- [ ] **Step 3: Implement the methods with dispatch boundaries**

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

Implement `Get` and `Delete` with the same normalize/preflight/key sequence. `Get` maps only
`redis.Nil` to `cache.ErrCacheMiss`, then rechecks `ctx.Err()` after bytes return and before
envelope/decode work. Command failures use `operationError`, which replaces the raw Redis cause
with an unexported package sentinel and joins only `ctx.Err()` when non-nil before constructing
`btredis.OpError`; tests prove `errors.Is` for cancellation/deadline and no provider/key/payload
marker through `Error()` or `Unwrap()`. `Delete` treats zero deleted keys as success. Never hold
the Fory mutex over Redis I/O.

- [ ] **Step 4: Run unit and race tests**

Run: `go test -count=1 ./cache/redisfory`

Expected: PASS.

Run: `go test -race -count=1 ./cache/internal/forynative ./cache/rediscoord/fory ./cache/redisfory`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cache/redisfory
git commit -m "Store native Fory values without late canceled writes" \
  -m "Constraint: Commands preserve caller context and redact keys." \
  -m "Rejected: Implement Clear | unbounded namespace deletion is outside contract." \
  -m "Confidence: high" -m "Scope-risk: moderate" \
  -m "Tested: redisfory unit and race tests"
```

### Task 5: Prove Redis Storage, TTL, Generation, And Concurrency

**Files:**
- Create: `cache/redisfory/integration_test.go`

- [ ] **Step 1: Write failing Testcontainers tests**

Create each integration context with `context.WithTimeout(context.Background(), 30*time.Second)`
and register its cancel function before calling `addr := redistestcontainer.Start(ctx, t)`. Use
separate failure messages for container startup/readiness and Redis operations. Construct
`redis.NewClient(&redis.Options{Addr: addr})`, and close that caller-owned client with `t.Cleanup`.
Register `integrationValue{Name string; Count int}` as `redisfory.integrationValue`. Test both
profiles, raw Redis bytes beginning `BTFV` and not JSON/base64, TTL expiry as
`cache.ErrCacheMiss`, explicit miss, idempotent delete, generation 1/2 key isolation, redacted
command failure, and 16 workers x 100 exact round trips with operation/miss/error counts.

- [ ] **Step 2: Run integration tests**

Run: `go test -count=1 ./cache/redisfory`

Expected: FAIL if any direct Redis path is incomplete.

- [ ] **Step 3: Make integration-only corrections**

Adjust cache code or fixtures without adding retries, ownership, fallback, loading, `Clear`, compression, or migration. Never close the caller client from `ValueCache`.

- [ ] **Step 4: Run serial integration and race gates**

Run: `go test -count=1 ./cache/redisfory`

Expected: PASS against Redis 7.4.

Run: `go test -race -count=1 ./cache/redisfory`

Expected: PASS. Do not run another Redis/DB Testcontainers command concurrently.

- [ ] **Step 5: Commit**

```bash
git add cache/redisfory
git commit -m "Prove direct Fory cache behavior against Redis 7.4" \
  -m "Constraint: Docker-backed tests run serially." \
  -m "Confidence: high" -m "Scope-risk: narrow" \
  -m "Tested: Redis 7.4 integration and race tests"
```

### Task 6: Add Examples And Bilingual Documentation

**Files:**
- Create: `cache/redisfory/example_test.go`
- Create: `cache/redisfory/README.md`
- Create: `cache/redisfory/README.ko.md`
- Modify: `README.md`
- Modify: `README.ko.md`
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Write compile-checked examples**

Add `ExampleNewNativeFast` and `ExampleNewNativeCompatible` with caller-owned client, explicit registration, `SchemaGeneration: 1`, positive TTL, and `Set`/`Get`/`Delete`. Omit `Output:` so no live Redis connection is required.

- [ ] **Step 2: Compile examples**

Run: `go test -run Example -count=1 ./cache/redisfory`

Expected: PASS.

- [ ] **Step 3: Write synchronized README contracts**

Both locales cover import/usage, profiles, Go-only/no-xlang, complete option tuple, roots, exact
defaults, all stable reason constants, `BTFV`, schema generation, visible Redis keys/values,
ACL/TLS, caller lifecycle, TTL, typed errors, no fallback/loading/Clear/compression, Cluster hash
tags, rollout/rollback, bounded `SCAN` cleanup, low-cardinality telemetry, and #599 benchmark
ownership. Cleanup distinguishes standalone Redis from Redis Cluster: every primary gets its own
dry-run count, bounded scan/delete, and re-scan; neither mode uses `KEYS`.

- [ ] **Step 4: Update root discovery and changelog**

Add `cache/redisfory` beside cache packages in both root README tables/lists. Add one unreleased `Added` bullet without a performance claim.

- [ ] **Step 5: Verify locale consistency**

Run: `rg -n 'redisfory|BTFV|SchemaGeneration|native-compatible' cache/redisfory/README.md cache/redisfory/README.ko.md README.md README.ko.md CHANGELOG.md`

Expected: references in both locales, root indexes, and changelog.

Compare both package README heading sets and manually check paired import snippets, defaults
table, reason list, non-goals, rollout/rollback, ACL/TLS, hash tags, standalone/Cluster cleanup,
and benchmark boundary. Compare both root README table/list entries; record parity in the review
artifact.

- [ ] **Step 6: Commit**

```bash
git add cache/redisfory README.md README.ko.md CHANGELOG.md
git commit -m "Document the boundary of direct Fory cache values" \
  -m "Constraint: Public behavior requires synchronized English and Korean docs." \
  -m "Confidence: high" -m "Scope-risk: narrow" \
  -m "Tested: compile-checked examples and locale reference scan"
```

### Task 7: Create And Audit The Architecture Diagram

**Files:**
- Create: `docs/images/readme-diagrams/redisfory-direct-value-flow.svg`
- Create: `docs/images/readme-diagrams/redisfory-direct-value-flow.png`
- Modify: `cache/redisfory/README.md`
- Modify: `cache/redisfory/README.ko.md`

- [ ] **Step 1: Load `bluetape4k-diagram` and create SVG**

Show caller, `ValueCache`, key/schema builder, `BTFV` validation, synchronized Fory runtime, caller-owned client, and Redis. Mark trust boundaries and visible binary storage. Show `rediscoord` as a separate JSON/base64 coordination path.

- [ ] **Step 2: Render PNG**

Run: `cairosvg docs/images/readme-diagrams/redisfory-direct-value-flow.svg -o docs/images/readme-diagrams/redisfory-direct-value-flow.png`

Expected: exit 0 and non-empty paired files.

- [ ] **Step 3: Embed PNG in both READMEs**

Use `../../docs/images/readme-diagrams/redisfory-direct-value-flow.png`; keep adjacent SVG as source.

- [ ] **Step 4: Audit and inspect**

Run the current diagram skill's XML, endpoint, kind, and sequence-style audit commands. Inspect PNG at original resolution with `view_image`. Expected: zero findings, legible labels, no overlap/cropping, correct arrows.

- [ ] **Step 5: Commit**

```bash
git add docs/images/readme-diagrams/redisfory-direct-value-flow.svg \
  docs/images/readme-diagrams/redisfory-direct-value-flow.png \
  cache/redisfory/README.md cache/redisfory/README.ko.md
git commit -m "Make the direct Fory cache trust boundary inspectable" \
  -m "Constraint: README visuals require paired editable and rendered assets." \
  -m "Confidence: high" -m "Scope-risk: narrow" \
  -m "Tested: diagram audits and original-size PNG inspection"
```

### Task 8: Run Verification And Review Gates

**Files:**
- Create: `docs/review/2026-07-10-issue-598-fory-redis-value-cache-implementation-review.md`
- Create: `docs/lessons/2026-07-10-issue-598-fory-redis-value-cache.md`
- Modify only on findings: files from Tasks 1-7

- [ ] **Step 1: Run targeted gates sequentially**

```bash
go test -p 1 -count=1 ./cache/internal/forynative ./cache/rediscoord/fory ./cache/redisfory
go test -race -p 1 -count=1 ./cache/internal/forynative ./cache/rediscoord/fory ./cache/redisfory
go test -run Example -count=1 ./cache/redisfory
go vet ./cache/internal/forynative ./cache/rediscoord/fory ./cache/redisfory
git diff --check
```

Expected: every command exits 0; Docker-backed commands are not parallelized.

- [ ] **Step 2: Run full repository verification**

Run: `make ci`

Expected: PASS for formatting, tidy, vet, lint, tests, and configured checks.

- [ ] **Step 3: Execute Step 6-R**

Run six independent performance, stability, security, operator/Ops, developer/API, and user/caller read-only reviews plus main integration. Record P0/P1/P2/P3 and evidence. Fix P0/P1 test-first and rerun affected lanes to P0=0/P1=0.

- [ ] **Step 4: Execute Step 7-R**

Review the complete branch diff against spec and plan with the same lanes plus main integration. Re-run the smallest proof after late changes, then `make ci` and `git diff --check`. Close only at P0=0/P1=0.

- [ ] **Step 5: Record lessons and evidence**

Document runtime boundaries, dispatch-time cancellation, envelope-first validation, key visibility, and benchmark separation. Do not invent benchmark results; link #599.

- [ ] **Step 6: Commit evidence**

```bash
git add docs/review/2026-07-10-issue-598-fory-redis-value-cache-implementation-review.md \
  docs/lessons/2026-07-10-issue-598-fory-redis-value-cache.md
git commit -m "Retain the evidence behind the direct Fory cache decision" \
  -m "Constraint: Type A delivery requires review and lesson artifacts." \
  -m "Confidence: high" -m "Scope-risk: narrow" \
  -m "Tested: targeted gates; make ci; Step 6-R and Step 7-R P0=0 P1=0"
```

### Task 9: Publish The Pull Request Without Merging

**Files:**
- No production file changes expected.

- [ ] **Step 1: Verify branch hygiene**

Run: `git status --short && git log --oneline origin/develop..HEAD && git diff --check origin/develop...HEAD`

Expected: clean status, intentional Lore commits, no whitespace errors.

Run: `gh issue view 598 --json state,assignees,milestone,labels && gh issue view 599 --json state,assignees,milestone,labels`

Expected: both issues have the intended assignee, milestone, labels, and state before publication.

- [ ] **Step 2: Push and create the issue-linked PR**

The PR body summarizes API/wire/operational boundaries, exact verification, and #599 benchmark ownership, and ends with `## DoD Status`.

- [ ] **Step 3: Verify live metadata and CI**

Use `gh pr view` and `gh pr checks` to verify closing reference, assignee, milestone, final body
section, head SHA, and checks. Fix failures within the approved plan automatically and rerun the
affected gate.

- [ ] **Step 4: Stop at merge approval**

Report PR URL, CI, Step 6-R/7-R counts, and tests. Do not merge without explicit user approval.
