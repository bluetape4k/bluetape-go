# Issue #33 JWT Helper Utilities Spec

Issue: #33
Milestone: 0.6.0
Follow-ups: #173, #174, #175

## Context

0.6.0 needs a small Go-native JWT helper package that ports the useful service
surface from `bluetape4k-projects/utils/jwt` without becoming an auth framework
or hiding security-sensitive defaults.

The Kotlin parity target includes:

- composer/builder APIs for headers, claims, issuer, subject, audience,
  not-before, issued-at, and expiration helpers;
- reader APIs for typed header/claim access, `kid`, expiry checks, and remaining
  TTL;
- provider/factory APIs that compose signing and parsing with a KeyChain;
- KeyChain generation and rotation;
- repository-backed KeyChain sharing for distributed environments;
- optional provider caches;
- compression where the underlying JWT stack supports it safely.

The Go issue body now splits distributed repositories, compression research, and
cache adapters into linked follow-ups:

- #173 owns distributed JWT KeyChain repositories, starting with Redis and
  deciding MongoDB timing separately.
- #174 owns safe JWT compression and JOSE dependency scope.
- #175 owns optional JWT provider cache adapters after the core provider API is
  stable.

## Dependency Decision

Use `github.com/golang-jwt/jwt/v5` for #33.

Current dependency evidence gathered on 2026-06-08:

| Candidate | Current evidence | Fit for #33 | Decision |
| --- | --- | --- | --- |
| `github.com/golang-jwt/jwt/v5` | Latest release `v5.3.1` published 2026-01-28; repository pushed 2026-06-02; MIT; focused JWT signing/parsing; v5 docs include `RegisteredClaims`, `ParseWithClaims`, `Keyfunc`, `WithValidMethods`, `WithLeeway`, `WithIssuer`, `WithAudience`, and `WithExpirationRequired`. | Directly covers signing, parsing, validation, clock skew, registered claims, and parser hardening without pulling a broad JOSE stack. | Select for #33. |
| `github.com/lestrrat-go/jwx` | Latest GitHub release `v4.0.2` published 2026-05-07; active JOSE/JWT/JWS/JWE/JWK project; MIT. | Strong JOSE feature set but broader than #33 core helpers; better candidate if compression/JWE/JWK become required. | Defer to #174. |
| `github.com/go-jose/go-jose/v4` | Latest release `v4.1.4` published 2026-04-04; Apache-2.0; standards-oriented JOSE implementation. | Good lower-level JOSE library, but #33 does not need JWE/JWK breadth. | Defer to #174. |

Rationale:

- `golang-jwt/jwt/v5` provides the exact hardening hooks #33 needs:
  explicit valid methods, key lookup, registered-claim validation, and leeway.
- Its narrower API keeps #33 reviewable and avoids committing bluetape-go to a
  full JOSE abstraction before compression/JWE/JWK requirements are proven.
- Compression is not implemented in #33 because a safe interoperable `zip` path
  was not proven for the selected dependency; #174 must decide that separately.

## Goals

1. Add package `jwt`.
2. Provide explicit signing algorithm selection; no package-level default
   secret, private key, or hidden algorithm fallback.
3. Implement composer-style token creation for registered claims, custom
   headers, and custom claims.
4. Implement reader-style parse results with typed header/claim helpers, `kid`,
   expiry checks, and remaining TTL.
5. Implement provider APIs that sign and parse through an in-memory KeyChain
   repository.
6. Implement first-class KeyChain rotation and forced rotation for in-memory
   repositories.
7. Require parser hardening: valid-method allow-list, `kid` lookup, and
   algorithm match between token header and selected KeyChain.
8. Add tests for wrong key, wrong algorithm, expired token, not-before, clock
   skew, missing/unknown `kid`, rotation, malformed tokens, and concurrent
   sign/parse behavior.
9. Update README/CHANGELOG/WIP so JWT core helpers are no longer described as
   purely planned once implemented.

## Non-Goals

- Do not add HTTP auth middleware, OIDC, JWKS endpoints, sessions, roles,
  permissions, or an auth framework.
- Do not implement Redis/Mongo/distributed KeyChain repositories in #33. #173
  owns that parity.
- Do not implement provider cache adapters in #33. #175 owns that parity.
- Do not implement JWT compression in #33. #174 owns dependency and safety
  research.
- Do not support `alg=none`.
- Do not log or expose token strings, private keys, or symmetric secrets in
  errors.
- Do not expose `github.com/golang-jwt/jwt/v5` concrete parser/token types as
  the stable bluetape-go API.

## Package Layout

Expected package shape:

```text
jwt/
  doc.go
  errors.go
  algorithm.go
  claims.go
  composer.go
  reader.go
  keychain.go
  repository.go
  provider.go
  jwt_test.go
  jwt_concurrency_test.go
  jwt_example_test.go
  README.md
  README.ko.md
```

