# Issue 569 Redis Foundation Package Spec

Date: 2026-07-10 KST
Issue: #569
Milestone: 0.19.0
Branch: `feat/issue-569-redis-foundation`

## Problem

`bluetape-go` now has several Redis-backed packages that solve similar safety
problems independently:

- `lock/redis` generates owner tokens, stores caller-owned keys, validates TTL,
  and unlocks with a Lua compare-and-delete script.
- `leader/redis` builds leader keys, stores `memberID:random` owner tokens, and
  releases or renews ownership with Lua compare-and-delete / compare-and-extend
  scripts.
- `ratelimit/redis` validates caller keys, builds namespaced Redis keys, and
  wraps script execution errors inline.
- `probabilistic/redis` builds Redis Cluster hash-tagged keys and redacts key
  identifiers with a SHA-256 prefix.
- `jwt` and `jwt/redis` validate namespace and TTL contracts independently and
  explicitly avoid leaking raw tokens or key payloads.

The duplication is now large enough to justify a small shared Redis foundation
package. The package must not become a generic Redis client wrapper, and this
issue must not migrate existing packages yet. Migration belongs to #570 after
the foundation API is reviewed and tested.

## Current Evidence

| Evidence | Observation | Design implication |
|---|---|---|
| `lock/redis/mutex.go` | `Lease` owns a key/token pair and unlocks with `Eval` compare-and-delete. | Promote the reusable lease primitive without changing lock behavior now. |
| `leader/redis/elector.go` | Single leader uses `SET NX`, `memberID:random` token, compare-delete release, and compare-extend renew. | Foundation needs both delete and extend script helpers. |
| `ratelimit/redis/limiter.go` | Caller key validation and key construction are package-local. | Foundation key builders should preserve caller logical keys and avoid canonicalization. |
| `probabilistic/redis/keys.go` | Hash-tag layout keeps related Redis Cluster keys in one slot and redacts key ids as `redis-key:<hash>`. | Foundation should expose hash-tag and redaction helpers. |
| `testcontainers/redis/redis.go` | Reusable Redis Testcontainers launcher uses `redis:7.4-alpine`. | Integration tests should reuse this fixture and run serially. |
| `docs/lessons/2026-06-04-issue-24-redis-distributed-lock.md` | Previous review fixed accidental key trimming. | Validation may inspect trimmed values, but stored Redis keys must remain verbatim. |
| `docs/lessons/2026-07-08-issue-412-redis-testcontainers.md` | Redis tests need bounded contexts and visible cleanup. | Integration tests need explicit timeouts and cleanup. |
| `docs/lessons/2026-07-09-issue-437-jwt-redis-contention.md` | Redis benchmarks/tests must not leak raw tokens or key material. | Error and test artifacts must use redacted IDs only. |

## External API Evidence

| Source | Relevant contract | Impact |
|---|---|---|
| Redis `SET` command docs, https://redis.io/docs/latest/commands/set/ | `SET` supports conditional `NX` and expiration options such as `PX`. | Foundation may support owner-token leases that are acquired by existing packages with `SetNX`, but it should not wrap acquisition in this issue. |
| Redis `EVAL` command docs, https://redis.io/docs/latest/commands/eval/ | Scripts receive accessed keys via `KEYS` and arguments via `ARGV`. | Compare-delete and compare-extend helpers must pass the lease key in `KEYS[1]` and token/TTL in `ARGV`. |
| Redis Cluster specification, https://redis.io/docs/latest/operate/oss_and_stack/reference/cluster-spec/ | Keys containing the same hash-tag substring in `{...}` map to the same slot. | Key builder must make hash-tag use explicit for multi-key atomicity, not implicit for every key. |
| Local `go doc github.com/redis/go-redis/v9.Script.Run` | `Script.Run` uses `EVALSHA` and falls back to `EVAL`. | Foundation script helpers should use `redis.NewScript` and accept narrow `redis.Scripter`/`redis.Cmdable` inputs where practical. |

## Goals

- Add public import path `github.com/bluetape4k/bluetape-go/redis` with package
  clause `btredis`.
