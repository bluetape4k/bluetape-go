# JWT Compression and JOSE Scope

Issue #174 resolved the JWT compression question as a standards boundary, not a
compression-library choice.

## Lessons

- Do not add `zip` support to signed JWT/JWS helpers. RFC `zip=DEF`
  compression belongs to JWE plaintext-before-encryption.
- Keep `golang-jwt/jwt/v5` for narrow signed JWT parsing, validation, and
  signing. Avoid pulling a full JOSE dependency into the default `jwt` helper
  without a real JWE use case.
- If compressed JWT support becomes necessary, create a separate JWE API
  boundary and prefer `go-jose/go-jose/v4` pinned to a patched release.
- Treat JWE as a new security surface: decompression size limits, expansion
  ratio limits, compact-token segment validation, `crit` handling, remote key
  header policy, and PBES2 `p2c` bounds must be part of acceptance before code.
- `jwx/v4` remains a strong JOSE library, but its `GOEXPERIMENT=jsonv2`
  requirement and broad API surface make it a poor default dependency for a
  small helper package.

## Evidence

- `docs/superpowers/research/2026-06-12-issue-174-jwt-compression-jose-scope.md`
- `docs/superpowers/reviews/2026-06-12-issue-174-jwt-compression-jose-research-review.md`
- `jwt/README.md`
- `jwt/README.ko.md`
