# Issue #182 Redis Probabilistic Filters Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Redis-backed Bloom filter package that shares membership state across Go service instances through plain Redis bitmap commands.

**Architecture:** Keep the public import path at `github.com/bluetape4k/bluetape-go/probabilistic/redis` with package clause `redisbloom`. Reuse `probabilistic.Config`, `Hasher[T]`, and the existing SHA-256 double-hash formula through a small internal helper. Store Redis state in a hash-tagged `{namespace}` key pair, validate immutable metadata with static Lua scripts, and perform bitmap operations in one server-side round trip.

**Tech Stack:** Go, `github.com/redis/go-redis/v9`, Redis Lua scripts through `redis.NewScript.Run`, Redis Testcontainers, `testing/concurrency.GoroutineStressTester`, `testing/concurrency.AsyncJobTester`, Graphviz-backed `$bluetape4k-diagram` assets.

---

## Source Specification

- Spec: `docs/superpowers/specs/2026-06-13-issue-182-redis-probabilistic-filters-design.md`
- Step 2-R evidence: `docs/review/2026-06-13-issue-182-step-2r-spec-review.md`
- Issue: #182
- Branch/worktree: `.worktrees/issue-182-redis-probabilistic-filters`

## File Structure

- Modify `probabilistic/hasher.go`: export `Hasher.Bytes(value T) ([]byte, error)` and route the unexported `bytes` helper through it.
- Create `probabilistic/internal/bloomhash/indexes.go`: shared deterministic Bloom index calculation.
- Create `probabilistic/internal/bloomhash/indexes_test.go`: parity tests for deterministic index behavior.
- Modify `probabilistic/hash.go`: delegate current in-memory index calculation to `bloomhash.Indexes`.
- Modify `probabilistic/bloom_filter.go`: keep in-memory behavior unchanged while using `Hasher.Bytes`.
- Modify `probabilistic/bloom_filter_test.go`: regression coverage for `Hasher.Bytes` and shared index parity.
- Create `probabilistic/redis/doc.go`: package docs and production caveats.
- Create `probabilistic/redis/errors.go`: sentinel errors and `RedisError`.
- Create `probabilistic/redis/options.go`: option normalization, namespace validation, context normalization, config limits.
- Create `probabilistic/redis/keys.go`: Redis Cluster-safe hash-tagged key derivation and redacted key id.
- Create `probabilistic/redis/scripts.go`: static Lua script constants and `redis.NewScript` wrappers.
- Create `probabilistic/redis/config.go`: config fingerprint, config hash encoding/decoding, constructor metadata init.
- Create `probabilistic/redis/filter.go`: public constructors and `BloomFilter[T]` implementation.
- Create `probabilistic/redis/options_test.go`: options, namespace, key layout, and error contract tests.
- Create `probabilistic/redis/config_test.go`: constructor/config/mismatch/corrupt/concurrency tests.
- Create `probabilistic/redis/filter_test.go`: Bloom behavior, no-false-negative, clear, external deletion, command-count tests.
- Create `probabilistic/redis/concurrency_test.go`: `GoroutineStressTester` and `AsyncJobTester` coverage.
- Create `probabilistic/redis/filter_benchmark_test.go`: hot-path benchmarks for `Put` and `MightContain`.
- Create `probabilistic/redis/example_test.go`: caller examples for construction, put/check, clear warning, diagnostics.
- Modify `probabilistic/README.md` and `probabilistic/README.ko.md`: API docs, operational caveats, diagram.
- Modify `README.md` and `README.ko.md`: package table/update note.
- Modify `CHANGELOG.md`: `[Unreleased]` item.
- Modify `WIP.md` only if the file still tracks #182 or 0.6.1 Redis probabilistic work.
- Create `scripts/generate-redis-bloom-diagram.mjs`: deterministic README diagram generator that follows `$bluetape4k-diagram`.
- Create diagram outputs under `docs/images/readme-diagrams/`:
  - `redis-bloom-key-layout-01.dot`
  - `redis-bloom-key-layout-01.plain`
  - `redis-bloom-key-layout-01-graphviz.svg`
  - `redis-bloom-key-layout-01-graphviz.png`
  - `redis-bloom-key-layout-01.svg`
  - `redis-bloom-key-layout-01.png`
- Create `docs/review/2026-06-13-issue-182-code-review.md` during Step 6-R.
- Create `docs/review/2026-06-13-issue-182-pr-review.md` during Step 7-R.

## Step 3-R Plan Review Plan

Before Step 4 implementation, run the mandatory 7-Tier gate as six independent subagent lanes plus main integration:

1. Tier 1 Performance: script round trips, `BITCOUNT` scope, benchmark gates, allocation risk.
2. Tier 2 Stability: cancellation, stale handles, Redis deletion/eviction, Testcontainers serial execution.
3. Tier 3 Security: namespace/key leakage, static Lua, ACL/TLS docs, error redaction.
4. Tier 4 Operator/Ops: Redis Cluster hash tags, runbook, migration, persistence/eviction.
5. Tier 5 Developer/API: Go package shape, errors, context handling, repo fit.
6. Tier 6 User/Caller: examples, misuse resistance, `Put(false)`, `Clear`, Kotlin migration.

Exit condition: latest integrated table shows `P0=0 P1=0`. Store evidence in `docs/review/2026-06-13-issue-182-step-3r-plan-review.md`.

## Step 4-T Implementation Tasks

### Task 1: Shared Hasher and Bloom Index Boundary

**Files:**
- Modify: `probabilistic/hasher.go`
- Modify: `probabilistic/hash.go`
- Modify: `probabilistic/bloom_filter.go`
- Modify: `probabilistic/bloom_filter_test.go`
- Create: `probabilistic/internal/bloomhash/indexes.go`
- Create: `probabilistic/internal/bloomhash/indexes_test.go`

- [ ] **Step 1: Write failing tests for exported hasher bytes**

Add to `probabilistic/bloom_filter_test.go`:

```go
func TestHasherBytesExposesValidatedByteBoundary(t *testing.T) {
	t.Parallel()

	hasher, err := NewHasher("custom:v1", func(value string) []byte {
		return []byte("prefix:" + value)
	})
	require.NoError(t, err)

	bytes, err := hasher.Bytes("alpha")
	require.NoError(t, err)
	require.Equal(t, []byte("prefix:alpha"), bytes)

	var zero Hasher[string]
	_, err = zero.Bytes("alpha")
	require.ErrorIs(t, err, ErrEmptyHasherKey)
}
```

- [ ] **Step 2: Write failing tests for shared index parity**

Create `probabilistic/internal/bloomhash/indexes_test.go`:

```go
package bloomhash

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIndexesAreDeterministic(t *testing.T) {
	t.Parallel()

	first := Indexes([]byte("alpha"), 7, 1024)
	second := Indexes([]byte("alpha"), 7, 1024)

	require.Equal(t, first, second)
	require.Len(t, first, 7)
	for _, index := range first {
		require.Less(t, index, uint64(1024))
	}
}

func TestIndexesHandleZeroSecondHashFallback(t *testing.T) {
	t.Parallel()

	indexes := Indexes([]byte{}, 3, 64)

	require.Len(t, indexes, 3)
	for _, index := range indexes {
		require.Less(t, index, uint64(64))
	}
}
```

- [ ] **Step 3: Run tests and verify failure**

Run:

```bash
go test -count=1 ./probabilistic ./probabilistic/internal/bloomhash -run 'HasherBytes|Indexes'
```

Expected: FAIL because `Hasher.Bytes` and `bloomhash.Indexes` do not exist.

- [ ] **Step 4: Implement `Hasher.Bytes`**

Modify `probabilistic/hasher.go`:

```go
// Bytes validates the hasher and returns stable hash input bytes for value.
func (h Hasher[T]) Bytes(value T) ([]byte, error) {
	if err := h.validate(); err != nil {
		return nil, err
	}
	return h.sum(value), nil
}

func (h Hasher[T]) bytes(value T) ([]byte, error) {
	return h.Bytes(value)
}
```

- [ ] **Step 5: Implement shared index helper**

Create `probabilistic/internal/bloomhash/indexes.go`:

```go
package bloomhash

import (
	"crypto/sha256"
	"encoding/binary"
)

const hash2Fallback = uint64(0x9e3779b97f4a7c15)

// Indexes returns Bloom bit offsets using SHA-256 double hashing.
func Indexes(bytes []byte, hashFunctionCount uint64, bitSize uint64) []uint64 {
	sum := sha256.Sum256(bytes)
	hash1 := binary.BigEndian.Uint64(sum[0:8])
	hash2 := binary.BigEndian.Uint64(sum[8:16])
	if hash2 == 0 {
		hash2 = hash2Fallback
	}

	result := make([]uint64, hashFunctionCount)
	for i := range hashFunctionCount {
		result[i] = (hash1 + i*hash2) % bitSize
	}
	return result
}
```

Modify `probabilistic/hash.go`:

```go
package probabilistic

import "github.com/bluetape4k/bluetape-go/probabilistic/internal/bloomhash"

func indexes(bytes []byte, hashFunctionCount uint64, bitSize uint64) []uint64 {
	return bloomhash.Indexes(bytes, hashFunctionCount, bitSize)
}
```

- [ ] **Step 6: Run focused tests**

Run:

```bash
go test -count=1 ./probabilistic ./probabilistic/internal/bloomhash -run 'HasherBytes|Indexes|Bloom'
```

Expected: PASS.

- [ ] **Step 7: Commit shared boundary**