Implementation may split files further if it keeps review clear, but #33 should
not add subpackages.

## API Direction

Names may be refined during implementation, but the public shape should stay
small and Go-native.

```go
package jwt

type Algorithm string

const (
    HS256 Algorithm = "HS256"
    HS384 Algorithm = "HS384"
    HS512 Algorithm = "HS512"
    RS256 Algorithm = "RS256"
    RS384 Algorithm = "RS384"
    RS512 Algorithm = "RS512"
    PS256 Algorithm = "PS256"
    PS384 Algorithm = "PS384"
    PS512 Algorithm = "PS512"
)

type Signer interface {
    Compose(options ...ComposeOption) (string, error)
}

type Parser interface {
    Parse(token string, options ...ParseOption) (*Reader, error)
    TryParse(token string, options ...ParseOption) (*Reader, bool)
}

type Rotator interface {
    CurrentKeyChain() (*KeyChain, error)
    Rotate() (*KeyChain, error)
    ForcedRotate() (*KeyChain, error)
    FindKeyChain(kid string) (*KeyChain, error)
}
```

The package may provide concrete provider types that implement multiple narrow
interfaces, but it should not force fixed-key signers/parsers to expose
repository-admin methods. Prefer returning concrete providers from constructors
and accepting the smallest useful interface in examples/tests.

Composer options should cover:

- `WithHeader(name string, value any)`;
- `WithClaim(name string, value any)`;
- `WithIssuer`, `WithSubject`, `WithAudience`;
- `WithIssuedAt`, `WithNotBefore`, `WithExpiresAt`;
- `WithExpiresAfter(duration time.Duration)`;
- `WithJWTID`.

Parse options should cover:

- `WithLeeway(duration time.Duration)`;
- `WithExpectedIssuer`, `WithExpectedAudience`, `WithExpectedSubject`;
- `WithExpirationRequired`;
- `WithClock(func() time.Time)`.

Reader shape:

```go
type Reader struct {
    // unexported fields
}

func (r *Reader) Kid() string
func (r *Reader) Algorithm() Algorithm
func (r *Reader) Header(name string) (any, bool)
func (r *Reader) Claim(name string) (any, bool)
func (r *Reader) ClaimString(name string) (string, bool)
func (r *Reader) ClaimTime(name string) (time.Time, bool)
func (r *Reader) Issuer() string
func (r *Reader) Subject() string
func (r *Reader) Audience() []string
func (r *Reader) ExpiresAt() (time.Time, bool)
func (r *Reader) NotBefore() (time.Time, bool)
func (r *Reader) IssuedAt() (time.Time, bool)
func (r *Reader) IsExpired(now time.Time) bool
func (r *Reader) RemainingTTL(now time.Time) time.Duration
```

The reader may expose a copy of claims/headers, but it must not let callers
mutate provider-owned state. It must not expose the original bearer token as a
stable API because token strings are sensitive and easy to log accidentally.

## Algorithm and Key Contract

#33 supports HMAC, RSA PKCS#1 v1.5, and RSA-PSS algorithms that
`golang-jwt/jwt/v5` supports directly:

- HMAC: HS256, HS384, HS512.
- RSA: RS256, RS384, RS512.
- RSA-PSS: PS256, PS384, PS512.

KeyChain rules:

- `KeyChain` includes `KID`, `Algorithm`, creation time, expiry time, signing
  key material, and verification key material.
- HMAC KeyChains store a symmetric secret internally and never expose it.
- RSA/PS KeyChains store a private key internally and expose only public-key
  verification behavior.
- `kid` is required for repository-backed provider parsing.
- `alg` from the token header must exactly match the located KeyChain
  algorithm before verification.
- Unsupported algorithms fail with an `errors.Is`-compatible invalid-options or
  invalid-token error.

Generation direction:

- Provide constructors for fixed HMAC and fixed RSA/PS providers so callers can
  bring existing keys.
- Fixed HMAC constructors must reject empty or short secrets. Minimum secret
  length is tied to the selected algorithm hash size: HS256 >= 32 bytes,
  HS384 >= 48 bytes, and HS512 >= 64 bytes.
- Provide generated RSA/PS KeyChains for in-memory rotation.
- Provide generated HMAC KeyChains only if implementation can produce enough
  random secret bytes for the requested hash strength with simple tests.
- Key generation must use `crypto/rand.Reader` by default and allow test-local
  entropy injection without package-global mutation.

## Repository and Rotation Contract

Repository contract:

```go
type keyChainRepository interface {
    Current() (*KeyChain, error)
    Find(kid string) (*KeyChain, error)
    Rotate(create func() (*KeyChain, error), now time.Time) (*KeyChain, error)
    ForcedRotate(create func() (*KeyChain, error), now time.Time) (*KeyChain, error)
    DeleteAll() error
}
```

