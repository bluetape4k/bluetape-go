# Issue #182 Redis Probabilistic Filters Spec

> 한국어 요구사항 경계: 이 spec/design/test-spec 문서는 한국어 독자가 요구사항을 추적할 수 있도록 목적과 검증 경계를 한국어로 보강한다. API 이름, command, code identifier, issue/PR 번호, compatibility matrix, acceptance keyword, DoD/test evidence는 요구사항 약화를 막기 위해 원문 그대로 보존한다. 변경자는 아래 literal contract를 삭제하거나 의미를 약하게 바꾸지 않아야 한다.
> 추가 한국어 검증 메모: 영어로 남은 항목은 대부분 code/API/evidence literal이다. 구현 전에는 한국어 경계 문장과 원문 acceptance checklist를 함께 읽고, 검증 gate가 줄어들지 않았는지 확인한다.\n

Issue: #182
Title: Add Redis-backed probabilistic filters
Date: 2026-06-13
Milestone: 0.6.1
Work type: Type A full feature
Target package: `probabilistic/redis`

## 목표

`bluetape-go` needs a Redis-backed Bloom filter that lets multiple Go service
instances share one probabilistic membership set through ordinary Redis
bitmap commands. The first slice must stay Go-shaped, context-aware, and
compatible with the in-memory `probabilistic` configuration model where that
does not hide Redis operational behavior.

This PR implements Redis-backed Bloom only. Redis Cuckoo filter and Redis
HyperLogLog remain explicit follow-up work because their API and failure modes
are materially different: Cuckoo owns deletion/kick-out/undo-log semantics, and
HyperLogLog owns cardinality estimation through Redis `PF*` commands.

## Step 1-R Research Summary

Current repository evidence:

- `probabilistic` contains the in-memory Bloom filter from #36:
  `Config`, `Hasher[T]`, `BloomFilter[T]`, `NewConfig`,
  `NewStringBloomFilter`, `NewBytesBloomFilter`, and deterministic
  double-hashing over SHA-256.
- `probabilistic/README.md` and `README.ko.md` state that Redis-backed Bloom,
  Cuckoo, and HyperLogLog support is deferred to #182.
- Redis/Testcontainers patterns already exist in `jwt`, `leader/redis`,
  `lock/redis`, `ratelimit/redis`, `cache/redisnear`, and `cache/rediscoord`.
  Tests use `testcontainers/redis.Start(ctx, t)`, `redis.NewClient`, readiness
  polling through `Ping`, and serial Testcontainers execution.
- Concurrency tests use `testing/concurrency.GoroutineStressTester`; context
  cancellation tests use `testing/concurrency.AsyncJobTester`.

External and primary API evidence:

- `github.com/redis/go-redis/v9` is already in `go.mod` and is the Redis driver
  for this repo. The new package will use `redis.Cmdable` so callers can pass a
  `*redis.Client`, recorder, or compatible test double.
- `probabilistic.Hasher[T]` currently keeps the byte-producing function
  unexported. A `probabilistic/redis` subpackage cannot call that helper
  directly, so this feature needs a minimal root-package helper boundary rather
  than duplicating incompatible hash input behavior.
- The local go-redis v9.20.0 module source exposes:
  - `SetBit(ctx, key, offset, value)`
  - `GetBit(ctx, key, offset)`
  - `BitCount(ctx, key, bitCount)`
  - `HSet`, `HGet`, `HGetAll`, `Del`, and `Eval`
- Redis official documentation confirms:
  - `SETBIT` grows a string-backed bitmap and requires offsets in
    `[0, 2^32)`.
  - `GETBIT` returns `0` when the key or offset is absent.
  - `BITCOUNT` counts set bits in the Redis string value and treats missing
    keys as empty.
  - `EVAL` executes Lua scripts with explicit `KEYS` and `ARGV` inputs.

Kotlin/JVM parity evidence:

- `bluetape4k-projects/infra/lettuce` has `LettuceBloomFilter` and
  `LettuceSuspendBloomFilter` using a bitmap key plus a `:config` hash.
- Kotlin Redis Cuckoo and HLL implementations exist, but they are separate
  conceptual surfaces and should not be mechanically copied into this Go slice.