```bash
git add probabilistic/hasher.go probabilistic/hash.go probabilistic/bloom_filter.go probabilistic/bloom_filter_test.go probabilistic/internal/bloomhash
git commit -m "Expose probabilistic hash input boundary

Constraint: Redis Bloom needs the same hash input and index formula as in-memory Bloom without exporting broad internals.
Rejected: Duplicate Redis-only hashing | would drift from in-memory semantics.
Confidence: high
Scope-risk: narrow
Directive: Keep Bloom index math centralized in probabilistic/internal/bloomhash.
Tested: go test -count=1 ./probabilistic ./probabilistic/internal/bloomhash -run 'HasherBytes|Indexes|Bloom'
Not-tested: Redis-backed package not implemented yet"
```

### Task 2: Redis Package Skeleton, Options, Keys, and Errors

**Files:**
- Create: `probabilistic/redis/doc.go`
- Create: `probabilistic/redis/errors.go`
- Create: `probabilistic/redis/options.go`
- Create: `probabilistic/redis/keys.go`
- Create: `probabilistic/redis/options_test.go`

- [ ] **Step 1: Write failing option and key tests**

Create `probabilistic/redis/options_test.go`:

```go
package redisbloom

import (
	"errors"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/probabilistic"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestBuildKeysUsesClusterHashTag(t *testing.T) {
	t.Parallel()

	keys, err := buildKeys("tenant-a:emails")

	require.NoError(t, err)
	require.Equal(t, "bluetape:probabilistic:bloom:v1:{tenant-a:emails}", keys.slot)
	require.Equal(t, "bluetape:probabilistic:bloom:v1:{tenant-a:emails}:bits", keys.bits)
	require.Equal(t, "bluetape:probabilistic:bloom:v1:{tenant-a:emails}:config", keys.config)
	require.NotContains(t, keys.redactedID, "tenant-a")
}

func TestNormalizeOptionsRejectsInvalidNamespace(t *testing.T) {
	t.Parallel()

	cfg, err := probabilistic.NewConfig(1000, 0.01)
	require.NoError(t, err)

	hasher, err := probabilistic.NewHasher("schema:v1", func(value string) []byte {
		return []byte(value)
	})
	require.NoError(t, err)

	_, err = normalizeOptions(Options[string]{
		Client:    stubCmdable{},
		Namespace: "raw@email.test",
		Config:    cfg,
		Hasher:    hasher,
	})

	require.ErrorIs(t, err, ErrInvalidOptions)
}

func TestNormalizeOptionsRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	validConfig, err := probabilistic.NewConfig(1000, 0.01)
	require.NoError(t, err)
	validHasher, err := probabilistic.NewHasher("schema:v1", func(value string) []byte {
		return []byte(value)
	})
	require.NoError(t, err)

	tests := []struct {
		name    string
		options Options[string]
	}{
		{
			name: "nil client",
			options: Options[string]{
				Namespace: "tenant-a:emails",
				Config:    validConfig,
				Hasher:    validHasher,
			},
		},
		{
			name: "typed nil client",
			options: Options[string]{
				Client:    (*redis.Client)(nil),
				Namespace: "tenant-a:emails",
				Config:    validConfig,
				Hasher:    validHasher,
			},
		},
		{
			name: "invalid config",
			options: Options[string]{
				Client:    stubCmdable{},
				Namespace: "tenant-a:emails",
				Config:    probabilistic.Config{},
				Hasher:    validHasher,
			},
		},
		{
			name: "empty hasher",
			options: Options[string]{
				Client:    stubCmdable{},
				Namespace: "tenant-a:emails",
				Config:    validConfig,
			},
		},
		{
			name: "namespace leading whitespace",
			options: Options[string]{
				Client:    stubCmdable{},
				Namespace: " tenant-a:emails",
				Config:    validConfig,
				Hasher:    validHasher,
			},
		},
		{
			name: "namespace trailing whitespace",
			options: Options[string]{
				Client:    stubCmdable{},
				Namespace: "tenant-a:emails ",
				Config:    validConfig,
				Hasher:    validHasher,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := normalizeOptions(tt.options)

			require.ErrorIs(t, err, ErrInvalidOptions)
		})
	}
}

func TestValidateNamespaceRejectsUnsafeNames(t *testing.T) {
	t.Parallel()

	tooLong := strings.Repeat("a", 129)
	for _, namespace := range []string{
		"",
		" tenant-a",
		"tenant-a ",
		":tenant-a",
		"tenant-a:",
		"tenant-a:bits",
		"tenant-a:config",
		"tenant{a}",
		"tenant-a:사용자",
		"alice@example.test",
		"token:secret",
		tooLong,
	} {
		t.Run(namespace, func(t *testing.T) {
			t.Parallel()

			require.Error(t, validateNamespace(namespace))
		})
	}
}

func TestHasherKeyRejectsSensitiveOrUnsafeNames(t *testing.T) {
	t.Parallel()

	cfg, err := probabilistic.NewConfig(1000, 0.01)
	require.NoError(t, err)
	for _, key := range []string{"", " schema:v1", "schema:v1 ", "schema\nv1", strings.Repeat("a", 129), "alice@example.test"} {
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			hasher, _ := probabilistic.NewHasher(key, func(value string) []byte { return []byte(value) })

			_, err := normalizeOptions(Options[string]{
				Client:    stubCmdable{},
				Namespace: "tenant-a:emails",
				Config:    cfg,
				Hasher:    hasher,
			})

			require.ErrorIs(t, err, ErrInvalidOptions)
		})
	}
}

func TestRedisErrorSupportsErrorsAs(t *testing.T) {
	t.Parallel()

	cause := errors.New("boom")
	err := RedisError{Operation: "put", KeyID: "redis-key:abc123", Err: cause}

	require.ErrorIs(t, err, cause)
	var redisErr RedisError
	require.ErrorAs(t, err, &redisErr)
	require.NotContains(t, err.Error(), "tenant-a")
}

func TestDefaultErrorsDoNotExposeSensitiveInputs(t *testing.T) {
	t.Parallel()

	err := RedisError{Operation: "put", KeyID: "redis-key:abc123", Err: errors.New("dial failure")}

	require.NotContains(t, err.Error(), "tenant-a:emails")
	require.NotContains(t, err.Error(), "bluetape:probabilistic")
	require.NotContains(t, err.Error(), "probabilistic:string:v1")
	require.NotContains(t, err.Error(), "inserted@example.test")
	require.Contains(t, err.Error(), "redis-key:abc123")
}

func TestMappedScriptErrorsDoNotExposeSensitiveInputs(t *testing.T) {
	t.Parallel()

	for _, cause := range []error{
		errors.New("ERR config_mismatch"),
		errors.New("ERR config_corrupt"),
	} {
		err := mapScriptError("put", "redis-key:abc123", cause)

		require.NotContains(t, err.Error(), "tenant-a:emails")
		require.NotContains(t, err.Error(), "bluetape:probabilistic")
		require.NotContains(t, err.Error(), "schema:v1")
		require.NotContains(t, err.Error(), "inserted@example.test")
		require.Contains(t, err.Error(), "redis-key:abc123")
	}
}

func TestUnrelatedRedisErrorsRemainRedisErrors(t *testing.T) {
	t.Parallel()

	cause := errors.New("network failure while reading config_mismatch telemetry")
	err := mapScriptError("put", "redis-key:abc123", cause)

	var redisErr RedisError
	require.ErrorAs(t, err, &redisErr)
	require.ErrorIs(t, err, cause)
}
```

Add this local `stubCmdable` in `options_test.go`:

```go
type stubCmdable struct {
	redis.Cmdable
}
```

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
go test -count=1 ./probabilistic/redis -run 'Options|Keys|RedisError'
```

Expected: FAIL because package files do not exist.

- [ ] **Step 3: Add package docs and errors**

Create `probabilistic/redis/doc.go`:

```go
// Package redisbloom provides Redis-backed Bloom filters using ordinary Redis
// bitmap commands and immutable metadata.
//
// Filters in this package store shared distributed state. A false result means
// a value is not present unless Redis keys were cleared, deleted, evicted, or
// overwritten after insertion. A true result may be a false positive.
package redisbloom
```

Create `probabilistic/redis/errors.go`:

```go
package redisbloom

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrInvalidOptions = errors.New("redis bloom: invalid options")
	ErrConfigMismatch = errors.New("redis bloom: config mismatch")
	ErrConfigCorrupt  = errors.New("redis bloom: config corrupt")
)

type RedisError struct {
	Operation string
	KeyID     string
	Err       error
}

func (e RedisError) Error() string {
	if e.KeyID == "" {
		return fmt.Sprintf("redis bloom %s: %v", e.Operation, e.Err)
	}
	return fmt.Sprintf("redis bloom %s %s: %v", e.Operation, e.KeyID, e.Err)
}

func (e RedisError) Unwrap() error {
	return e.Err
}
```

- [ ] **Step 4: Add key derivation and option normalization**

Create `probabilistic/redis/keys.go`:

```go
package redisbloom

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const keyPrefix = "bluetape:probabilistic:bloom:v1"

type redisKeys struct {
	slot       string
	bits       string
	config     string
	redactedID string
}