- Provide reusable primitives for Redis key construction, hash-tagged key
  families, owner tokens, leases, compare-delete, compare-extend, TTL
  validation, redacted operational errors, and script result parsing.
- Keep public API small, Go-shaped, context-aware, and compatible with
  `errors.Is` / `errors.As`.
- Make zero-value behavior explicit.
- Document package boundaries and non-goals in `redis/README.md` and
  `redis/README.ko.md`.
- Update root README locale set and `CHANGELOG.md` because this adds a public
  package.

## Non-Goals

- Do not create a generic `go-redis` facade.
- Do not expose every Redis command.
- Do not add global clients, global logging, global metrics, package-owned
  connection pools, or process-wide state.
- Do not migrate `lock/redis`, `leader/redis`, `ratelimit/redis`,
  `probabilistic/redis`, `cache/rediscoord`, `cache/redisnear`, or `jwt` in
  this issue.
- Do not canonicalize caller-owned Redis keys unless a function name and docs
  explicitly say that it does.
- Do not add benchmarks in this issue. Benchmark/provider comparison belongs to
  issue #560 and later migration issues. Before #570 migrates existing
  packages, the migration plan must include baseline/candidate microbenchmarks
  for key building, redaction, token generation, TTL conversion, and script
  helper command paths.

## Design Options

### Option A - Narrow Foundation Package (Selected)

Create `redis` with package name `btredis`. Include only cross-package
foundation primitives:

- opaque `OwnerToken` generation and validation.
- immutable `Lease` value containing key and token.
- `KeyBuilder` / key helper functions for versioned prefixes, structural
  segments, exactly one verbatim logical key segment, and optional hash-tags.
- Redacted key IDs.
- TTL validation and millisecond conversion.
- Compare-delete and compare-extend script helpers using package-level
  `redis.NewScript` values reused across calls.
- `OpError` wrapping Redis operation failures with family, operation, redacted
  key id, and causal `%w`.

Pros:

- Removes the highest-risk duplication without pretending to own all Redis
  command behavior.
- Lets #570 migrate package-by-package after the API is proven.
- Keeps the public package easy to review and test.

Cons:

- Existing duplication remains until #570.
- API must be conservative because downstream packages will depend on it.

### Option B - Internal Foundation Only

Create `internal/redis` and refactor existing packages immediately.

Pros:

- Avoids public API compatibility pressure.
- Can shape helpers around current call sites only.

Cons:

- Violates #569's candidate public import path.
- Makes later ecosystem provider reuse harder.
- Combines foundation design and behavioral migration, increasing review risk.

### Option C - Full Redis Client Abstraction

Wrap `go-redis` behind a bluetape-go client interface and expose command
helpers broadly.

Pros:

- Could standardize diagnostics across every Redis package.

Cons:

- Explicitly rejected by #569 non-goals.
- Adds abstraction over a mature client without repeated Go call-site evidence.
- Likely to become Kotlin-shaped and harder for callers to reason about.

## Selected API Shape

Package clause:

```go
package btredis
```

Candidate public types and functions:

```go
var (
    ErrInvalidKey        = errors.New("redis: invalid key")
    ErrInvalidHashTag    = errors.New("redis: invalid hash tag")
    ErrInvalidOwnerToken = errors.New("redis: invalid owner token")
    ErrInvalidTTL        = errors.New("redis: invalid ttl")
)

type OwnerToken struct { /* opaque */ }

func NewOwnerToken() (OwnerToken, error)
func ParseOwnerToken(value string) (OwnerToken, error)
func (t OwnerToken) String() string
func (t OwnerToken) RedisValue() string
func (t OwnerToken) Validate() error

type Lease struct { /* immutable */ }

func NewLease(key string, token OwnerToken) (Lease, error)
func (l Lease) Key() string
func (l Lease) RedactedKeyID() string
func (l Lease) Token() OwnerToken
func (l Lease) Validate() error

type KeyBuilder struct { /* zero value invalid until built by constructor */ }

func NewKeyBuilder(prefix string) (KeyBuilder, error)
func (b KeyBuilder) Structural(parts ...string) (KeyBuilder, error)
func (b KeyBuilder) WithHashTag(tag string) (KeyBuilder, error)
func (b KeyBuilder) StructuralKey(parts ...string) (Key, error)
func (b KeyBuilder) LogicalKey(logicalKey string) (Key, error)

type Key struct {
    Value      string
    RedactedID string
}

func RedactedKeyID(key string) string
func ValidateRedactedKeyID(id string) error
func ValidateTTL(name string, ttl time.Duration) error
func TTLMillis(name string, ttl time.Duration) (int64, error)

type OpLabels struct {
    Family    string
    Operation string
}

type OpError struct {
    Family    string
    Operation string
    KeyID     string // redacted key id only
    Err       error
}

func NewOpError(labels OpLabels, rawKey string, err error) error
func NewOpErrorWithRedactedKey(labels OpLabels, redactedKeyID string, err error) error
func (e OpError) Error() string
func (e OpError) Unwrap() error
func (e OpError) Is(target error) bool

func CompareAndDelete(ctx context.Context, client redis.Scripter, lease Lease, family string) (bool, error)
func CompareAndExtend(ctx context.Context, client redis.Scripter, lease Lease, ttl time.Duration, family string) (bool, error)
```

Exact names may change during plan review if the review finds a simpler
contract, but the API must stay within this boundary.

## Key Contract

- Prefix is a low-cardinality package-owned prefix string. It may contain `:`
  delimiters to preserve existing package families such as
  `bluetape:probabilistic:bloom:v1`, but each prefix part between delimiters is
  validated as a structural segment.
- Structural key segments added after the prefix are low-cardinality
  package-owned segments. They may be trimmed for validation and must be joined
  with `:`.
- Structural segments reject empty values, embedded braces, and `:` delimiters.
- `LogicalKey(logicalKey string)` accepts exactly one caller-owned logical key
  segment and appends it verbatim after the structural prefix.
- `StructuralKey(parts ...string)` is the terminal builder for package-owned
  key families such as `...:{namespace}:bits`; all parts follow structural
  segment validation.
- Caller-owned logical key segments are preserved verbatim.
- Blank detection may use `strings.TrimSpace`, but the resulting key must use
  the original logical key segment.
- The foundation does not impose a default max key byte size because Redis key
  limits are deployment-specific. Callers that expose untrusted keys must set a
  package-level max size before calling the builder; #570 migrations must keep
  existing package max-key checks.
- Hash-tags are opt-in via an explicit tag argument. The foundation must not
  silently hash-tag all keys.
- Hash-tags must reject empty tags and braces that would produce ambiguous Redis
  Cluster slot behavior. They must not reject `:` because existing
  `probabilistic/redis` namespaces use colon-bearing values inside Redis Cluster
  hash tags and #570 parity must preserve those key strings.
- Hash-tags are not tenant isolation or authorization boundaries. Multi-tenant
  packages must include tenant scope in their own namespace/key contract and
  enforce access control outside this package.
- Example same-slot family: `NewKeyBuilder("bluetape:probabilistic:bloom:v1")`
  with `WithHashTag(namespace)` and `StructuralKey("bits")` /
  `StructuralKey("config")` may build `...:{namespace}:bits` and
  `...:{namespace}:config` so scripts can touch both keys in one Redis Cluster
  slot.
- Example non-slot-sensitive family: a single lock key should use the caller's
  exact logical key and should not add a hash-tag unless the owning package has
  a multi-key script requirement.
- #570 migrations must add parity tests that compare old and new key strings for
  every migrated package before replacing package-local builders.
- Redaction uses a deterministic hash prefix such as
  `redis-key:<sha256-12-byte-prefix>`. This is a stable correlation ID, not a
  privacy boundary for low-entropy keys. It must not be used for authorization
  or tenant isolation decisions.
- `KeyBuilder{}` methods must return sentinel-compatible validation errors and
  never panic.

## Owner Token Contract

- Generated tokens use 32 bytes from `crypto/rand`, encoded as 64 lowercase hex
  characters; no global PRNG.
