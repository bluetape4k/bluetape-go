# Issue #173 Distributed JWT KeyChain Repositories Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build Redis-backed distributed JWT KeyChain repositories and a context-aware `DistributedProvider` for issue #173.

**Architecture:** `DistributedProvider` uses private named composition over `*Provider` and delegates all distributed key lookup, rotation, and reset operations through a context-aware `DistributedKeyChainRepository`. Redis core storage and DTO reconstruction live inside package `jwt` so private `KeyChain` material is never exposed through public raw-key APIs; package `jwt/redis` remains the user-facing import facade. Redis is signing authority, with O(1)-style `kid` lookup, Lua-backed atomic rotation, strict DTO validation, and Testcontainers-backed verification.

**Tech Stack:** Go 1.25, `github.com/golang-jwt/jwt/v5`, `github.com/redis/go-redis/v9`, `github.com/testcontainers/testcontainers-go/modules/redis`, repo-local `testing/concurrency`, `$bluetape-go-patterns`, `$bluetape4k-diagram`, `$vega`.

---

Issue: #173
Milestone: 0.6.1
Spec: `docs/superpowers/specs/2026-06-12-issue-173-distributed-jwt-keychain-repositories-design.md`
Spec review: `docs/superpowers/reviews/2026-06-12-issue-173-distributed-jwt-keychain-repositories-step-2r-spec-review.md`
MongoDB backlog: #198

## Execution Boundary

Implement only the Redis-backed distributed KeyChain repository scope:

- package `jwt` changes needed for `DistributedProvider` and `DistributedKeyChainRepository`;
- package `jwt` for Redis storage core, DTO validation, Lua scripts, tests, and benchmarks;
- package `jwt/redis` for the public Redis facade, package docs, and examples;
- package README pair updates for distributed JWT usage and operations;
- root docs or release notes only when #173 metadata needs visibility;
- Step 6-R and Step 7-R artifacts after implementation.

Do not implement:

- MongoDB backend, which stays in #198;
- fixed/local raw-key export/import or seamless old-token migration;
- JWKS, JWE, OIDC, sessions, roles, auth middleware, background rotation timers, or Kotlin/Redisson wire compatibility;
- public raw HMAC/RSA key material accessors.

Apply `$bluetape-go-patterns` to every Go API, test, benchmark, example, README, and review task. Apply `$bluetape4k-diagram` and `$vega` if benchmark results are added to docs, verifier evidence, PR evidence, or README content.

## Current Evidence

- `jwt/provider.go` owns signing, parsing, parser hardening, provider config, `createKeyChain`, and local `Provider` methods.
- `jwt/repository.go` has only private in-memory repository behavior.
- `jwt/keychain.go` keeps key material private and exposes only metadata plus package-private signing/verification material helpers.
- `ratelimit/redis` shows existing `redis.Cmdable`, option normalization, namespace key construction, Lua `Eval`, and `%w` wrapping patterns.
- `testcontainers/redis` provides `Start(ctx, t)` for Redis-backed integration tests.
- `testing/concurrency` provides `GoroutineStressTester` and `AsyncJobTester`.
- `go.mod` already contains `github.com/redis/go-redis/v9` and `testcontainers-go/modules/redis`.
- Step 2-R closed with `P0=0 P1=0`; P2 carry-forward items are Redis command-count and benchmark budget, configured `KeyTTL` safety rule, and README runbook evidence.

## File Structure

| Path | Responsibility |
| --- | --- |
| `jwt/distributed_provider.go` | Public `DistributedProvider`, constructors, context-aware compose/parse/rotation/reset methods, and internal provider composition. |
| `jwt/distributed_repository.go` | Exported `DistributedKeyChainRepository` interface and repository contract helpers that are backend-neutral. |
| `jwt/distributed_provider_test.go` | Fake-repository provider tests for bootstrap, cross-instance use, parse/rotate behavior, errors, context propagation, and migration boundaries. |
| `jwt/redis_options.go` | Redis `Options`, validation, namespace normalization, capacity, payload size, `KeyTTL`, and key naming inside package `jwt`. |
| `jwt/redis_dto.go` | Internal JSON DTO encode/decode for HMAC/RSA KeyChain material with size and algorithm validation inside package `jwt`; no exported raw-key constructor. |
| `jwt/redis_repository.go` | Redis repository implementation for `Current`, `Find`, `Rotate`, `ForcedRotate`, and `DeleteAll`. |
| `jwt/redis_scripts.go` | Lua scripts and script result parsing for atomic current-hit, CAS store, forced rotate, trim, and delete. |
| `jwt/redis_repository_test.go` | Redis Testcontainers coverage for repository behavior, malformed state, namespace isolation, command counts, TTL, and cancellation. |
| `jwt/redis_benchmark_test.go` | Opt-in Redis benchmark harness for find, rotate current-hit, expired rotate, forced rotate, compose, and parse. |
| `jwt/redis/doc.go` | Facade package docs for Redis backend trust boundary and caller-owned client lifecycle. |
| `jwt/redis/redis.go` | Facade `Options`, `Repository`, and `New` aliases over the package-`jwt` Redis repository. |
| `jwt/redis/example_test.go` | Compile-checked distributed provider Redis examples. |
| `jwt/README.md`, `jwt/README.ko.md` | Public usage, migration, operational runbook, Redis trust boundary, key format, and MongoDB #198 deferral. |
| `docs/research/outputs/issue-173/` | Raw benchmark outputs when benchmark results are recorded. |
| `docs/images/readme-charts/` | Generated benchmark chart SVG/PNG if benchmark results enter docs or PR evidence. |
| `docs/superpowers/reviews/2026-06-12-issue-173-distributed-jwt-keychain-repositories-*.md` | Verifier, Step 6-R code review, Step 7-R PR review, and benchmark evidence artifacts. |

## Task Plan

### Task 0: Recheck Inputs and Lock Pre-Implementation Evidence

**complexity: medium**

**Files:**
- Create: `docs/superpowers/reviews/2026-06-12-issue-173-distributed-jwt-keychain-repositories-preimplementation-risk.md`
- Read: `jwt/provider.go`, `jwt/repository.go`, `jwt/keychain.go`, `ratelimit/redis/limiter.go`, `ratelimit/redis/options.go`, `testcontainers/redis/redis.go`, `testing/concurrency/*.go`

- [ ] **Step 1: Confirm branch and clean baseline**

Run:

```bash
pwd
git status --short --branch
git merge-base --is-ancestor origin/develop HEAD
```

Expected: working directory is the issue worktree, branch is `issue-173-distributed-jwt-keychain-repositories`, and command exit code confirms the branch contains `origin/develop` history.

- [ ] **Step 2: Record current dependency and API evidence**

Create the risk note with these sections:

```markdown
# Issue #173 Pre-Implementation Risk Note

## Current Source Evidence

- `jwt/provider.go`: `Provider` owns option normalization, key creation, signing, parsing, and local rotation.
- `jwt/repository.go`: in-memory repository is private and context-free.
- `jwt/keychain.go`: key material stays private; package-local methods allow Redis DTO reconstruction inside package `jwt`, while `jwt/redis` remains a facade.
- `ratelimit/redis`: Redis code uses caller-owned `redis.Cmdable`, Lua `Eval`, namespace keys, and `%w` wrapping.
- `testing/concurrency`: `GoroutineStressTester` covers rotate/sign/parse stress; `AsyncJobTester` covers cancellation/deadline paths.

## Locked Decisions

- `DistributedProvider` uses `provider *Provider`, not anonymous embedding.
- Redis core lives in package `jwt`, while package `jwt/redis` is a facade; this avoids public raw-key reconstruction helpers.
- Constructors require non-nil `context.Context`.
- Constructors require non-nil `DistributedKeyChainRepository` before bootstrap.
- Repository IO preserves caller cancellation and deadlines for `errors.Is`.
- Redis is signing authority; key values never appear in error strings, logs, README examples, or PR body.
- Redis `KeyTTL` default is `0`; a configured positive TTL must be greater than or equal to retained key validity plus repository-level `RetentionLeeway`.
- Benchmark results require a real chart asset if they are shown outside raw test output.

## Step 2-R P2 Carry Forward

- Pin Redis command-count expectations for hot paths before implementation.
- Pin benchmark budget expectations before implementation.
- Add README runbook tasks with safe Redis inspection and recovery checks.
```

- [ ] **Step 3: Verify evidence commands**

Run:

```bash
rg -n "type Provider|func \\(p \\*Provider\\) Compose|func \\(p \\*Provider\\) Parse|func \\(p \\*Provider\\) createKeyChain" jwt/provider.go
rg -n "type keyChainRepository|func \\(r \\*keyChainRepository\\) rotate|func \\(r \\*keyChainRepository\\) find" jwt/repository.go
rg -n "type Limiter|redis\\.Cmdable|Eval\\(" ratelimit/redis
rg -n "GoroutineStressTester|AsyncJobTester|type Task" testing/concurrency
git diff --check
```

Expected: every command prints matching evidence and `git diff --check` passes.

- [ ] **Step 4: Commit point**

Do not commit Task 0 alone unless Step 3-R requested preimplementation notes before the plan commit. Otherwise include the risk note in the first implementation commit.

### Task 1: Distributed Provider Contract Tests

**complexity: high**

**Files:**
- Create: `jwt/distributed_provider_test.go`
- Use: `$bluetape-go-patterns`

- [ ] **Step 1: Write fake repository tests before production code**

Add a fake repository inside `jwt/distributed_provider_test.go` with context recording and deterministic failures:

```go
type fakeDistributedRepository struct {
	mu         sync.Mutex
	keys       []*KeyChain
	err        error
	seenCtx    []context.Context
	rotateHits int
	forceHits  int
	deleteHits int
}

func (r *fakeDistributedRepository) Current(ctx context.Context, now time.Time) (*KeyChain, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seenCtx = append(r.seenCtx, ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.err != nil {
		return nil, r.err
	}
	for _, key := range r.keys {
		if !key.Expired(now) {
			return key, nil
		}
	}
	return nil, KeyError{Kind: ErrKeyNotFound, Err: errorsNew("current key not found")}
}
```

The same fake implements `Find`, `Rotate`, `ForcedRotate`, and `DeleteAll` by preserving context errors, wrapping configured repository errors, and trimming to test-local capacity.

- [ ] **Step 2: Add constructor and cross-instance tests**

Add tests with these exact names:

```go
func TestNewDistributedHMACProviderBootstrapsCurrentKey(t *testing.T) {}
func TestDistributedProviderComposeAndParseAcrossInstances(t *testing.T) {}
func TestDistributedProviderParsesRetainedKeyAfterForcedRotate(t *testing.T) {}
func TestDistributedProviderConstructorRejectsNilContext(t *testing.T) {}
func TestDistributedProviderConstructorRejectsNilRepository(t *testing.T) {}
func TestDistributedProviderConstructorRejectsTypedNilRepository(t *testing.T) {}
func TestDistributedProviderConstructorPreservesCanceledContext(t *testing.T) {}
func TestDistributedProviderConstructorPreservesExpiredDeadline(t *testing.T) {}
func TestDistributedProviderDoesNotExposeContextFreeProviderMethods(t *testing.T) {}
```

Required assertions:

- constructor calls `repo.Rotate(ctx, createKeyChain, now)`;
- one provider composes and a second provider parses through the shared fake repo;
- forced rotation keeps old retained `kid` verifiable;
- `context.Canceled` survives `errors.Is`;
- `context.DeadlineExceeded` survives `errors.Is`;
- nil context returns an `ErrInvalidOptions` compatible error;
- nil and typed-nil repositories return an `ErrInvalidOptions` compatible error before any bootstrap call;
- `DistributedProvider` has no anonymous `Provider` field visible through reflection.

- [ ] **Step 3: Add failure and boundary tests**

Add tests with these exact names:

```go
func TestDistributedProviderParseRejectsMissingKID(t *testing.T) {}
func TestDistributedProviderParseUnknownKIDReturnsKeyNotFound(t *testing.T) {}
func TestDistributedProviderParseExpiredRetainedKeyReturnsInvalidKey(t *testing.T) {}
func TestDistributedProviderRepositoryErrorsWrapCause(t *testing.T) {}
func TestDistributedProviderDeleteKeyChainsDelegatesToRepository(t *testing.T) {}
func TestDistributedProviderRSAAlgorithmValidation(t *testing.T) {}
func TestDistributedProviderFixedLocalMigrationIsNotImplicit(t *testing.T) {}
func TestDistributedProviderRejectsWrongAlgorithmRepositoryKey(t *testing.T) {}
func TestDistributedProviderRejectsWrongAlgorithmOnEveryRepositoryResult(t *testing.T) {}
```

Required assertions:

- missing `kid` fails at parse boundary with `ErrInvalidToken`;
- unknown `kid` remains `ErrKeyNotFound`;
- expired retained key remains `ErrInvalidKey`;
- repository error is preserved with `errors.Is`;
- `DeleteKeyChainsContext` increments fake delete count;
- HMAC constructor rejects RSA algorithm and RSA constructor rejects HMAC algorithm;
- constructor, compose, current, find, rotate, forced rotate, and parse reject repository keys whose algorithm does not match the provider algorithm;
- no public import/export migration method exists on `DistributedProvider`.

- [ ] **Step 4: Verify failing tests**

Run:

```bash
go test -count=1 ./jwt -run 'TestNewDistributed|TestDistributedProvider'
```

Expected before implementation: build fails because `DistributedProvider`, constructors, and `DistributedKeyChainRepository` do not exist.

### Task 2: Distributed Provider Implementation

**complexity: high**

**Files:**
- Create: `jwt/distributed_repository.go`
- Create: `jwt/distributed_provider.go`
- Modify: `jwt/provider.go`
- Test: `jwt/distributed_provider_test.go`
- Use: `$bluetape-go-patterns`

- [ ] **Step 1: Add repository interface and context helper**

Create `jwt/distributed_repository.go`:

```go
package jwt

import (
	"context"
	"reflect"
	"time"
)

// DistributedKeyChainRepository stores JWT KeyChains for multi-instance providers.
type DistributedKeyChainRepository interface {
	Current(ctx context.Context, now time.Time) (*KeyChain, error)
	Find(ctx context.Context, kid string, now time.Time) (*KeyChain, error)
	Rotate(ctx context.Context, create func() (*KeyChain, error), now time.Time) (*KeyChain, error)
	ForcedRotate(ctx context.Context, create func() (*KeyChain, error), now time.Time) (*KeyChain, error)
	DeleteAll(ctx context.Context) error
}

func requireContext(ctx context.Context) error {
	if ctx == nil {
		return OptionError{Option: "context", Err: errorsNew("must not be nil")}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func requireDistributedRepository(repo DistributedKeyChainRepository) error {
	if repo == nil {
		return OptionError{Option: "repository", Err: errorsNew("must not be nil")}
	}
	value := reflect.ValueOf(repo)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if value.IsNil() {
			return OptionError{Option: "repository", Err: errorsNew("must not be nil")}
		}
	}
	return nil
}

func createWithContext(ctx context.Context, create func() (*KeyChain, error)) func() (*KeyChain, error) {
	return func() (*KeyChain, error) {
		if err := requireContext(ctx); err != nil {
			return nil, err
		}
		key, err := create()
		if err != nil {
			return nil, err
		}
		if err := requireContext(ctx); err != nil {
			return nil, err
		}
		return key, nil
	}
}
```

- [ ] **Step 2: Add constructor shape**

Create `jwt/distributed_provider.go` with this public shape:

```go
type DistributedProvider struct {
	provider *Provider
	repo     DistributedKeyChainRepository
}

func NewDistributedHMACProvider(ctx context.Context, repo DistributedKeyChainRepository, algorithm Algorithm, options ...ProviderOption) (*DistributedProvider, error) {
	if _, ok := algorithm.hmacSecretLength(); !ok {
		return nil, OptionError{Option: "algorithm", Err: errorsNew("algorithm must be hmac")}
	}
	return newDistributedProvider(ctx, repo, algorithm, options...)
}

func NewDistributedRSAProvider(ctx context.Context, repo DistributedKeyChainRepository, algorithm Algorithm, options ...ProviderOption) (*DistributedProvider, error) {
	if !algorithm.isRSA() {
		return nil, OptionError{Option: "algorithm", Err: errorsNew("algorithm must be rsa")}
	}
	return newDistributedProvider(ctx, repo, algorithm, options...)
}
```

`newDistributedProvider` first calls `requireContext(ctx)` and `requireDistributedRepository(repo)` before any bootstrap call. It then normalizes provider options, sets default entropy when nil, builds `&Provider{algorithm: algorithm, cfg: cfg}`, calls `repo.Rotate(ctx, createWithContext(ctx, p.createKeyChain), p.now())`, validates returned key algorithm, and returns `&DistributedProvider{provider: p, repo: repo}`.

- [ ] **Step 3: Add context-aware public methods**

Add methods:

```go
func (p *DistributedProvider) ComposeContext(ctx context.Context, options ...ComposeOption) (string, error)
func (p *DistributedProvider) ParseContext(ctx context.Context, token string, options ...ParseOption) (*Reader, error)
func (p *DistributedProvider) CurrentKeyChainContext(ctx context.Context) (*KeyChain, error)
func (p *DistributedProvider) RotateContext(ctx context.Context) (*KeyChain, error)
func (p *DistributedProvider) ForcedRotateContext(ctx context.Context) (*KeyChain, error)
func (p *DistributedProvider) FindKeyChainContext(ctx context.Context, kid string) (*KeyChain, error)
func (p *DistributedProvider) DeleteKeyChainsContext(ctx context.Context) error
```

Method rules:

- `ctx == nil` fails with `ErrInvalidOptions`;
- canceled or expired context returns the original context error;
- `ComposeContext` obtains the signing key via `repo.Rotate(ctx, create, now)`;
- `ParseContext` uses `golangjwt.ParseWithClaims` with a distributed `Keyfunc` that validates `kid`, rejects unsupported inbound JOSE headers, calls `repo.Find(ctx, kid, now)`, checks algorithm equality, and returns verification material;
- methods do not call context-free `Provider.Compose`, `Provider.Parse`, or in-memory repository operations.

- [ ] **Step 4: Extract reusable signing/parsing helpers from `Provider` without widening public API**

Modify `jwt/provider.go` only enough to reuse package-private helpers:

```go
func (p *Provider) composeWithKey(key *KeyChain, options ...ComposeOption) (string, error)
func (p *Provider) parseWithKeyFunc(tokenValue string, keyFunc golangjwt.Keyfunc, options ...ParseOption) (*Reader, error)
```

`Provider.Compose` becomes `return p.composeWithKey(key, options...)`. `Provider.Parse` becomes `return p.parseWithKeyFunc(tokenValue, p.keyFunc(cfg.now), options...)` or an equivalent helper that avoids changing public behavior.

- [ ] **Step 5: Run targeted provider tests**

Run:

```bash
gofmt -w jwt/distributed_repository.go jwt/distributed_provider.go jwt/provider.go jwt/distributed_provider_test.go
go test -count=1 ./jwt -run 'TestNewDistributed|TestDistributedProvider'
go test -count=1 ./jwt
```

Expected: distributed provider tests pass, and existing JWT tests keep passing.

### Task 3: Redis Options, Namespace, and Key Layout

**complexity: high**

**Files:**
- Create: `jwt/redis_options.go`
- Create: `jwt/redis_options_test.go`
- Create: `jwt/redis/doc.go`
- Use: `$bluetape-go-patterns`

- [ ] **Step 1: Write options tests first**

Add tests:

```go
func TestOptionsNormalizeRequiresClient(t *testing.T) {}
func TestOptionsNormalizeRequiresNamespace(t *testing.T) {}
func TestOptionsNormalizeRejectsUnsafeNamespace(t *testing.T) {}
func TestOptionsNormalizeCapacityBounds(t *testing.T) {}
func TestOptionsNormalizePayloadBounds(t *testing.T) {}
func TestRepositoryKeyNamesAreVersionedAndNamespaced(t *testing.T) {}
```

Required namespace cases:

```go
valid := []string{"prod", "tenant-1", "tenant_1", "tenant.1"}
invalid := []string{"", " ", "a:b", "tenant/name", "tenant\nname", "tenant name", "tenant\tname", "tenant{name}", "tenant*name", "tenant%name", "한글", strings.Repeat("x", 129)}
```

Required namespace rule: trim surrounding whitespace, require non-empty, enforce max `128` bytes, allow only ASCII `[A-Za-z0-9._-]`, and reject all whitespace, controls, Redis delimiters, glob/metacharacters, and non-ASCII. Static options tests cover only static bounds and format; TTL-vs-key-validity safety is tested in Task 6 where provider key validity exists.

- [ ] **Step 2: Implement normalized options**

Create `Options`:

```go
type RedisRepositoryOptions struct {
	Client      redis.Cmdable
	Namespace   string
	Capacity    int
	KeyTTL      time.Duration
	RetentionLeeway time.Duration
	MaxKeyBytes int
}
```

Internal normalized fields in package `jwt`:

```go
type options struct {
	client      redis.Cmdable
	namespace   string
	capacity    int
	keyTTL      time.Duration
	retentionLeeway time.Duration
	maxKeyBytes int
}
```

Defaults and bounds:

- capacity default `10`, min `2`, max `1000`;
- max key payload default `32 << 10`, min `1024`, max `1 << 20`;
- retention leeway default `0`, must not be negative, and represents the maximum parse leeway that Redis TTL retention is expected to preserve;
- namespace byte limit `128`;
- key prefix `bluetape:jwt:v1:<namespace>`.

- [ ] **Step 3: Add key-name helpers**

Implement helpers:

```go
func (o options) metaKey() string
func (o options) currentKey() string
func (o options) keysKey() string
func (o options) orderKey() string
```

Expected names for namespace `prod`:

```text
bluetape:jwt:v1:prod:meta
bluetape:jwt:v1:prod:current
bluetape:jwt:v1:prod:keys
bluetape:jwt:v1:prod:order
```

- [ ] **Step 4: Verify**

Run:

```bash
gofmt -w jwt/redis_options.go jwt/redis_options_test.go jwt/redis/doc.go
go test -count=1 ./jwt -run 'TestOptions|TestRepositoryKeyNames'
```

Expected: tests pass after implementation.

### Task 4: Redis DTO Codec and Secret-Safe Validation

**complexity: high**

**Files:**
- Create: `jwt/redis_dto.go`
- Create: `jwt/redis_dto_test.go`
- Use: `$bluetape-go-patterns`

- [ ] **Step 1: Write DTO tests first**

Add tests:

```go
func TestEncodeDecodeHMACKeyChain(t *testing.T) {}
func TestEncodeDecodeRSAKeyChain(t *testing.T) {}
func TestDecodeRejectsOversizedPayloadBeforeJSON(t *testing.T) {}
func TestDecodeRejectsUnknownVersion(t *testing.T) {}
func TestDecodeRejectsInvalidKID(t *testing.T) {}
func TestDecodeRejectsAlgorithmFamilyMismatch(t *testing.T) {}
func TestDecodeRejectsShortHMACMaterial(t *testing.T) {}
func TestDecodeRejectsInvalidRSAMaterial(t *testing.T) {}
func TestDTOErrorsDoNotLeakKeyMaterial(t *testing.T) {}
func TestRedisDTORequiresPackagePrivateReconstruction(t *testing.T) {}
```

Use representative secret strings such as `super-secret-material` in inputs and assert they are absent from every returned error string.

- [ ] **Step 2: Implement internal DTO**

Create internal DTO:

```go
type keyChainDTO struct {
	Version   int       `json:"version"`
	KID       string    `json:"kid"`
	Algorithm string   `json:"algorithm"`
	Family    string   `json:"family"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	HMAC      string    `json:"hmac,omitempty"`
	RSA       string    `json:"rsa,omitempty"`
}
```

Rules:

- version is `1`;
- family is `hmac` or `rsa`;
- HMAC uses base64 encoding;
- RSA private key uses PKCS#1 PEM or DER wrapped as base64;
- decode validates payload byte length before `json.Unmarshal`;
- decode calls package-private constructors in package `jwt`, not public raw-key constructors and not struct field mutation from outside package.

- [ ] **Step 3: Preserve the no public raw-key API boundary**

Keep DTO encode/decode and Redis repository core in package `jwt` so they can use `newHMACKeyChain`, `newRSAKeyChain`, and package-private signing material helpers. Do not add any exported functions that accept raw HMAC secrets or RSA private keys for repository loading. Package `jwt/redis` must remain a facade around `jwt.NewRedisRepository` and must not expose DTO bytes, signing material, or repository seed/import helpers.

- [ ] **Step 4: Verify**

Run:

```bash
gofmt -w jwt/redis_dto.go jwt/redis_dto_test.go jwt/keychain.go
go test -count=1 ./jwt -run 'TestEncodeDecode|TestDecode|TestDTO|TestRedisDTO'
go test -count=1 ./jwt
```

Expected: DTO tests pass and `jwt` remains green.

### Task 5: Redis Repository Read Paths and Delete

**complexity: high**

**Files:**
- Create: `jwt/redis_repository.go`
- Create: `jwt/redis_scripts.go`
- Create: `jwt/redis_repository_test.go`
- Create: `jwt/redis/redis.go`
- Create: `jwt/redis/redis_test.go`
- Use: `$bluetape-go-patterns`

- [ ] **Step 1: Write read/delete integration tests first**

Use Testcontainers Redis serially. Add tests:

```go
func TestRepositoryCurrentReturnsNewestNonExpiredKey(t *testing.T) {}
func TestRepositoryFindUsesKIDHashLookup(t *testing.T) {}
func TestRepositoryFindRejectsMissingUnknownAndExpiredKID(t *testing.T) {}
func TestRepositoryDeleteAllRemovesNamespacedState(t *testing.T) {}
func TestRepositoryNamespaceIsolation(t *testing.T) {}
func TestRepositoryAlgorithmFamilyMismatchFails(t *testing.T) {}
func TestRepositoryContextCancellationPreserved(t *testing.T) {}
func TestRepositoryDeadlinePreserved(t *testing.T) {}
func TestRepositoryFindCommandBudget(t *testing.T) {}
func TestRepositoryCurrentCommandBudget(t *testing.T) {}
func TestRedisFacadeNewReturnsRepository(t *testing.T) {}
```

Expected Redis commands for `Find`: exactly one `HGET` by requested `kid`, DTO decode, and no retained-list scan. Expected Redis commands for `Current`: current-pointer read plus one `HGET`. Implement command-capture tests with `redis.Hook`, an instrumented `redis.Cmdable`, or an equivalent wrapper. Assert no `SCAN`, `KEYS`, `LRANGE`, `ZRANGE`, `HGETALL`, or retained-list read appears in the captured command log. Record command logs in verifier evidence.

- [ ] **Step 2: Implement repository constructor and read paths**

Public shape:

```go
type RedisRepository struct {
	client redis.Cmdable
	opts   options
}

var _ DistributedKeyChainRepository = (*RedisRepository)(nil)

func NewRedisRepository(options RedisRepositoryOptions) (*RedisRepository, error)
```

Package `jwt/redis` exposes aliases only after `jwt.RedisRepository` and `jwt.NewRedisRepository` exist:

```go
type Options = jwt.RedisRepositoryOptions
type Repository = jwt.RedisRepository

