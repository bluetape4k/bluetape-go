# Issue #173 Distributed JWT KeyChain Repositories Design

Issue: #173
Milestone: 0.6.1
Backlog follow-up: #198
Date: 2026-06-12

## Context

Issue #173 extends the `jwt` package with distributed KeyChain sharing so
multiple service instances can sign and verify JWTs across key rotation. The
current #33 implementation supports fixed-key providers and process-local
in-memory rotation only.

Current repository evidence:

- `jwt/provider.go` owns signing, parsing, parser hardening, and local
  `Provider` rotation APIs.
- `jwt/repository.go` has a private in-memory `keyChainRepository` with
  current, find, rotate, forced rotate, and capacity trimming behavior.
- `jwt/keychain.go` keeps `KeyChain` fields private and exposes only safe
  metadata/accessor methods.
- `jwt/README.md` and `jwt/README.ko.md` mark distributed repositories as
  deferred to #173.
- Redis-backed packages in this repo use caller-owned `redis.Cmdable`,
  explicit `context.Context` propagation, namespace-scoped keys, Lua for atomic
  state transitions where needed, and Testcontainers Redis integration tests.
- `testing/concurrency` provides `GoroutineStressTester` and `AsyncJobTester`,
  both required by the issue acceptance criteria where applicable.

Kotlin parity evidence:

- Kotlin `KeyChainRepository` defines current, `kid` lookup, rotate,
  forced-rotate, delete-all, and capacity semantics.
- Kotlin `RedisKeyChainRepository` stores keychains in a Redis deque, keeps the
  newest key first, and trims oldest entries when capacity is exceeded.
- Kotlin `DefaultJwtProvider` composes provider operations through a repository.

The Go design borrows the behavior and boundaries, not the JVM implementation
details. Go Redis state is Go-owned and is not wire-compatible with the Kotlin
Redisson deque format.

## User Decisions

- Use Approach 3: introduce a separate `DistributedProvider` that embeds or
  composes the existing `*Provider` and adds distributed, context-aware
  operations.
- Implement Redis in #173.
- Do not implement MongoDB in #173. MongoDB is tracked separately in #198.

## Goals

1. Add a Go-native distributed JWT provider surface without breaking the #33
   `Provider` API.
2. Add a context-aware distributed KeyChain repository contract for external IO.
3. Add a Redis-backed repository first, reusing existing bluetape-go Redis and
   Testcontainers patterns.
4. Preserve explicit algorithm and `kid` validation from #33.
5. Prove cross-instance signing and parsing across rotation by `kid`.
6. Cover cancellation/deadline propagation for Redis operations.
7. Document Redis backend behavior and the MongoDB deferral.

## Non-Goals

- Do not implement MongoDB in this issue. Use #198 for MongoDB backend work.
- Do not make Go Redis state compatible with Kotlin/JVM Redisson state.
- Do not add a public raw-key export/import or seed API in #173. Existing
  fixed/local providers do not expose raw key material as stable API, so token
  continuity from those providers requires a future migration design or an
  explicit invalidation decision.
- Do not add JWKS, JWE, OIDC, HTTP auth middleware, sessions, roles, or an auth
  framework.
- Do not expose HMAC secrets, RSA private keys, raw token strings, or
  `github.com/golang-jwt/jwt/v5` concrete token/parser types as stable API.
- Do not add background rotation timers. Rotation remains caller-driven.

## Selected Architecture

Add a separate `DistributedProvider` type in package `jwt`:

```go
type DistributedProvider struct {
    provider *Provider
    repo DistributedKeyChainRepository
}
```

Go has no class inheritance, so the "extends Provider" model is implemented by
named composition. The private `*Provider` keeps the existing algorithm,
configuration, key generation, signing, and parsing helper behavior, while
`DistributedProvider` owns the external repository boundary and exposes only
distributed, context-aware public operations:

- `ComposeContext(ctx context.Context, options ...ComposeOption) (string, error)`
- `ParseContext(ctx context.Context, token string, options ...ParseOption) (*Reader, error)`
- `CurrentKeyChainContext(ctx context.Context) (*KeyChain, error)`
- `RotateContext(ctx context.Context) (*KeyChain, error)`
- `ForcedRotateContext(ctx context.Context) (*KeyChain, error)`
- `FindKeyChainContext(ctx context.Context, kid string) (*KeyChain, error)`
- `DeleteKeyChainsContext(ctx context.Context) error`

Constructors should be explicit:

- `NewDistributedHMACProvider(ctx context.Context, repo DistributedKeyChainRepository, algorithm Algorithm, options ...ProviderOption) (*DistributedProvider, error)`
- `NewDistributedRSAProvider(ctx context.Context, repo DistributedKeyChainRepository, algorithm Algorithm, options ...ProviderOption) (*DistributedProvider, error)`

The constructor normalizes provider options, validates algorithm family, creates
the private provider state, and ensures the distributed repository has a usable
current key through the caller-owned `ctx`. If `ctx` is nil, canceled, or
expired, construction fails with an `ErrInvalidOptions` / context-compatible
error and never substitutes `context.Background`.

Context propagation covers repository IO and store decisions. HMAC entropy reads
and RSA key generation use the existing provider key factory and are not
interruptible by `context.Context`; the implementation must check `ctx` before
calling the factory and again before storing the candidate. If the context is
canceled or expired after key generation but before Redis store, it must return
the context-compatible error and avoid persisting the candidate.

Distributed examples and docs must make the context-aware methods the primary
API. `DistributedProvider` must not anonymously embed `*Provider`; otherwise
callers could still reach context-free local-provider APIs through `dp.Provider`.
If convenience non-context methods are added later, they must be explicit
forwarding methods on `DistributedProvider` and must not perform distributed IO
without a caller context. This keeps the Approach 3 extension shape while
enforcing the distributed boundary.

## Repository Contract

Add a small exported interface in package `jwt`:

```go
type DistributedKeyChainRepository interface {
    Current(ctx context.Context, now time.Time) (*KeyChain, error)
    Find(ctx context.Context, kid string, now time.Time) (*KeyChain, error)
    Rotate(ctx context.Context, create func() (*KeyChain, error), now time.Time) (*KeyChain, error)
    ForcedRotate(ctx context.Context, create func() (*KeyChain, error), now time.Time) (*KeyChain, error)
    DeleteAll(ctx context.Context) error
}
```

Contract rules:

- `ctx` is caller-owned and must be passed to external IO without being replaced
  by `context.Background`.
- `now` comes from the provider clock so JWT key expiry rules stay consistent
  with existing provider behavior.
- `create` is provider-owned. Repository implementations do not know entropy,
  algorithm generation details, or signing material construction rules.
- `Current` returns the newest non-expired key or `ErrKeyNotFound` compatible
  error.
- `Find` requires non-empty `kid` and returns a non-expired matching key or an
  `ErrKeyNotFound` / `ErrInvalidKey` compatible error.
- `Rotate` returns the current non-expired key if one exists, otherwise stores
  and returns a newly created key.
- `ForcedRotate` always stores and returns a newly created key.
- Implementations trim retained keys to capacity while preserving newest keys.
- `DeleteAll` exists for tests and explicit operator reset flows; docs must warn
  against casual production use.

## Redis Repository

Add package `jwt/redis` for the Redis backend.

Expected public shape:

```go
type Options struct {
    Client redis.Cmdable
    Namespace string
    Capacity int
    KeyTTL time.Duration
    MaxKeyBytes int
}

type Repository struct {
    // unexported fields
}

func New(options Options) (*Repository, error)
```

Design rules:

- Redis client is caller-owned and never closed by the repository.
- Namespace is required for this backend because Redis contains signing
  authority. It must be explicit, non-empty after trimming, and validated with a
  conservative character set (`[A-Za-z0-9._-]`), byte-length limit, and
  delimiter/control-character rejection.
- Capacity follows the #33 bounds: default 10, minimum 2, maximum 1000.
- Redis state is namespaced and versioned:
  - `bluetape:jwt:v1:<namespace>:meta` stores format version and algorithm
    family.
  - `bluetape:jwt:v1:<namespace>:current` stores the current `kid`.
  - `bluetape:jwt:v1:<namespace>:keys` is a hash from `kid` to serialized DTO.
  - `bluetape:jwt:v1:<namespace>:order` is a sorted set for newest-first trim.
- `Find(ctx, kid, now)` must be an O(1)-style lookup by `kid`, not a full
  retained-key list scan. The normal parse hot path should use a bounded Redis
  command count and decode only the requested key payload.
- Repository reads must reject a stored algorithm family that does not match the
  provider algorithm family selected by the constructor. Reusing one namespace
  across incompatible algorithms is a hard failure, not a silent fallback.
- Redis state is Go-owned. Kotlin/JVM Redisson deque compatibility is not part
  of this issue.
