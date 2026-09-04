# JWKS Provider Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `test-driven-development` for
> every implementation task and run the listed verification command before
> moving to the next task.

**Goal:** Issue #545의 `jwt/jwks` optional provider를 구현해 원격 JWKS에서
RSA/ECDSA/EdDSA public key를 context-aware로 조회하고, symmetric key/JWE는
거부하며, TTL·rotation·unknown `kid` refresh·single-flight를 안전하게
제공한다.

**Architecture:** `jwt` root API는 변경하지 않고 `jwt/jwks`에만 provider를
둔다. `go-jose/v4`는 JWK JSON decode와 public-key conversion에만 사용한다.
Provider는 immutable snapshot publication record와 instance-local mutex/flight를
소유한다. HTTP/read/decode는 lock 밖에서 수행하고, 명시적 `Refresh`는
cooldown을 우회하며 lookup 내부 refresh는 generation/cooldown으로 합친다.
Caller는 `KeyFunc`를 request마다 만들고 claims/issuer/audience/expiration 정책을
`golang-jwt/v5` parser에 별도로 지정한다.

**Tech Stack:** Go 1.26.3 module, `github.com/golang-jwt/jwt/v5`,
`github.com/go-jose/go-jose/v4 v4.1.4`, `net/http`, `httptest`, `sync`,
`crypto/rsa`, `crypto/ecdsa`, `crypto/ed25519`, `go test -race`, Makefile gates.

---

## 작업 1 — 의존성·package 골격과 오류 계약

**Files:** `go.mod`, `go.sum`, `jwt/jwks/doc.go`, `jwt/jwks/errors.go`,
`jwt/jwks/options.go`, `jwt/jwks/provider_test.go`

1. **RED:** `provider_test.go`에 `New` endpoint/option 오류, package-local
   sentinel과 root `jwt` sentinel의 `errors.Is`, `FetchError`/`SetError`의
   `errors.As`, context error 보존, `New(endpoint, nil)`과 option 목록 중간의
   nil option 거부를 먼저 작성한다. `WithAllowedAlgorithms(HS256)`와
   symmetric/unknown 값도 constructor에서 거부되는지 고정한다. 기대 결과는
   새 package가 아직 없어서 컴파일 실패다.
2. `go get github.com/go-jose/go-jose/v4@v4.1.4`를 실행하고
   `go mod tidy`로 checksum을 고정한다. `go list -m -json` 결과와 license/
   module version 근거를 구현 PR evidence에 남긴다.
3. `doc.go`에 package 목적·public key 타입·JWE 제외를 한국어 Go doc으로
   기록한다. `Algorithm` type과 `RS256`, `RS384`, `RS512`, `PS256`, `PS384`,
   `PS512`, `ES256`, `ES384`, `ES512`, `EdDSA` 상수를 정의한다.
4. `errors.go`에 `ErrFetch`, `ErrMalformedSet`, `ErrUnsupportedAlgorithm`과
   `ErrInvalidOptions`, `ErrInvalidKey`, `ErrKeyNotFound` root alias를 둔다.
   `FetchClass`, `FetchError`, `SetError`는 endpoint/token/body/JWK material을
   문자열에 넣지 않고, `FetchError.Is`가 `ErrFetch`와 sanitized context
   cause를 보고하며 `SetError.Is`가 `ErrMalformedSet`과
   `jwt.ErrInvalidKey`를 보고하도록 구현한다.
5. `options.go`의 config 기본값을 cache TTL 5분, fetch timeout 10초, body
   limit 1 MiB, hard body cap 8 MiB, refresh cooldown 1초, asymmetric
   allowlist로 고정한다.
   `WithHTTPClient(nil)`, `WithCacheTTL <= 0`, `WithRefreshCooldown <= 0`,
   `WithFetchTimeout <= 0`, `WithMaxBodySize <= 0`, 빈/중복/알 수 없는
   `WithAllowedAlgorithms`는 `jwt.OptionError`로 거부한다. 기본 HTTP client는
   `CheckRedirect`에서 `http.ErrUseLastResponse`를 반환한다.
6. **GREEN:** `go test ./jwt/jwks -run 'Test(New|Option|Error)'`가 option/error
   RED 테스트를 통과하도록 최소 구현한다.
7. **REFACTOR:** 모든 exported type/function/constant에 한국어 Go doc을
   보완하고 `gofmt`/`git diff --check`를 실행한다.

Expected result: package가 network I/O 없이 생성되고, 모든 option/error 경계가
root sentinel과 typed error로 검사된다.

## 작업 2 — JWK snapshot 검증과 defensive public-key copy

