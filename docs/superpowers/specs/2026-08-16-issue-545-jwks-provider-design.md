# Issue #545 JWKS key provider 설계

## 설계 상태

- 이슈: [#545](https://github.com/bluetape4k/bluetape-go/issues/545)
- 상위 Epic: [#540](https://github.com/bluetape4k/bluetape-go/issues/540)
- 작업 유형: Type A - Full Feature
- 대상 브랜치: `feat/jwks-provider-545`
- 대상 패키지: `jwt/jwks`
- 설계 결정: RSA/ECDSA/EdDSA 공개키를 허용하고, 대칭키와 JWE는 제외한다.

## SPW-01 — 독자와 목적

이 문서는 `jwt` 호출자가 원격 JWKS 문서에서 서명 검증 key를 안전하게
조회할 수 있도록 선택적 provider의 경계와 동작을 정의한다. 구현자는 이
문서만으로 public API, 의존성, 오류, 캐시, 동시성, 테스트, 문서 변경 범위를
판단할 수 있어야 한다.

## 문제와 근거

현재 `jwt` core는 `github.com/golang-jwt/jwt/v5` 기반의 HMAC/RSA signing과
검증 helper를 제공하지만 원격 JWK set fetch/cache는 제공하지 않는다. Issue
#497의 조사 결정은 JWKS key-source/cache를 먼저 구현하고 JWE는 실제 호출자가
필요로 할 때까지 미루는 것이다. 따라서 이번 변경은 기존 `jwt.Provider`를
대체하지 않고 별도 하위 패키지에 한정한다.

## 접근안 비교

| 접근안 | 장점 | 거부 또는 선택 근거 |
|---|---|---|
| 표준 library로 JWK parser와 crypto 변환을 직접 구현 | 새 외부 package가 없음 | RSA/ECDSA/EdDSA JWK 검증과 RFC 경계가 구현 범위로 유입되어 보안 위험이 커지므로 거부 |
| `github.com/go-jose/go-jose/v4`로 JWK 변환만 맡기고 fetch/cache는 provider가 소유 | 좁은 JWK parser, 공개키 타입 변환, snapshot 정책을 호출자 계약에 맞게 통제 가능 | **선택**. `v4.1.4`를 사용하며 core import에는 추가하지 않고 `jwt/jwks`에서만 사용 |
| `github.com/lestrrat-go/jwx/v3/jwk`의 fetcher/cache를 직접 노출 | fetch/cache가 이미 있음 | 범위가 넓고 async cache/background 동작이 이번 계약의 명시적 `no package-global background loop`와 맞지 않아 거부 |

선택 근거와 API는 [go-jose JWK API](https://pkg.go.dev/github.com/go-jose/go-jose/v4),
[jwx JWK fetch/cache API](https://pkg.go.dev/github.com/lestrrat-go/jwx/v3/jwk),
그리고 [#497 연구 gate](https://github.com/bluetape4k/bluetape4k-wiki/blob/develop/research/2026-07-09-bluetape-go-ecosystem-parity-research-gate.md)를 기준으로 한다.

## 설계 결정

### 패키지와 의존성 경계

- 새 package는 `github.com/bluetape4k/bluetape-go/jwt/jwks`이다.
- root `jwt`의 기존 public API와 signing behavior는 변경하지 않는다.
- root module의 `go.mod`에는 `github.com/go-jose/go-jose/v4 v4.1.4`가 추가되지만,
  해당 import는 `jwt/jwks` 내부로 한정한다. 기존 `jwt` 호출자가 새 package를
  import하지 않으면 JWKS parser를 사용하지 않는다.
- 기본 HTTP client는 redirect를 자동 추적하지 않는다. JWKS endpoint는
  trusted operator configuration의 직접 URL이며 일반 입력값으로 취급하지
  않는다. 기본 endpoint는 `https`만 허용하고, `http`는 loopback host의
  `httptest`/개발 경로에만 허용한다. 기본 dial 정책은 loopback, link-local,
  private, cloud-metadata literal address를 거부하며, HTTP endpoint에 한해
  loopback을 endpoint-scoped 예외로 허용한다. HTTPS loopback literal과 DNS
  resolution 결과는 모두 차단한다.
  custom `http.Client`를 주입하는 호출자는 TLS 검증, proxy, DNS/dial,
  redirect/allowlist 정책을 자신의 trusted configuration에서 책임지며,
  custom client가 이 경계를 완화할 수 있음을 문서화한다. endpoint에는
  non-empty host가 필요하며 userinfo와 fragment는 허용하지 않는다.
- `jwx`, JWE, OIDC discovery, package-global cache, background refresh goroutine은
  이번 범위에 포함하지 않는다.

### Public API 초안

```go
package jwks

type Provider struct

func New(endpoint string, options ...Option) (*Provider, error)
func (p *Provider) Lookup(ctx context.Context, kid string, algorithm Algorithm) (any, error)
func (p *Provider) KeyFunc(ctx context.Context) (golangjwt.Keyfunc, error)
func (p *Provider) Refresh(ctx context.Context) error

type Option func(*config) error
type Algorithm string

func WithHTTPClient(client *http.Client) Option
func WithCacheTTL(ttl time.Duration) Option
func WithRefreshCooldown(cooldown time.Duration) Option
func WithFetchTimeout(timeout time.Duration) Option
func WithMaxBodySize(limit int64) Option
func WithAllowedAlgorithms(algorithms ...Algorithm) Option
```

- `New`는 기본적으로 `https` endpoint와 유효한 option만 받아 provider를
  만든다. loopback host의 `http`는 테스트/개발 목적에 한해 허용한다.
- `New`는 network-free constructor다. 첫 `Lookup`은 snapshot이 없으면
  요청 경로에서 fetch할 수 있으므로, startup readiness가 필요한 caller는
  traffic을 열기 전에 명시적으로 `Refresh`를 호출하고 known `kid`를
  검증한다.
- `Lookup`은 `kid`와 unverified token header의 `alg`를 함께 받아 검증용
  public key를 반환한다. `Algorithm`과 공개 상수는 `RS256`, `RS384`, `RS512`,
  `PS256`, `PS384`, `PS512`, `ES256`, `ES384`, `ES512`, `EdDSA`를 표현하며
  그 밖의 값은 constructor option 또는 lookup에서 거부한다. `ctx`
  취소/deadline은 fetch에 전달한다.
- `KeyFunc(ctx)`는 `golang-jwt/v5`의 `Keyfunc` adapter와 construction error를
  반환한다. callback은 token의
  `kid`와 `alg`를 읽고 `Lookup`을 호출한다. token 문자열이나 원격 URL을
  오류 메시지에 넣지 않는다. nil context, nil token, 누락/비문자열 header는
  root `jwt` option/key error로 즉시 거부한다. request마다 해당 request
  context로 새 adapter를 만들고, 장수하는 closure에 취소된 context를 재사용하지
  않는 caller 규칙을 README에 명시한다.
- `KeyFunc`는 key lookup과 signature algorithm allowlist만 담당하며 issuer,
  audience, subject, `exp`/`nbf`, `WithExpirationRequired` 같은 claims 신뢰
  정책을 적용하지 않는다. README 예제는 `ParseWithClaims`에
  `WithValidMethods`, `WithIssuer`, `WithAudience`,
  `WithExpirationRequired`를 함께 지정하고, caller가 이 정책을 생략하면
  서명만 검증된다는 경계를 명시한다. 별도 context-aware parse wrapper는
  추가하지 않아 `golang-jwt/v5` parser 계약을 그대로 유지한다.
- callback은 기존 root `jwt`와 같은 inbound JOSE header 경계를 적용한다.
  `zip`, `crit`, `jku`, `jwk`, `x5u`, `x5c`가 있으면 거부하며, root helper가
  unexported이므로 `jwt/jwks`에는 동일한 상수 집합을 검증하는 package-local
  helper와 table-driven 회귀 테스트를 둔다.
- `Refresh`는 현재 snapshot을 강제로 교체한다. 호출자가 관리하는 명시적
  refresh이며 background goroutine을 만들지 않는다. 명시적 `Refresh`는
  `WithRefreshCooldown`을 우회하지만 이미 진행 중인 flight에는 합류한다.
  lookup 내부의 TTL/unknown-`kid` refresh만 cooldown을 적용한다.
- 기본 허용 알고리즘은 `RS256`, `RS384`, `RS512`, `PS256`, `PS384`, `PS512`,
  `ES256`, `ES384`, `ES512`, `EdDSA`다. `WithAllowedAlgorithms`가 지정되면
  그 집합으로 좁힌다.
- `WithCacheTTL`의 기본값은 5분이며 `ttl <= 0`, nil `WithHTTPClient`, 빈
  `WithAllowedAlgorithms()`와 중복/알 수 없는 알고리즘은
  `jwt.ErrInvalidOptions`로 거부한다. 만료 경계는
  `now >= fetchedAt+TTL`이고, clock은 production에서 `time.Now`를 사용하며
  테스트에서는 package-internal clock seam으로 deterministic하게 고정한다.
- `WithRefreshCooldown`의 기본값은 1초이며 `cooldown <= 0`은
  `jwt.ErrInvalidOptions`로 거부한다. 이는 unknown `kid` forced refresh와
  실패 refresh의 재시도 폭주를 제한하며, caller가 더 짧은 값을 선택할 수
  있도록 양의 custom 값을 허용한다.
- direct `Lookup`, `Refresh`, `KeyFunc`는 nil context를
  `jwt.ErrInvalidOptions`로 거부하고, nil receiver는
  `jwt.ErrInvalidOptions`를 감싼 오류를 반환한다. 허용되지 않은 알고리즘과
  빈/비문자열/128-byte 초과/non-printable ASCII `kid`는 원격 I/O 전에
  거부하며 JWK와 inbound token header에 동일한 정책을 적용한다.

### Key policy

- JWK `kty`가 RSA/EC/OKP(Ed25519)인 공개키만 허용한다.
- `oct`, private key material, 알 수 없는 `kty`는 거부한다.
- JWK `use`가 비어 있거나 `sig`인 경우만 허용한다. 다른 `use`는 거부한다.
- JWK `alg`가 비어 있으면 token의 허용 알고리즘으로 검증한다. 값이 있으면
  token `alg`와 정확히 일치해야 한다.
- RSA는 최소 2048-bit를 요구하고 public exponent는 3 이상인 odd,
  representable 값이어야 한다. ECDSA는 `ES256`/`ES384`/`ES512`와 각각
  P-256/P-384/P-521 curve가 일치해야 한다. EdDSA는 Ed25519만 허용한다.
- `kid`는 비어 있지 않은 최대 128-byte printable ASCII여야 하며 snapshot
  안에서 중복될 수 없다. 중복은 임의 key 선택을 막기 위해 전체 snapshot
  오류로 처리한다.
- 비어 있는 `keys` 배열은 검증에 사용할 snapshot이 없으므로 malformed set으로
  처리한다. JWK의 `x5u`/`x5c`/certificate metadata는 embedded public key가
  유효하면 무시하고 원격으로 추가 fetch하지 않는다. `go-jose`가 embedded
  `x5c`와 JWK public key 불일치/잘못된 chain을 거부하는 결과는
  `ErrMalformedSet`으로 매핑한다.
- JWK에 `key_ops`가 있으면 `verify`만 허용하며, `sign`, unknown operation,
  conflicting operation은 `ErrMalformedSet`으로 거부한다. go-jose decode
  전에 Ed25519 JWK의 raw base64url `x`가 정확히 32-byte인지 직접 검증한다.
- JWKS는 최대 256개 key만 허용한다. 초과 set은 bounded resource 오류로
  거부하며, symmetric `oct`와 private material은 어떤 allowlist에서도
  허용하지 않는다.
- HMAC(`HS*`)와 기타 대칭 서명 알고리즘은 명시적으로 거부한다.

### Fetch와 오류 계약

- HTTP 요청은 `http.NewRequestWithContext`로 생성하며 caller context를
  취소/마감 전파의 기준으로 삼는다.
- 기본 fetch timeout은 10초이며 caller deadline이 더 짧으면 caller deadline을
  따른다. `WithFetchTimeout`으로 양의 timeout을 지정할 수 있다.
- 기본 response body 제한은 1 MiB이며 `WithMaxBodySize`로 양의 제한을
  지정할 수 있지만 hard cap 8 MiB를 넘을 수 없다. `Content-Length`를
  early reject하고 `maxBodySize+1` overflow를 방어한다. 성공 status는
  `200 OK`로 제한하며, 그 밖의 status 또는 body 제한 초과는 snapshot을
  교체하지 않는다.
- HTTP status, transport error, context cancellation/deadline, JSON/JWK
  validation error는 제한된 typed error로 매핑한다. 오류에는 endpoint URL,
  token, response body를 포함하지 않는다. response body는 반드시 close하며,
  redirect 거부와 body-limit 초과도 같은 redacted fetch error 경계로 매핑한다.
- package-local sentinel은 `ErrFetch`, `ErrMalformedSet`,
  `ErrUnsupportedAlgorithm`으로 고정한다. `FetchError`는 redacted `Class`,
  HTTP status, sanitized cause만 보존하고 `errors.Is`가 `ErrFetch`와 원인
  context error를 모두 보고한다. `SetError`는 malformed JSON/key policy를
  감싸며 `ErrMalformedSet`와 `jwt.ErrInvalidKey`를 함께 보고한다.
  algorithm/key mismatch와 대칭 알고리즘은 `ErrUnsupportedAlgorithm` 및
  `jwt.ErrInvalidKey`, option 오류는 `jwt.ErrInvalidOptions`, 최종 miss는
  `jwt.ErrKeyNotFound`로 분류한다. README error matrix는 각 분류의 retry
  가능 여부와 `errors.Is`/`errors.As` 사용 예를 고정한다.
- package는 호출 편의를 위해 root sentinel의 `ErrInvalidOptions`,
  `ErrInvalidKey`, `ErrKeyNotFound` alias도 노출한다. unknown `kid`는 현재
  snapshot을 확인한 뒤 generation/cooldown당 최대 한 번 강제 refresh한다.
  refresh 뒤에도 없으면 `ErrKeyNotFound`다.

### Cache와 동시성

- provider 인스턴스마다 하나의 immutable key snapshot과 `fetchedAt`/generation을
  보유한다. package-global 상태는 없다. `snapshot`, `fetchedAt`, generation은
  하나의 publication record로 원자적으로 교체한다.
- snapshot이 TTL 안이면 cache hit이며 HTTP 요청을 만들지 않는다. TTL이
  만료되면 첫 호출이 refresh하고 같은 시점의 동시 호출은 하나의 refresh를
  공유한다.
- refresh는 `sync.Mutex`와 wait channel 기반 single-flight로 구현한다. HTTP
  request, body read, JSON/JWK decode는 mutex 밖에서 수행하고 mutex는 flight
  상태와 snapshot publication에만 사용한다. 대기 중인 호출의 context가 먼저
  취소되면 다른 호출을 방해하지 않고 즉시 취소 오류를 반환한다. leader
  context가 취소되면 live waiter 하나가 자신의 context로 최대 한 번 takeover하고,
  나머지는 같은 flight 결과를 받는다. caller-owned context cancellation/deadline은
  failure cooldown에 기록하지 않으며, waiter가 없으면 flight를 정리해 다음
  호출이 즉시 재시도한다. 각 flight는 고유 identity/generation을 가지며 현재
  owner만 publication과 completion을 수행하고 takeover 전 leader의 늦은 결과는
  무시한다.
- 성공한 refresh만 snapshot을 원자적으로 교체한다. 실패한 refresh는 이전
  snapshot을 변경하지 않으며, TTL 만료 후에는 stale snapshot을 자동으로
  검증에 사용하지 않는다.
- forced refresh는 현재 generation과 `forcedAt`를 함께 비교해 concurrent 또는
  `WithRefreshCooldown` 구간의 서로 다른 unknown `kid` 요청도 하나의 flight로
  합친다. 실패와 성공 모두 cooldown 동안 같은 generation에서 재-fetch하지
  않으며, cooldown 뒤에는 새 요청이 다시 시도한다. TTL 만료 refresh 실패도
  동일한 cooldown으로 제한하되 stale snapshot은 반환하지 않는다.
- snapshot에 저장한 key material은 lookup 반환 시 공개키 타입별 defensive
  copy를 만들어 caller가 내부 cache를 mutate할 수 없게 한다.
- key rotation은 원격 set의 새 snapshot으로 반영된다. unknown `kid` 강제
  refresh가 새 key를 발견하면 다음 lookup부터 새 key를 사용한다.

### 운영 복구 계약

- library는 retry/backoff, endpoint failover, logger, metric, background
  refresh를 소유하지 않는다. caller는 `Refresh`를 bounded timeout으로
  명시적으로 호출하고, `errors.Is(err, ErrFetch)`/`ErrMalformedSet`와
  `errors.As(err, *FetchError)`를 low-cardinality event/metric으로 기록한다.
  권장 event field는 `operation`, bounded `FetchClass`, `outcome`, bounded
  HTTP status이며 endpoint URL, token, body, JWK material, raw transport error,
  high-cardinality `kid`는 기록하지 않는다.
- README runbook의 기본 운영 예시는 첫 refresh 실패를 warning으로 기록하고,
  연속 3회 실패 또는 5분 이상 refresh 실패 중 먼저 도달한 경우 page한다.
  성공한 refresh는 실패 counter를 reset한다. 이는 library 내부 threshold가
  아니라 caller가 채택할 수 있는 service 운영 기본값이다.
- 복구 순서는 endpoint health/allowlist 확인 → 새 bounded `Refresh` →
  알려진 `kid`와 허용 알고리즘으로 signature 검증 → traffic 재개다. TTL
  만료 뒤에는 stale snapshot을 허용하지 않으므로, refresh가 성공하기 전
  인증 요청은 fail closed한다.
- endpoint 변경이나 장애 rollback은 caller configuration의 이전 endpoint를
  복원한 뒤 새 provider를 만들고 readiness `Refresh`를 통과시켜야 한다.
  library는 endpoint failover를 자동화하지 않으며, mixed-version rollout은
  endpoint owner가 overlap key와 consumer refresh 완료를 보장해야 한다.

## 테스트 설계

`httptest.Server`를 사용해 네트워크 경계를 고정하고, package-global state나
실제 외부 JWKS endpoint에 의존하지 않는다.

1. constructor option, HTTPS/loopback-HTTP endpoint, private-address/redirect,
   body limit hard cap, content-length overflow, timeout validation
2. 첫 miss/fetch와 동일 key의 cache hit
3. TTL 만료 refresh와 rotated `kid` 발견
4. unknown `kid`의 동시/서로 다른 miss가 generation/cooldown당 한 번만
   refresh하고 최종 `ErrKeyNotFound`를 반환하는 경로
5. non-200, malformed JSON, invalid/duplicate/over-limit key, unsupported
   algorithm, wrong `use`/`alg`/`key_ops`, symmetric key rejection, RSA
   exponent, Ed25519 raw `x` length, printable 128-byte JWK and inbound `kid`
   boundary
6. caller context cancellation/deadline과 fetch timeout
7. concurrent lookup의 single refresh와 cache read safety, caller cancellation
   후 cooldown 제외, takeover 전 leader의 늦은 결과 무시, blocked refresh 중
   warm lookup 비차단
8. `KeyFunc`가 `golang-jwt/v5` parser에서 RSA/ECDSA/EdDSA 서명을 검증하고
   허용되지 않은 `alg`를 거부하는 경로, claims validation option을 명시하지
   않으면 signature-only 경계임을 보여주는 README example
9. `go test -race ./jwt/jwks`로 race 검증
10. bounded `go test -run '^$' -bench . -benchmem ./jwt/jwks`로 cache hit,
    concurrent miss, refresh round-trip의 baseline을 남기고 lock/HTTP 호출
    수가 명시한 수치와 일치하는지 확인한다. Step 2-R/검증 artifact에는
    `go version`, `go env GOOS GOARCH`, CPU, raw benchmark output과 비교한
    HTTP/lock count를 보존한다.

## 문서와 호환성

- `jwt/jwks/README.md`와 `jwt/jwks/README.ko.md`를 source-equivalent로
  추가하고 import, fetch/cache, algorithm policy, JWE 제외를 설명한다.
- 두 README에는 `New`가 network-free이고 startup `Refresh`가 readiness
  gate라는 점, claims validation 경계와 안전한 `ParseWithClaims` 예제,
  error matrix/재시도 가능성, 3회/5분 page 기본값, endpoint rollback 및
  mixed-version runbook을 source-equivalent로 포함한다. endpoint는 직접
  JWKS JSON URL만 받으며 OIDC discovery와 JWE는 지원하지 않는다고 명시한다.
- root `jwt/README.md`와 `jwt/README.ko.md`의 Deferred 표에서 JWKS를
  optional `jwt/jwks` provider로 갱신하고 JWE는 계속 Deferred로 남긴다.
- 운영 관측(요청 수, refresh 실패율, 마지막 성공 시각, endpoint health)은
  library가 global logger/metric을 만들지 않고 caller가 typed error와 명시적
  `Refresh` 결과를 계측한다. endpoint 소유자는 key rotation, outage, rollback
  시 stale snapshot을 허용하지 않는 현재 정책을 runbook에 반영한다.
- 새 public type/function의 Go doc comment는 한국어로 작성한다.
- `go.mod`/`go.sum` 변경은 `go-jose/v4 v4.1.4`의 license, module checksum,
  `go list -m -json` 버전 근거를 기록하고, 구현 PR은 `CHANGELOG.md`
  `[Unreleased]`에 optional JWKS provider와 JWE deferred 경계를 한 줄로
  남긴다. downstream caller는 `go mod tidy && go test ./...`로 영향 없음을
  확인한다.
- API는 새 하위 package에만 추가되므로 기존 `jwt`, `jwt/mongo`, `jwt/redis`
  caller의 compile/runtime behavior를 바꾸지 않는다.

## 수용 기준

- `jwt/jwks`가 지정한 key policy와 context-aware fetch/cache 계약을 모두
  만족한다.
- cache hit/miss/rotation/error와 동시 lookup race 테스트가 통과한다.
- root와 하위 README가 source-equivalent로 갱신된다.
- 운영 runbook, error matrix, startup/readiness, rollback/mixed-version,
  claims validation boundary가 두 README에 source-equivalent로 기록된다.
- `go test ./...`, `go test -race ./jwt/jwks`, `make fmt-check`, `make tidy-check`,
  `make vet`, `make lint`, `go mod tidy`가 통과한다. benchmark raw output과
  dependency/version evidence가 Step 2-R/검증 artifact에 보존된다.
- Step 2-R의 Performance, Stability, Security, Operator/Ops, Developer/API,
  User/Caller 독립 리뷰와 main integration review에서 P0/P1이 0이다.

## SPW-02~05 기록

- SPW-02 구조: 문제, 접근안, API, key policy, fetch/cache, 테스트, 문서,
  수용 기준을 의존성 순서로 구성했다.
- SPW-03 한국어 품질: 독자-facing 설명은 한국어로 작성하고 Go 식별자,
  명령, URL, 오류 sentinel은 원문 token을 보존한다.
- SPW-04 추적성: Issue #545, #497, root `jwt` API, `go-jose/v4` 및 `jwx/v3`
  공식 API 문서와 각 수용 기준을 연결했다.
- SPW-05 read-back: 파일 저장 후 제목, 이슈 번호, package 경계, 알고리즘,
  비목표, 테스트 명령, README 요구사항을 다시 대조하고 2-R 검토 결과에
  반영한다.
