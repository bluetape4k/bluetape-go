# Issue #33 JWT Pre-Implementation Risk Note

Task: T0 dependency and API risk note
Issue: #33
Date: 2026-06-08

## Dependency Decision

Adopt `github.com/golang-jwt/jwt/v5 v5.3.1` as a direct dependency for the #33
core JWT helper package.

Local evidence:

- `go list -m -json github.com/golang-jwt/jwt/v5` reports version `v5.3.1`,
  module time `2026-01-28T19:58:13Z`, and Go version `1.21`.
- `go doc` confirms:
  - `ParseWithClaims(tokenString string, claims Claims, keyFunc Keyfunc,
    options ...ParserOption) (*Token, error)`.
  - `NewWithClaims(method SigningMethod, claims Claims, opts ...TokenOption)`.
  - `RegisteredClaims` includes issuer, subject, audience, expiration,
    not-before, issued-at, and JWT ID fields.
  - `WithValidMethods` validates allowed algorithms and is explicitly
    recommended to prevent JWT algorithm-confusion attacks.
  - `WithLeeway`, `WithIssuer`, `WithAudience`, `WithSubject`,
    `WithExpirationRequired`, and `WithTimeFunc` cover #33 parse-validation
    needs.

## Rejected Alternatives

| Candidate | Decision |
| --- | --- |
| `lestrrat-go/jwx` | Defer to #174. It is a broader JOSE/JWS/JWE/JWK stack and is not needed for the #33 core signing/parsing helper. |
| `go-jose/go-jose/v4` | Defer to #174. It is a lower-level JOSE implementation and should be evaluated only if compression/JWE/JWK scope becomes concrete. |

## API Boundary

The public `jwt` package must not expose `github.com/golang-jwt/jwt/v5` parser,
token, or claims concrete types as stable bluetape-go API. The implementation
may use them internally.

Public API should use:

- repo-owned `Algorithm` constants;
- repo-owned `Reader`;
- narrow `Signer`, `Parser`, and `Rotator` interfaces;
- concrete provider constructors;
- package sentinel errors compatible with `errors.Is`.

## Security Controls Required Before PR

- Parse uses `WithValidMethods`.
- Key lookup compares token `alg` against the selected KeyChain algorithm before
  returning verification key material.
- Inbound signed tokens carrying `zip`, `crit`, `jku`, `jwk`, `x5u`, or `x5c`
  are rejected until #174 decides a safe JOSE/compression path.
- Fixed HMAC secrets are rejected unless at least the selected algorithm hash
  size: HS256 32 bytes, HS384 48 bytes, HS512 64 bytes.
- Errors never include raw bearer tokens, private keys, or symmetric secrets.
- The in-memory repository stays private and context-free in #33; #173 owns the
  future context-aware distributed repository contract.

## Verification Commands

```bash
go get github.com/golang-jwt/jwt/v5@v5.3.1
go list -m -json github.com/golang-jwt/jwt/v5
go doc github.com/golang-jwt/jwt/v5.ParseWithClaims
go doc github.com/golang-jwt/jwt/v5.NewWithClaims
go doc github.com/golang-jwt/jwt/v5.RegisteredClaims
go doc github.com/golang-jwt/jwt/v5.WithValidMethods
go doc github.com/golang-jwt/jwt/v5.WithLeeway
go doc github.com/golang-jwt/jwt/v5.WithIssuer
go doc github.com/golang-jwt/jwt/v5.WithAudience
go doc github.com/golang-jwt/jwt/v5.WithSubject
go doc github.com/golang-jwt/jwt/v5.WithExpirationRequired
go doc github.com/golang-jwt/jwt/v5.WithTimeFunc
```