## Non-Goals

- No RedisBloom module dependency.
- No new Redis driver dependency.
- No Cuckoo filter implementation in this PR.
- No HyperLogLog implementation in this PR.
- No cross-language Redis wire compatibility guarantee with the Kotlin
  Lettuce implementation.
- No deletion of individual Bloom entries. Bloom filters cannot safely delete
  individual values without a counting/deletion-capable structure.
- No background goroutines or caller-owned Redis client closing.

## 설계 Approaches

### Approach 1 - Redis Bloom Only With Plain Redis Bitmap Commands

Add `probabilistic/redis` with a context-aware Redis Bloom filter backed by a
bitmap key and metadata hash. Use `go-redis/v9` through `redis.Cmdable`.

Pros:

- Matches #182's "Build Redis-backed Bloom filter first" direction.
- Reuses the existing Redis/Testcontainers dependency and conventions.
- Keeps the review surface small enough for a reliable Type A PR.
- Gives callers distributed membership semantics without a Redis module.

Cons:

- `BITCOUNT` is O(N) over the bitmap string when `BitCount` or
  `ApproximateElementCount` is requested.
- Batch add/check may need Lua scripting to reduce round trips.

Decision: accepted.

### Approach 2 - Bloom, Cuckoo, and HyperLogLog in One PR

Implement all parity targets from `infra/lettuce` now.

Pros:

- Closes the whole source parity family at once.

Cons:

- Cuckoo deletion, kick-out retries, rollback, capacity saturation, and HLL
  cardinality APIs introduce separate semantics and test matrices.
- A single PR would mix membership, deletion-capable membership, and
  cardinality estimation behavior.

Decision: rejected for this PR. Track Cuckoo and HLL as follow-up work after
the Redis Bloom API boundary is reviewed.

### Approach 3 - RedisBloom Module Wrapper

Depend on RedisBloom commands instead of using core Redis bitmaps.

Pros:

- RedisBloom has specialized Bloom commands and may offer operational features
  plain bitmaps do not.

Cons:

- Requires a Redis module not used by current Testcontainers fixtures or CI.
- Adds deployment requirements that conflict with the existing plain Redis
  package family.
- Issue #182 asks for repo Redis/Testcontainers conventions, not a module
  adoption spike.

Decision: rejected for this PR.

## Public API

Import path: `github.com/bluetape4k/bluetape-go/probabilistic/redis`

Package clause: `package redisbloom`

The package name must not be `redis`, because the implementation imports
`github.com/redis/go-redis/v9` as `redis`. This follows the repo convention used
by `lock/redis` (`redislock`), `leader/redis` (`redisleader`), and
`ratelimit/redis` (`redisratelimit`).

Public types:

```go
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

type Options[T any] struct {
    Client redis.Cmdable
    Namespace string
    Config probabilistic.Config
    Hasher probabilistic.Hasher[T]
}

func NewBloomFilter[T any](ctx context.Context, options Options[T]) (BloomFilter[T], error)
func NewStringBloomFilter(ctx context.Context, client redis.Cmdable, namespace string, cfg probabilistic.Config) (BloomFilter[string], error)
func NewBytesBloomFilter(ctx context.Context, client redis.Cmdable, namespace string, cfg probabilistic.Config) (BloomFilter[[]byte], error)
```

Root `probabilistic` helper addition:

```go
func (h Hasher[T]) Bytes(value T) ([]byte, error)
```

`Bytes` exposes the existing deterministic hash input boundary without exposing
or accepting mutation of `Hasher` internals. The existing in-memory Bloom filter
will use the same method internally, so Redis and in-memory filters share
hasher validation and byte production.

Internal helper:

```go
package bloomhash

func Indexes(bytes []byte, hashFunctionCount uint64, bitSize uint64) []uint64
```

The current unexported `indexes` logic moves to
`probabilistic/internal/bloomhash` so both `probabilistic` and
`probabilistic/redis` can use the same SHA-256 double-hashing formula without
adding broad public API surface.

The API contract is fixed for implementation:

- construction validates client, namespace, config, and hasher;
- construction atomically initializes or validates Redis metadata;
- `Put`, `MightContain`, `BitCount`, `IsEmpty`, `ApproximateElementCount`,
  `ExpectedFPP`, and `Clear` validate the stored config fingerprint before
  touching bitmap state;
- all Redis I/O accepts caller-owned `context.Context`; nil contexts normalize
  to `context.Background()` before Redis calls, matching existing Redis package
  behavior;
- operational errors wrap causal errors with `%w`;
- context cancellation and deadline errors remain visible through `errors.Is`.

## Redis Key Layout

Given namespace `tenant-a:emails`, the package derives logical slot key
`bluetape:probabilistic:bloom:v1:{tenant-a:emails}`.

The braces are intentional Redis Cluster hash tags. `{key}:bits` and
`{key}:config` must share the same hash slot so Lua scripts can validate config
and read/write the bitmap atomically in Redis Cluster deployments.

Namespace rules:

- caller supplies a namespace, not raw Redis keys;
- namespace must be trimmed non-empty ASCII and at most 128 bytes;
- namespace may contain letters, numbers, `.`, `_`, `-`, and `:`;
- namespace must not start or end with `:`, and must not contain `:bits` or
  `:config` as a terminal reserved suffix;
- namespace must not contain `{` or `}` because the package owns Redis Cluster
  hash-tag placement;
- namespace must be operator-controlled or already normalized/authorized by the
  caller; raw user IDs, secrets, tokens, emails, or untrusted tenant strings
  must not be used directly;
- namespace and hasher keys must be non-sensitive operational identifiers,
  because even redacted logs and Redis diagnostics may reveal category or
  tenant-shaped metadata.

| Redis key | Type | Purpose |
|---|---|---|
| `{key}:bits` | string bitmap | Bloom bitset stored with `SETBIT` / `GETBIT` |
| `{key}:config` | hash | immutable metadata and compatibility guard |

Config hash fields:

| Field | Value |
|---|---|
| `version` | `1` |
| `family` | `redis-bloom` |
| `expected_insertions` | `Config.ExpectedInsertions()` |
| `false_positive_probability` | `Config.FalsePositiveProbability()` |
| `bit_size` | `Config.BitSize()` |
| `hash_function_count` | `Config.HashFunctionCount()` |
| `hasher_key` | `Hasher.Key()` |
| `fingerprint` | stable digest of version, family, config, and hasher key |

Initialization must be atomic. Use one static/versioned Lua script loaded
through `redis.NewScript.Run` or an equivalent `EVALSHA`-cached path to
compare-or-create the whole config hash. Concurrent constructors for the same
namespace and different configs must produce exactly one initialized
configuration; the incompatible constructor must return a typed config mismatch
error before it can write bitmap bits.

Initialization is idempotent only when all stored fields and the fingerprint
match the requested configuration. Any mismatch returns a typed/sentinel-
compatible configuration error. A partial, missing, externally deleted, or
malformed config hash is treated as incompatible while bitmap state exists and
must not silently rebuild over existing bitmap state.

Every operation that reads or writes bitmap state must validate the stored
fingerprint in the same static/versioned Lua script as the bitmap command.
`Put`, `MightContain`, `Clear`, and `BitCount` must each use a single Redis
round trip after local hash preparation. `IsEmpty`, `ApproximateElementCount`,
and `ExpectedFPP` may call the validated bitmap scripts internally, but must not
perform an unvalidated bitmap read. A stale handle must not keep reading,
writing, or clearing after an operator or another process changes metadata.

Lua script bodies must be static constants. Namespace, keys, fingerprints,
config fields, hasher keys, values, and bit offsets must be passed only through
`KEYS` and `ARGV`; implementation must not build Lua source through string
interpolation.

## Hashing and Capacity

Redis Bloom must use the same effective configuration values as
`probabilistic.Config`:

- expected insertions;
- target false-positive probability;
- calculated bit size;
- calculated hash function count.

The Redis bitmap offset limit is lower than Go's `uint64` type. `SETBIT`
requires offsets smaller than `2^32`, so construction must reject any config
whose `BitSize()` cannot fit Redis bitmap offsets. This is a Redis-specific
constraint even if in-memory Bloom can represent a larger bitset.

