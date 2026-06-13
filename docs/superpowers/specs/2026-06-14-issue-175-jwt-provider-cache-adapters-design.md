# Issue #175 JWT Provider Cache Adapters Spec

Issue: #175
Title: Add optional JWT provider cache adapters
Date: 2026-06-14
Milestone: 0.6.1
Work type: Type A full feature
Target package: `jwt`

## Goal

`bluetape-go` needs optional JWT parse-result cache adapters that reduce
repeated parsing and signature verification cost for reusable tokens without
turning the cache into a trust boundary. The adapter must wrap the existing
`Provider` and `DistributedProvider` APIs, reuse the generic `cache.Cache`
contract, and keep `Provider`/`DistributedProvider` as the source of truth for
algorithm checks, `kid` lookup, key validity, signature verification, and claim
validation.

## Step 1-R Research Summary

Current repository evidence:

- `jwt.Provider` implements `Signer`, `Parser`, and `Rotator` with
  context-free `Compose`, `Parse`, `TryParse`, `CurrentKeyChain`, `Rotate`,
  `ForcedRotate`, and `FindKeyChain`.
- `jwt.DistributedProvider` deliberately exposes only context-aware methods:
  `ComposeContext`, `ParseContext`, `CurrentKeyChainContext`,
  `RotateContext`, `ForcedRotateContext`, `FindKeyChainContext`, and
  `DeleteKeyChainsContext`.
- `Reader` already copies headers and claims and exposes `RemainingTTL(now)`,
  which can clip cache entries to token expiration.
- `KeyChain` exposes `ExpiresAt()` and `Expired(now)`, so cache entries can be
  clipped to signing-key validity in addition to claim expiry.
- `cache.Cache[K,V]` provides context-aware `Get`, `Set`, `Delete`, and
  `Clear`. `cache.LoadingCache` exists, but its `GetOrLoad(ctx,key,ttl,loader)`
  requires a fixed TTL before loading, so it cannot directly express
  token/key-derived dynamic TTL.
- Existing tests already provide `fakeDistributedRepository`, in-memory cache
  TTL behavior, `GoroutineStressTester`, and `AsyncJobTester` patterns.

Diagram source model:

![JWT provider cache adapter flow](../../images/readme-diagrams/jwt-provider-cache-adapter-flow.png)

Diagram evidence:

- Selected catalog baseline: `architecture-layered-exposed-mvc-virtualthread`
  for responsibility bands and `sequence-workflow-sample` for clear call order.
- Rejected patterns checked: no relationship-heavy grid, no card text
  overflow, no subtle decorator, no connector/card intersections.
- Generator: `scripts/generate-jwt-provider-cache-adapter-diagram.mjs`.
- Gate result: `nodes=11 routes=9 segments=18 badEndpointAngle=0 badBends=0
  interiorCrossings=0 nodeOverlaps=0 laneClearance=0 marginImbalance=0
  margins=L48/R48/T48/B48 titleGap=58`.

## Non-Goals

- No new cache package abstraction.
- No Redis-specific JWT cache implementation in this PR.
- No token revocation store, auth middleware, sessions, OIDC, JWE, JWK, or
  JWKS behavior.
- No raw bearer token storage as a cache key or error string.
- No untrusted or cross-service external cache backend for `*Reader` values.
  This first slice targets trusted application-process cache backends. Shared
  or external cache entries require a future authenticated/tamper-evident entry
  format.
- No caching of malformed, expired, not-yet-valid, wrong-algorithm,
  wrong-key, missing-key, or rejected-claim parse failures.
- No cache bypass for provider validation rules.

## Brainstorming Outcomes

### Approach 1 - Safe Decorator With Dynamic TTL and Hit Revalidation

Add two optional decorators in `jwt`:

- `CachedProvider` wraps `*Provider`.
- `CachedDistributedProvider` wraps `*DistributedProvider`.

Both use `cache.Cache[string, *Reader]`, compute cache keys from a SHA-256
token digest plus a normalized parse profile, and store only successful
provider-validated readers. On cache hit, the adapter rechecks key
presence/validity through the wrapped provider before returning the cached
reader. On cache miss, it delegates to the wrapped provider, finds the
validated `KeyChain`, clips TTL to token expiration, key expiration, and
`MaxTTL`, then stores the positive result only when the computed TTL is
positive.