- JSON DTO serialization is acceptable, but the DTO must be internal and must
  not make key material public API.
- DTO decoding must validate format version, `kid`, algorithm, key family,
  HMAC length, RSA private/public parse and validation, and algorithm/key-family
  match before reconstructing a `KeyChain`.
- `MaxKeyBytes` limits each serialized Redis DTO payload. It must have a safe
  default and upper bound, and payload length must be checked before JSON
  unmarshal.
- HMAC secret and RSA private key material must be persisted because a
  distributed signer needs every instance to sign and verify with the current
  key. Redis must therefore be treated as signing authority: docs must require a
  private/trusted Redis deployment with TLS/ACL guidance, persistence/eviction
  policy guidance, and no shared untrusted Redis. Error messages and logs must
  never print key values.
- Rotation, forced rotation, and capacity trim are atomic at the Redis state
  boundary. Use Lua scripts for compare/store/trim state changes.
- `Rotate` uses a two-phase CAS protocol:
  1. Lua checks the current pointer and returns the current non-expired key when
     one exists. The repository must not call `create` on this fast path.
  2. If no current key exists, Go calls provider-owned `create` once for the
     attempt.
  3. Lua rechecks the current pointer, stores the candidate only if no other
     non-expired key won, updates `current`, records order, trims to capacity,
     and returns either the stored candidate or the winning concurrent key.
  4. If `ctx` is canceled or expired, do not retry and preserve the context
     error for `errors.Is`.
- `ForcedRotate` always stores the candidate atomically, sets it current, and
  trims retained keys.
- `KeyTTL` defaults to `0` meaning no Redis key expiration. If configured, it
  must not expire Redis state before the latest retained `KeyChain.ExpiresAt()`
  plus allowed parse leeway. JWT key validity is still governed by
  `KeyChain.ExpiresAt()` and provider clock, not Redis TTL.

## Data Flow

### Construction

1. Normalize provider config.
2. Validate algorithm family.
3. Build embedded provider state without creating a private in-memory
   repository for distributed operations.
4. Call `repo.Rotate(ctx, createKeyChain, now)` to atomically ensure a current
   key and avoid cold-start races across multiple instances.
5. Verify the returned key algorithm matches the provider algorithm.
6. Return `DistributedProvider`.

### Compose

1. `ComposeContext(ctx, options...)` validates provider readiness and context.
2. It calls `repo.Rotate(ctx, createKeyChain, now)` to obtain the current
   non-expired key.
3. It builds claims and headers using existing compose options.
4. It sets the `kid` header from the repository key.
5. It signs with the located key material.

### Parse

1. `ParseContext(ctx, token, options...)` validates provider readiness, token,
   parse options, and context.
2. The JWT key callback requires a non-empty `kid`.
3. It calls `repo.Find(ctx, kid, now)`.
4. It rejects unsupported header controls already rejected by #33.
5. It requires token `alg`, provider algorithm, and located KeyChain algorithm
   to match before returning verification material.
6. It returns the existing `Reader` shape.

### Rotate

`RotateContext(ctx)` creates a key only when the repository has no current
non-expired key. `ForcedRotateContext(ctx)` always creates a key. Both paths
must preserve capacity trimming and caller context errors.

### Delete

`DeleteKeyChainsContext(ctx)` delegates to `repo.DeleteAll(ctx)`. It is covered
by tests and documented as a reset/testing operation.

## Error Contract

Errors must remain `errors.Is` friendly:

- Nil provider, repository, Redis client, or option: `ErrInvalidOptions`
  compatible.
- Empty or missing `kid`: `ErrKeyNotFound` compatible at repository level and
  `ErrInvalidToken` compatible at parse boundary.
- Unknown or evicted `kid`: `ErrKeyNotFound`.
- Expired retained key: `ErrInvalidKey` compatible.
- Redis command, Lua, or serialization failure: wrap causal error with `%w`.
- Caller cancellation and deadline errors must be preserved so
  `errors.Is(err, context.Canceled)` and
  `errors.Is(err, context.DeadlineExceeded)` work.

Error strings must not include token strings, HMAC secrets, RSA private keys, or
serialized key payloads.

## Testing Strategy

Use TDD. Production code starts only after failing tests define the contract.

Provider-level tests with a fake repository:

- Constructor bootstraps the first key when repository is empty.
- Two `DistributedProvider` instances share keys through one repository.
- A token signed by one instance parses on another.
- Cross-instance parse works after forced rotation by retained `kid`.
- Missing, unknown, expired, and evicted `kid` fail with compatible errors.
- Repository failures propagate without losing typed errors.
- Context cancellation/deadline errors propagate.
- Constructor/bootstrap cancellation tests cover canceled context before key
  creation and cancellation before Redis store. RSA key generation itself is not
  claimed to be interruptible.

Redis repository tests with Testcontainers Redis:

- `Current`, `Find`, `Rotate`, `ForcedRotate`, and `DeleteAll`.
- Capacity trimming preserves newest keys and evicts oldest keys.
- Expiry filtering rejects stale keys.
- Namespace isolation.
- Malformed stored payload handling.
- Redis command failure path where practical.
- Constructor/bootstrap cancellation and deadline propagation.
- Redis `KeyTTL` does not expire non-expired retained key material.
- Redis payload size is checked before DTO decode.
- Tampered DTOs fail for unknown version, invalid `kid`, mismatched algorithm
  family, short HMAC material, invalid RSA material, and oversized payloads.
- Exact Redis key format, namespace validation, namespace isolation, and
  algorithm mismatch failure.

Stress and race:

- Use `GoroutineStressTester` for concurrent rotate/sign/parse across multiple
  provider instances backed by the same Redis repository.
- Stress concurrent empty/expired rotate and assert one current winner, bounded
  duplicate key generation, and returned-`kid` convergence.
- Use `AsyncJobTester` for Redis cancellation/deadline paths.
- Run targeted race tests for changed packages:
  `go test -race -count=1 ./jwt ./jwt/redis`.

Benchmarks:

- Add opt-in `-benchmem` benchmarks for distributed `ComposeContext`,
  `ParseContext`, Redis `Find`, Redis `Rotate` current-hit, and expired/forced
  rotate paths.
- Record `ns/op`, `B/op`, `allocs/op`, Redis command-count notes, benchmark
  environment, and exclusions.
- If benchmark results are included in docs or PR evidence, present them as a
  real chart asset, not only a table of numbers.

Docs and examples:

- Update `jwt/README.md` and `jwt/README.ko.md`.
- Add compile-checked examples for Redis-backed distributed providers showing
  Redis repository construction, caller-owned Redis client lifecycle,
  `context.WithTimeout`, context-aware distributed constructors,
  `ComposeContext`, and `ParseContext`.
- Document migration from `NewHMACProvider` / `NewRSAProvider`, plus staged
  rollout and rollback constraints.
- Document MongoDB #198 deferral.
- Document Go-owned Redis key format and Kotlin wire-compatibility non-goal.
- Document that Redis contains JWT signing authority, requires trusted
  deployment boundaries, and that `DeleteKeyChainsContext` is for tests or
  explicit operator reset flows only.

## Rollout and Operations

- #173 does not provide fixed/local-to-Redis raw key import because current
  `KeyChain` material is private and raw key export is a separate security
  design. Existing fixed/local provider deployments that require old-token
  continuity must defer migration until that import design exists. Otherwise,
  they must make an explicit token invalidation decision before switching
  signers.
- Roll out new signing domains by first verifying Redis repository reads and
  cross-instance parsing, then enabling distributed signing, then monitoring
  unknown-`kid`, `ErrKeyNotFound`, and `ErrInvalidKey` rates.
- Rollback must avoid `DeleteKeyChainsContext`. Rolling back to fixed/local
  providers without a future import/export design requires an explicit token
  invalidation decision.
- README runbook coverage must include safe Redis inspection commands for key
  cardinality/current pointer/PTTL, Redis ACL/TLS/persistence expectations,
  secret-safe logging guidance, and recovery steps for Redis outage or
  namespace misconfiguration.

## Acceptance Criteria

- `DistributedProvider` embeds or composes `*Provider` and adds context-aware
  distributed operations.
- Redis repository supports current key, `kid` lookup, rotate, forced rotate,
  capacity trimming, expiry handling, and delete-all.
- Two provider instances verify each other's tokens across rotation by `kid`.
- Tests cover missing/unknown `kid`, stale key eviction, concurrent
  rotate/sign/parse, repository errors, and cancellation/deadline propagation.
- Constructor/bootstrap operations are context-aware and tested for cancellation
  and deadline propagation.
- Redis `Find(kid)` uses bounded by-`kid` lookup suitable for parse hot paths.
- Redis namespace, key format, DTO validation, TTL retention, and signing
  authority warnings are documented and tested.