func New(options Options) (*Repository, error) {
	return jwt.NewRedisRepository(options)
}
```

Method behavior:

- `Current(ctx, now)` reads current pointer, then the hash entry for that `kid`, then decodes and expiry-checks;
- `Find(ctx, kid, now)` validates `kid`, performs `HGET keys kid`, decodes only that payload, and expiry-checks;
- `DeleteAll(ctx)` deletes `meta`, `current`, `keys`, and `order`;
- every method rejects nil context with `ErrInvalidOptions` and preserves canceled/deadline errors.

- [ ] **Step 3: Use `AsyncJobTester` for cancellation tests**

Add cancellation test structure:

```go
tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
	Workers:       2,
	RoundsPerTask: 5,
	Timeout:       2 * time.Second,
})
tester.RunT(t, func(ctx context.Context) error {
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err := repo.Find(canceled, "kid", time.Now())
	if !errors.Is(err, context.Canceled) {
		return fmt.Errorf("expected context.Canceled, got %v", err)
	}
	return nil
})
```

- [ ] **Step 4: Verify**

Run:

```bash
gofmt -w jwt/redis_repository.go jwt/redis_scripts.go jwt/redis_repository_test.go jwt/redis/redis.go jwt/redis/redis_test.go
go test -p 1 -count=1 ./jwt -run 'TestRepository(Current|Find|DeleteAll|Namespace|Algorithm|Context|Deadline)'
go test -count=1 ./jwt/redis -run 'TestRedisFacade'
```

Expected: Testcontainers Redis suite passes serially.

### Task 6: Redis Atomic Rotate, Forced Rotate, Capacity, and TTL

**complexity: high**

**Files:**
- Modify: `jwt/redis_repository.go`
- Modify: `jwt/redis_scripts.go`
- Modify: `jwt/redis_repository_test.go`
- Use: `$bluetape-go-patterns`

- [ ] **Step 1: Write rotate tests first**

Add tests:

```go
func TestRepositoryRotateReturnsCurrentWithoutCallingCreate(t *testing.T) {}
func TestRepositoryRotateStoresCandidateWhenNoCurrentKeyExists(t *testing.T) {}
func TestRepositoryRotateCASReturnsConcurrentWinner(t *testing.T) {}
func TestRepositoryForcedRotateAlwaysStoresCandidate(t *testing.T) {}
func TestRepositoryCapacityTrimPreservesNewestKeys(t *testing.T) {}
func TestRepositoryKeyTTLZeroLeavesKeysWithoutRedisExpiration(t *testing.T) {}
func TestRepositoryConfiguredKeyTTLRetainsNonExpiredKeys(t *testing.T) {}
func TestRepositoryRejectsKeyTTLShorterThanRetainedKeyValidityAndRetentionLeeway(t *testing.T) {}
func TestRepositoryRotateCanceledAfterCreateDoesNotPersistCandidate(t *testing.T) {}
func TestRepositoryForcedRotateCanceledAfterCreateDoesNotPersistCandidate(t *testing.T) {}
func TestRepositoryRotateCurrentHitCommandBudget(t *testing.T) {}
```

The current-hit test must fail if `create` is called:

```go
created := false
key, err := repo.Rotate(ctx, func() (*jwt.KeyChain, error) {
	created = true
	return nil, errors.New("create must not run on current hit")
}, now)
if err != nil {
	t.Fatalf("rotate current hit: %v", err)
}
if created {
	t.Fatalf("create ran on current-hit rotate")
}
_ = key
```

- [ ] **Step 2: Implement two-phase CAS rotate**

`Rotate` algorithm:

1. Lua/read phase returns current non-expired payload when current pointer references a valid key.
2. If no valid current exists, Go checks `ctx`, calls `create` once, checks `ctx` again, encodes candidate, validates `KeyTTL >= candidate.ExpiresAt().Sub(now)+RetentionLeeway` when `KeyTTL > 0`.
3. Lua CAS phase rechecks current pointer, stores candidate only when no concurrent non-expired current won, updates meta/current/hash/zset, trims to capacity, applies Redis TTL only when `KeyTTL > 0`, and returns stored or winning payload.
4. Context cancellation after key creation but before store returns the context error and avoids storing the candidate.

- [ ] **Step 3: Implement forced rotate**

`ForcedRotate` algorithm:

1. Check context.
2. Call `create` exactly once.
3. Check context again.
4. Store candidate atomically, update current pointer, trim by capacity, apply configured Redis TTL when valid.
5. Return decoded stored candidate.

Add a forced-rotate cancellation test where `create` succeeds, the test hook cancels context before the store phase, and Redis `current`, `keys`, and `order` remain unchanged.

- [ ] **Step 4: Pin Redis command-count and benchmark budget in evidence**

Add comments and verifier-ready evidence:

```markdown
## Redis Command Budget

- `Find`: one `HGET` against `keys`, plus local DTO decode.
- `Current`: one current-pointer read plus one `HGET`.
- `Rotate` current-hit: one Lua/read phase, no `create` call.
- `Rotate` empty/expired: one Lua/read phase, one provider `create`, one Lua/CAS store phase.
- `ForcedRotate`: one provider `create`, one Lua/store phase.
- Command-capture tests assert the command budget and fail on scan/list/all-key commands.

## Benchmark Budget