Pros:

- Preserves provider-owned signature, algorithm, `kid`, and claim validation.
- Handles dynamic token/key expiry that `LoadingCache.GetOrLoad` cannot know
  before parsing.
- Keeps distributed key eviction/rotation safe because hits revalidate the
  key through `FindKeyChainContext` on the `CachedDistributedProvider`
  adapter.
- Reuses the existing `cache.Cache` contract.

Cons:

- `CachedDistributedProvider` cache hits still perform a key lookup. This
  intentionally trades maximum throughput for correctness at the signing-key
  boundary.
- Manual miss loading is needed instead of the existing `GetOrLoad` helper.
  The adapter must replace the lost same-key singleflight behavior with
  per-adapter miss coalescing because the dynamic TTL is known only after the
  wrapped provider validates the token.

Decision: accepted.

### Approach 2 - LoadingCache-Only Decorator

Use `cache.LoadingCache.GetOrLoad` and treat the configured TTL as the cache
entry lifetime.

Pros:

- Simpler code and built-in singleflight behavior for `cache.Memory`.
- Familiar to callers already using `cache.LoadingCache`.

Cons:

- TTL must be known before provider parsing, but token expiration and key
  expiration are only available after validation.
- A fixed TTL can outlive token or key validity.
- Cache hits cannot naturally revalidate distributed key presence.

Decision: rejected.

### Approach 3 - Cache Parsed Claims Without Provider Revalidation

Cache `Reader` by token digest after the first successful parse and return it
until TTL expiry.

Pros:

- Fastest cache-hit path.

Cons:

- A cached entry can survive external distributed key eviction, admin reset,
  or forced rotation unless every caller uses only the same adapter instance.
- Makes the cache behave like a trust boundary.

Decision: rejected.

## Public API

New public types in package `jwt`:

```go
type CacheOption func(*cacheConfig) error

type CachedProvider struct { /* unexported fields */ }
type CachedDistributedProvider struct { /* unexported fields */ }

func NewCachedProvider(provider *Provider, c cache.Cache[string, *Reader], options ...CacheOption) (*CachedProvider, error)
func NewCachedDistributedProvider(provider *DistributedProvider, c cache.Cache[string, *Reader], options ...CacheOption) (*CachedDistributedProvider, error)
```

The cache value is `*Reader`; entry lifetime is enforced by the cache TTL
passed to `Set`. The adapter must not require callers to use an unexported
entry type.

Cache options:

```go
func WithCacheMaxTTL(ttl time.Duration) CacheOption
func WithCacheKeyPrefix(prefix string) CacheOption
func WithCacheTrustScope(scope string) CacheOption
func WithCacheClock(now func() time.Time) CacheOption
```

Default behavior:

- `MaxTTL`: `5 * time.Minute`.
- `KeyPrefix`: package-owned non-secret prefix such as `jwt:provider-cache:v1`.
- `TrustScope`: per-adapter random scope generated during construction. Callers
  may provide an explicit stable scope only when the cache backend is private to
  one provider/tenant/key namespace and protected from untrusted writes.
- `Clock`: `time.Now`.

Constructor and option validation:

- Nil or typed-nil providers return `OptionError{Option: "provider"}`.
- Nil or typed-nil cache backends return `OptionError{Option: "cache"}`.
- Nil `CacheOption` returns `OptionError{Option: "cache_option"}`.
- `WithCacheMaxTTL` rejects zero or negative values with
  `OptionError{Option: "cache_max_ttl"}`.
- `WithCacheKeyPrefix` rejects empty strings and control characters with
  `OptionError{Option: "cache_key_prefix"}`.
- `WithCacheTrustScope` rejects empty strings and control characters with
  `OptionError{Option: "cache_trust_scope"}`.
- `WithCacheClock` rejects nil clocks with
  `OptionError{Option: "cache_clock"}`.
- Every exported type, function, and method gets a Go doc comment. Add
  compile-time assertions for intended interfaces, including
  `var _ Signer = (*CachedProvider)(nil)`, `var _ Parser =
  (*CachedProvider)(nil)`, and `var _ Rotator = (*CachedProvider)(nil)`.