This is an unexported #33 in-memory contract only. It must not become the
distributed repository API because Redis/Mongo implementations need
`context.Context` for caller deadlines and cancellation. #173 owns the exported
or pluggable context-aware distributed repository contract.

The exact private signature may change, but behavior must hold:

- In-memory repository is concurrency-safe.
- Capacity defaults to 10 and must reject values below 2 or above 1000 unless a
  reviewed reason changes the bounds.
- `Current` returns the newest non-expired key if one exists.
- `Rotate` returns the current non-expired key and creates a new key only when
  the current key is missing or expired.
- `ForcedRotate` always creates a new key.
- Rotation trims old keys after the capacity limit while preserving the newest
  keys.
- Parse must still verify tokens signed by a previous retained, non-expired key
  by `kid`.
- Expired retained keys are not usable for parse and fail with a typed
  key-not-found or invalid-key error compatible with `ErrInvalidToken`.
- Unknown, expired, or evicted `kid` fails with a typed key-not-found or
  invalid-key error.

Provider concurrency:

- A shared provider must be safe for concurrent `Compose`, `Parse`, `Rotate`,
  and `ForcedRotate`.
- Provider locking must not hold a write lock while doing expensive signature
  verification if a narrower read path is possible.
- No package-global parser, entropy, or clock mutation is allowed.

## Claim and Header Rules

Reserved headers:

- `alg` and `kid` are provider-owned and cannot be overridden by composer
  options.
- `zip` is rejected until #174 proves a safe interoperable compression path.
- JOSE key-discovery or critical-extension headers that #33 does not implement
  are rejected rather than ignored: `crit`, `jku`, `jwk`, `x5u`, and `x5c`.
  Rejection applies both when composing tokens and when parsing inbound signed
  tokens, so #33 never accepts unsupported `zip` or critical/key-discovery
  semantics as if they were ordinary inert headers.
- `typ` may default to `JWT`, but callers may set it only if tests prove it
  cannot change signing or parsing security.

Reserved claims:

- `iss`, `sub`, `aud`, `exp`, `nbf`, `iat`, and `jti` should use typed helper
  options.
- A generic custom-claim setter must reject attempts to overwrite reserved
  registered claims unless the plan proves a clean precedence rule.
- If `iat` is not supplied, the composer should set it from the provider clock.
- Expiration is not automatically required for every parse unless the caller or
  provider option requires it, but docs must recommend expiration.

## Error Contract

Package errors should be `errors.Is` compatible:

- `ErrInvalidOptions`
- `ErrInvalidToken`
- `ErrInvalidKey`
- `ErrKeyNotFound`
- `ErrExpiredToken`
- `ErrNotYetValid`

Implementation may wrap dependency errors, but public errors must not include
token strings, private keys, or symmetric secrets.

Expected behavior:

- Nil provider options return `ErrInvalidOptions`.
- Empty token, malformed token, bad Base64/JSON, unsupported `alg`, wrong
  `alg`, missing `kid`, unknown `kid`, wrong key, and invalid signature return
  `ErrInvalidToken` or a more specific compatible error.
- Expired token returns `ErrExpiredToken` compatible with `ErrInvalidToken`.
- Not-before violation returns `ErrNotYetValid` compatible with
  `ErrInvalidToken`.
- Repository lookup failure preserves enough cause for diagnostics without
  leaking key material.

## Documentation Contract

Update:

- `jwt/README.md` and `jwt/README.ko.md`;
- root `README.md` and `README.ko.md` package summary;
- `CHANGELOG.md`;
- `WIP.md`.

Docs must explain:

- #33 is a JWT helper package, not an auth framework.
- Signing algorithm and keys must be explicit.
- Parser uses valid-method allow-list and `kid` lookup.
- In-memory KeyChain rotation is implemented; distributed repository parity is
  #173.
- JWT compression is a non-goal until #174 decides a safe path.
- Provider caches are optional follow-up work in #175.
- Secrets/private keys must be supplied through process secret management and
  never logged.

## Test Requirements

Targeted tests under `jwt`:

- successful compose/parse for HMAC.
- successful compose/parse for RSA or RSA-PSS with generated KeyChain.
- fixed HMAC key validation rejects empty and under-sized secrets for HS256,
  HS384, and HS512 with `ErrInvalidKey`.
- custom headers and typed custom claims.
- issuer, subject, audience, expiration, issued-at, not-before, and JWT ID.
- deterministic injected clock assertions for implicit `iat`,
  `WithExpiresAfter`, `IsExpired`, and `RemainingTTL`.