- Rollout/rollback and safe-operation guidance is present in README pair,
  including the #173 limitation that fixed/local token-continuity migration is
  not available without a future import/export design or explicit invalidation.
- Stress tests use `GoroutineStressTester`.
- Context-aware Redis operations use `AsyncJobTester` where applicable.
- README pair documents Redis backend behavior and MongoDB #198 deferral.
- Step 2-R, Step 3-R, Step 6-R, and Step 7-R review gates close only with
  `P0=0 P1=0`.

## Approach Comparison

| Approach | Summary | Decision |
| --- | --- | --- |
| 1 | Add context-aware methods directly around the existing `Provider` and repository contract. | Rejected by user. It keeps one provider surface but does not make the distributed boundary as explicit as desired. |
| 2 | Keep provider API unchanged and make only repository operations context-aware. | Rejected. It cannot prove caller cancellation/deadline propagation through provider operations. |
| 3 | Add separate `DistributedProvider` that embeds/composes `*Provider` and adds distributed context-aware operations. | Selected. It preserves the #33 provider API while making distributed IO explicit and reusable for Redis now and MongoDB later. |

## Risks and Mitigations

| Risk | Mitigation |
| --- | --- |
| Context-free provider methods are used accidentally for distributed IO. | `DistributedProvider` uses private named composition instead of anonymous embedding, and docs/examples make `*Context` methods primary. |
| Key material persistence leaks secrets. | Treat Redis as signing authority, require trusted Redis/TLS/ACL guidance, keep DTO internal, avoid logging payloads, test representative error strings, and avoid public key-material accessors. |
| Redis rotation races create multiple current keys or lose retained keys. | Use Lua CAS for atomic rotate/forced rotate/capacity trim and stress with multiple providers. |
| Redis TTL evicts valid signing material. | Default `KeyTTL=0`; configured TTL must outlive retained key expiry plus leeway and is covered by tests. |
| Parse hot path scans all retained keys. | Use hash lookup by `kid` with bounded payload decode and benchmark/stress evidence. |
| Migration invalidates live tokens. | #173 docs explicitly limit fixed/local token-continuity migration, require an explicit invalidation decision when switching existing signers, and cover staged rollout, rollback constraints, and `DeleteAll` warnings. |
| Expired retained keys still verify tokens. | Repository `Current`/`Find` filter by provider clock and tests cover stale key rejection. |
| MongoDB scope expands this PR. | MongoDB is deferred to #198 and mentioned only as future reuse of the contract. |
| Kotlin compatibility assumptions creep in. | Docs state Go-owned Redis format and no Kotlin/JVM wire compatibility. |
| Cancellation is lost under wrapping. | Tests use `errors.Is` against `context.Canceled` and `context.DeadlineExceeded`; `AsyncJobTester` covers Redis path. |

## Validation Commands

```bash
git diff --check
go test -count=1 ./jwt ./jwt/redis
go test -race -count=1 ./jwt ./jwt/redis
go test -run '^$' -bench 'Distributed|Redis' -benchmem ./jwt ./jwt/redis
go test -count=1 ./...
make ci
```

## Step 2 Checklist Completion Report

| Item | Status | Notes |
| --- | --- | --- |
| Architecture pre-design ran or skip reason recorded | Done | Approach comparison and approved sections captured. |
| Step 1-R research incorporated | Done | Current Go source, #33 docs, Redis patterns, Kotlin parity, and issue #173 are cited. |
| Current-behavior claims cite current source/test/doc evidence | Done | Context section lists inspected files and docs. |
| Spec path confirmed inside feature worktree | Done | This file is under `docs/superpowers/specs`. |
| Risks/failure modes included when scope is non-trivial | Done | Risk table included. |
| Approach comparison and rejection rationale are research-based | Done | Approaches 1-3 recorded; Approach 3 selected by user. |
| `superpowers:brainstorming` process ran | Done | Context exploration, clarifying question, approach comparison, and section approvals completed. |
| User approval obtained per material design section | Done | Architecture, components/contract, and data/error/testing sections approved. |
| Spec code/API/test examples conform to `$bluetape-go-patterns` | Done | API is narrow, context-aware for IO, and includes error/concurrency validation. |
| Open questions resolved or explicitly escalated | Done | MongoDB deferred to #198. |
| Draft task list returned | Deferred to Step 3 | Implementation tasks belong in the Step 3 plan after Step 2-R review passes. |
