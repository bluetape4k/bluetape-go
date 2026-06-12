# jwt

[English](README.md) | [한국어](README.ko.md)

`jwt` provides Go-native helper APIs for signing, parsing, validating, and
rotating JSON Web Tokens with explicit algorithms and repo-owned errors.

## Import

```go
import "github.com/bluetape4k/bluetape-go/jwt"
```

## Selection Guide

| Need | Use | Notes |
|---|---|---|
| Fixed symmetric signing key | `NewFixedHMACProvider` | HS256 requires at least a 32-byte secret, HS384 48 bytes, and HS512 64 bytes. |
| Fixed asymmetric signing key | `NewFixedRSAProvider` | Supports RS256/384/512 and PS256/384/512 with validated 2048-bit-or-larger RSA private keys. |
| Local in-memory key rotation | `NewHMACProvider` or `NewRSAProvider` | Uses an in-memory KeyChain repository, `kid` headers, TTL, and retained keys. |
| Distributed key repository | Deferred | Context-aware Redis/Mongo/etc repositories are tracked in #173. |
| Signed JWT compression | Non-goal | `zip` belongs to a JWE boundary, not to the signed JWT helper. |
| JWE, JWK, JWKS | Deferred | JWE can be added later as an explicit optional JOSE boundary if a real use case appears. |
| External provider cache adapters | Deferred | Optional cache-backed provider adapters are tracked in #175. |

## Usage

```go
provider, err := jwt.NewFixedHMACProvider(
    jwt.HS256,
    []byte("0123456789abcdef0123456789abcdef"),
)
if err != nil {
    return err
}

token, err := provider.Compose(
    jwt.WithSubject("account-42"),
    jwt.WithAudience("api"),
    jwt.WithExpiresAfter(time.Hour),
    jwt.WithClaim("role", "admin"),
)
if err != nil {
    return err
}

reader, err := provider.Parse(
    token,
    jwt.WithExpectedSubject("account-42"),
    jwt.WithExpectedAudience("api"),
    jwt.WithExpirationRequired(),
)
if err != nil {
    return err
}
role, ok := reader.ClaimString("role")
```

## Behavior

- `jwt` is not an auth framework. It does not provide HTTP middleware,
  sessions, OIDC, JWKS, authorization rules, or user models.
- Parsing always constrains accepted algorithms with `WithValidMethods`, and
  the token `alg` header must match the provider's configured algorithm before
  verification key material is returned.
- Reader APIs expose verified headers and claims, but they do not expose the raw
  bearer token.
- Fixed providers allow a missing inbound `kid` only when exactly one fixed key
  is configured. Rotating providers require `kid` for lookup.
- In-memory rotation stores retained keys by `kid` and evicts older keys by
  repository capacity. A retained key can verify old tokens until the key
  expires or is evicted.
- HMAC fixed secrets must be at least the selected hash size. Weak secrets
  return `ErrInvalidKey`.
- RSA provider constructors require validated 2048-bit-or-larger private keys
  for signing. The provider stores an internal copy and uses public key material
  internally for verification, so later caller mutation of the original key does
  not change provider behavior.
- Errors wrap repo-owned sentinels for `errors.Is`, including
  `ErrInvalidToken`, `ErrExpiredToken`, `ErrNotYetValid`, `ErrInvalidKey`, and
  `ErrKeyNotFound`; error strings do not include raw tokens, HMAC secrets, or
  private keys.
- Inbound signed tokens containing unsupported JOSE/compression headers
  `zip`, `crit`, `jku`, `jwk`, `x5u`, or `x5c` are rejected. Issue #174
  concluded that signed JWT compression is a non-goal; standards-compatible
  compression belongs to a future explicit JWE API.
- The current repository is process-local only. Use #173 for future
  context-aware distributed key storage.

## Rotation Contract

`Rotate` returns the current non-expired key when one exists and creates a new
key only after expiration. `ForcedRotate` always creates a new key for rotating
providers. Fixed providers do not rotate.

Key generation uses caller-provided entropy when configured, otherwise
`crypto/rand`. Custom entropy readers, clocks, and key ID generators must be
safe for concurrent use when one provider is shared across goroutines.

## Test

```bash
go test -count=1 ./jwt
go test -race -count=1 ./jwt
```