- Tokens are opaque values for Redis comparison. The default `String()` form is
  redacted and must not return the Redis comparison value.
- `GoString()` and `slog.LogValuer` formatting must also redact the token so
  debug formatting and structured logging do not expose the Redis comparison
  value.
- `RedisValue()` returns the raw comparison value for Redis command arguments
  only. Docs and examples must call it sensitive and must not log it.
- Validation rejects empty, whitespace-only, undersized, oversized, or
  non-lowercase-hex values.
- `ParseOwnerToken` accepts only canonical 64-character lowercase hex strings.
- Error messages must not include token values.

## Lease Script Contract

Compare-delete Lua:

```lua
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
end
return 0
```

Compare-extend Lua:

```lua
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("PEXPIRE", KEYS[1], ARGV[2])
end
return 0
```

Rules:

- Script helpers reject `ctx == nil` with a sentinel-compatible validation
  error. Existing packages may normalize nil contexts internally, but this
  public foundation should make external Redis IO timeout ownership explicit.
- Script helpers reject a nil `redis.Scripter` with a sentinel-compatible
  validation error before script dispatch. A typed nil client should also return
  a validation error when it is detectable without unsafe reflection.
- Docs and examples must use `context.WithTimeout` or a caller-owned deadline
  around Redis script helpers.
- `ctx.Err()` is checked before script execution.
- After a script has been dispatched to Redis, cancellation/deadline errors may
  make the commit state indeterminate from the caller's point of view. The docs
  must state that callers should inspect Redis state or retry through an
  idempotent owner workflow before assuming delete/extend did or did not commit.
- `lease.Validate()` runs before Redis IO. Invalid or zero-value leases return
  sentinel-compatible validation errors without calling `redis.Scripter`.
- `CompareAndExtend` validates TTL and converts it to milliseconds before Redis
  IO. Invalid TTLs must return sentinel-compatible validation errors without
  calling `redis.Scripter`.
- Script errors wrap causal errors with `OpError`.
- Returned `0` means the caller no longer owns the lease and is not an error.
- TTL for extend must be positive and convertible to at least one millisecond.
- Compare-delete and compare-extend scripts are package-level `redis.NewScript`
  values reused across calls, not constructed per helper invocation.

## Zero-Value Behavior

- `OwnerToken{}` is invalid and returns `ErrInvalidOwnerToken` from `Validate`.
- `Lease{}` is invalid and returns a wrapped validation error from `Validate`;
  script helpers must not call Redis for invalid leases.
- `KeyBuilder{}` is invalid; callers must use `NewKeyBuilder`.
- `OpError{}` is printable and unwraps nil; normal construction should include
  a causal error.

## Error Contract

- Validation errors wrap the relevant sentinel with `%w`.
- Redis operation errors return `OpError` so callers can use `errors.As`.
- `OpError.Unwrap()` returns the underlying Redis/context error.
- `OpError.Is(target)` delegates to `errors.Is(e.Err, target)` so callers can
  match `context.Canceled`, `context.DeadlineExceeded`, or Redis sentinel errors
  when applicable.
- `OpError.Error()` must not print raw Redis keys, owner tokens, or the wrapped
  cause string. It may include low-cardinality family/operation labels, the
  redacted key id, and the cause type/category. Callers can still inspect the
  original cause through `errors.Unwrap` / `errors.Is` / `errors.As`.
- `OpError.KeyID` is a redacted key id only. Normal construction uses
  `NewOpError`, which accepts a raw Redis key and stores only `RedactedKeyID`.
  `NewOpErrorWithRedactedKey` exists only for callers that already have a
  redacted key id and must validate the exact `redis-key:<24 lowercase hex>`
  shape. Manual `OpError` literals are documented as advanced/testing use and
  must never use raw Redis keys. Tests must prove raw keys/tokens do not appear
  in error strings, including when the wrapped cause text contains those values.
- `OpLabels` groups low-cardinality `Family` and `Operation` labels so public
  constructors do not use adjacent positional string parameters for family,
  operation, and key material. Label validation rejects empty, overlong, and
  delimiter-heavy values without echoing rejected label text.