**Files:** `jwt/jwks/keys.go`, `jwt/jwks/provider_test.go`

1. **RED:** RSA 2048 미만/비정상 exponent, ECDSA curve/alg mismatch, Ed25519, `oct`, private
   key, unknown/empty/overlong/non-printable `kid`, duplicate `kid`, empty set,
   over-limit key count, wrong `use`/`key_ops`, non-empty mismatched `alg`,
   valid embedded key with `x5u`/`x5c`, malformed JSON/JWK fixture를 추가한다.
2. `go-jose/v4`의 `JSONWebKeySet`을 decode한 뒤 각 key가
   `*rsa.PublicKey`, `*ecdsa.PublicKey`, `ed25519.PublicKey`인지 확인한다.
   `JSONWebKey.IsPublic()`이 false인 key와 `Use`가 `""`/`"sig"`가 아닌 key는
   `SetError`로 거부한다. `x5u`/`x5c`/certificate metadata는 유효한 embedded
   JWK public key가 있으면 무시하고 추가 HTTP 요청을 하지 않으며, go-jose가
   검출한 malformed/mismatched embedded `x5c`는 `SetError`로 매핑한다.
   set는 최대 256개 key로 제한하고 초과 시 bounded `SetError`를 반환한다.
3. `Algorithm`별 key type/curve/RSA bit size를 확인하고 RSA exponent는
   최소 3 이상, odd, representable 범위인지 raw JWK decode 전에 검증한다.
   JWK `Algorithm`이 비어 있지 않으면 요청 token algorithm과 정확히 비교한다.
   raw JWK를
   go-jose로 decode하기 전에 Ed25519 `x`의 base64url decoded 길이가 정확히
   32-byte인지 확인하고, `key_ops`가 있으면 `verify`만 허용한다. `HS*` 및
   기타 symmetric/unknown algorithm은 I/O 전에 `ErrUnsupportedAlgorithm`과
   `jwt.ErrInvalidKey`를 반환한다.
4. snapshot map에는 검증된 public key만 보관하고, `Lookup` 반환 시 RSA/EC
   구조체와 Ed25519 byte slice를 깊은 copy한다. caller가 반환값을 mutate해도
   다음 lookup 결과가 바뀌지 않는 회귀 테스트를 둔다.
5. **GREEN:** `go test ./jwt/jwks -run 'Test(Key|JWK|Algorithm|Defensive)'`를
   통과시킨다.
6. **REFACTOR:** key validation 오류는 raw JWK 값/kid material을 노출하지
   않도록 `SetError` 경계를 하나로 정리한다.

Expected result: 허용된 asymmetric public key만 immutable snapshot으로 들어가며
JWE, x5u/x5c 추가 fetch, HMAC key는 구현 경로에 없다.

## 작업 3 — HTTP fetch와 context/redirect/body 경계

**Files:** `jwt/jwks/provider.go`, `jwt/jwks/provider_test.go`

1. **RED:** `httptest.Server`로 200 성공, non-200, HTTPS endpoint, loopback
   HTTP test endpoint, private/link-local/metadata literal address 거부,
   default redirect 거부, custom client redirect/TLS/dial ownership, body
   limit hard cap과 Content-Length early reject, malformed JSON,
   `context.Canceled`, caller deadline, provider fetch timeout을 검증한다.
   모든 실패에서 URL/token/body가 error string에 없는지 확인한다.
2. endpoint는 `url.Parse` 후 기본 `https` 또는 loopback host의 테스트용
   `http`, non-empty host, no userinfo, no fragment를 검사한다. 기본 dial은
   private/link-local/metadata 주소를 거부하고, custom client가 TLS/proxy/
   DNS/redirect 정책을 완화할 수 있음은 caller-owned security boundary로
   문서화한다. `New`는 요청을 만들지 않고 provider만 반환한다.
3. `fetch(ctx)`는 `http.NewRequestWithContext`와 `context.WithTimeout`을
   사용하고, response body를 항상 `defer Close`한다. hard cap과 overflow를
   확인한 뒤 `Content-Length`를 early reject하고, `io.LimitReader`에
   `maxBodySize+1`을 전달해 초과를 감지한다. status가 `200 OK`가 아니면
   sanitized `FetchError`를 반환한다.
4. 기본 `http.Client`는 redirect를 자동 추적하지 않으며, custom client의
   redirect/allowlist 정책은 호출자 소유로 문서화한다.
5. **GREEN:** `go test ./jwt/jwks -run 'Test(Fetch|Endpoint|Context|Redirect|Body)'`
   를 통과시킨다.