func buildKeys(namespace string) (redisKeys, error) {
	if err := validateNamespace(namespace); err != nil {
		return redisKeys{}, err
	}
	slot := fmt.Sprintf("%s:{%s}", keyPrefix, namespace)
	sum := sha256.Sum256([]byte(slot))
	return redisKeys{
		slot:       slot,
		bits:       slot + ":bits",
		config:     slot + ":config",
		redactedID: "redis-key:" + hex.EncodeToString(sum[:6]),
	}, nil
}
```

Create `probabilistic/redis/options.go`:

```go
package redisbloom

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/bluetape4k/bluetape-go/probabilistic"
	"github.com/redis/go-redis/v9"
)

const redisMaxBitOffset = uint64(1) << 32

type Options[T any] struct {
	Client    redis.Cmdable
	Namespace string
	Config    probabilistic.Config
	Hasher    probabilistic.Hasher[T]
}

type normalizedOptions[T any] struct {
	client redis.Cmdable
	keys   redisKeys
	config probabilistic.Config
	hasher probabilistic.Hasher[T]
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func normalizeOptions[T any](options Options[T]) (normalizedOptions[T], error) {
	if isNilClient(options.Client) {
		return normalizedOptions[T]{}, fmt.Errorf("%w: nil redis client", ErrInvalidOptions)
	}
	if options.Config.ExpectedInsertions() == 0 || options.Config.FalsePositiveProbability() <= 0 ||
		options.Config.BitSize() == 0 || options.Config.HashFunctionCount() == 0 {
		return normalizedOptions[T]{}, fmt.Errorf("%w: invalid config", ErrInvalidOptions)
	}
	if options.Config.BitSize() >= redisMaxBitOffset {
		return normalizedOptions[T]{}, fmt.Errorf("%w: bit size exceeds redis bitmap offset limit", ErrInvalidOptions)
	}
	if err := validateIdentifier("hasher key", options.Hasher.Key()); err != nil {
		return normalizedOptions[T]{}, fmt.Errorf("%w: %w", ErrInvalidOptions, err)
	}
	keys, err := buildKeys(options.Namespace)
	if err != nil {
		return normalizedOptions[T]{}, fmt.Errorf("%w: namespace: %w", ErrInvalidOptions, err)
	}
	return normalizedOptions[T]{client: options.Client, keys: keys, config: options.Config, hasher: options.Hasher}, nil
}

func isNilClient(client redis.Cmdable) bool {
	if client == nil {
		return true
	}
	value := reflect.ValueOf(client)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func validateIdentifier(kind, value string) error {
	if value == "" || value != strings.TrimSpace(value) || len(value) > 128 || !utf8.ValidString(value) {
		return fmt.Errorf("%s: invalid length or whitespace", kind)
	}
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-' || r == ':' {
			continue
		}
		return fmt.Errorf("%s: invalid character %q", kind, r)
	}
	if strings.Contains(value, "@") || strings.Contains(strings.ToLower(value), "token") {
		return fmt.Errorf("%s: use a non-sensitive schema identifier", kind)
	}
	return nil
}

func validateNamespace(namespace string) error {
	if namespace == "" {
		return fmt.Errorf("empty")
	}
	if namespace != strings.TrimSpace(namespace) || len(namespace) > 128 || !utf8.ValidString(namespace) {
		return fmt.Errorf("invalid length or whitespace")
	}
	if strings.HasPrefix(namespace, ":") || strings.HasSuffix(namespace, ":") {
		return fmt.Errorf("invalid colon boundary")
	}
	if strings.HasSuffix(namespace, ":bits") || strings.HasSuffix(namespace, ":config") {
		return fmt.Errorf("reserved suffix")
	}
	for _, r := range namespace {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-' || r == ':' {
			continue
		}
		return fmt.Errorf("invalid character %q", r)
	}
	return nil
}
```

- [ ] **Step 5: Run focused tests**

Run:

```bash
go test -count=1 ./probabilistic/redis -run 'Options|Keys|RedisError'
```

Expected: PASS.

- [ ] **Step 6: Commit skeleton**

```bash
git add probabilistic/redis
git commit -m "Define Redis Bloom package boundary

Constraint: Redis Bloom must avoid go-redis package-name collision and Redis Cluster multi-key slot failures.
Rejected: package redis | conflicts with github.com/redis/go-redis/v9 imports.
Confidence: high
Scope-risk: narrow
Directive: Keep Redis key names hash-tagged and default errors redacted.
Tested: go test -count=1 ./probabilistic/redis -run 'Options|Keys|RedisError'
Not-tested: Redis integration behavior not implemented yet"
```

### Task 3: Metadata Fingerprint and Constructor Atomicity

**Files:**
- Create: `probabilistic/redis/scripts.go`
- Create: `probabilistic/redis/config.go`
- Create: `probabilistic/redis/config_test.go`
- Modify: `probabilistic/redis/options.go`

- [ ] **Step 1: Write constructor/config tests**

Create `probabilistic/redis/config_test.go`:

```go
package redisbloom

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/bluetape4k/bluetape-go/probabilistic"
	redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	ctx := context.Background()
	addr := redistestcontainer.Start(ctx, t)
	client := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() {
		require.NoError(t, client.Close())
	})
	require.NoError(t, client.Ping(ctx).Err())
	return client
}

func testConfig(t *testing.T, expected uint64) probabilistic.Config {
	t.Helper()
	cfg, err := probabilistic.NewConfig(expected, 0.01)
	require.NoError(t, err)
	return cfg
}

func testNamespace(t *testing.T) string {
	t.Helper()
	return "test:" + strings.ReplaceAll(t.Name(), "/", ":")
}

func cleanupNamespace(t *testing.T, client redis.Cmdable, namespace string) {
	t.Helper()
	keys, err := buildKeys(namespace)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Del(context.Background(), keys.bits, keys.config).Err())
	})
}

func TestNewBloomFilterInitializesAndReusesConfig(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(t)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)

	filter, err := NewStringBloomFilter(ctx, client, namespace, testConfig(t, 1000))
	require.NoError(t, err)
	require.Equal(t, uint64(1000), filter.ExpectedInsertions())

	again, err := NewStringBloomFilter(ctx, client, namespace, testConfig(t, 1000))
	require.NoError(t, err)
	require.Equal(t, filter.BitSize(), again.BitSize())
}

func TestNewBloomFilterRejectsChangedConfig(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(t)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)

	_, err := NewStringBloomFilter(ctx, client, namespace, testConfig(t, 1000))
	require.NoError(t, err)

	_, err = NewStringBloomFilter(ctx, client, namespace, testConfig(t, 2000))
	require.ErrorIs(t, err, ErrConfigMismatch)
}

func TestNewBloomFilterRejectsCorruptMetadataWithBitmap(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(t)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)

	filter, err := NewStringBloomFilter(ctx, client, namespace, testConfig(t, 1000))
	require.NoError(t, err)
	changed, err := filter.Put(ctx, "alpha")
	require.NoError(t, err)
	require.True(t, changed)

	keys, err := buildKeys(namespace)
	require.NoError(t, err)
	require.NoError(t, client.HDel(ctx, keys.config, "fingerprint").Err())

	_, err = NewStringBloomFilter(ctx, client, namespace, testConfig(t, 1000))
	require.ErrorIs(t, err, ErrConfigCorrupt)
}

func TestNewBloomFilterRejectsMissingConfigWithBitmap(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(t)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)

	filter, err := NewStringBloomFilter(ctx, client, namespace, testConfig(t, 1000))
	require.NoError(t, err)
	changed, err := filter.Put(ctx, "alpha")
	require.NoError(t, err)
	require.True(t, changed)

	keys, err := buildKeys(namespace)
	require.NoError(t, err)
	require.NoError(t, client.Del(ctx, keys.config).Err())

	_, err = NewStringBloomFilter(ctx, client, namespace, testConfig(t, 1000))
	require.ErrorIs(t, err, ErrConfigCorrupt)
}

func TestNewBloomFilterRejectsPartialMetadataEvenWithFingerprint(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(t)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)

	filter, err := NewStringBloomFilter(ctx, client, namespace, testConfig(t, 1000))
	require.NoError(t, err)

	keys, err := buildKeys(namespace)
	require.NoError(t, err)
	fingerprint := client.HGet(ctx, keys.config, "fingerprint").Val()
	require.NoError(t, client.Del(ctx, keys.config).Err())
	require.NoError(t, client.HSet(ctx, keys.config, "fingerprint", fingerprint, "expected_insertions", "1000").Err())

	_, err = NewStringBloomFilter(ctx, client, namespace, testConfig(t, 1000))
	require.ErrorIs(t, err, ErrConfigCorrupt)
	require.Equal(t, filter.HasherKey(), "probabilistic:string:v1")
}

func TestConcurrentIncompatibleConstructorsLeaveOneConfig(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(t)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, expected := range []uint64{1000, 2000} {
		wg.Add(1)
		go func(expected uint64) {
			defer wg.Done()
			_, err := NewStringBloomFilter(ctx, client, namespace, testConfig(t, expected))
			errs <- err
		}(expected)
	}
	wg.Wait()
	close(errs)

	var success, mismatch int
	for err := range errs {
		if err == nil {
			success++
			continue
		}
		if errors.Is(err, ErrConfigMismatch) {
			mismatch++
		}
	}
	require.Equal(t, 1, success)
	require.Equal(t, 1, mismatch)

	keys, err := buildKeys(namespace)
	require.NoError(t, err)
	require.Zero(t, client.Exists(ctx, keys.bits).Val())
	metadata := client.HGetAll(ctx, keys.config).Val()
	require.Len(t, metadata, 8)
	require.Equal(t, "1", metadata["version"])
	require.Equal(t, "redis-bloom", metadata["family"])
	require.Contains(t, []string{"1000", "2000"}, metadata["expected_insertions"])
	require.NotEmpty(t, metadata["false_positive_probability"])
	require.NotEmpty(t, metadata["bit_size"])
	require.NotEmpty(t, metadata["hash_function_count"])
	require.Equal(t, "probabilistic:string:v1", metadata["hasher_key"])
	require.NotEmpty(t, metadata["fingerprint"])
}
```

- [ ] **Step 2: Run constructor tests and verify failure**

Run:

```bash
go test -p 1 -count=1 ./probabilistic/redis -run 'NewBloomFilter|ConcurrentIncompatible'
```

Expected: FAIL because constructors and scripts are not implemented.

- [ ] **Step 3: Add static script wrappers**

Create `probabilistic/redis/scripts.go`:

```go
package redisbloom