- TTL validation label parameters are low-cardinality public labels such as
  `"lease ttl"` or `"idle ttl"`, not Redis keys, tenant IDs, or user input.
  Validation errors must avoid echoing arbitrary high-cardinality labels.

## Test Plan

Unit tests:

- key builder preserves caller-owned logical key segments verbatim;
- hash-tagged builder places related keys in the same tag family;
- key builder structural segments and logical key APIs cannot be confused in
  tests: structural segments reject `:`/braces while logical key is preserved
  verbatim;
- invalid prefix/tag/key segments return sentinel-compatible errors;
- redacted key ID is deterministic and does not include the raw key;
- redacted key ID validates as `redis-key:<24 lowercase hex>` and rejects raw
  keys passed to the already-redacted constructor path without echoing them;
- redacted key ID uses the longer stable correlation prefix and tests state it
  is not a privacy boundary;
- owner token generation returns canonical 64-character lowercase hex values
  backed by 256 bits of randomness;
- owner token parsing rejects empty, blank, undersized, oversized, mixed-case,
  and non-hex values and redacts errors;
- default token formatting is redacted; tests cover `fmt.Sprint(token)`,
  `%#v`, `%+v`, `GoString()`, and structured `slog.Any`; examples must not log
  `RedisValue()`;
- lease validation rejects missing key/token;
- script helpers reject nil contexts, nil clients, and invalid leases before
  Redis IO;
- fake `redis.Scripter` tests prove nil context, nil client, pre-canceled
  context, invalid lease, and invalid TTL do not dispatch Redis IO;
- TTL validation rejects zero, negative, and sub-millisecond TTLs for script
  helpers;
- `OpError` supports `errors.As`, `errors.Is`, `Unwrap`, and redacted strings;
- zero-value `KeyBuilder` methods return sentinel-compatible errors and do not
  panic.

Integration tests with Redis Testcontainers:

- compare-delete removes only the matching owner token;
- compare-delete returns false without deleting a new owner after token drift;
- compare-extend updates TTL only for the matching owner token;
- compare-extend returns false without updating TTL after token drift;
- canceled/deadline context returns caller-visible context error;
- first-run script execution against a real client succeeds through go-redis
  `EVALSHA`/`EVAL` behavior; the foundation does not reimplement that fallback;
- in-flight cancellation after script dispatch is documented as an indeterminate
  commit-state case; tests should prove pre-dispatch canceled contexts do not
  call Redis and integration docs should describe post-dispatch inspection;
- integration tests use bounded contexts, `t.Cleanup`, per-test key namespaces,
  and explicit key cleanup so failed tests do not contaminate later serial tests.

Race/stress tests:

- `go test -p 1 -race -count=1 ./redis` is mandatory for Redis-backed tests.
- Owner token generation gets a bounded concurrent uniqueness/stability test
  because it claims reusable safe generation. The test must use a bounded sample
  and phrase duplicate failures as unexpected probabilistic collisions, not as a
  mathematical impossibility.
- Lease script helpers get bounded Redis-backed concurrent stress tests proving
  interleaved owners cannot delete or extend a later owner, and the same stress
  coverage must pass under `go test -p 1 -race -count=1 ./redis`.

Validation commands:

```bash
go test -p 1 -count=1 ./redis
go test -p 1 -race -count=1 ./redis
go test -run Example -count=1 ./redis
git diff --check
make ci
```

Testcontainers-backed commands must run serially. If full `make ci` fails due to
an unrelated environment failure, record the exact failing package and run the
next-best targeted validation.

## Documentation Plan

- `redis/README.md`: public API boundary, non-goals, key/hash-tag examples,
  owner-token lease examples, timeout/cancellation guidance, operator runbook,
  error handling examples, and test command. README examples must be copied from
  or directly linked to compile-checked `Example...` tests.
- `redis/README.ko.md`: Korean-localized mirror.
- `redis/doc.go`: package overview, safety boundary, token secrecy warning, and
  statement that key prefixes/hash-tags are not tenant isolation.