Hash positions should be derived from the package's deterministic hash strategy
instead of inventing a second incompatible formula. Implementation must move
the current index calculation into `probabilistic/internal/bloomhash` and must
use `Hasher.Bytes` for value-to-byte conversion.

## Behavior

- `MightContain(ctx, value)` returns `false, nil` only when at least one target
  bit is unset.
- `MightContain(ctx, value)` returning `true, nil` means the value may be
  present; false positives are expected.
- A successful `Put(ctx, value)` must not create false negatives for that value
  unless the Redis keys are later cleared, deleted, evicted, or overwritten.
- `Put(ctx, value)` returns whether at least one bit changed from 0 to 1.
  A `false, nil` result does not prove the exact value was already inserted; it
  only means every target bit was already set and may be caused by Bloom false
  positives.
- `Clear(ctx)` clears only the bitmap key and preserves config metadata. It is
  a destructive shared-state operation for all clients using the same namespace,
  but it cannot change configuration or permit incompatible reinitialization.
  `Clear` must validate the fingerprint and delete the bitmap in one static Lua
  script.
- The public API does not expose a config-reset operation. Config migration
  requires a new namespace and coordinated rebuild/switch-over; operators may
  retire old Redis keys after all callers are moved.
- Metadata methods that do not require Redis I/O return local construction
  metadata directly.
- `BitCount`, `ApproximateElementCount`, and `ExpectedFPP` read the current
  Redis bitmap state through `BITCOUNT`, which is O(N) over the Redis string.
- `IsEmpty` should avoid full `BITCOUNT` scans when possible. It may use a
  validated `EXISTS` or `STRLEN`-style script because a missing or zero-length
  bitmap is empty after config validation.
- The filter is safe for concurrent use when the caller's `redis.Cmdable` is
  safe for concurrent use. `go-redis` clients meet that expectation.

## Error Contract

Sentinel or typed errors must support `errors.Is` / `errors.As` for:

- invalid Redis Bloom options;
- config mismatch or malformed stored config;
- Redis operation failures;
- context cancellation and deadline propagation.

Public errors:

| Error/type | Meaning |
|---|---|
| `ErrInvalidOptions` | nil client, invalid namespace, invalid config, or invalid hasher |
| `ErrConfigMismatch` | stored metadata differs from requested config or hasher key |
| `ErrConfigCorrupt` | stored metadata is partial, malformed, or missing while bitmap state exists |
| `RedisError` | wraps Redis command/script failures with operation and redacted key id |

Error messages should include the operation and a redacted key id or stable
short digest of the logical slot key, but must not log or return inserted
values. Error messages must not echo full caller-provided hasher keys. Docs must
define namespaces and hasher keys as non-sensitive algorithm/schema identifiers
and keep full Redis key inspection as an explicit operator diagnostic step, not
the default error path.

## Testing Requirements

Unit and integration tests:

- invalid options: nil client, empty namespace, invalid namespace, invalid
  config, nil/empty hasher;
- initial construction writes config and accepts repeated construction with the
  same config;
- construction rejects changed config, changed hasher key, malformed metadata,
  or partial metadata;
- concurrent constructors with incompatible configs leave exactly one config in
  Redis and make the losing constructor fail before any bitmap write;
- command-count tests or recorder tests prove `Put`, `MightContain`, `Clear`,
  and config initialization use one script round trip each after local hash
  preparation;
- script tests prove Lua bodies are static and all user/config-derived values
  travel through `KEYS`/`ARGV`;
- `Put` returns true for new bits and false when all target bits already exist;
- `MightContain` has no false negative for successfully inserted values;
- `Put` and `MightContain` reject missing, changed, or corrupt config metadata
  before touching bitmap state;
- `Put`, `MightContain`, `Clear`, and `BitCount` validate config and perform the
  bitmap operation atomically in one script;
- `Clear` removes only the bitmap key, preserves config, and rejects missing,
  changed, or corrupt config metadata;
- external bitmap deletion while config remains is tested and documented as an
  empty filter state that can create false negatives for previously inserted
  values;
