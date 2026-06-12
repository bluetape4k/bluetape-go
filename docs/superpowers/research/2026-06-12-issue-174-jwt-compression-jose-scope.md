# Issue #174 JWT Compression and JOSE Scope Research

Issue: #174
Milestone: 0.6.1
Date: 2026-06-12

## Research Question

`bluetape-go` already has a small signed JWT helper package built on
`github.com/golang-jwt/jwt/v5`. The open question is whether source parity with
the Kotlin JWT stack should add JWT compression now, and which JOSE dependency
boundary is safe if compression is ever needed.

## Decision

Do not add compression to the current signed JWT/JWS helper.

Keep `github.com/golang-jwt/jwt/v5` as the only runtime dependency for the
current `jwt` package. Continue rejecting inbound and outbound signed JWTs that
try to use unsupported JOSE/compression headers such as `zip`, `crit`, `jku`,
`jwk`, `x5u`, and `x5c`.

If interoperable compression is required later, implement it as a separate,
explicit JWE boundary. The preferred future dependency for that boundary is
`github.com/go-jose/go-jose/v4` pinned at `v4.1.4` or newer.

No implementation follow-up is opened from #174 because the research rejects
compression for signed JWTs. A future JWE issue should be created only when a
real product use case requires encrypted JWTs or compressed JWE payloads.

## Primary Evidence

| Source | Evidence | Decision impact |
|---|---|---|
| RFC 7515, JSON Web Signature | JWS protects a payload with a digital signature or MAC. Its registered header set covers signature and key selection headers such as `alg`, `jku`, `jwk`, `kid`, `x5u`, `x5c`, `typ`, `cty`, and `crit`; `zip` is not the JWS compression mechanism. | Do not treat `zip` as a standard signed JWT/JWS option. |
| RFC 7516, JSON Web Encryption | `zip` is the compression algorithm applied to plaintext before encryption, and `DEF` is the value defined by the specification. The header must be integrity-protected in the JWE Protected Header. | Compression belongs to a JWE boundary, not to the current signed JWT helper. |
| RFC 7518, JSON Web Algorithms | The JOSE compression registry is for JWE `zip` member values. PBES2 JWE key management uses `p2c` and `p2s`, which introduces denial-of-service and policy-limit considerations. | A future JWE feature needs explicit algorithm and resource-limit policy. |
| `github.com/golang-jwt/jwt/v5` | The library supports JWT parsing, verification, generation, signing, and signing-method extensibility, with stable v5 API expectations. Current repo dependency is `v5.3.1`. | Best fit for the current narrow signed JWT helper. |
| `github.com/go-jose/go-jose/v4` | The library implements JWE, JWS, and JWT standards, supports compact and JSON serializations, and documents `DEF` compression support. Latest release checked: `v4.1.4`, 2026-04-04. | Best future optional JOSE/JWE boundary if JWE becomes necessary. |
| `github.com/lestrrat-go/jwx/v4` | Broad JOSE suite with active maintenance, latest release `v4.0.2` on 2026-05-07, but it currently requires Go 1.26 plus `GOEXPERIMENT=jsonv2`. | Too broad and operationally heavier for the default helper dependency. |
| Current `jwt` package | `jwt/composer.go` reserves `zip`, `crit`, `jku`, `jwk`, `x5u`, and `x5c`; `jwt/reader.go` rejects inbound tokens containing those unsupported headers. | Existing fail-closed behavior is correct and should be documented as the #174 result. |

## Candidate Comparison

| Candidate | Current version checked | Scope | License | Fit |
|---|---:|---|---|---|
| `github.com/golang-jwt/jwt/v5` | `v5.3.1` | Signed JWT parsing, validation, generation, and signing | MIT | Adopt for current signed JWT helper. |
| `github.com/go-jose/go-jose/v4` | `v4.1.4` | JOSE implementation covering JWE, JWS, JWT, compact/JSON serializations, and `DEF` JWE compression | Apache-2.0 | Prefer for a future optional JWE boundary. |
| `github.com/lestrrat-go/jwx/v4` | `v4.0.2` | Full JOSE suite with JWT/JWS/JWE/JWK and richer policy surface | MIT | Reject for current default dependency; reconsider only if the project intentionally adopts a broad JOSE stack. |
| First-party adapter around `compression` | N/A | Custom signed-JWT payload compression | Project license | Reject for interoperability and security; it would create a non-standard signed JWT shape. |