import "github.com/redis/go-redis/v9"

const initConfigLua = `
local existing = redis.call("HGETALL", KEYS[2])
if #existing == 0 then
	if redis.call("STRLEN", KEYS[1]) > 0 then
		return redis.error_reply("config_corrupt")
	end
	redis.call("HSET", KEYS[2],
		"version", ARGV[1],
		"family", ARGV[2],
		"expected_insertions", ARGV[3],
		"false_positive_probability", ARGV[4],
		"bit_size", ARGV[5],
		"hash_function_count", ARGV[6],
		"hasher_key", ARGV[7],
		"fingerprint", ARGV[8])
	return "created"
end
local stored = redis.call("HGET", KEYS[2], "fingerprint")
if stored == false then
	return redis.error_reply("config_corrupt")
end
if stored ~= ARGV[8] then
	return redis.error_reply("config_mismatch")
end
if redis.call("HGET", KEYS[2], "version") ~= ARGV[1] then return redis.error_reply("config_corrupt") end
if redis.call("HGET", KEYS[2], "family") ~= ARGV[2] then return redis.error_reply("config_corrupt") end
if redis.call("HGET", KEYS[2], "expected_insertions") ~= ARGV[3] then return redis.error_reply("config_corrupt") end
if redis.call("HGET", KEYS[2], "false_positive_probability") ~= ARGV[4] then return redis.error_reply("config_corrupt") end
if redis.call("HGET", KEYS[2], "bit_size") ~= ARGV[5] then return redis.error_reply("config_corrupt") end
if redis.call("HGET", KEYS[2], "hash_function_count") ~= ARGV[6] then return redis.error_reply("config_corrupt") end
if redis.call("HGET", KEYS[2], "hasher_key") ~= ARGV[7] then return redis.error_reply("config_corrupt") end
return "matched"
`

var initConfigScript = redis.NewScript(initConfigLua)
```

Add script constants for `putLua`, `mightContainLua`, `clearLua`, and `bitCountLua` in Task 4, not here.

- [ ] **Step 4: Add metadata encoding and constructor**

Create `probabilistic/redis/config.go` with these functions:

```go
package redisbloom

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/bluetape4k/bluetape-go/probabilistic"
	"github.com/redis/go-redis/v9"
)

const (
	configVersion = "1"
	configFamily  = "redis-bloom"
)

type metadata struct {
	expectedInsertions       uint64
	falsePositiveProbability float64
	bitSize                  uint64
	hashFunctionCount        uint64
	hasherKey                string
	fingerprint              string
}

func newMetadata(cfg probabilistic.Config, hasherKey string) metadata {
	fpInput := fmt.Sprintf("%s|%s|%d|%g|%d|%d|%s",
		configVersion,
		configFamily,
		cfg.ExpectedInsertions(),
		cfg.FalsePositiveProbability(),
		cfg.BitSize(),
		cfg.HashFunctionCount(),
		hasherKey,
	)
	sum := sha256.Sum256([]byte(fpInput))
	return metadata{
		expectedInsertions:       cfg.ExpectedInsertions(),
		falsePositiveProbability: cfg.FalsePositiveProbability(),
		bitSize:                  cfg.BitSize(),
		hashFunctionCount:        cfg.HashFunctionCount(),
		hasherKey:                hasherKey,
		fingerprint:              hex.EncodeToString(sum[:]),
	}
}

func (m metadata) argv() []any {
	return []any{
		configVersion,
		configFamily,
		strconv.FormatUint(m.expectedInsertions, 10),
		strconv.FormatFloat(m.falsePositiveProbability, 'g', -1, 64),
		strconv.FormatUint(m.bitSize, 10),
		strconv.FormatUint(m.hashFunctionCount, 10),
		m.hasherKey,
		m.fingerprint,
	}
}

func initializeConfig(ctx context.Context, client redis.Cmdable, keys redisKeys, meta metadata) error {
	ctx = normalizeContext(ctx)
	if err := initConfigScript.Run(ctx, client, []string{keys.bits, keys.config}, meta.argv()...).Err(); err != nil {
		return mapScriptError("init", keys.redactedID, err)
	}
	return nil
}
```

Add `mapScriptError` to `errors.go`:

```go
func mapScriptError(operation string, keyID string, err error) error {
	if err == nil {
		return nil
	}
	switch scriptErrorMarker(err) {
	case "config_mismatch":
		return fmt.Errorf("%w: %s", ErrConfigMismatch, keyID)
	case "config_corrupt":
		return fmt.Errorf("%w: %s", ErrConfigCorrupt, keyID)
	default:
		return RedisError{Operation: operation, KeyID: keyID, Err: err}
	}
}

func scriptErrorMarker(err error) string {
	message := strings.TrimSpace(err.Error())
	message = strings.TrimPrefix(message, "ERR ")
	switch message {
	case "config_mismatch", "config_corrupt":
		return message
	default:
		return ""
	}
}
```

- [ ] **Step 5: Add constructors with metadata initialization**

Create the constructor part of `probabilistic/redis/filter.go`:

```go
package redisbloom

import (
	"context"

	"github.com/bluetape4k/bluetape-go/probabilistic"
	"github.com/redis/go-redis/v9"
)

type BloomFilter[T any] interface {
	ExpectedInsertions() uint64
	FalsePositiveProbability() float64
	BitSize() uint64
	HashFunctionCount() uint64
	HasherKey() string
	BitCount(ctx context.Context) (uint64, error)
	IsEmpty(ctx context.Context) (bool, error)
	MightContain(ctx context.Context, value T) (bool, error)
	Put(ctx context.Context, value T) (bool, error)
	ApproximateElementCount(ctx context.Context) (uint64, error)
	ExpectedFPP(ctx context.Context) (float64, error)
	Clear(ctx context.Context) error
}

type bloomFilter[T any] struct {
	client redis.Cmdable
	keys   redisKeys
	config probabilistic.Config
	hasher probabilistic.Hasher[T]
	meta   metadata
}

func NewBloomFilter[T any](ctx context.Context, options Options[T]) (BloomFilter[T], error) {
	normalized, err := normalizeOptions(options)
	if err != nil {
		return nil, err
	}
	meta := newMetadata(normalized.config, normalized.hasher.Key())
	if err := initializeConfig(ctx, normalized.client, normalized.keys, meta); err != nil {
		return nil, err
	}
	return &bloomFilter[T]{
		client: normalized.client,
		keys:   normalized.keys,
		config: normalized.config,
		hasher: normalized.hasher,
		meta:   meta,
	}, nil
}

func NewStringBloomFilter(ctx context.Context, client redis.Cmdable, namespace string, cfg probabilistic.Config) (BloomFilter[string], error) {
	hasher, err := probabilistic.NewHasher("probabilistic:string:v1", func(value string) []byte {
		return []byte(value)
	})
	if err != nil {
		return nil, err
	}
	return NewBloomFilter(ctx, Options[string]{Client: client, Namespace: namespace, Config: cfg, Hasher: hasher})
}

func NewBytesBloomFilter(ctx context.Context, client redis.Cmdable, namespace string, cfg probabilistic.Config) (BloomFilter[[]byte], error) {
	hasher, err := probabilistic.NewHasher("probabilistic:bytes:v1", func(value []byte) []byte {
		copied := make([]byte, len(value))
		copy(copied, value)
		return copied
	})
	if err != nil {
		return nil, err
	}
	return NewBloomFilter(ctx, Options[[]byte]{Client: client, Namespace: namespace, Config: cfg, Hasher: hasher})
}
```

- [ ] **Step 6: Run constructor tests**

Run:

```bash
go test -p 1 -count=1 ./probabilistic/redis -run 'NewBloomFilter|ConcurrentIncompatible'
```

Expected: PASS.

- [ ] **Step 7: Commit constructor**

```bash
git add probabilistic/redis
git commit -m "Initialize Redis Bloom metadata atomically