`CachedProvider` methods:

```go
func (p *CachedProvider) Compose(options ...ComposeOption) (string, error)
func (p *CachedProvider) Parse(token string, options ...ParseOption) (*Reader, error)
func (p *CachedProvider) TryParse(token string, options ...ParseOption) (*Reader, bool)
func (p *CachedProvider) ParseContext(ctx context.Context, token string, options ...ParseOption) (*Reader, error)
func (p *CachedProvider) CurrentKeyChain() (*KeyChain, error)
func (p *CachedProvider) Rotate() (*KeyChain, error)
func (p *CachedProvider) ForcedRotate() (*KeyChain, error)
func (p *CachedProvider) FindKeyChain(kid string) (*KeyChain, error)
func (p *CachedProvider) ClearCache(ctx context.Context) error
```

`CachedProvider.ParseContext` context applies only to cache `Get`, `Set`,
`Delete`, `ClearCache`, and singleflight wait cancellation. The wrapped
`Provider.Parse` path remains the existing synchronous context-free validation
path. A nil, canceled, or expired context must prevent cache access and return
the context error before mutation. `CachedProvider.Parse` is a convenience
wrapper that uses `context.Background()` for cache operations. `TryParse` uses
`Parse` and returns `(nil, false)` on any parse or cache error.

`CachedDistributedProvider` methods:

```go
func (p *CachedDistributedProvider) ComposeContext(ctx context.Context, options ...ComposeOption) (string, error)
func (p *CachedDistributedProvider) ParseContext(ctx context.Context, token string, options ...ParseOption) (*Reader, error)
func (p *CachedDistributedProvider) CurrentKeyChainContext(ctx context.Context) (*KeyChain, error)
func (p *CachedDistributedProvider) RotateContext(ctx context.Context) (*KeyChain, error)
func (p *CachedDistributedProvider) ForcedRotateContext(ctx context.Context) (*KeyChain, error)
func (p *CachedDistributedProvider) FindKeyChainContext(ctx context.Context, kid string) (*KeyChain, error)
func (p *CachedDistributedProvider) DeleteKeyChainsContext(ctx context.Context) error
func (p *CachedDistributedProvider) ClearCache(ctx context.Context) error
```

Every `CachedDistributedProvider` context method rejects nil context, preserves
`context.Canceled` and `context.DeadlineExceeded`, and must not read or mutate
the cache or delegate to the wrapped provider after the context is already
done.

Rotation semantics:

- Adapter-owned `ForcedRotate`, `ForcedRotateContext`, and
  `DeleteKeyChainsContext` clear the cache after the wrapped operation
  succeeds. If the wrapped operation succeeds and `ClearCache` fails, return
  the clear error with `errors.Is` preserving the cache failure; do not silently
  ignore stale-cache cleanup failures.
- `Rotate`/`RotateContext` may keep cache entries when the current key remains
  valid, but cache hits still revalidate the key.
- If callers mutate the wrapped provider outside the adapter, cache hits still
  revalidate key validity and delete stale entries before reparsing.
- `ClearCache` scope is exactly the supplied trusted `cache.Cache` backend
  scope. With `cache.Memory`, clearing is process-local. This PR does not
  support untrusted shared/external cache backends for `*Reader` values.
  Multi-instance deployments using process-local caches rely on cache-hit key
  revalidation for safety, not global invalidation.

## Caller Migration

Local provider:

```go
provider, err := jwt.NewFixedHMACProvider(jwt.HS256, secret)
if err != nil {
    return err
}
readerCache := cache.NewMemory[string, *jwt.Reader]()
cached, err := jwt.NewCachedProvider(provider, readerCache)
if err != nil {
    return err
}
reader, err := cached.Parse(token, jwt.WithExpirationRequired())
```

Distributed provider:

```go
provider, err := jwt.NewDistributedHMACProvider(ctx, repo, jwt.HS256)
if err != nil {
    return err
}
readerCache := cache.NewMemory[string, *jwt.Reader]()
cached, err := jwt.NewCachedDistributedProvider(provider, readerCache)
if err != nil {
    return err
}
reader, err := cached.ParseContext(ctx, token, jwt.WithExpirationRequired())
```