## Security Boundary

- Signed JWT parsing must keep `WithValidMethods` and exact provider algorithm
  matching.
- `zip` on signed JWT/JWS remains unsupported and rejected.
- `crit` remains rejected unless a future feature explicitly implements every
  critical extension listed in the token.
- Remote key headers `jku`, `jwk`, `x5u`, and `x5c` remain rejected by default;
  a future trust model would need pinned issuers, TLS/server identity rules,
  key-source allowlists, and cache invalidation policy.
- JWE support, if added later, must be a distinct API path rather than an
  auto-detected extension inside `Provider.Parse`.

## Future JWE Acceptance Shape

A future issue should require all of the following before adopting
`go-jose/go-jose/v4`:

- Separate package or constructor boundary for JWE; do not change existing
  signed JWT behavior.
- Pin `github.com/go-jose/go-jose/v4` at `v4.1.4` or newer.
- Allowlist JWE `alg` and `enc` combinations.
- Accept `zip=DEF` only in the JWE Protected Header.
- Enforce maximum compact token size, maximum decompressed size, and maximum
  expansion ratio.
- Reject malformed compact tokens with excessive segment counts.
- Reject unsupported `crit` entries.
- Keep remote key headers disabled unless an explicit pinned trust policy is
  designed and tested.
- Prefer service-to-service key management over PBES2. If PBES2 is ever
  enabled, enforce `p2c` limits and random `p2s`.

## Adopt / Borrow / Skip Decisions

| Decision | Rationale |
|---|---|
| Adopt `golang-jwt/jwt/v5` for current `jwt` package | Narrow, stable, already implemented, and sufficient for signed JWT creation and validation. |
| Borrow `go-jose/go-jose/v4` only for future JWE | It matches the standards boundary where compression belongs without forcing all signed-JWT users into a JOSE-wide dependency. |
| Skip `jwx/v4` for default dependency | Active and capable, but too broad for this package and currently requires `GOEXPERIMENT=jsonv2`. |
| Skip first-party signed-JWT compression | Non-standard shape, weak interop, and easy to confuse with JWE `zip`. |

## Source Links

- RFC 7515 - JSON Web Signature:
  https://datatracker.ietf.org/doc/html/rfc7515
- RFC 7516 - JSON Web Encryption:
  https://datatracker.ietf.org/doc/html/rfc7516
- RFC 7518 - JSON Web Algorithms:
  https://datatracker.ietf.org/doc/html/rfc7518
- `golang-jwt/jwt`:
  https://github.com/golang-jwt/jwt
- `go-jose/go-jose/v4`:
  https://pkg.go.dev/github.com/go-jose/go-jose/v4
- `lestrrat-go/jwx`:
  https://github.com/lestrrat-go/jwx
- go-jose parsing DoS advisory:
  https://github.com/go-jose/go-jose/security/advisories/GHSA-c6gw-w398-hv78
- jwx PBES2 `p2c` DoS advisory:
  https://github.com/lestrrat-go/jwx/security/advisories/GHSA-7f9x-gw85-8grf

## Checklist Completion Report

| Item | Status | Notes |
|---|---|---|
| `golang-jwt/jwt/v5` evaluated | Done | Keep for signed JWT only. |
| `go-jose/go-jose/v4` evaluated | Done | Preferred future JWE dependency, not adopted now. |
| `lestrrat-go/jwx/v4` evaluated | Done | Rejected for default helper dependency because of breadth and `GOEXPERIMENT=jsonv2`. |
| Compression decision recorded | Done | Signed JWT compression is a non-goal; JWE is future-only. |
| Security risks recorded | Done | Header confusion, remote key headers, decompression limits, compact parsing DoS, and PBES2 limits. |
| Follow-up decision | Done | No implementation issue opened because #174 rejects signed JWT compression. |