- Benchmarks are opt-in because Redis Testcontainers are external IO.
- PR evidence must include `ns/op`, `B/op`, `allocs/op`, and Redis command-count notes.
- Before Step 6-R, record either explicit per-benchmark ceilings or a documented no-regression baseline decision with rationale. A regression beyond the recorded budget blocks release evidence until explained or fixed.
- Any published benchmark table must have a chart asset in `docs/images/readme-charts`.
```

- [ ] **Step 5: Verify**

Run:

```bash
gofmt -w jwt/redis_repository.go jwt/redis_scripts.go jwt/redis_repository_test.go
go test -p 1 -count=1 ./jwt -run 'TestRepository(Rotate|ForcedRotate|Capacity|.*KeyTTL|ConfiguredKeyTTL|RejectsKeyTTL)'
```

Expected: Redis rotation and TTL tests pass.

### Task 7: Cross-Instance Integration, Stress, Race, and Cancellation

**complexity: high**

**Files:**
- Modify: `jwt/distributed_provider_test.go`
- Modify: `jwt/redis_repository_test.go`
- Create: `jwt/redis_integration_test.go`
- Use: `$bluetape-go-patterns`

- [ ] **Step 1: Write cross-instance Redis tests**

Add tests:

```go
func TestRedisDistributedProvidersShareHMACKeysAcrossInstances(t *testing.T) {}
func TestRedisDistributedProvidersShareRSAKeysAcrossInstances(t *testing.T) {}
func TestRedisDistributedProviderParsesAfterForcedRotationByKID(t *testing.T) {}
func TestRedisDistributedProviderRejectsEvictedKID(t *testing.T) {}
func TestRedisDistributedProviderRepositoryFailurePropagates(t *testing.T) {}
func TestRedisDistributedProviderConstructorCanceledAfterCreateDoesNotPersistCandidate(t *testing.T) {}
func TestRedisDistributedProviderConstructorDeadlineAfterCreateDoesNotPersistCandidate(t *testing.T) {}
```

Each test uses caller-owned `redis.Client`, defers `Close`, and constructs a fresh namespace.

- [ ] **Step 2: Add `GoroutineStressTester` stress tests**

Add:

```go
func TestRedisDistributedProviderConcurrentRotateSignParseStress(t *testing.T) {}
func TestRedisRepositoryConcurrentEmptyRotateConvergesOnOneCurrentWinner(t *testing.T) {}
```

Use:

```go
tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
	Workers:       8,
	RoundsPerTask: 20,
	Timeout:       10 * time.Second,
})
```

Stress invariants:

- every composed token either parses immediately or fails only with an expected retained-key eviction after capacity pressure;
- concurrent empty rotate converges to one current `kid`;
- create call count is bounded by concurrency attempts and no unbounded retry loop exists;
- no goroutine leak or data race appears under the race gate.

- [ ] **Step 3: Add cancellation/deadline provider tests with `AsyncJobTester`**

Add:

```go
func TestRedisDistributedProviderContextCancellationStress(t *testing.T) {}
func TestRedisDistributedProviderDeadlineStress(t *testing.T) {}
```

Use `AsyncJobTester` and assert `errors.Is(err, context.Canceled)` or `errors.Is(err, context.DeadlineExceeded)`.

- [ ] **Step 4: Verify targeted and race tests**

Run:

```bash
gofmt -w jwt/distributed_provider_test.go jwt/redis_repository_test.go jwt/redis_integration_test.go
go test -p 1 -count=1 ./jwt ./jwt/redis
go test -race -p 1 -count=1 ./jwt ./jwt/redis
```

Expected: targeted package tests and race tests pass.

### Task 8: Benchmarks and Chart Asset Gate

**complexity: medium**

**Files:**
- Create: `jwt/redis_benchmark_test.go`
- Create when benchmark results are recorded: `docs/research/outputs/issue-173/distributed-jwt-redis-bench.txt`
- Create when benchmark results are recorded: `docs/images/readme-charts/distributed-jwt-redis-benchmark.svg`
- Create when benchmark results are recorded: `docs/images/readme-charts/distributed-jwt-redis-benchmark.png`
- Use: `$bluetape-go-patterns`
- Use: `$bluetape4k-diagram`
- Use: `$vega`

- [ ] **Step 1: Add benchmark harness**

Add benchmarks:

```go
func BenchmarkRedisRepositoryFind(b *testing.B) {}
func BenchmarkRedisRepositoryRotateCurrentHit(b *testing.B) {}
func BenchmarkRedisRepositoryRotateExpired(b *testing.B) {}
func BenchmarkRedisRepositoryForcedRotate(b *testing.B) {}
func BenchmarkRedisDistributedProviderComposeContext(b *testing.B) {}
func BenchmarkRedisDistributedProviderParseContext(b *testing.B) {}
```

Benchmark rules:

- Testcontainers setup runs outside the timed loop.
- Use `b.ReportAllocs()`.
- Use a fresh namespace per benchmark.
- Do not run Testcontainers benchmarks in parallel lanes.
- Add `b.Cleanup` to close Redis client and terminate container.

- [ ] **Step 2: Run opt-in benchmark smoke**

Run:

```bash
go test -p 1 -run '^$' -bench 'BenchmarkRedis(Repository|Distributed)' -benchtime=100ms -benchmem ./jwt
```

Expected: benchmark smoke produces `ns/op`, `B/op`, and `allocs/op`.

- [ ] **Step 3: If benchmark results are used in docs, verifier, PR, or README, create chart assets**

Store raw output:

```bash
mkdir -p docs/research/outputs/issue-173
go test -p 1 -run '^$' -bench 'BenchmarkRedis(Repository|Distributed)' -benchtime=100ms -benchmem ./jwt | tee docs/research/outputs/issue-173/distributed-jwt-redis-bench.txt
```

Create a Vega-Lite spec that renders bar charts for `ns/op`, `B/op`, and `allocs/op` from parsed benchmark rows. Save the spec next to the chart source if a JSON chart source is used, and render to:

```text
docs/images/readme-charts/distributed-jwt-redis-benchmark.svg
docs/images/readme-charts/distributed-jwt-redis-benchmark.png
```

Chart requirements:

- use bars with labeled axes, not a numeric heatmap;
- show benchmark names on the category axis;
- separate latency from allocation metrics so the scale is readable;
- include command-count notes in nearby prose, not inside the bars;
- render PNG and visually verify dimensions and legibility.

- [ ] **Step 4: Verify chart and benchmark evidence**

Run the published-benchmark branch when benchmark results are included in docs, verifier, PR evidence, or README:

```bash
go test -p 1 -run '^$' -bench 'BenchmarkRedis(Repository|Distributed)' -benchtime=100ms -benchmem ./jwt
test -s docs/images/readme-charts/distributed-jwt-redis-benchmark.svg
test -s docs/images/readme-charts/distributed-jwt-redis-benchmark.png
file docs/images/readme-charts/distributed-jwt-redis-benchmark.png
git diff --check
```

Run the smoke-only branch when benchmark numbers are not published:

```bash
go test -p 1 -run '^$' -bench 'BenchmarkRedis(Repository|Distributed)' -benchtime=100ms -benchmem ./jwt
rg -n "chart asset N/A: benchmark results not included in docs or PR evidence" docs/superpowers/reviews/2026-06-12-issue-173-distributed-jwt-keychain-repositories-verifier.md
git diff --check
```

Expected: published benchmark evidence has SVG/PNG files and a readable PNG; unpublished smoke-only evidence has the explicit verifier N/A line.

### Task 9: Examples, README Pair, and Operator Runbook

**complexity: medium**

**Files:**
- Create: `jwt/redis/example_test.go`
- Modify: `jwt/README.md`
- Modify: `jwt/README.ko.md`
- Modify when needed: `README.md`
- Modify when needed: `README.ko.md`
- Use: `$bluetape-go-patterns`

- [ ] **Step 1: Add compile-checked examples**

Examples:

```go
func ExampleRepository_distributedHMACProvider() {}
func ExampleRepository_distributedRSAProvider() {}
func ExampleRepository_contextTimeout() {}
```

Example content must show:

- caller-owned Redis client and `defer client.Close()`;
- `context.WithTimeout`;
- `redisjwt.New(redisjwt.Options{Client: client, Namespace: "service-auth"})`;
- `jwt.NewDistributedHMACProvider(ctx, repo, jwt.HS256)`;
- `ComposeContext` and `ParseContext`;
- no raw secret or private key printing.

- [ ] **Step 2: Update README pair for usage and migration**

Add sections to both README files:

- Redis-backed distributed providers;
- context-aware constructors and methods;
- cross-instance rotate/sign/parse behavior;
- MongoDB deferred to #198;
- Go-owned Redis key format and no Kotlin/JVM wire compatibility;
- fixed/local token continuity limitation and explicit invalidation decision;
- `DeleteKeyChainsContext` is for tests or explicit operator reset only.
- unsupported capabilities remain out of scope: JWKS, JWE, OIDC, auth middleware, sessions, roles, and background rotation timers.

- [ ] **Step 3: Add operator runbook**

README runbook must include these Redis inspection commands:

```bash
redis-cli --tls HGET bluetape:jwt:v1:<namespace>:meta version
redis-cli --tls GET bluetape:jwt:v1:<namespace>:current
redis-cli --tls HLEN bluetape:jwt:v1:<namespace>:keys
redis-cli --tls ZCARD bluetape:jwt:v1:<namespace>:order
redis-cli --tls PTTL bluetape:jwt:v1:<namespace>:keys
```

Runbook must also cover:

- trusted Redis boundary, TLS, ACL, persistence, and eviction policy;
- TLS required for remote Redis;
- ACL user with least-privilege access to `bluetape:jwt:v1:<namespace>:*`;
- no shared untrusted Redis and no cross-tenant namespace reuse;
- persistence or backup expectations for retained signing keys;
- `maxmemory-policy noeviction` or equivalent sizing guidance;
- warning that eviction of retained keys can invalidate still-live tokens;
- Redis outage behavior and retry ownership;
- namespace misconfiguration recovery;
- connection/TLS/ACL failure vs empty namespace vs wrong namespace diagnostics;
- current pointer, hash cardinality, zset cardinality, and PTTL checks;
- roll-forward recovery by fixing namespace/config before any reset;
- explicit token invalidation decision when rollback or reset loses retained keys;
- safe rollback without `DeleteKeyChainsContext`;
- unknown-`kid`, `ErrKeyNotFound`, and `ErrInvalidKey` monitoring signals;
- Redis command errors and context timeout/deadline monitoring;
- secret-safe logging guidance that permits namespace and `kid` but never token strings, HMAC secrets, RSA private keys, or serialized key payloads.

- [ ] **Step 4: Verify docs and examples**

Run:

```bash
gofmt -w jwt/redis/example_test.go
go test -count=1 ./jwt/redis -run Example
rg -n "DistributedProvider|ComposeContext|ParseContext|MongoDB|#198|Redis|signing authority|DeleteKeyChainsContext|Kotlin|wire compatibility|token invalidation|redis-cli --tls|ACL|PTTL|JWKS|JWE|OIDC|auth middleware|background rotation" jwt/README.md
rg -n "DistributedProvider|ComposeContext|ParseContext|MongoDB|#198|Redis|signing authority|DeleteKeyChainsContext|Kotlin|wire compatibility|token invalidation|redis-cli --tls|ACL|PTTL|JWKS|JWE|OIDC|auth middleware|background rotation" jwt/README.ko.md
git diff --check
```

Expected: examples compile and both README files contain usage, trust boundary, runbook, and deferral coverage.

### Task 10: Validation, Verifier, and Step 6-R 7-Tier Code Review

**complexity: high**

**Files:**
- Create: `docs/superpowers/reviews/2026-06-12-issue-173-distributed-jwt-keychain-repositories-verifier.md`
- Create: `docs/superpowers/reviews/2026-06-12-issue-173-distributed-jwt-keychain-repositories-step-6r-code-review.md`
- Use: `$bluetape-go-patterns`

- [ ] **Step 1: Run validation commands**

Run:

```bash
gofmt -w jwt jwt/redis
go test -p 1 -count=1 ./jwt ./jwt/redis
go test -race -p 1 -count=1 ./jwt ./jwt/redis
go test -p 1 -run '^$' -bench 'BenchmarkRedis(Repository|Distributed)' -benchtime=100ms -benchmem ./jwt
go test -p 1 -count=1 ./...
golangci-lint config verify
make ci
git diff --check
```

If a broad command fails due to an unrelated package or environment dependency, record the exact package, command, and error in the verifier, then run the next-best targeted package commands.

- [ ] **Step 2: Write verifier artifact**

Verifier must map every acceptance criterion to evidence:

```markdown
| Acceptance | Evidence | Status |
| --- | --- | --- |
| `DistributedProvider` composes `*Provider` and adds context-aware operations. | `jwt/distributed_provider.go`; provider tests. | PASS |
| Redis supports current, find, rotate, forced rotate, trim, expiry, delete. | `jwt/redis_repository.go`; Redis tests. | PASS |
| Two instances verify tokens across rotation by `kid`. | `TestRedisDistributedProviderParsesAfterForcedRotationByKID`. | PASS |
| Stress tests use `GoroutineStressTester`. | `jwt/redis_integration_test.go`. | PASS |
| Redis cancellation uses `AsyncJobTester`. | `jwt/redis_repository_test.go`. | PASS |
| Redis command budget is enforced. | Command-capture tests and verifier command log. | PASS |
| Benchmark results have chart asset when published. | Chart path or explicit N/A reason. | PASS |
```

- [ ] **Step 3: Run Step 6-R as 6 independent subagent lanes plus main integration**

Mandatory lanes:

1. performance;
2. stability;
3. security;
4. operator/Ops;
5. developer/API;
6. user/caller.

Each lane reviews the implemented diff, spec, plan, verifier, and validation evidence. Each lane returns only P0/P1/P2/P3 findings with file:line evidence. Main session integrates results, covers documentation/release/evidence integrity, deduplicates findings, normalizes severity, fixes P0/P1, reruns affected lanes, and closes only when latest integrated table has `P0=0 P1=0`.

- [ ] **Step 4: Verify Step 6-R closure**

Run:

```bash
rg -n "Tier 1 Performance|Tier 2 Stability|Tier 3 Security|Tier 4 Operator|Tier 5 Developer|Tier 6 User|P0=0 P1=0|Gate verdict: PASS" docs/superpowers/reviews/2026-06-12-issue-173-distributed-jwt-keychain-repositories-step-6r-code-review.md
git diff --check
```

Expected: review artifact proves 6 lane results, main integration, and final blocker-free gate.

### Task 11: Commit, PR, Step 7-R Review, CI, and Merge Gate

**complexity: medium**

**Files:**
- Git commit and GitHub PR state
- PR body from `~/.codex/skills/bluetape4k-workflow/templates/pr-body-step-dod.md`
- Create: `docs/superpowers/reviews/2026-06-12-issue-173-distributed-jwt-keychain-repositories-step-7r-pr-review.md`
- Use: `$bluetape-go-patterns`

- [ ] **Step 1: Commit implementation with Lore trailers**

Run:

```bash
git status --short
git add jwt docs README.md README.ko.md CHANGELOG.md WIP.md
git commit
```

Commit message must be English and follow Lore protocol:

```text
Enable distributed JWT key rotation over Redis

