# jwt/jwks

`jwt/jwks` is an optional, caller-owned JWKS provider for signature
verification with `github.com/golang-jwt/jwt/v5`.

It accepts RSA, ECDSA, and Ed25519 public keys. Symmetric `oct` keys, JWE,
OIDC discovery, package-global caches, and background refresh are deliberately
out of scope.

## Import

```go
import (
    "context"
    "net/http"
    "time"

    "github.com/bluetape4k/bluetape-go/jwt/jwks"
    jwt "github.com/golang-jwt/jwt/v5"
)
```

## Quick start

`New` is network-free. Create one provider during startup, run an explicit
bounded `Refresh` during readiness, and inject that provider into each request.
Create a request-scoped `KeyFunc` and keep claims policy in the JWT parser:

```go
func newJWKSProvider() (*jwks.Provider, error) {
    provider, err := jwks.New(
        "https://issuer.example.com/.well-known/jwks.json",
        jwks.WithAllowedAlgorithms(jwks.RS256, jwks.PS256),
    )
    if err != nil {
        return nil, err
    }

    refreshCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := provider.Refresh(refreshCtx); err != nil {
        return nil, err
    }
    return provider, nil
}

func verifyJWKS(req *http.Request, signedToken string, provider *jwks.Provider) error {
    if provider == nil {
        return jwks.ErrInvalidOptions
    }

    requestCtx, cancel := context.WithTimeout(req.Context(), time.Second)
    defer cancel()
    keyFunc, err := provider.KeyFunc(requestCtx)
    if err != nil {
        return err
    }
    parser := jwt.NewParser(
        jwt.WithValidMethods([]string{"RS256", "PS256"}),
        jwt.WithIssuer("issuer"),
        jwt.WithAudience("api"),
        jwt.WithExpirationRequired(),
    )
    claims := &jwt.RegisteredClaims{}
    _, err = parser.ParseWithClaims(signedToken, claims, keyFunc)
    return err
}
```

The provider performs key lookup and signature-algorithm matching only. The
parser must explicitly enforce issuer, audience, subject, `exp`, `nbf`, and
other claims. Omitting those parser options means that only the signature
boundary is configured.

Build a new `KeyFunc` for each request context. Do not reuse a closure after
its context has been cancelled.

## Endpoint and trust boundary

- The default endpoint scheme is HTTPS. Loopback HTTP is allowed only for
  tests and local development.
- The endpoint must have a host and cannot contain userinfo or a fragment.
- The default client rejects private, link-local, unspecified, and other
  non-global dial targets, disables environment proxies, and caps response
  headers at 64 KiB. Redirects are not followed. HTTP endpoints must use a
  loopback IP literal.
- `WithHTTPClient` transfers TLS verification, proxy, DNS/dial, redirect, and
  allowlist policy to the caller. Do not use `InsecureSkipVerify` unless that
  trust decision is intentional and documented by the caller. A custom
  `RoundTripper` must honor `Request.Context()` cancellation for the request
  and response body lifetime; a non-cooperative transport can leave a canceled
  refresh operation running while a takeover proceeds.
- The endpoint is a direct JWKS JSON URL. OIDC discovery and automatic issuer
  metadata lookup are not performed.

## Cache and rotation

The default cache TTL is five minutes, fetch timeout is ten seconds, response
body limit is 1 MiB (hard cap 8 MiB), and unknown-`kid` refresh cooldown is one
second. A warm hit performs no I/O. TTL expiry and an unknown `kid` trigger a
bounded single-flight refresh; concurrent callers share one request. Explicit
`Refresh` bypasses the cooldown and never starts a background loop.

Only a successful refresh replaces the immutable snapshot. If an expired
snapshot cannot be refreshed, the provider fails closed rather than returning
stale key material. Returned RSA/ECDSA values and Ed25519 byte slices are
defensive copies.

## Key policy