Constraint: Concurrent constructors must not create incompatible Redis Bloom metadata.
Rejected: HGETALL plus HSET outside Lua | leaves compare-create race.
Confidence: high
Scope-risk: moderate
Directive: Keep metadata immutable and fingerprint-checked before bitmap operations.
Tested: go test -p 1 -count=1 ./probabilistic/redis -run 'NewBloomFilter|ConcurrentIncompatible'
Not-tested: Full bitmap operation suite not implemented yet"
```

### Task 4: Redis Bloom Operations

**Files:**
- Modify: `probabilistic/redis/scripts.go`
- Modify: `probabilistic/redis/filter.go`
- Create: `probabilistic/redis/filter_test.go`

- [ ] **Step 1: Write failing operation tests**

Create `probabilistic/redis/filter_test.go`:

```go
package redisbloom

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestPutAndMightContainHaveNoFalseNegative(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(t)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)
	filter, err := NewStringBloomFilter(ctx, client, namespace, testConfig(t, 1000))
	require.NoError(t, err)

	changed, err := filter.Put(ctx, "alpha")
	require.NoError(t, err)
	require.True(t, changed)
	changed, err = filter.Put(ctx, "alpha")
	require.NoError(t, err)
	require.False(t, changed)

	ok, err := filter.MightContain(ctx, "alpha")
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = filter.MightContain(ctx, "definitely-missing")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestClearPreservesConfigAndClearsBitmap(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(t)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)
	filter, err := NewStringBloomFilter(ctx, client, namespace, testConfig(t, 1000))
	require.NoError(t, err)
	changed, err := filter.Put(ctx, "alpha")
	require.NoError(t, err)
	require.True(t, changed)

	require.NoError(t, filter.Clear(ctx))

	empty, err := filter.IsEmpty(ctx)
	require.NoError(t, err)
	require.True(t, empty)

	keys, err := buildKeys(namespace)
	require.NoError(t, err)
	require.True(t, client.Exists(ctx, keys.config).Val() == 1)
}

func TestExternalBitmapDeletionCreatesEmptyState(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(t)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)
	filter, err := NewStringBloomFilter(ctx, client, namespace, testConfig(t, 1000))
	require.NoError(t, err)
	changed, err := filter.Put(ctx, "alpha")
	require.NoError(t, err)
	require.True(t, changed)

	keys, err := buildKeys(namespace)
	require.NoError(t, err)
	require.NoError(t, client.Del(ctx, keys.bits).Err())

	ok, err := filter.MightContain(ctx, "alpha")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestOperationsRejectChangedConfigBeforeBitmapTouch(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(t)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)
	filter, err := NewStringBloomFilter(ctx, client, namespace, testConfig(t, 1000))
	require.NoError(t, err)

	keys, err := buildKeys(namespace)
	require.NoError(t, err)
	require.NoError(t, client.HSet(ctx, keys.config, "fingerprint", "changed").Err())

	changed, err := filter.Put(ctx, "alpha")
	require.False(t, changed)
	require.ErrorIs(t, err, ErrConfigMismatch)
	ok, err := filter.MightContain(ctx, "alpha")
	require.False(t, ok)
	require.ErrorIs(t, err, ErrConfigMismatch)
	require.Zero(t, client.StrLen(ctx, keys.bits).Val())
}

func TestConcurrentPutAndMightContain(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(t)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)
	filter, err := NewStringBloomFilter(ctx, client, namespace, testConfig(t, 10000))
	require.NoError(t, err)

	var wg sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				value := strconv.Itoa(worker) + ":" + strconv.Itoa(i)
				_, putErr := filter.Put(ctx, value)
				require.NoError(t, putErr)
				ok, containsErr := filter.MightContain(ctx, value)
				require.NoError(t, containsErr)
				require.True(t, ok)
			}
		}(worker)
	}
	wg.Wait()
}

func TestContextCancellationVisible(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := newRedisClient(t)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)
	filter, err := NewStringBloomFilter(context.Background(), client, namespace, testConfig(t, 1000))
	require.NoError(t, err)

	_, err = filter.MightContain(ctx, "alpha")
	require.True(t, errors.Is(err, context.Canceled))
}

func TestDeadlineExceededVisible(t *testing.T) {
	client := newRedisClient(t)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)
	filter, err := NewStringBloomFilter(context.Background(), client, namespace, testConfig(t, 1000))
	require.NoError(t, err)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	_, err = filter.MightContain(ctx, "alpha")
	require.True(t, errors.Is(err, context.DeadlineExceeded))
}
```

- [ ] **Step 2: Run operation tests and verify failure**

Run:

```bash
go test -p 1 -count=1 ./probabilistic/redis -run 'Put|MightContain|Clear|ExternalBitmap|OperationsReject|ContextCancellation|DeadlineExceeded'
```

Expected: FAIL because operation scripts and methods are not implemented.

- [ ] **Step 3: Add static operation scripts**

Append to `probabilistic/redis/scripts.go`:

```go
const putLua = `
local stored = redis.call("HGET", KEYS[2], "fingerprint")
if stored == false then return redis.error_reply("config_corrupt") end
if stored ~= ARGV[1] then return redis.error_reply("config_mismatch") end
local changed = 0
for i = 2, #ARGV do
	if redis.call("GETBIT", KEYS[1], ARGV[i]) == 0 then
		changed = 1
	end
	redis.call("SETBIT", KEYS[1], ARGV[i], 1)
end
return changed
`

const mightContainLua = `
local stored = redis.call("HGET", KEYS[2], "fingerprint")
if stored == false then return redis.error_reply("config_corrupt") end
if stored ~= ARGV[1] then return redis.error_reply("config_mismatch") end
for i = 2, #ARGV do
	if redis.call("GETBIT", KEYS[1], ARGV[i]) == 0 then
		return 0
	end
end
return 1
`

const clearLua = `
local stored = redis.call("HGET", KEYS[2], "fingerprint")
if stored == false then return redis.error_reply("config_corrupt") end
if stored ~= ARGV[1] then return redis.error_reply("config_mismatch") end
return redis.call("DEL", KEYS[1])
`

const bitCountLua = `
local stored = redis.call("HGET", KEYS[2], "fingerprint")
if stored == false then return redis.error_reply("config_corrupt") end
if stored ~= ARGV[1] then return redis.error_reply("config_mismatch") end
return redis.call("BITCOUNT", KEYS[1], 0, ARGV[2])
`

const isEmptyLua = `
local stored = redis.call("HGET", KEYS[2], "fingerprint")
if stored == false then return redis.error_reply("config_corrupt") end
if stored ~= ARGV[1] then return redis.error_reply("config_mismatch") end
return redis.call("STRLEN", KEYS[1])
`

var (
	putScript          = redis.NewScript(putLua)
	mightContainScript = redis.NewScript(mightContainLua)
	clearScript        = redis.NewScript(clearLua)
	bitCountScript     = redis.NewScript(bitCountLua)
	isEmptyScript      = redis.NewScript(isEmptyLua)
)
```

- [ ] **Step 4: Implement methods**

Append to `probabilistic/redis/filter.go`:

```go
func (f *bloomFilter[T]) ExpectedInsertions() uint64 {
	return f.config.ExpectedInsertions()
}

func (f *bloomFilter[T]) FalsePositiveProbability() float64 {
	return f.config.FalsePositiveProbability()
}

func (f *bloomFilter[T]) BitSize() uint64 {
	return f.config.BitSize()
}

func (f *bloomFilter[T]) HashFunctionCount() uint64 {
	return f.config.HashFunctionCount()
}

func (f *bloomFilter[T]) HasherKey() string {
	return f.hasher.Key()
}

func (f *bloomFilter[T]) offsets(value T) ([]any, error) {
	bytes, err := f.hasher.Bytes(value)
	if err != nil {
		return nil, err
	}
	indexes := bloomhash.Indexes(bytes, f.config.HashFunctionCount(), f.config.BitSize())
	args := make([]any, 0, len(indexes)+1)
	args = append(args, f.meta.fingerprint)
	for _, index := range indexes {
		args = append(args, strconv.FormatUint(index, 10))
	}
	return args, nil
}

func (f *bloomFilter[T]) Put(ctx context.Context, value T) (bool, error) {
	args, err := f.offsets(value)
	if err != nil {
		return false, err
	}
	result, err := putScript.Run(normalizeContext(ctx), f.client, []string{f.keys.bits, f.keys.config}, args...).Int()
	if err != nil {
		return false, mapScriptError("put", f.keys.redactedID, err)
	}
	return result == 1, nil
}

func (f *bloomFilter[T]) MightContain(ctx context.Context, value T) (bool, error) {
	args, err := f.offsets(value)
	if err != nil {
		return false, err
	}
	result, err := mightContainScript.Run(normalizeContext(ctx), f.client, []string{f.keys.bits, f.keys.config}, args...).Int()
	if err != nil {
		return false, mapScriptError("might contain", f.keys.redactedID, err)
	}
	return result == 1, nil
}

func (f *bloomFilter[T]) Clear(ctx context.Context) error {
	if err := clearScript.Run(normalizeContext(ctx), f.client, []string{f.keys.bits, f.keys.config}, f.meta.fingerprint).Err(); err != nil {
		return mapScriptError("clear", f.keys.redactedID, err)
	}
	return nil
}

func (f *bloomFilter[T]) BitCount(ctx context.Context) (uint64, error) {
	lastByte := strconv.FormatUint((f.config.BitSize()-1)/8, 10)
	result, err := bitCountScript.Run(normalizeContext(ctx), f.client, []string{f.keys.bits, f.keys.config}, f.meta.fingerprint, lastByte).Uint64()
	if err != nil {
		return 0, mapScriptError("bit count", f.keys.redactedID, err)
	}
	return result, nil
}