6. **REFACTOR:** response/transport/context 분류를 `FetchClass`와 typed error로
   통합하고 원격 URL을 `fmt.Errorf`에 삽입하는 경로를 제거한다.

Expected result: bounded, context-aware fetch가 body/redirect/HTTP 오류를
redacted error로 반환하고 실패 시 snapshot을 교체하지 않는다.

## 작업 4 — snapshot cache, TTL, rotation, single-flight

**Files:** `jwt/jwks/provider.go`, `jwt/jwks/provider_test.go`

1. **RED:** 첫 lookup fetch와 warm cache hit(요청 1회), `now >= fetchedAt+TTL`
   경계, rotated `kid`, explicit `Refresh`의 cooldown 우회, TTL 만료 실패의
   stale fail-closed, concurrent lookup의 HTTP 1회 테스트를 작성한다.
2. `publication`을 `{keys, fetchedAt, generation}` 한 record로 두고 mutex 아래
   pointer를 교체한다. mutex는 snapshot/flight 상태에만 사용하며 HTTP/read/
   decode/JWK conversion은 lock 밖에서 한다. 테스트 전용 package-internal
   clock seam으로 시간을 고정한다.
3. `refreshFlight{done, err}`와 instance-local flight pointer를 구현한다.
   cache hit는 I/O 없이 반환하고, TTL 만료 첫 caller만 fetch leader가 된다.
   waiter context 취소는 독립적으로 반환하며, leader context 취소 시 live
   waiter 한 명은 자신의 context로 최대 한 번 takeover한다. caller-owned
   `context.Canceled`/deadline은 `forcedAt` 또는 failure cooldown에 기록하지
   않으며 flight를 정리한다. leader가 취소되고 live waiter가 없으면 다음
   정상 caller가 즉시 새 flight를 만들 수 있어야 한다.
4. 성공한 refresh만 publication을 교체한다. TTL 만료 refresh error는 stale
   key를 반환하지 않는다. explicit `Refresh`는 cooldown을 우회하되 진행 중인
   flight에는 합류한다. flight는 고유 identity/generation을 가지며 현재
   owner만 snapshot 교체와 completion을 수행한다. takeover 전 leader의
   늦은 결과는 publication을 덮어쓰지 않는다.
5. `forcedAt`, generation, `WithRefreshCooldown`을 사용해 서로 다른 unknown
   `kid` burst와 실패 refresh 재시도를 cooldown당 한 flight로 합친다. 성공한
   forced refresh 뒤에도 같은 generation/cooldown 안에서는 재-fetch하지 않고
   `ErrKeyNotFound`를 반환한다.
6. blocked `httptest` endpoint → TTL 만료 fail-closed → 이전 endpoint 설정
   복원 → 새 Provider와 readiness `Refresh` → known `kid` signature 검증 →
   traffic 재개 순서를 하나의 bounded rollback-drill 테스트로 고정하고,
   overlap key가 retirement 전에 모든 consumer refresh를 통과했는지 증거를
   남긴다.
7. **GREEN:** `go test ./jwt/jwks -run 'Test(Cache|TTL|Rotation|Refresh|SingleFlight|Cooldown|Rollback)'`
   를 통과시킨다. leader/waiter/전원 취소, transport/JWK 실패, takeover
   성공·실패 뒤 bounded 후속 lookup과 HTTP count, delayed `RoundTripper`의
   늦은 leader 결과 무시, blocked explicit refresh 중 warm lookup의
   deadline 내 성공을 함께 검증한다.
8. **REFACTOR:** lock scope와 flight cleanup을 주석/헬퍼로 고정하고,
   `go test -race ./jwt/jwks`를 이 작업 후 최초로 실행한다.

Expected result: cache hit/miss/expiry/rotation과 concurrent miss가 정해진
HTTP round trip으로 동작하고, 만료 뒤 stale 인증을 하지 않는다.

## 작업 5 — `KeyFunc` adapter와 end-to-end JWT 검증

**Files:** `jwt/jwks/provider.go`, `jwt/jwks/provider_test.go`,
`jwt/jwks/api_example_test.go`

1. **RED:** RSA/PS, ECDSA, EdDSA signed token fixture를 만들고
   `golang-jwt/v5` parser에서 `KeyFunc`로 검증한다. allowlist 밖 `alg`, nil
   token, missing/non-string/overlong/non-printable `kid`/`alg`, unsupported
   inbound `jku`/`jwk`/`x5u`/`x5c`/`zip`/`crit` header, canceled request context를
   실패시킨다. nil
   receiver와 nil construction context도 `jwt.ErrInvalidOptions`를 보존하는
   construction error로 거부한다. `WithAllowedAlgorithms(Algorithm("HS256"))`
   및 unknown/symmetric 값도 같은 단계에서 거부한다.
   root `jwt`의 rejection helper는 unexported이므로 package-local 동일 집합
   검증 helper를 두고 여섯 header 전부를 table-driven 회귀 테스트한다.