`WithParseClock` bypasses caching in the first slice. Tests and custom-clock
callers must not expect cache hits when supplying a custom parse clock.

## Cache Key and Entry Rules

Cache key:

- Use SHA-256 of the raw token.
- Include normalized parse profile fields: leeway, expected issuer,
  expected audience in exact supplied slice order, expected subject,
  expiration-required flag, and whether a custom parse clock was supplied.
- Include the cache key prefix, trust scope, and provider algorithm. The default
  generated trust scope prevents accidental cache sharing across provider
  instances. Explicit trust scopes must not be reused across providers,
  tenants, algorithms, or key namespaces.
- Do not include the raw token in the key, logs, or errors.
- Treat token digests as stable token-correlation metadata. Do not log cache
  keys, token digests, or parse-profile hashes.
- Do not cache when `WithParseClock` is supplied unless the implementation can
  make the custom clock part of both key identity and TTL calculation without
  stale hits. The conservative first slice should bypass caching for custom
  parse clocks.
- Build keys deterministically with a low-allocation helper that appends
  normalized fields into a byte buffer before hashing the profile. Avoid
  generic `fmt.Sprintf` key construction on the hot path.

Cache entry:

- Store only successful parse results.
- Store `*Reader` only.
- Use the computed positive TTL in `cache.Set`.
- Rely on `cache.Cache` TTL semantics for claim/key expiry and delete entries
  when hit revalidation finds an invalid key.

Hit handling:

- Only `cache.ErrCacheMiss` falls through to provider parsing.
- Non-miss `Get` errors remain caller-visible with `errors.Is` preserving the
  cache cause; provider parsing must not run after such failures.
- Cached reader must be non-nil.
- Cached `Reader.Algorithm()` must match the wrapped provider algorithm.
- Cached `Reader.Kid()` must resolve to a live key through the wrapped
  provider's key lookup path.
- Revalidated `KeyChain.Algorithm()` must match the reader and provider
  algorithm.
- On nil reader, algorithm mismatch, missing/stale key, or key-algorithm
  mismatch, delete the cache entry and reparse through the provider. If that
  stale-entry `Delete` fails, return the delete error and never return the stale
  reader.
- `Set`, `Delete`, and `Clear` errors remain caller-visible unless a method
  explicitly documents a best-effort `Set` policy. The first slice uses
  caller-visible errors for all cache mutations.

TTL calculation:

```text
ttl = min(
  MaxTTL,
  Reader.RemainingTTL(now) when exp exists,
  KeyChain.ExpiresAt().Sub(now) when key expiry exists
)
```

When the computed TTL is zero or negative, return the parsed reader without
writing the cache.

Miss coalescing:

- Each adapter instance owns a `singleflight.Group` keyed by the final cache
  key.
- Concurrent same-token/same-parse-profile misses must run the wrapped provider
  parse at most once per adapter instance.
- The singleflight loader computes dynamic TTL after validation and calls
  `cache.Set` only when TTL is positive.
- Cross-process stampede protection is a non-goal for this PR; distributed
  caches may still see one miss per process.

Parse-profile helper:

- Add an internal `customClock bool` field to `parseConfig`; `WithParseClock`
  sets it to true.
- Add an internal cache-profile normalizer that applies `ParseOption`s without
  invoking or comparing function values.
- The normalizer returns `cacheable=false` when `customClock` is true.
- The normalizer serializes expected audience in exact supplied slice order.
- Tests prove `WithParseClock` bypasses cache entirely: no `Get`, no `Set`, no
  stale-entry `Delete`, and repeated calls delegate to the wrapped provider.

## Acceptance Criteria

- Adapter API composes with `Provider`, `DistributedProvider`, and
  `cache.Cache` without new dependencies.
- Cache hits return provider-validated readers and do not call signature parse
  again.
- Cache misses delegate to the wrapped provider exactly once for successful
  parses.
- Concurrent same-token cold misses are coalesced per adapter instance.
- TTL is clipped by token expiration, key expiration, and `MaxTTL`.
- Malformed token, wrong algorithm, wrong key, unknown `kid`, expired token,
  not-yet-valid token, and rejected claim options are not cached.