func (f *bloomFilter[T]) IsEmpty(ctx context.Context) (bool, error) {
	result, err := isEmptyScript.Run(normalizeContext(ctx), f.client, []string{f.keys.bits, f.keys.config}, f.meta.fingerprint).Int()
	if err != nil {
		return false, mapScriptError("is empty", f.keys.redactedID, err)
	}
	return result == 0, nil
}
```

Add imports for `math`, `strconv`, and `github.com/bluetape4k/bluetape-go/probabilistic/internal/bloomhash`. Then implement `ApproximateElementCount` and `ExpectedFPP` using `BitCount` and the same formulas as in-memory Bloom. `BitCount` must pass `(f.config.BitSize()-1)/8` as `ARGV[2]` so Redis scans only the bitmap's configured byte range. `IsEmpty` must keep using `STRLEN` and must not call `BITCOUNT`.

- [ ] **Step 5: Run operation tests**

Run:

```bash
go test -p 1 -count=1 ./probabilistic/redis -run 'Put|MightContain|Clear|ExternalBitmap|OperationsReject|ContextCancellation|DeadlineExceeded'
```

Expected: PASS.

- [ ] **Step 6: Commit operations**

```bash
git add probabilistic/redis
git commit -m "Implement Redis Bloom bitmap operations

Constraint: Redis bitmap operations must validate metadata and touch bitmap state atomically.
Rejected: HGET plus GETBIT/SETBIT command loops | permits stale-handle races and excessive round trips.
Confidence: high
Scope-risk: moderate
Directive: Keep operation Lua scripts static and pass caller data only through KEYS/ARGV.
Tested: go test -p 1 -count=1 ./probabilistic/redis -run 'Put|MightContain|Clear|ExternalBitmap|OperationsReject|ContextCancellation|DeadlineExceeded'
Not-tested: Race/stress/benchmark/doc gates pending"
```

### Task 5: Stress, Cancellation, Command Count, and Benchmarks

**Files:**
- Modify: `probabilistic/redis/filter_test.go`
- Create: `probabilistic/redis/concurrency_test.go`
- Create: `probabilistic/redis/filter_benchmark_test.go`

- [ ] **Step 1: Add go-redis command recorder hook**

Add to `probabilistic/redis/filter_test.go`:

```go
type commandRecorder struct {
	mu       sync.Mutex
	commands []string
}

func (r *commandRecorder) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (r *commandRecorder) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		r.mu.Lock()
		r.commands = append(r.commands, cmd.Name())
		r.mu.Unlock()
		return next(ctx, cmd)
	}
}

func (r *commandRecorder) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func (r *commandRecorder) Count(name string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, command := range r.commands {
		if command == name {
			count++
		}
	}
	return count
}

func (r *commandRecorder) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.commands = nil
}

func (r *commandRecorder) ScriptDelta() int {
	return r.Count("evalsha") + r.Count("eval")
}
```

- [ ] **Step 2: Add command-count test**

Add to `probabilistic/redis/filter_test.go`:

```go
func TestHotPathUsesOneScriptRoundTripPerCall(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(t)
	recorder := &commandRecorder{}
	client.AddHook(recorder)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)

	cfg := testConfig(t, 1000)
	filter, err := NewStringBloomFilter(ctx, client, namespace, cfg)
	require.NoError(t, err)
	_, err = filter.Put(ctx, "alpha")
	require.NoError(t, err)
	_, err = filter.MightContain(ctx, "alpha")
	require.NoError(t, err)
	_, err = filter.BitCount(ctx)
	require.NoError(t, err)
	_, err = filter.IsEmpty(ctx)
	require.NoError(t, err)
	require.NoError(t, filter.Clear(ctx))

	assertOneScript := func(name string, run func() error) {
		t.Helper()
		recorder.Reset()
		require.NoError(t, run(), name)
		require.Equal(t, 1, recorder.ScriptDelta(), name)
		require.Equal(t, 1, recorder.Count("evalsha"), name)
		require.Zero(t, recorder.Count("eval"), name)
		for _, direct := range []string{"getbit", "setbit", "bitcount", "hget", "hgetall", "hset", "del"} {
			require.Zero(t, recorder.Count(direct), name+"/"+direct)
		}
	}

	assertOneScript("init matched", func() error {
		_, err := NewStringBloomFilter(ctx, client, namespace, cfg)
		return err
	})
	assertOneScript("put", func() error {
		_, err := filter.Put(ctx, "alpha")
		return err
	})
	assertOneScript("might contain", func() error {
		ok, err := filter.MightContain(ctx, "alpha")
		require.True(t, ok)
		return err
	})
	assertOneScript("bit count", func() error {
		_, err := filter.BitCount(ctx)
		return err
	})
	assertOneScript("is empty", func() error {
		_, err := filter.IsEmpty(ctx)
		return err
	})
	assertOneScript("clear", func() error {
		return filter.Clear(ctx)
	})
}

func TestLuaScriptsAreStaticAndUseKeysArgv(t *testing.T) {
	for _, script := range []string{initConfigLua, putLua, mightContainLua, clearLua, bitCountLua, isEmptyLua} {
		require.NotContains(t, script, "tenant-a")
		require.NotContains(t, script, "bluetape:probabilistic")
		require.NotContains(t, script, "schema:v1")
		require.NotContains(t, script, "alpha")
		require.Contains(t, script, "KEYS[")
		require.Contains(t, script, "ARGV[")
	}
}
```

- [ ] **Step 3: Add stress and async tests**

Create `probabilistic/redis/concurrency_test.go`:

```go
package redisbloom

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"testing"
	"time"

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
	"github.com/stretchr/testify/require"
)

func TestGoroutineStressPutAndMightContain(t *testing.T) {
	ctx := context.Background()
	client := newRedisClient(t)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)
	filter, err := NewStringBloomFilter(ctx, client, namespace, testConfig(t, 50000))
	require.NoError(t, err)

	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       max(32, runtime.GOMAXPROCS(0)*4),
		RoundsPerTask: 64,
		Timeout:       20 * time.Second,
	})

	tester.RunT(t,
		func(ctx context.Context) error {
			value := fmt.Sprintf("value:%d", time.Now().UnixNano())
			changed, err := filter.Put(ctx, value)
			if err != nil {
				return err
			}
			_, _ = changed, value
			return nil
		},
		func(ctx context.Context) error {
			_, err := filter.MightContain(ctx, "alpha")
			return err
		},
	)
}

func TestAsyncJobTesterCancellation(t *testing.T) {
	client := newRedisClient(t)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)
	filter, err := NewStringBloomFilter(context.Background(), client, namespace, testConfig(t, 1000))
	require.NoError(t, err)

	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers:       4,
		RoundsPerTask: 8,
		Timeout:       5 * time.Second,
	})

	err = tester.Run(context.Background(), func(context.Context) error {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := filter.MightContain(ctx, "alpha")
		if !errors.Is(err, context.Canceled) {
			return fmt.Errorf("expected context canceled, got %v", err)
		}
		return nil
	})
	require.NoError(t, err)
}

func TestAsyncJobTesterDeadlineExceeded(t *testing.T) {
	client := newRedisClient(t)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)
	filter, err := NewStringBloomFilter(context.Background(), client, namespace, testConfig(t, 1000))
	require.NoError(t, err)

	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers:       4,
		RoundsPerTask: 8,
		Timeout:       5 * time.Second,
	})

	err = tester.Run(context.Background(), func(context.Context) error {
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		defer cancel()
		_, err := filter.MightContain(ctx, "alpha")
		if !errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("expected deadline exceeded, got %v", err)
		}
		return nil
	})
	require.NoError(t, err)
}
```

- [ ] **Step 4: Add benchmarks**

Create `probabilistic/redis/filter_benchmark_test.go`:

```go
package redisbloom