Constraint: Redis stores JWT signing authority and must remain namespace-isolated.
Rejected: Anonymous Provider embedding | It exposes context-free local APIs on distributed providers.
Confidence: high
Scope-risk: broad
Directive: Keep MongoDB and raw-key import/export out of issue #173.
Tested: go test -p 1 -count=1 ./jwt ./jwt/redis; go test -race -p 1 -count=1 ./jwt ./jwt/redis; targeted Redis benchmarks; git diff --check
Not-tested: Production Redis ACL/TLS deployment; MongoDB backend
```

- [ ] **Step 2: Push and create PR**

Run:

```bash
git push -u origin issue-173-distributed-jwt-keychain-repositories
gh pr create --base develop --head issue-173-distributed-jwt-keychain-repositories --title "Add Redis-backed distributed JWT KeyChain repositories" --body-file /tmp/issue-173-pr-body.md --assignee debop
gh pr edit --add-label "type: task" --add-label "priority: p1" --add-label "area: utilities"
gh pr view --json url,number,title,body,assignees,labels,milestone,state
```

PR body must include `Fixes #173`, link #198 as follow-up, summarize validation, and end with `## DoD Status`.

- [ ] **Step 3: Run Step 7-R as 6 independent subagent lanes plus main integration**

Use the same fixed lane split:

1. performance;
2. stability;
3. security;
4. operator/Ops;
5. developer/API;
6. user/caller.

Main session integrates PR metadata, live PR body, issue linkage, CI state, docs/release evidence, and final P0/P1 verdict. Do not spawn a seventh subagent for integration.

- [ ] **Step 4: Verify PR and CI**

Run:

```bash
gh issue view 173 --json assignees,labels,milestone,title,state
gh pr view --json url,number,title,body,assignees,labels,milestone,state,mergeStateStatus
gh pr checks
rg -n "## DoD Status" /tmp/issue-173-pr-body.md
```

Expected: PR metadata matches issue #173 where applicable, PR body has final `## DoD Status`, checks are passing or exact failures are recorded.

- [ ] **Step 5: Stop at merge gate**

Do not run `gh pr merge` until the user explicitly approves merge after reviewing the PR and evidence.

## Acceptance Mapping

| Spec acceptance | Plan coverage |
| --- | --- |
| `DistributedProvider` composes `*Provider` and adds context-aware distributed operations. | T1, T2, T10 |
| Redis repository supports current key, `kid` lookup, rotate, forced rotate, trim, expiry, delete-all. | T3-T6, T10 |
| Two providers verify each other's tokens across rotation by `kid`. | T1, T7 |
| Missing/unknown `kid`, stale eviction, errors, cancellation/deadline covered. | T1, T5, T7 |
| Constructor/bootstrap is context-aware. | T1, T2, T7 |
| Redis `Find(kid)` uses bounded by-`kid` lookup. | T5 command-capture tests, T6 command budget, T8, T10 |
| Namespace, key format, DTO validation, TTL retention, signing authority warning are documented and tested. | T3, T4, T6, T9 |
| Rollout/rollback and fixed/local migration limitation are documented. | T0, T9, T10 |
| Stress tests use `GoroutineStressTester`. | T7 |
| Redis cancellation uses `AsyncJobTester`. | T5, T7 |
| README pair documents Redis backend and MongoDB #198 deferral. | T9 |
| Benchmark results use chart asset when published. | T8, T10 |
| Public API does not expose raw-key repository constructors. | T4 and T10 |
| Step 3-R, Step 6-R, Step 7-R close only with `P0=0 P1=0`. | This review, T10, T11 |

## Ordering and Recheck Points

1. Commit spec, spec review, plan, and Step 3-R plan review before implementation.
2. Run T0 before code changes and refresh current code evidence.
3. Run T1 before T2 so public provider behavior is locked by failing tests.
4. Run T3 and T4 before Redis repository implementation so key names and DTO safety are structural.
5. Run T5 before T6 so read/delete paths are stable before Lua CAS rotation.
6. Run T7 only after Redis rotate/store is complete; Testcontainers-backed stress tests run serially.
7. Run T8 after functional behavior is green; benchmark results cannot appear in docs or PR evidence without chart assets.
8. Run T9 after API names stabilize and before Step 6-R.
9. Run T10 after code, tests, docs, examples, benchmarks, and verifier evidence are present.
10. Run T11 only after Step 6-R has `P0=0 P1=0`.

## Risk Controls

| Risk | Control |
| --- | --- |
| Context-free distributed IO | `DistributedProvider` uses private named composition and exposes only `*Context` distributed methods. |
| Redis stores signing secrets | DTO remains internal, error strings are secret-safe, README documents trusted Redis, TLS, ACL, persistence, and eviction requirements. |
| Redis rotation race | Two-phase Lua CAS, stress tests, and race gate. |
| Parse hot path scans retained keys | Hash `HGET` by `kid` and command-count evidence. |
| Redis TTL evicts valid signing material | Default `KeyTTL=0`; positive TTL validated against retained key validity plus `RetentionLeeway` before store. |
| Cancellation is lost | `requireContext`, pre/post-create context checks, `%w` wrapping, `AsyncJobTester`, and `errors.Is` tests. |
| Migration invalidates tokens silently | README documents #173 limitation and explicit invalidation decision. |
| Benchmark evidence is unreadable | `$bluetape4k-diagram` and `$vega` chart asset gate. |
| Testcontainers flake or unsafe parallelism | Redis tests run serially; failures record exact command and environment. |

## Validation Commands

```bash
gofmt -w jwt jwt/redis
go test -p 1 -count=1 ./jwt ./jwt/redis
go test -race -p 1 -count=1 ./jwt ./jwt/redis
go test -p 1 -run '^$' -bench 'BenchmarkRedis(Repository|Distributed)' -benchtime=100ms -benchmem ./jwt
go test -p 1 -count=1 ./...
golangci-lint config verify
make ci
rg -n "GoroutineStressTester|AsyncJobTester" jwt docs/superpowers/reviews
rg -n "DistributedProvider|ComposeContext|ParseContext|MongoDB|#198|signing authority|DeleteKeyChainsContext|redis-cli --tls|ACL|PTTL" jwt/README.md jwt/README.ko.md
git diff --check
```

## Step 3 Self-Review

| Check | Result | Evidence |
| --- | --- | --- |
| Spec coverage | PASS | Acceptance mapping covers every spec acceptance criterion. |
| Placeholder scan | PASS | Plan avoids unresolved placeholder phrases and assigns exact paths, commands, and test names. |
| Type consistency | PASS | `DistributedProvider`, `DistributedKeyChainRepository`, `Options`, `Repository`, and method names match the spec. |
| Step 2-R P2 carry-forward | PASS | T6 pins command-count and benchmark budget; T8 pins chart gate; T9 pins Redis runbook and recovery commands. |
| Code-bearing task pattern coverage | PASS | T1-T11 require `$bluetape-go-patterns`; T8 also requires `$bluetape4k-diagram` and `$vega`. |

## Step 3 Checklist Completion Report

| Item | Status | Notes |
| --- | --- | --- |
| Plan path confirmed inside feature worktree | Done | `docs/superpowers/plans/2026-06-12-issue-173-distributed-jwt-keychain-repositories-plan.md`. |
| All tasks have complexity labels | Done | T0-T11 include complexity labels. |
| `$bluetape-go-patterns` is explicitly applied to every code-bearing task | Done | T1-T11 list the support skill. |
| Plan code/test snippets conform to `$bluetape-go-patterns` | Done | Snippets use context, typed errors, repo-local stress helpers, `%w`-compatible contracts, and narrow APIs. |
| Thread/cancellation tests use repo helpers | Done | T5/T7 use `AsyncJobTester`; T7 uses `GoroutineStressTester`. |
| Tests and verification tasks included | Done | T1-T8, T10, and validation commands. |
| Multilingual README and contributor-doc tasks included when scope requires | Done | T9 covers `jwt/README.md` and `jwt/README.ko.md`; T11 covers PR body. |
| README diagram/chart asset tasks included when benchmark results are shown | Done | T8 requires `$bluetape4k-diagram`, `$vega`, SVG, and PNG. |
| Risky ordering/dependency assumptions are explicit | Done | Ordering and risk controls sections included. |
| Spec + plan committed before implementation | Done | Spec already committed in the Step 2 closure; this plan and Step 3-R review are committed before implementation starts. |