- Redis command errors wrap causal errors;
- context cancellation and deadline paths preserve `context.Canceled` and
  `context.DeadlineExceeded`;
- Testcontainers success path uses `testcontainers/redis.Start(ctx, t)`;
- `GoroutineStressTester` covers concurrent `Put` / `MightContain`;
- `AsyncJobTester` covers cancellation/deadline behavior;
- benchmarks cover `Put` and `MightContain` latency/allocation across realistic
  hash-function counts, and verify script caching does not resend Lua source on
  every hot-path call;
- targeted tests pass under the race detector.

Validation commands:

```bash
go test -count=1 ./probabilistic
go test -p 1 -count=1 ./probabilistic/redis
go test -race -count=1 ./probabilistic
go test -p 1 -race -count=1 ./probabilistic/redis
go test -p 1 -count=1 ./probabilistic/redis -run 'Redis|Stress|Cancellation|Deadline'
go test -p 1 -run '^$' -bench 'BenchmarkRedisBloom(Put|MightContain)' -benchmem ./probabilistic/redis
git diff --check
make ci
```

Testcontainers-backed commands must run serially with `-p 1`, and Redis
Testcontainers tests must not call `t.Parallel`.

## Documentation and Diagram Requirements

Update:

- `probabilistic/README.md`
- `probabilistic/README.ko.md`
- root `README.md`
- root `README.ko.md`
- `CHANGELOG.md`
- `WIP.md` when it contains #182 or 0.6.1 Redis probabilistic tracking entries

Docs must explain:

- Redis-backed Bloom is distributed shared state, not a local in-memory copy;
- false positives are possible;
- successful writes should not create false negatives unless Redis state is
  cleared, evicted, overwritten, or lost;
- individual deletion is unsupported;
- key lifecycle and `Clear` behavior;
- `Clear` belongs in operator/admin paths, should require caller-side
  confirmation or authorization, and recovery means rebuilding the namespace
  from source data when accidental clear/delete causes false negatives;
- Redis key layout and operational caveats;
- namespace validation and the rule that raw user IDs, secrets, tokens, and
  emails do not belong in Redis keys;
- Redis TLS/ACL guidance for remote Redis and least-privilege access scoped to
  the Bloom prefix. Minimum Redis command permissions are the commands actually
  used by the implementation, including `GETBIT`, `SETBIT`, `BITCOUNT`, `EXISTS`
  or `STRLEN`, `HGET`, `HGETALL`, `HSET`, `DEL`, and `EVAL`/`EVALSHA`;
- memory sizing from `BitSize()`, persistence requirements, and eviction policy
  guidance; deployments that require no false negatives should use persistence
  appropriate for their durability target, avoid TTLs on Bloom keys, and avoid
  eviction policies that can delete Bloom keys;
- diagnostics such as `HLEN`, `HGETALL` for metadata, `STRLEN` and `BITCOUNT`
  for bitmap state, and `PTTL` to confirm no unintended expiration;
- config migration: create a new namespace, rebuild or dual-write, switch
  readers, then retire old keys after the rollout;
- Kotlin Lettuce Bloom data is not wire-compatible by contract; migrating from
  existing Kotlin Redis Bloom data requires a new namespace and rebuild or
  dual-write rollout;
- `Put(ctx, value) == false` is not a duplicate guarantee because Bloom false
  positives can set all target bits before the exact value was inserted;
- config mismatch/corrupt errors should direct callers to inspect metadata,
  create a new namespace when changing config, and rebuild from source data
  rather than modifying metadata in place;
- Cuckoo and HLL are follow-up work.

Add a `$bluetape4k-diagram`-compliant README diagram for Redis Bloom data flow
and key layout. The diagram must have SVG and PNG outputs under
`docs/images/readme-diagrams/`, Graphviz evidence because it is node-and-
connector shaped, and rendered PNG inspection evidence before PR.

## 위험 and Failure Modes

