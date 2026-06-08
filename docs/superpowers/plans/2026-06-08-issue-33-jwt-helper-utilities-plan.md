# Issue #33 JWT Helper Utilities Plan

Issue: #33
Milestone: 0.6.0
Spec: `docs/superpowers/specs/2026-06-08-issue-33-jwt-helper-utilities-spec.md`
Spec review: `docs/superpowers/reviews/2026-06-08-issue-33-jwt-helper-utilities-spec-review.md`
Follow-ups: #173, #174, #175

## Execution Boundary

Implement only the #33 core JWT helper package:

- top-level package `jwt`;
- `github.com/golang-jwt/jwt/v5` dependency;
- explicit algorithm selection for HMAC, RSA, and RSA-PSS;
- composer options for registered claims, custom claims, and safe headers;
- reader helpers for typed claims/headers, `kid`, expiry, and remaining TTL;
- fixed-key providers and in-memory rotating KeyChain providers;
- in-memory-only private repository/rotation behavior;
- package README pair, root README pair, WIP, CHANGELOG, tests, examples, and
  Step 6-R/Step 7-R review artifacts.

Do not implement:

- HTTP auth middleware, sessions, OIDC, JWKS, roles, or permissions;
- Redis/Mongo/distributed KeyChain repositories (#173);
- JWT compression or JOSE `zip` support (#174);
- provider cache adapters (#175).

Apply `bluetape-go-patterns` to every Go API, Go test, example, README, and
review task. Public APIs must be narrow, Go-native, `errors.Is` compatible, and
must not expose raw bearer tokens, private keys, symmetric secrets, or
`golang-jwt/jwt/v5` concrete parser/token types.

## Current Evidence

- Spec Step 2-R is closed with P0=0/P1=0 after subagent review.
- `docs/package-layout.md` requires small public packages, package docs,
  examples, error semantics, and concurrency/context behavior.
- `testing/concurrency` provides `GoroutineStressTester` and `AsyncJobTester`.
- Current repo patterns use narrow sentinel errors and wrapper errors, as in
  `id/errors.go`.
- `github.com/golang-jwt/jwt/v5` is not yet in `go.mod`; it must be added as a
  direct dependency.
- Follow-up #173 now requires the future distributed repository contract to be
  context-aware.

## Task Plan

| Task | Complexity | Expected files | Actions | Verification |
| --- | --- | --- | --- | --- |
| T0 - Dependency and API risk note | high | `docs/superpowers/reviews/2026-06-08-issue-33-jwt-preimplementation-risk.md`, `go.mod`, `go.sum` | Add `github.com/golang-jwt/jwt/v5` as a direct dependency. Record adopted version, current maintenance evidence, API assumptions (`RegisteredClaims`, `MapClaims`, `ParseWithClaims`, `Keyfunc`, `WithValidMethods`, `WithLeeway`, issuer/audience/subject options), and rejected `jwx`/`go-jose` scope. Confirm public API does not expose dependency parser/token types. | `go get github.com/golang-jwt/jwt/v5@v5.3.1`; `go list -m github.com/golang-jwt/jwt/v5`; risk note records decision and assumptions. |
| T1 - Package scaffold and errors | medium | `jwt/doc.go`, `jwt/errors.go` | Create package docs and sentinel errors: `ErrInvalidOptions`, `ErrInvalidToken`, `ErrInvalidKey`, `ErrKeyNotFound`, `ErrExpiredToken`, `ErrNotYetValid`. Add wrapper errors for option, key, token, and validation failures. Error strings must not include raw tokens, private keys, or symmetric secrets. Exported declaration comments follow `docs/package-layout.md`: comments start with the exported identifier and may use concise Korean text. | `go test -count=1 ./jwt` once tests exist; error tests prove `errors.Is` and non-leakage. |
| T2 - Algorithms and keychain model | high | `jwt/algorithm.go`, `jwt/keychain.go`, `jwt/*_test.go` | Define `Algorithm` constants for HS256/384/512, RS256/384/512, PS256/384/512. Map each to `golang-jwt/jwt/v5` signing methods and key expectations. Add `KeyChain` with `KID`, algorithm, created/expiry times, internal signing key, and verification key behavior. Add constructors for fixed HMAC and fixed RSA/PS keychains, generated RSA/PS keychains, and generated HMAC only if entropy sizing is simple and tested. Fixed HMAC secrets must be at least the selected hash size: HS256 32 bytes, HS384 48 bytes, HS512 64 bytes. Use `crypto/rand.Reader` by default with test-local entropy injection. | Unit tests for algorithm support/rejection, fixed-key construction, empty/short fixed HMAC secret rejection with `ErrInvalidKey`, generated key behavior, deterministic HMAC entropy length, entropy failure/short-read wrapping, and no global entropy mutation. |
| T3 - Composer options and safe headers/claims | high | `jwt/claims.go`, `jwt/composer.go`, `jwt/*_test.go` | Implement compose options for headers, custom claims, issuer, subject, audience, issued-at, not-before, expires-at, expires-after, and JWT ID. Set implicit `iat` from provider clock. Reject caller overrides for `alg`, `kid`, `zip`, `crit`, `jku`, `jwk`, `x5u`, `x5c`, and registered claims through generic custom-claim APIs. | Tests for registered claims, custom claims, custom safe headers, deterministic clock `iat`/`WithExpiresAfter`, reserved header rejection, reserved claim rejection, and token/key non-leakage in option errors. |
| T4 - Reader and parse validation | high | `jwt/reader.go`, `jwt/provider.go`, `jwt/*_test.go` | Implement parse with `golang-jwt/jwt/v5` using `WithValidMethods`, `Keyfunc`, registered-claim validation, leeway, expected issuer/audience/subject, expiration-required option, and injected parse clock. Reject inbound signed tokens carrying unsupported `zip`, `crit`, `jku`, `jwk`, `x5u`, or `x5c` headers. Reader exposes `Kid`, `Algorithm`, typed headers/claims, registered claims, `IsExpired`, and `RemainingTTL`, but not raw token. Copy maps/slices so callers cannot mutate provider-owned state. | Tests for successful parse, typed accessors, expected issuer/audience/subject, expiration failure/leeway success, not-before failure/leeway success, deterministic `IsExpired` and `RemainingTTL` using fixed times, inbound unsupported JOSE/compression header rejection, malformed token, wrong algorithm, wrong key, missing/unknown `kid`, `TryParse` success/failure as a boolean wrapper over hardened `Parse`, reader copy isolation, and no raw-token API. |
| T5 - In-memory repository and rotating provider | high | `jwt/repository.go`, `jwt/provider.go`, `jwt/*_test.go` | Implement private in-memory `keyChainRepository` with capacity default 10 and bounds 2..1000. Implement fixed-key providers and rotating in-memory providers. `Rotate` creates a new key only when missing/expired; `ForcedRotate` always creates; retained non-expired keys verify by `kid`; expired/evicted/unknown keys fail. Keep repository API private and context-free for #33 only. Fixed providers may accept tokens without `kid` only when exactly one fixed key is configured; rotating/repository-backed providers reject missing `kid`. | Tests for current key, rotate no-op with live key, rotate after expiry, forced rotate changes `kid`, retained old-key parse, expired retained-key parse failure, capacity trim eviction, unknown `kid`, invalid capacity, no-`kid` fixed-provider accept boundary, missing-`kid` rotating-provider rejection, and fixed provider behavior. |
| T6 - Stress, race, and cancellation applicability | high | `jwt/jwt_concurrency_test.go`, `docs/superpowers/reviews/2026-06-08-issue-33-jwt-concurrency-notes.md` | Use `GoroutineStressTester` to run concurrent compose, parse, and forced-rotate tasks against a shared provider. Prove retained-key parse success during rotation, evicted/expired-key failures only after capacity/expiry rules, reader copy isolation, and no data races. Add a structural lock-scope note proving parse/signature verification does not hold a repository/provider write lock; use code references in the concurrency notes and Step 6-R review. If no context-aware #33 API is added, record the required `AsyncJobTester N/A` note. | `go test -race -count=1 ./jwt`; `rg -n "GoroutineStressTester" jwt docs/superpowers/reviews`; `rg -n "write lock|signature verification" docs/superpowers/reviews/2026-06-08-issue-33-jwt-concurrency-notes.md docs/superpowers/reviews/2026-06-08-issue-33-jwt-helper-utilities-code-review.md`; `rg -n "AsyncJobTester N/A: #33 core JWT operations are local CPU/crypto work with no caller-observable cancellation boundary" docs/superpowers/reviews`. |
| T7 - Examples and package README pair | medium | `jwt/jwt_example_test.go`, `jwt/README.md`, `jwt/README.ko.md`, `jwt/doc.go` | Add compile-checked examples for fixed HMAC provider and rotating RSA/PS provider. README pair documents package scope, explicit algorithms, key management warning, in-memory rotation, parser hardening, typed claims, clock skew, non-auth-framework boundary, and follow-ups #173/#174/#175. | `go test -count=1 ./jwt -run Example`; `rg -n "not an auth framework|WithValidMethods|kid|rotation|compression|#173|#174|#175|secret|private key" jwt/README.md jwt/README.ko.md jwt/doc.go`. |
| T8 - Root docs and release notes | medium | `README.md`, `README.ko.md`, `CHANGELOG.md`, `WIP.md` | Promote `jwt` from planned to active package in root README pair and note #33 in release/work-in-progress docs. Keep English/Korean summaries synchronized and mention deferred distributed/compression/cache follow-ups. | `rg -n "jwt|JWT|#33|#173|#174|#175|0.6.0" README.md README.ko.md CHANGELOG.md WIP.md`. |
| T9 - Targeted validation | medium | changed files | Run formatter and targeted validations before review. Keep Testcontainers out of #33 scope. If full repo CI fails from unrelated packages, record exact package/error and keep `./jwt` evidence primary. Verify exported comments against `docs/package-layout.md` before Step 6-R. | `gofmt -w jwt`; `go test -count=1 ./jwt`; `go test -race -count=1 ./jwt`; `go test -count=1 ./...`; `golangci-lint config verify`; `make ci`; `git diff --check`; comment policy spot-check recorded in verifier. |
| T10 - Verifier and Step 6-R subagent 7-Tier code review | high | `docs/superpowers/reviews/2026-06-08-issue-33-jwt-verifier.md`, `docs/superpowers/reviews/2026-06-08-issue-33-jwt-helper-utilities-code-review.md` | Map spec/plan acceptance to evidence. Run subagent-based 7-Tier code review with security/dependency, architecture/API, tests/concurrency, performance/stability, docs/user, and integration lanes. Include exported-comment policy and lock-scope evidence in the review. P0/P1 block PR work and require fixes plus affected-lane reruns. | Review artifacts record subagent lanes, P0/P1/P2/P3 counts, fixes, reruns, and final `P0=0 P1=0`. |
| T11 - Commit, PR, metadata, PR review, CI, and DoD | medium | git/GitHub state, PR body | Commit with Lore trailers after validation/review passes. Push branch and create PR with `Fixes #33` and follow-up links #173/#174/#175. Set PR assignee, milestone, and labels to match #33. Verify PR body DoD, compare PR metadata to issue metadata, run Step 7-R subagent PR review, check CI, and do not merge without user request. | `git status --short`; `gh issue view 33 --json assignees,labels,milestone,title,state`; `gh pr view --json body,assignees,labels,milestone,url,state`; `gh pr checks`; Step 7-R artifact with final gate. |

## Acceptance Mapping

| Spec acceptance | Plan coverage |
| --- | --- |
| PR metadata matches issue #33. | T11 |
| Dependency decision uses `golang-jwt/jwt/v5` and defers `jwx`/`go-jose` to #174. | T0 |
| Core API covers signing, parsing, validation, typed claims/headers, issuer, subject, audience, expiration, not-before, issued-at, clock skew, `kid`, KeyChain rotation, and in-memory repository behavior. | T1-T5 |
| Public API stays narrow and in-memory repository remains private for #173 context-aware future API. | T1, T4, T5, T10 |
| Distributed repositories deferred to #173. | T0, T5, T7, T8 |
| Compression deferred to #174. | T0, T3, T7, T8 |
| Provider cache adapters deferred to #175. | T7, T8 |
| Tests cover deterministic clock/entropy, wrong key/algorithm, expired/nbf/skew, missing/unknown `kid`, fixed HMAC key strength, inbound unsupported JOSE headers, `TryParse`, fixed-provider missing-`kid` boundary, key expiry, rotation, malformed tokens, error non-leakage, and concurrency stress. | T2-T6, T9 |
| Parse/signature verification does not hold a write lock where a narrower read path is possible. | T6, T10 |
| Validation commands pass before PR. | T9 |
| Step 6-R and Step 7-R P0/P1 are zero. | T10, T11 |

## Ordering and Recheck Points

1. Commit spec, spec review, plan, and plan review before implementation.
2. Run T0 before code beyond package scaffold; do not expose dependency concrete
   parser/token types as public API.
3. Implement errors before algorithm/keychain code so all failures wrap package
   sentinels.
4. Implement algorithm/keychain before composer/parser so `kid` and `alg`
   hardening are structural, not bolted on afterward.
5. Implement reader map/slice copy behavior and deterministic
   `IsExpired`/`RemainingTTL` tests before stress tests so concurrency tests
   assert the final API contract.
6. Add unit tests before stress tests; stress tests should prove specified
   semantics, not discover missing ones.
7. Add docs after API names stabilize, before Step 6-R.
8. Run targeted `./jwt` tests and race before full `./...`.
9. Run Step 6-R only after code, tests, examples, README pair, root docs, and
   release notes are present.

## Risk Controls

| Risk | Control |
| --- | --- |
| Algorithm confusion | T4 uses `WithValidMethods` and exact token `alg` to KeyChain algorithm match; tests cover wrong algorithm. |
| Weak fixed HMAC secrets | T2 rejects empty/short fixed secrets based on selected hash size and tests `ErrInvalidKey`. |
| Missing `kid` silently uses current key | T4/T5 require repository-backed parse to reject missing/unknown `kid`; fixed providers are separate. |
| Token/key/secret leakage | T1/T3/T4 tests check representative error strings; reader does not expose raw token. |
| Future Redis/Mongo API breaking change | T5 keeps #33 repository private; #173 owns context-aware public/pluggable API. |
| JOSE/compression header confusion | T3 rejects composer-supplied `zip`, `crit`, `jku`, `jwk`, `x5u`, `x5c`; T4 rejects inbound signed tokens carrying those headers; #174 owns compression research. |
| Rotation races | T5 implements locked in-memory repository; T6 runs `GoroutineStressTester`, race tests, and a structural lock-scope note for parse/signature verification. |
| Expired retained keys keep verifying forever | T5 explicitly rejects expired retained keys and tests it. |
| Clock tests are flaky | T2-T4 use injected clocks and deterministic assertions. |
| Entropy tests mutate global state | T2 uses provider-local entropy injection and tests failure/short-read paths. |
| Package becomes auth framework | Execution boundary, docs, and examples avoid HTTP middleware/OIDC/session/permission scope. |

## Validation Commands

```bash
gofmt -w jwt
go test -count=1 ./jwt
go test -race -count=1 ./jwt
go test -count=1 ./...
golangci-lint config verify
make ci
rg -n "GoroutineStressTester" jwt docs/superpowers/reviews
rg -n "write lock|signature verification" docs/superpowers/reviews/2026-06-08-issue-33-jwt-concurrency-notes.md docs/superpowers/reviews/2026-06-08-issue-33-jwt-helper-utilities-code-review.md
rg -n "AsyncJobTester N/A: #33 core JWT operations are local CPU/crypto work with no caller-observable cancellation boundary" docs/superpowers/reviews
rg -n "not an auth framework|WithValidMethods|kid|rotation|compression|#173|#174|#175|secret|private key" jwt/README.md jwt/README.ko.md jwt/doc.go
rg -n "jwt|JWT|#33|#173|#174|#175|0.6.0" README.md README.ko.md CHANGELOG.md WIP.md
git diff --check
```

## Step 3 Checklist Completion Report

| Item | Status | Notes |
| --- | --- | --- |
| Plan path confirmed inside feature worktree | Done | `docs/superpowers/plans/2026-06-08-issue-33-jwt-helper-utilities-plan.md`. |
| All tasks have complexity labels | Done | T0-T11 include labels. |
| `bluetape-go-patterns` applied to code-bearing tasks | Done | Execution boundary and T0-T11 require Go API/test/docs/review checks. |
| Plan code/test snippets conform to Go patterns | Done | Public snippets are narrow interfaces and commands; no broad dependency types. |
| Thread/cancellation helpers assigned | Done | T6 uses `GoroutineStressTester`; `AsyncJobTester` is evidence-required N/A unless context-aware API is added. |
| Tests and verification tasks included | Done | T1-T6, T9, T10. |
| Multilingual README and release docs included | Done | T7/T8 cover package/root README pairs and release docs. |
| Risky ordering/dependency assumptions explicit | Done | T0, ordering/recheck points, and risk controls. |
| Spec + plan committed before implementation | Pending | Commit after Step 3-R passes. |