- Root `README.md` and `README.ko.md`: package index row for `redis`.
- `CHANGELOG.md`: `[Unreleased]` bullet for Redis foundation package.
- `docs/lessons/2026-07-10-issue-569-redis-foundation.md`: lessons before PR.
- `docs/review/2026-07-10-issue-569-redis-foundation-review.md`: final local
  review evidence before PR.
- README locale parity must cover boundary, non-goals, key preservation, token
  secrecy, timeout/cancellation, error handling, and the fact that #569 does not
  migrate existing Redis packages yet.
- Operator runbook requirements: `(false, nil)` lease drift, context
  cancellation/deadline, post-dispatch indeterminate commit state, Redis
  script/client errors, cleanup, and rollback/no-migration behavior.

No diagram is required for this package because the behavior is a small code
primitive surface, not a topology or protocol that benefits from README visual
assets. If later migration docs explain multi-package adoption flow, diagrams
can be added under #570.

## Risks And Mitigations

| Risk | Severity | Mitigation |
|---|---|---|
| API becomes too broad and duplicates `go-redis`. | P1 | Keep functions limited to keys, tokens, leases, TTL, scripts, and errors. |
| Key builder silently changes caller-owned keys. | P1 | Add verbatim-preservation tests including leading/trailing spaces. |
| Script helper hides ownership drift. | P1 | Return `(false, nil)` for non-owner state and test token drift. |
| Error strings leak raw keys or tokens. | P1 | Add negative string tests for raw key/token values. |
| Existing packages accidentally change behavior. | P1 | Do not import the new package from existing Redis packages in #569. |
| Redis script commit state is unclear after cancellation. | P1 | Reject nil/pre-canceled contexts before IO and document post-dispatch indeterminate state. |
| Owner tokens are too weak or too easy to leak. | P1 | Require 256-bit canonical tokens, redacted `String()`, sensitive `RedisValue()`, and leakage tests. |
| Testcontainers tests become flaky or unbounded. | P2 | Use bounded contexts, cleanup, and serial package commands. |

## Acceptance Criteria

- Public `redis` package compiles with package name `btredis`.
- Public API is documented and small enough to fit the selected scope.
- Unit and Testcontainers tests cover key, token, lease, script, TTL, error, and
  cancellation contracts.
- Lease script stress tests prove interleaved owners cannot delete or extend a
  later owner and pass under race.
- `go test -p 1 -count=1 ./redis`, `go test -p 1 -race -count=1 ./redis`,
  and `make ci` pass.
- Existing Redis packages have no behavioral changes in this issue.
- README locale set and changelog are updated; README examples are
  compile-checked or linked to compile-checked examples.
- Issue and PR metadata are live-checked before merge: assignee `debop`,
  milestone `0.19.0`, labels matching #569, and changelog `[Unreleased]`
  coverage for the new public package.
- Step 6-R local 7-Tier review reports `P0=0 P1=0`.

## Step 2 Checklist Completion Report

| Item | Status | Notes |
|------|--------|-------|
| Architecture pre-design ran or skip reason recorded | Done | Scope is utility-level foundation; architecture options are captured in Design Options. |
| Step 1-R research incorporated | Done | Current repo, lessons, GNO/context-mode, Redis docs, and go-redis docs incorporated. |
| Current-behavior claims cite current source/test/doc evidence | Done | Evidence table names concrete source and lesson files. |
| Spec path confirmed inside feature worktree | Done | `docs/superpowers/specs/2026-07-10-issue-569-redis-foundation-spec.md` under `.worktrees/issue-569-redis-foundation`. |
| Risks/failure modes included | Done | Risks and mitigations table included. |
| Approach comparison and rejection rationale are research-based | Done | Options A/B/C compare public foundation, internal-only, and broad facade. |
| Brainstorming process ran or skip reason recorded | Done | User approved the material design after context and approach comparison. |
| User approval obtained per material design section | Done | User replied `진행해` after design summary. |
| Go API/test examples conform to bluetape-go patterns | Done | API uses context, narrow functions, sentinels, `%w`, and race/test gates. |
| Open questions resolved or escalated | Done | No blocker remains for Step 2-R review. |