2. `KeyFunc(ctx)`는 nil receiver/context를 construction error로 거부하고,
   callback은 매 token의 header를 검증한 뒤 `Lookup`을 호출한다. callback
   오류는 root `jwt.TokenError`를 사용하되 원래 `errors.Is` sentinel을
   보존한다. `api_example_test.go`는 `package jwks_test`에서
   `New → Refresh → KeyFunc → ParseWithClaims`와 root `jwt.Algorithm`에서
   `jwks.Algorithm`으로의 명시적 변환을 실제 caller처럼 compile-check한다.
3. adapter는 key lookup과 signature algorithm만 담당한다. issuer, audience,
   subject, `exp`/`nbf`/expiration-required 검증은 parser options에 남긴다.
4. **GREEN:** `go test ./jwt/jwks -run 'Test(KeyFunc|Parse|Signature|ClaimsBoundary)'`
   를 통과시킨다.
5. **REFACTOR:** request마다 `provider.KeyFunc(req.Context())`를 만드는 사용
   규칙과 canceled closure 재사용 금지를 example/테스트에서 명확히 한다.

Expected result: 세 asymmetric 계열의 서명 검증이 통과하고, claims 신뢰 경계가
caller parser 설정으로 명시된다.

## 작업 6 — package README 및 root 문서/릴리스 bookkeeping

**Files:** `jwt/jwks/README.md`, `jwt/jwks/README.ko.md`, `jwt/README.md`,
`jwt/README.ko.md`, `CHANGELOG.md`

1. `jwt/jwks` 두 README에 source-equivalent 내용으로 import/quick start,
   `New` network-free와 readiness `Refresh`, RSA/ECDSA/EdDSA와 대칭키 거부,
   TTL/rotation/unknown kid/cooldown, redirect/body/error matrix, defensive
   copy, direct JWKS JSON URL만 허용하는 경계, OIDC discovery/JWE 제외를
   작성한다. 기본 endpoint는 HTTPS이며 loopback HTTP는 테스트/개발 전용이고,
   custom client의 `InsecureSkipVerify`, proxy, DNS/dial, redirect 완화는
   caller-owned trust boundary임을 명시한다.
2. runnable `ParseWithClaims` 예제에는
   `WithValidMethods`, `WithIssuer`, `WithAudience`,
   `WithExpirationRequired`를 함께 넣고, `KeyFunc`가 signature-only라는
   경계를 굵게 설명한다. 매 request context와 closure 수명 규칙을 포함한다.
   root와 값이 겹치는 RS/PS 알고리즘만 `jwks.Algorithm(rootAlgorithm)`으로
   명시 변환하고, ES/EdDSA는 `jwks` 상수를 사용하며 HS 변환은 거부된다고
   적는다.
3. 운영 runbook에는 첫 refresh warning, 연속 3회 또는 5분 page, endpoint
   health/allowlist 확인 → bounded Refresh → known kid 검증 → traffic 재개,
   이전 endpoint rollback, mixed-version overlap key ownership을 기록한다.
   service owner / endpoint owner / release owner별 책임, allowlist/health
   preflight 명령과 기대 결과, page/clear 조건, rollback config version,
   overlap key retirement 조건을 표로 고정한다.
4. error matrix는 option/malformed/fetch/context/key-not-found별
   `errors.Is`/`errors.As`, retry 가능성, stale fail-closed를 설명한다.
   caller-owned event field는 `operation`, bounded `FetchClass`, `outcome`,
   bounded status만 허용하고 endpoint URL, token, body, JWK material, raw
   cause, high-cardinality `kid`를 제외한다. 기존 root `jwt.Algorithm` 값은
   `jwks.Algorithm(rootAlgorithm)`으로 변환하는 예제를 둔다.
5. root README의 Deferred 표와 Behavior 문장에서 JWKS optional provider와
   JWE deferred를 분리한다. `CHANGELOG.md` `[Unreleased]`에 optional
   `jwt/jwks` provider 추가와 JWE deferred 경계를 한국어 한 항목으로 기록한다.
6. 새 Go 하위 package는 root module 아래 자동으로 `go test ./...`/Makefile
   CI에 포함되므로 settings/BOM/coverage aggregation/CI workflow 등록은
   추가하지 않는다. 기존 test resource나 Docker fixture도 사용하지 않는다.
7. **GREEN:** `git diff --check`, README source-equivalence 수동 대조,
   `go test ./...`를 실행한다.