- Cache keys never include raw tokens.
- Cache keys include trust scope and provider algorithm and are never logged.
- Cache backend is documented as trusted application-process cache only for
  this first slice; untrusted/shared external cache is rejected as unsupported.
- Adapter-owned forced rotation and distributed delete/reset clear the cache.
- `CachedDistributedProvider` cache hits revalidate key presence/validity
  before returning.
- Cache hit revalidation checks nil reader, reader algorithm, key lookup,
  key liveness, and key algorithm before returning.
- Non-miss cache operation errors remain caller-visible and are tested.
- Nil context is rejected for every adapter method that accepts a
  `context.Context`; canceled/deadline contexts are preserved, and no cache
  mutation or provider delegation occurs after an already-done context.
- Custom `WithParseClock` bypass is enforced by `parseConfig.customClock` and
  tested with no cache calls plus repeated provider delegation.
- Concurrent parse/cache access is covered with `GoroutineStressTester`.
- Cancellation-aware cache operation behavior is covered with
  `AsyncJobTester`.
- Race/stress validation includes `go test -race -count=1 ./jwt ./cache
  ./testing/concurrency` plus stress scenarios for cache hits/misses concurrent
  with forced rotation, distributed delete/reset, stale-key invalidation, cache
  operation errors, and failure non-caching.
- Benchmarks cover uncached parse, cached cold miss, cached warm hit, HMAC,
  RSA, distributed warm hit with key revalidation, miss+`Set`, hot
  `b.RunParallel`, same-key cold burst, and focused cache-key construction.
  Record `-benchmem` output and parse/key lookup counts where practical.
- `README.md` and `README.ko.md` describe cache adapters as optional
  performance helpers, not an auth framework or trust bypass.
- `jwt/README.md` and `jwt/README.ko.md` update the selection guide, replace
  the previous "Deferred" cache-adapter row, add local cached-provider and
  distributed cached-provider examples, document local vs shared cache clear
  scope, and state that `WithParseClock` bypasses caching.
- README text states cache adapters do not provide revocation, authorization,
  middleware, session management, or protection against using an untrusted or
  shared writable cache backend.
- `cache/README.md` and `cache/README.ko.md` are not changed for this feature;
  JWT README files own adapter documentation because `cache.Cache` remains a
  generic backing contract.
- Operator notes document monitoring guidance for parse errors by sentinel
  class, unknown `kid`, key revalidation failures, cache `Get`/`Set`/`Delete`
  and `Clear` errors, timeout/cancellation, stale-entry deletion, and cache
  clear failures. Raw tokens must remain absent from logs and errors.
- Diagram source, SVG, PNG, DOT, plain, Graphviz SVG, and Graphviz PNG are
  generated and committed.

## Risks and Controls

| Risk | Control |
|---|---|
| Cached result outlives token expiry | Clip TTL by `Reader.RemainingTTL(now)` and entry `expiresAt`. |
| Cached result outlives key validity | Clip TTL by `KeyChain.ExpiresAt()` and revalidate hits with provider key lookup. |
| Parse options produce unsafe shared hits | Include normalized parse profile in the cache key; bypass custom parse clocks in first slice. |
| Raw token leak through cache keys | Use SHA-256 digest and redacted errors only. |
| Distributed external reset invalidates keys | Hit revalidation deletes stale entries before reparsing. |
| Performance regression from revalidation | Document that correctness wins; benchmarks show avoided parse/signature cost separately from key lookup cost. |
| Cold miss stampede | Use per-adapter same-key `singleflight` and stress-test one wrapped parse per adapter instance. |
| Cache-key construction dominates hit path | Use low-allocation deterministic key building and benchmark representative token/profile sizes. |
| Cross-provider cache confusion | Include trust scope and algorithm in keys; default scope is per-adapter and non-shared. |
| Cache poisoning | Limit first slice to trusted application-process cache backends for `*Reader`; reject untrusted/shared external cache as unsupported. |

## Step 2 DoD

| Item | Status |
|---|---|
| Issue #175 scope captured | Done |
| Current `jwt` and `cache` APIs researched | Done |
| Brainstorming alternatives recorded | Done |
| Accepted approach chosen with rationale | Done |
| Diagram included and generated through `$bluetape4k-diagram` rules | Done |
| Acceptance criteria mapped to tests/docs | Done |