import (
	"context"
	"strconv"
	"testing"

	"github.com/bluetape4k/bluetape-go/probabilistic"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

func startBenchmarkRedis(b *testing.B) *redis.Client {
	b.Helper()
	ctx := context.Background()
	container, err := tcredis.Run(ctx, "redis:7.4-alpine", testcontainers.WithWaitStrategy(
		wait.ForLog("Ready to accept connections"),
	))
	if err != nil {
		b.Fatalf("start redis container: %v", err)
	}
	b.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			b.Fatalf("terminate redis container: %v", err)
		}
	})
	host, err := container.Host(ctx)
	if err != nil {
		b.Fatalf("redis container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "6379/tcp")
	if err != nil {
		b.Fatalf("redis container port: %v", err)
	}
	client := redis.NewClient(&redis.Options{Addr: host + ":" + port.Port()})
	b.Cleanup(func() {
		if err := client.Close(); err != nil {
			b.Fatalf("close redis client: %v", err)
		}
	})
	if err := client.Ping(ctx).Err(); err != nil {
		b.Fatalf("ping redis: %v", err)
	}
	return client
}

func benchmarkRedisBloomConfigs(b *testing.B) []struct {
	name string
	cfg  probabilistic.Config
} {
	b.Helper()
	low, err := probabilistic.NewConfig(100000, 0.05)
	if err != nil {
		b.Fatal(err)
	}
	medium, err := probabilistic.NewConfig(100000, 0.01)
	if err != nil {
		b.Fatal(err)
	}
	high, err := probabilistic.NewConfig(100000, 0.001)
	if err != nil {
		b.Fatal(err)
	}
	return []struct {
		name string
		cfg  probabilistic.Config
	}{
		{name: "low-fpp-0.05", cfg: low},
		{name: "medium-fpp-0.01", cfg: medium},
		{name: "high-fpp-0.001", cfg: high},
	}
}

func BenchmarkRedisBloomPut(b *testing.B) {
	ctx := context.Background()
	client := startBenchmarkRedis(b)
	for _, bc := range benchmarkRedisBloomConfigs(b) {
		b.Run(bc.name, func(b *testing.B) {
			filter, err := NewStringBloomFilter(ctx, client, "bench:put:"+bc.name, bc.cfg)
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := filter.Put(ctx, strconv.Itoa(i)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkRedisBloomMightContain(b *testing.B) {
	ctx := context.Background()
	client := startBenchmarkRedis(b)
	for _, bc := range benchmarkRedisBloomConfigs(b) {
		b.Run(bc.name, func(b *testing.B) {
			filter, err := NewStringBloomFilter(ctx, client, "bench:might-contain:"+bc.name, bc.cfg)
			if err != nil {
				b.Fatal(err)
			}
			for i := 0; i < 1000; i++ {
				if _, err := filter.Put(ctx, strconv.Itoa(i)); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := filter.MightContain(ctx, strconv.Itoa(i%1000)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkRedisBloomOffsets(b *testing.B) {
	ctx := context.Background()
	client := startBenchmarkRedis(b)
	for _, bc := range benchmarkRedisBloomConfigs(b) {
		b.Run(bc.name, func(b *testing.B) {
			filter, err := NewStringBloomFilter(ctx, client, "bench:offsets:"+bc.name, bc.cfg)
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := filter.offsets(strconv.Itoa(i)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
```

- [ ] **Step 5: Run stress, cancellation, and benchmark checks**

Run:

```bash
go test -p 1 -count=1 ./probabilistic/redis -run 'HotPath|LuaScripts|GoroutineStress|AsyncJobTester'
go test -p 1 -race -count=1 ./probabilistic/redis -run 'GoroutineStress|AsyncJobTester'
go test -p 1 -run '^$' -bench 'BenchmarkRedisBloom(Put|MightContain|Offsets)' -benchmem ./probabilistic/redis
```

Expected: PASS. Record benchmark output for PR evidence and charts only if benchmark results are included in docs.

- [ ] **Step 6: Commit verification tests**

```bash
git add probabilistic/redis
git commit -m "Verify Redis Bloom concurrency and hot paths

Constraint: Redis Bloom correctness depends on script atomicity, cancellation propagation, and concurrent callers.
Rejected: Ad hoc goroutine loops only | misses repo-required tester evidence.
Confidence: high
Scope-risk: moderate
Directive: Keep Testcontainers Redis tests serial with -p 1.
Tested: go test -p 1 -count=1 ./probabilistic/redis -run 'HotPath|LuaScripts|GoroutineStress|AsyncJobTester'; go test -p 1 -race -count=1 ./probabilistic/redis -run 'GoroutineStress|AsyncJobTester'; go test -p 1 -run '^$' -bench 'BenchmarkRedisBloom(Put|MightContain|Offsets)' -benchmem ./probabilistic/redis
Not-tested: Full make ci pending"
```

### Task 6: Examples, Documentation, Diagram, and Release Notes

**Files:**
- Create: `probabilistic/redis/example_test.go`
- Create: `scripts/generate-redis-bloom-diagram.mjs`
- Create: `docs/images/readme-diagrams/redis-bloom-key-layout-01.dot`
- Create: `docs/images/readme-diagrams/redis-bloom-key-layout-01.plain`
- Create: `docs/images/readme-diagrams/redis-bloom-key-layout-01-graphviz.svg`
- Create: `docs/images/readme-diagrams/redis-bloom-key-layout-01-graphviz.png`
- Create: `docs/images/readme-diagrams/redis-bloom-key-layout-01.svg`
- Create: `docs/images/readme-diagrams/redis-bloom-key-layout-01.png`
- Modify: `probabilistic/README.md`
- Modify: `probabilistic/README.ko.md`
- Modify: `README.md`
- Modify: `README.ko.md`
- Modify: `CHANGELOG.md`
- Modify: `WIP.md` only if it contains #182 or 0.6.1 Redis probabilistic entries.

- [ ] **Step 1: Load diagram catalog before drawing**

Run:

```bash
test -f /Users/debop/work/bluetape4k/bluetape4k-wiki/docs/diagrams/best-practices/catalog.yaml
test -f /Users/debop/work/bluetape4k/bluetape4k-wiki/docs/diagrams/best-practices/rejected/catalog.yaml
```

Expected: both commands pass. If either fails, stop Task 6 and report the `$bluetape4k-diagram` catalog gap.

- [ ] **Step 2: Add diagram generator**

Create `scripts/generate-redis-bloom-diagram.mjs` as a deterministic generator that:

- writes the DOT graph for `Client -> RedisBloom API -> Lua Scripts -> Redis Key Pair`;
- runs `dot -Tplain`, `dot -Tsvg`, and `dot -Tpng` for evidence files;
- writes a decorated final SVG with outer frame, title, subtitle, layer bands, hash-tagged key callout, static Lua callout, and footer evidence;
- converts final SVG to PNG;
- checks generated SVG text does not contain `undefined`, a to-do marker, `Mermaid`, `Graphviz only`, or layout-remediation text.

Use these output names exactly:

```text
docs/images/readme-diagrams/redis-bloom-key-layout-01.dot
docs/images/readme-diagrams/redis-bloom-key-layout-01.plain
docs/images/readme-diagrams/redis-bloom-key-layout-01-graphviz.svg
docs/images/readme-diagrams/redis-bloom-key-layout-01-graphviz.png
docs/images/readme-diagrams/redis-bloom-key-layout-01.svg
docs/images/readme-diagrams/redis-bloom-key-layout-01.png
```

- [ ] **Step 3: Generate and inspect diagram**

Run:

```bash
node scripts/generate-redis-bloom-diagram.mjs
file docs/images/readme-diagrams/redis-bloom-key-layout-01.png
file docs/images/readme-diagrams/redis-bloom-key-layout-01-graphviz.png
rg -n "undefined|T[O]DO|Mermaid|Graphviz only|layout fixed|Grid residue" docs/images/readme-diagrams/redis-bloom-key-layout-01.svg docs/images/readme-diagrams/redis-bloom-key-layout-01.dot
```

Expected: generator succeeds, both PNG files are valid images, and the `rg` command returns no matches.

Open or inspect the rendered PNG before reporting the diagram ready. Reject visible card overlap, text overflow, card/card crowding, connector-through-card, missing outer decorator, or missing title/subtitle/footer evidence.

- [ ] **Step 4: Add runnable caller examples**

Create `probabilistic/redis/example_test.go` with examples that compile and document caller-facing behavior:

- `ExampleNewStringBloomFilter`: constructor, shared namespace, no raw user IDs in namespace.
- `ExampleBloomFilter_Put_falseNotDuplicate`: `Put(ctx, value) == false` means all bits were already set, not duplicate certainty.
- `ExampleBloomFilter_Clear_adminOnly`: `Clear` guarded by caller-side approval/authorization and not used on ordinary request paths.
- `ExampleBloomFilter_diagnostics`: `BitCount`, `ApproximateElementCount`, and `ExpectedFPP`.
- `Example_errors`: `errors.Is` for `ErrConfigMismatch`/`ErrConfigCorrupt` and `errors.As` for `RedisError`; examples must not print namespace, full Redis keys, hasher key, or inserted value.

Verify examples:

```bash
go test -p 1 -count=1 ./probabilistic/redis -run 'Example'
```

Expected: PASS.

- [ ] **Step 5: Update package READMEs**

Update `probabilistic/README.md` and `probabilistic/README.ko.md` with:

- Redis-backed Bloom API import and constructors;
- distributed shared-state caveat;
- false-positive and no-false-negative boundary;
- `Put(ctx, value) == false` caveat;
- `Clear(ctx)` as operator/admin destructive shared state;
- key layout table with `bluetape:probabilistic:bloom:v1:{namespace}:bits` and `:config`;
- namespace validation and no raw user IDs, secrets, tokens, or emails;
- Redis Cluster hash-tag behavior;
- TLS/AUTH/ACL guidance and minimum command set;
- persistence, no TTL, and eviction policy guidance;
- diagnostics using `HLEN`, `HGETALL`, `STRLEN`, `BITCOUNT`, and `PTTL`;
- `Clear` runbook: caller-side approval/authorization, no ordinary request-path usage, accidental clear/delete recovery from source data into a new namespace, rollback decision points, and old key retirement;
- actionable error table for `ErrConfigMismatch`, `ErrConfigCorrupt`, and `RedisError`, including `errors.Is`/`errors.As` snippets and remediation through metadata inspection, new namespace, source rebuild, or operator escalation;
- Kotlin Lettuce migration requiring new namespace, source rebuild or dual-write, reader switch, verification window, and old-key retirement; explicitly state Kotlin wire layout is incompatible;
- Cuckoo and HLL follow-up scope; state this PR exposes Redis Bloom only and does not add Cuckoo/HLL constructors;
- embedded PNG:

```markdown
![Redis Bloom key layout](../docs/images/readme-diagrams/redis-bloom-key-layout-01.png)
```

- [ ] **Step 6: Update root docs and release notes**

Update:

- `README.md` and `README.ko.md`: package list entry for `probabilistic/redis`.
- `CHANGELOG.md`: `[Unreleased]` bullet for Redis-backed Bloom filters.
- `WIP.md`: mark #182 progress only if the file has a matching open tracking line.

- [ ] **Step 7: Verify docs, examples, and diagram links**

Run:

```bash
go test -p 1 -count=1 ./probabilistic/redis -run 'Example'
rg -n "probabilistic/redis|Redis Bloom|redis-bloom-key-layout-01.png|Put\\(ctx, value\\).*false|Clear\\(ctx\\)|approval|authorization|rebuild|dual-write|retire old keys|ErrConfigMismatch|ErrConfigCorrupt|errors\\.Is|errors\\.As|PTTL|EVALSHA|TLS|AUTH|ACL|GETBIT|SETBIT|BITCOUNT|STRLEN|HGET|HGETALL|HSET|DEL|EVAL" README.md README.ko.md probabilistic/README.md probabilistic/README.ko.md CHANGELOG.md
rg -n "Cuckoo" README.md README.ko.md probabilistic/README.md probabilistic/README.ko.md
rg -n "HLL|HyperLogLog" README.md README.ko.md probabilistic/README.md probabilistic/README.ko.md
test -f docs/images/readme-diagrams/redis-bloom-key-layout-01.png
test -f docs/images/readme-diagrams/redis-bloom-key-layout-01.svg
test -f docs/images/readme-diagrams/redis-bloom-key-layout-01.dot
git diff --check
```

Expected: all commands pass.

- [ ] **Step 8: Commit examples, docs, and diagram**

```bash
git add probabilistic/redis/example_test.go scripts/generate-redis-bloom-diagram.mjs docs/images/readme-diagrams/redis-bloom-key-layout-01* README.md README.ko.md probabilistic/README.md probabilistic/README.ko.md CHANGELOG.md WIP.md
git commit -m "Document Redis Bloom operations

Constraint: Public Redis Bloom docs must include diagram-backed key layout and operational caveats.
Rejected: Table-only README explanation | does not satisfy diagram requirement.
Confidence: high
Scope-risk: moderate
Directive: Keep generated SVG and PNG in sync with the generator and Graphviz evidence.
Tested: go test -p 1 -count=1 ./probabilistic/redis -run 'Example'; node scripts/generate-redis-bloom-diagram.mjs; git diff --check; README image files exist
Not-tested: Full make ci pending"
```

### Task 7: Full Local Verification and Review Evidence

**Files:**
- Create: `docs/review/2026-06-13-issue-182-code-review.md`
- Modify: PR body draft file under `.omx/artifacts/` if used for `gh pr create --body-file`

- [ ] **Step 1: Run targeted package tests**

Run:

```bash
go test -count=1 ./probabilistic ./probabilistic/internal/bloomhash
go test -p 1 -count=1 ./probabilistic/redis
```

Expected: PASS.

- [ ] **Step 2: Run race tests**

Run:

```bash
go test -race -count=1 ./probabilistic ./probabilistic/internal/bloomhash
go test -p 1 -race -count=1 ./probabilistic/redis
```

Expected: PASS.

- [ ] **Step 3: Run benchmark evidence**

Run:

```bash
go test -p 1 -run '^$' -bench 'BenchmarkRedisBloom(Put|MightContain|Offsets)' -benchmem ./probabilistic/redis
```

Expected: PASS. If benchmark values are shown in PR or docs, render a chart through `$bluetape4k-diagram`; otherwise record benchmark output as validation evidence only.

- [ ] **Step 4: Run repository gates**

Run:

```bash
git diff --check
make ci
```

Expected: PASS. If `make ci` fails outside touched packages, capture the exact package and failure and run the next-best targeted command.

- [ ] **Step 5: Run Step 6-R 7-Tier code review**

Run six independent subagent lanes plus main integration:

1. Tier 1 Performance: scripts, command count, benchmark allocation, `BITCOUNT`.
2. Tier 2 Stability: stale handle, cancellation, external deletion, concurrency, race.
3. Tier 3 Security: Lua static source, redaction, ACL/TLS, namespace rules.
4. Tier 4 Operator/Ops: Redis Cluster, persistence/eviction, diagnostics, runbook.
5. Tier 5 Developer/API: Go API, package naming, errors, maintainability.
6. Tier 6 User/Caller: README examples, misuse resistance, migration clarity.

Store integrated findings in `docs/review/2026-06-13-issue-182-code-review.md`.

Exit condition: `P0=0 P1=0`.

- [ ] **Step 6: Commit final verification artifact**

```bash
git add docs/review/2026-06-13-issue-182-code-review.md
git commit -m "Record Redis Bloom review evidence

Constraint: Pre-PR review evidence must be tracked under docs/review.
Rejected: Chat-only review result | cannot support PR merge evidence.
Confidence: high
Scope-risk: narrow
Directive: Keep future review artifacts severity-counted with P0/P1 verdicts.
Tested: Step 6-R latest integrated review shows P0=0 P1=0
Not-tested: GitHub PR review pending"
```

## Step 5 Verifier Checklist

- Redis-backed Bloom constructors initialize immutable metadata and reject incompatible configs.
- Redis key layout uses a package-owned hash tag so `:bits` and `:config` share one cluster slot.
- All Lua script bodies are static constants and dynamic values pass through `KEYS`/`ARGV`.
- `Put`, `MightContain`, `Clear`, and `BitCount` validate fingerprint and bitmap operation in one script.
- `Put(false)` is documented as bitset saturation, not duplicate certainty.
- `Clear` preserves config and is documented as destructive shared state for operator/admin use.
- `Clear` docs include caller approval/authorization, ordinary request-path prohibition, source rebuild recovery, rollback decision points, and old-key retirement.
- Context cancellation/deadline errors remain visible with `errors.Is`.
- Error contracts support `errors.Is` / `errors.As` and default messages redact full logical keys.
- Runnable examples cover construction, `Put(false)`, admin-only `Clear`, diagnostics, and actionable errors.
- Testcontainers-backed commands run serially with `-p 1`.
- `GoroutineStressTester` covers concurrent `Put`/`MightContain`.
- `AsyncJobTester` covers cancellation/deadline behavior.
- Race tests pass for changed packages.
- README diagram includes SVG, PNG, Graphviz DOT/plain/SVG/PNG evidence, and rendered PNG inspection.
- README/README.ko parity is maintained.
- Cuckoo and HLL remain follow-up scope.

## Step 6-R Code Review Gate

Use six independent subagent lanes plus main integration. Do not spawn a seventh subagent for integration. Main session owns deduplication, severity normalization, documentation/release/evidence integrity, and rerun decisions.

Mandatory output:

- reviewed scope and base/head;
- lane table with P0/P1/P2/P3 counts;
- commands run;
- repaired findings and affected lane reruns;
- exact final line `P0=0 P1=0`.

## Step 7 PR Preparation

- Push branch `issue-182-redis-probabilistic-filters`.
- Create PR against `develop`.
- PR title: `Add Redis-backed probabilistic Bloom filters`
- PR body must include `Fixes #182`.
- PR body final heading must be exactly `## DoD Status`.
- Use `--body-file`, then verify live body:

```bash
gh pr create --base develop --head issue-182-redis-probabilistic-filters --title "Add Redis-backed probabilistic Bloom filters" --body-file .omx/artifacts/issue-182-pr-body.md
PR_NUMBER=$(gh pr view --json number --jq .number)
gh pr view "$PR_NUMBER" --json body
```

PR metadata must match issue #182:

- assignee: `debop`;
- milestone: `0.6.1`;
- labels from the live issue.

## Step 7-R PR Review Gate

Before CI gate or merge request, run six independent subagent lanes plus main integration against the PR diff and live PR body.

Store evidence in `docs/review/2026-06-13-issue-182-pr-review.md`, commit it, push it, and add both:

- a PR comment summarizing the review result;
- a formal GitHub review entry when applicable.

Exit condition: `P0=0 P1=0`.

## Step 8 Merge and Local Sync

Do not merge until the user explicitly says merge.

After user merge approval:

```bash
gh pr merge "$PR_NUMBER" --rebase --delete-branch
git -C /Users/debop/work/bluetape4k/bluetape-go fetch origin
git -C /Users/debop/work/bluetape4k/bluetape-go switch develop
git -C /Users/debop/work/bluetape4k/bluetape-go pull --ff-only origin develop
git -C /Users/debop/work/bluetape4k/bluetape-go worktree remove .worktrees/issue-182-redis-probabilistic-filters
git -C /Users/debop/work/bluetape4k/bluetape-go status --short --branch
```

Expected: root `develop` is clean and up to date with `origin/develop`.

## Step 9 Final DoD

Report a Step DoD table with:

- Step 0 worktree evidence;
- Step 1/1-R issue/research evidence;
- Step 2 spec and Step 2-R review evidence;
- Step 3 plan and Step 3-R review evidence;
- Step 4 implementation commit/test evidence;
- Step 5 verification checklist evidence;
- Step 6-R code review evidence;
- Step 7 PR URL/body verification/CI evidence;
- Step 7-R PR review evidence;
- Step 8 merge/local sync evidence when merge is approved and completed.

## Plan Self-Review

Spec coverage:

- Redis Bloom-only scope: Task 2 through Task 6.
- `go-redis/v9` and `redis.Cmdable`: Task 2 and Task 3.
- `Hasher.Bytes` and shared index helper: Task 1.
- Hash-tagged Redis key layout: Task 2.
- Static Lua, atomic metadata/bitmap operations, command count: Task 3 through Task 5.
- Context cancellation/deadline: Task 4 and Task 5.
- `GoroutineStressTester` and `AsyncJobTester`: Task 5.
- Testcontainers serial execution: Task 3 through Task 7 commands.
- Docs, diagram, README parity, Cuckoo/HLL follow-up: Task 6.
- 7-Tier gates: Step 3-R, Task 7, Step 7-R.

Placeholder scan:

- This plan avoids deferred implementation markers and names exact files, commands, and expected outcomes.

Type consistency:

- Public import path remains `probabilistic/redis`; package clause is `redisbloom`.
- Constructors and interface signatures match the approved spec.
- Redis error and config error names match the approved spec.