Expected result: 신규/기존 README가 API와 운영 계약을 독자에게 동일하게 전달하고
release-facing 기록이 milestone 0.21.0 범위와 맞는다.

## 작업 7 — benchmark, full verification, review evidence

**Files:** `jwt/jwks/benchmark_test.go`, `jwt/jwks/api_example_test.go`,
`docs/review/2026-08-16-issue-545-step-2r-design-review.md`,
`docs/review/2026-08-16-issue-545-step-3r-plan-review.md`,
`docs/review/2026-08-16-issue-545-verification.md`,
`docs/lessons/2026-08-16-issue-545-jwks-provider.md`

1. named benchmark `BenchmarkLookupCacheHit`, `BenchmarkLookupParallelHit`,
   `BenchmarkLookupForcedRefresh`를 작성하고 warm hit/cold miss/TTL expiry/
   forced refresh/parallel hit·miss fixture를 분리한다. HTTP request count,
   refresh count, allocations를 benchmark helper로 검증한다.
2. 아래 순서로 검증하고 `docs/review/2026-08-16-issue-545-verification.md`에
   `go version`, `go env GOOS GOARCH`, CPU와 raw benchmark 결과, HTTP/lock
   count, `go mod verify`, `go list -m -json github.com/go-jose/go-jose/v4`,
   `go mod graph` dependency delta, module `LICENSE` 경로 evidence를 보존한다.
   `govulncheck`가 설치된 환경이면 `govulncheck ./jwt/jwks` 결과를 추가하고,
   설치되지 않았으면 그 사실을 verification gap으로 명시한다.
   ```bash
   go test ./jwt/jwks
   go test -race ./jwt/jwks
   go test -run '^$' -bench . -benchmem ./jwt/jwks
   make fmt-check
   make tidy-check
   make vet
   make lint
   go test ./...
   make ci
   ```
3. Step 3-R 계획 리뷰와 final Step 6-R/7-R review에서 six lenses와 main
   integration을 다시 실행한다. P0/P1은 각각 0이어야 한다.
4. lesson에는 실제 실패/복구, go-jose boundary, cooldown/lock scope,
   claims boundary와 다음 변경자를 위한 directive를 한국어로 남긴다.
5. 검증 실패 시 가장 작은 원인별 수정만 하고 해당 작업의 RED/GREEN 및 전체
   gate를 재실행한다.
6. 이 작업은 release promotion을 수행하지 않는다. release PR 전 release
   owner는 `docs/release/release-guide.md` preflight를 수행하고 `WIP.md`,
   versioned `CHANGELOG` section, milestone open-issue count, tag/release
   부재, `make ci`, downstream `go get`/`go mod tidy`/test 결과를
   verification artifact에 기록한다.

Expected result: 모든 code/README/benchmark/review/lesson evidence가 fresh
검증으로 연결되고 PR을 merge-ready 상태로 만들 수 있다.

## 위험·롤백·재실행

- `go-jose/v4` API/Go version mismatch: `go list -m -json`과 module source를
  먼저 확인하고, dependency를 추가로 늘리지 않는다. `go mod tidy`가 예상치
  않은 module을 지우면 go.mod/go.sum만 원복하고 package 구현을 진행하지
  않는다.
- malformed/oversized/redirected JWKS: fetch 단계에서 snapshot을 교체하지
  않고 typed error를 반환한다. 코드 rollback은 새 파일과 README/CHANGELOG
  변경을 revert하는 대신 해당 커밋을 revert할 수 있게 단일 Lore commit으로
  묶는다.
- refresh storm/deadlock/race: named request-count 테스트, blocked-server
  test, `go test -race`가 실패하면 cache/flight 구현을 확장하지 말고 lock
  scope와 generation publication을 먼저 고친다.
- public API 불일치: Step 3-R 승인 전에는 구현 파일을 커밋하지 않는다. 계획
  변경이 API/의존성/운영 정책을 바꾸면 계획 리뷰를 다시 열고 main integration
  판정을 갱신한다.
- diagram은 새 사용자-facing 관계가 없어 N/A다. 구현 중 시각 자료가 추가되면
  `$bluetape-diagram` skill과 endpoint/sequence audit을 별도로 적용한다.

## 완료 조건

계획 문서 자체는 exact file path, RED/GREEN/REFACTOR 순서, 명령과 기대 결과,
rollback/재실행, 문서·운영·benchmark·review·lesson 산출물을 포함한다. 이 계획의
Step 3-R에서 `P0=0 P1=0`이 확인되기 전에는 `jwt/jwks` 구현 파일을 수정하지
않는다.
