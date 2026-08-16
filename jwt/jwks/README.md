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
    "github.com/bluetape4k/bluetape-go/jwt/jwks"
)
```

## Quick start

`New` is network-free. Use an explicit bounded `Refresh` during readiness, then
create a request-scoped `KeyFunc` and keep claims policy in the JWT parser:

```go
provider, err := jwks.New(
    "https://issuer.example.com/.well-known/jwks.json",
    jwks.WithAllowedAlgorithms(jwks.RS256, jwks.PS256),
)
if err != nil {
    return err
}

refreshCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
if err := provider.Refresh(refreshCtx); err != nil {
    return err
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
token, err := parser.ParseWithClaims(signedToken, claims, keyFunc)
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
  non-global dial targets. Redirects are not followed.
- `WithHTTPClient` transfers TLS verification, proxy, DNS/dial, redirect, and
  allowlist policy to the caller. Do not use `InsecureSkipVerify` unless that
  trust decision is intentional and documented by the caller.
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

Recommended runbook: record the first refresh failure as a warning; page after
three consecutive failures or five minutes, whichever comes first. Verify
endpoint health and allowlists, perform a bounded `Refresh`, validate a known
`kid` signature, and only then resume traffic. For rollback, restore the prior
endpoint configuration, construct a new provider, run readiness `Refresh`, and
verify a known token. During mixed-version rotation, retain overlap keys until
every consumer has refreshed before retiring the old key.

## Non-goals

This package does not implement OIDC discovery, JWE decryption, token claims
policy, endpoint failover, logging/metrics, retries/backoff, or background
refresh. Those policies belong to the service that owns the provider.