| Risk | Mitigation |
|---|---|
| Config mismatch silently corrupts membership semantics. | Immutable config hash; constructor rejects mismatches and malformed metadata. |
| Redis bitmap exceeds supported offset range. | Reject `BitSize() > 2^32` before any Redis write. |
| Concurrent constructors race to create incompatible metadata. | Atomic compare-or-create initialization script and concurrent incompatible-constructor test. |
| Stale handles write after config reset or external deletion. | Preserve config on `Clear`; validate config fingerprint before every bitmap operation; no public config reset API. |
| Multi-command `Put` yields excessive round trips or partial observation. | Require static Lua scripts with one Redis round trip for `Put`, `MightContain`, `Clear`, and initialization; test command count. |
| Redis Cluster rejects multi-key scripts across slots. | Use a package-owned hash-tagged slot key so `{key}:bits` and `{key}:config` share one hash slot. |
| `BITCOUNT` on large bitmaps is expensive. | Document O(N) metadata reads, avoid calling it inside `Put`/`MightContain`, and keep `IsEmpty` on a cheaper validated path when possible. |
| Cancellation is swallowed by wrapping. | Tests assert `errors.Is` for context cancellation/deadline. |
| Redis key eviction or manual deletion creates false negatives. | README operational caveat and key lifecycle docs. |
| Tenant/user input collides with another Redis keyspace. | Namespace validation, fixed prefix, reserved suffix rejection, and docs warning against raw user IDs or secrets in keys. |
| Lua script source is accidentally built from caller input. | Keep script bodies static and pass all dynamic values through `KEYS`/`ARGV`; test this contract. |
| Error logs expose tenant-shaped key names. | Treat namespace and hasher keys as non-sensitive identifiers and redact full logical keys from default errors. |
| New package drifts from in-memory config/hash semantics. | Reuse `probabilistic.Config` and shared hash/index helper where practical. |
| Diagram repeats prior layout defects. | Apply `$bluetape4k-diagram` gates, render PNG, inspect before review. |

## Acceptance Criteria

- `probabilistic/redis` provides a Redis-backed Bloom filter API using
  `go-redis/v9` through `redis.Cmdable`.
- API covers add/put, might-contain, clear/delete-all behavior, metadata reads,
  initialization, config mismatch rejection, and Redis error wrapping.
- Redis key layout and config compatibility are documented.
- Redis Cluster hash-slot behavior is documented and covered by hash-tagged key
  layout tests.
- Cuckoo and HLL are explicitly recorded as follow-up work.
- Tests use Redis Testcontainers and cover success, invalid config,
  missing/changed config, cancellation/timeout, cleanup, and concurrent
  put/might-contain stress.
- Command-count and benchmark evidence cover hot-path script usage for
  `Put`/`MightContain`.
- Targeted tests and race tests pass.
- `make ci` passes or any unrelated external blocker is recorded with exact
  evidence.
- Step 2-R, Step 3-R, Step 6-R, and Step 7-R reviews use six independent
  perspective lanes plus main-session integration, with `P0=0 P1=0`.

## Step 2 Checklist Completion Report

| Item | Status | Notes |
|---|---|---|
| Architecture pre-design ran or skip reason recorded | Done | Approach comparison covers Redis-only, full parity, and RedisBloom module choices. |
| Step 1-R research incorporated | Done | Issue, current repo, go-redis source, Redis docs, Kotlin parity, and GNO results are reflected. |
| Current-behavior claims cite current source/test/doc evidence | Done | Current package, README, Testcontainers, #36 artifacts, and go-redis source are named. |
| Spec path confirmed inside feature worktree | Done | This file lives under `.worktrees/issue-182-redis-probabilistic-filters/docs/superpowers/specs/`. |
| Risks/failure modes included | Done | Risk table included. |
| Approach comparison and rejection rationale are research-based | Done | Three approaches compared and rejected/accepted with evidence. |
| `superpowers:brainstorming` process ran | Done | User approved Approach 1 before this spec was written. |
| User approval obtained per material design section | Done | User approved Approach 1. |
| Spec code/API/test examples conform to `$bluetape-go-patterns` | Done | Context-aware API, error wrapping, race/stress, Testcontainers, and README impact are explicit. |
| Open questions resolved or explicitly escalated | Done | Cuckoo/HLL split is resolved as follow-up scope. |
| Draft task list returned | Done | Testing/docs/diagram/validation requirements are included for Step 3 planning. |