- RSA keys must be public, at least 2048 bits, and have a representable odd
  exponent of at least 3.
- ECDSA keys must match P-256/ES256, P-384/ES384, or P-521/ES512.
- EdDSA means Ed25519 and requires a 32-byte public key.
- `use` may be empty or `sig`; `key_ops`, when present, must be exactly
  `verify`.
- Private material, `oct`, unknown key types, empty/duplicate/invalid `kid`,
  and unsupported algorithms are rejected. A `kid` is at most 128 printable
  ASCII bytes and a set is limited to 256 keys.
- `x5u`/`x5c` metadata does not cause an additional fetch. An embedded public
  key must still pass the JWK validation policy.

The default algorithm set is `RS256`, `RS384`, `RS512`, `PS256`, `PS384`,
`PS512`, `ES256`, `ES384`, `ES512`, and `EdDSA`. `WithAllowedAlgorithms` only
narrow this set; it cannot enable HMAC or another symmetric algorithm. When a
root `jwt.Algorithm` value is passed, convert it explicitly:

```go
import rootjwt "github.com/bluetape4k/bluetape-go/jwt"

jwks.WithAllowedAlgorithms(jwks.Algorithm(rootjwt.RS256))
```

## Errors and operations

| Class | `errors.Is` / `errors.As` | Retry guidance |
|---|---|---|
| Invalid option | `jwt.ErrInvalidOptions` | Fix configuration; do not retry unchanged input. |
| Malformed or unsafe set | `jwks.ErrMalformedSet`, `jwt.ErrInvalidKey`, `jwks.SetError` | Stop using that payload and page the endpoint owner. |
| Unsupported algorithm | `jwks.ErrUnsupportedAlgorithm` | Align the caller allowlist and issuer contract. Never broaden it to HMAC. |
| Fetch/status/body/context | `jwks.ErrFetch`, `jwks.FetchError`; context errors are preserved | Retry only with a bounded caller context and normal service policy. |
| Unknown `kid` | `jwt.ErrKeyNotFound` | Allow the bounded rotation refresh; investigate after the cooldown/page threshold. |

Event fields should be limited to `operation`, bounded `FetchClass`, outcome,
and bounded HTTP status. Do not log endpoint URLs, bearer tokens, raw bodies,
JWK material, raw transport causes, or high-cardinality `kid` values.

Recommended runbook:

| Phase | Owner | Action | Clear condition |
|---|---|---|---|
| Preflight | service/on-call owner | Check endpoint health, TLS, allowlist, and current `FetchClass`; keep the prior endpoint configuration ready. | A bounded `Refresh` can be attempted with a caller deadline. |
| Warning | service/on-call owner | Record the first refresh failure without endpoint URL, raw body, token, JWK, transport cause, or high-cardinality `kid`. | Failure counter resets after a successful refresh. |
| Page | service/on-call owner | Page after three consecutive failures or five minutes, whichever comes first. | A readiness `Refresh` succeeds and a known `kid` signature verifies. |
| Rollback | release owner | Restore the prior endpoint configuration, construct a new provider, run readiness `Refresh`, and verify a known token before resuming traffic. | Known-token verification passes on the restored provider. |
| Rotation | issuer/release owner | Retain overlap keys until every consumer has refreshed; then retire the old key and verify the next snapshot. | All consumer readiness checks pass and old-key lookups fail closed. |

During mixed-version rotation, retain overlap keys until every consumer has
refreshed before retiring the old key.

Readiness should use the same bounded commands before traffic opens:

```go
readinessCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
if err := provider.Refresh(readinessCtx); err != nil {
    return err
}
if _, err := provider.Lookup(readinessCtx, knownKid, jwks.RS256); err != nil {
    return err
}
```

## Non-goals

This package does not implement OIDC discovery, JWE decryption, token claims
policy, endpoint failover, logging/metrics, retries/backoff, or background
refresh. Those policies belong to the service that owns the provider.