- deterministic injected entropy assertions for generated HMAC secret length,
  generated RSA/PS key behavior where practical, entropy failure/short-read
  wrapping, and proof that test entropy does not mutate package-global state.
- expiration failure and leeway success.
- not-before failure and leeway success.
- wrong key failure.
- wrong algorithm failure with valid-method hardening.
- missing `kid` failure for repository-backed provider.
- unknown `kid` failure.
- malformed token failure.
- reserved header/claim rejection, including `alg`, `kid`, `zip`, `crit`, `jku`,
  `jwk`, `x5u`, `x5c`, and registered-claim overwrite attempts. Inbound parse
  tests must reject signed tokens carrying unsupported `zip`, `crit`, `jku`,
  `jwk`, `x5u`, or `x5c` headers.
- `TryParse` returns `(reader, true)` only when `Parse` would succeed and
  returns `(nil, false)` for expired, not-yet-valid, wrong algorithm, wrong key,
  missing/unknown `kid`, malformed, or unsupported-header tokens.
- fixed providers without repository lookup may accept a missing `kid` only
  when exactly one fixed key is configured; tests must cover no-`kid` accept and
  multi-key/rotating provider rejection boundaries.
- representative error text for malformed token, wrong key, unknown `kid`, and
  repository failure does not contain token strings, private keys, or symmetric
  secrets.
- rotation keeps old retained key usable and signs new tokens with the new
  `kid`.
- forced rotation always changes `kid`.
- capacity trimming evicts old keys and then old-token parse fails by `kid`.
- expired retained keys fail parse by `kid`.
- concurrent compose/parse/rotate stress using
  `testing/concurrency.NewGoroutineStressTester`. The stress must prove
  concurrent `ForcedRotate`, retained-key parse success during rotation,
  evicted/expired-key failure only after capacity/expiry rules apply, no
  write-lock held across signature verification where measurable by structure,
  and no mutation of returned reader/header/claim state.
- `AsyncJobTester` is required only if #33 adds context-aware provider or
  repository operations. If no context-aware API is added, the verifier must
  record: `AsyncJobTester N/A: #33 core JWT operations are local CPU/crypto work
  with no caller-observable cancellation boundary.` This verifier note is
  required because issue #33 asks for `AsyncJobTester` stress coverage where
  applicable.
- examples for fixed HMAC provider and rotating RSA/PS provider.

Validation commands:

```bash
git diff --check
go test -count=1 ./jwt
go test -race -count=1 ./jwt
go test -count=1 ./...
make ci
```

## Acceptance Criteria

- PR metadata matches issue #33: assignee `debop`, milestone `0.6.0`, labels
  `type: task`, `priority: p1`, `area: utilities`.
- Dependency decision uses `github.com/golang-jwt/jwt/v5` and records why
  `jwx`/`go-jose` are deferred to #174.
- Core API covers signing, parsing, validation, typed claims, typed headers,
  issuer, subject, audience, expiration, not-before, issued-at, clock skew,
  `kid`, KeyChain rotation, and in-memory repository behavior.
- Public APIs use narrow interfaces or concrete providers; the in-memory
  repository contract remains private so #173 can design a context-aware
  distributed repository API without a breaking change.
- Distributed repositories are explicitly deferred to #173 with source-parity
  rationale.
- Compression is explicitly deferred to #174 with source-parity rationale.
- Provider cache adapters are explicitly deferred to #175 with source-parity
  rationale.
- Tests cover deterministic clock/entropy behavior, wrong key, wrong algorithm,
  expired token, not-before, clock skew, missing/unknown `kid`, fixed HMAC key
  strength, inbound unsupported JOSE headers, `TryParse`, key expiry, rotation,
  malformed tokens, error non-leakage, and concurrency stress.
- Validation commands in this spec pass before PR.
- P0/P1 findings are zero after Step 6-R and Step 7-R 7-Tier reviews.

## Risks and Mitigations

| Risk | Mitigation |
| --- | --- |
| Algorithm confusion | Always parse with `WithValidMethods`; compare token `alg` to located KeyChain algorithm before returning the verification key. |
| Weak HMAC secrets | Fixed HMAC constructors reject secrets shorter than the selected hash size; generated HMAC secrets use at least the same length. |
| Missing `kid` silently using current key | Repository-backed parse rejects missing `kid`; fixed providers may parse without repository lookup only when configured with exactly one fixed key. |
| Token or key material leaks in errors | Typed errors exclude token string and key material; tests check representative error text. |
| Rotation races | In-memory repository uses explicit locking and race tests. |
| Expired keys evict active tokens too early | Docs and tests make capacity and key TTL behavior explicit; old retained keys verify until evicted or expired. |
| Broad JOSE dependency churn | Keep #33 on narrow JWT dependency, reject unsupported inbound JOSE control headers, and defer compression/JWE/JWK to #174. |
