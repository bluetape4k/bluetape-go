# Issue #174 JWT Compression and JOSE Scope Research

Issue: #174
Milestone: 0.6.1
Date: 2026-06-12

## Research Question

`bluetape-go`에는 이미 `github.com/golang-jwt/jwt/v5` 기반의 작은 signed JWT helper
package가 있다. 열린 질문은 Kotlin JWT stack과 source parity를 맞추기 위해 지금 JWT
compression을 추가해야 하는지, 그리고 compression이 필요해질 때 어떤 JOSE dependency boundary가
안전한지다.

## 결정

현재 signed JWT/JWS helper에는 compression을 추가하지 않는다.

현재 `jwt` package의 유일한 runtime dependency는 `github.com/golang-jwt/jwt/v5`로 유지한다.
`zip`, `crit`, `jku`, `jwk`, `x5u`, `x5c` 같은 unsupported JOSE/compression header를 쓰려는
inbound/outbound signed JWT는 계속 거절한다.

나중에 interoperable compression이 필요해지면 별도 explicit JWE boundary로 구현한다. 그 boundary의
preferred future dependency는 `v4.1.4` 이상으로 pin한 `github.com/go-jose/go-jose/v4`다.

#174는 signed JWT compression을 거절하므로 implementation follow-up을 만들지 않는다. encrypted
JWT 또는 compressed JWE payload가 필요한 실제 product use case가 생길 때만 future JWE issue를 만든다.

## Primary Evidence

| Source | Evidence | Decision impact |
|---|---|---|
| RFC 7515, JSON Web Signature | JWS는 payload를 digital signature 또는 MAC으로 보호한다. registered header set은 `alg`, `jku`, `jwk`, `kid`, `x5u`, `x5c`, `typ`, `cty`, `crit` 같은 signature/key selection header를 다루며 `zip`은 JWS compression mechanism이 아니다. | `zip`을 standard signed JWT/JWS option으로 취급하지 않는다. |
| RFC 7516, JSON Web Encryption | `zip`은 encryption 전 plaintext에 적용되는 compression algorithm이고, specification은 `DEF` 값을 정의한다. header는 JWE Protected Header 안에서 integrity-protected여야 한다. | compression은 현재 signed JWT helper가 아니라 JWE boundary에 속한다. |
| RFC 7518, JSON Web Algorithms | JOSE compression registry는 JWE `zip` member value를 위한 것이다. PBES2 JWE key management는 `p2c`와 `p2s`를 사용하므로 denial-of-service와 policy-limit 고려가 필요하다. | future JWE feature에는 explicit algorithm 및 resource-limit policy가 필요하다. |
| `github.com/golang-jwt/jwt/v5` | JWT parsing, verification, generation, signing, signing-method extensibility를 지원한다. 현재 repo dependency는 `v5.3.1`이다. | 현재 narrow signed JWT helper에 가장 잘 맞는다. |
| `github.com/go-jose/go-jose/v4` | JWE, JWS, JWT standard, compact/JSON serialization, `DEF` compression support를 구현한다. checked latest release: `v4.1.4`, 2026-04-04. | JWE가 필요해질 때 가장 적합한 optional JOSE/JWE boundary다. |
| `github.com/lestrrat-go/jwx/v4` | broad JOSE suite이며 active maintenance가 있다. latest release `v4.0.2` on 2026-05-07, 하지만 현재 Go 1.26 plus `GOEXPERIMENT=jsonv2`를 요구한다. | default helper dependency로는 너무 넓고 운영 부담이 크다. |
| Current `jwt` package | `jwt/composer.go`는 `zip`, `crit`, `jku`, `jwk`, `x5u`, `x5c`를 reserve하고, `jwt/reader.go`는 해당 unsupported header를 포함한 inbound token을 거절한다. | 기존 fail-closed behavior가 맞으며 #174 결과로 문서화해야 한다. |

## Candidate Comparison

| Candidate | Current version checked | Scope | License | Fit |
|---|---:|---|---|---|
| `github.com/golang-jwt/jwt/v5` | `v5.3.1` | Signed JWT parsing, validation, generation, signing | MIT | current signed JWT helper에 채택한다. |
| `github.com/go-jose/go-jose/v4` | `v4.1.4` | JWE, JWS, JWT, compact/JSON serialization, `DEF` JWE compression | Apache-2.0 | future optional JWE boundary에 선호한다. |
| `github.com/lestrrat-go/jwx/v4` | `v4.0.2` | JWT/JWS/JWE/JWK와 richer policy surface를 가진 full JOSE suite | MIT | current default dependency로 거절한다. broad JOSE stack을 의도적으로 채택할 때만 재검토한다. |
| First-party adapter around `compression` | N/A | Custom signed-JWT payload compression | Project license | interoperability와 security 때문에 거절한다. non-standard signed JWT shape가 된다. |

## Security Boundary

- signed JWT parsing은 `WithValidMethods`와 exact provider algorithm matching을 유지해야 한다.
- signed JWT/JWS의 `zip`은 unsupported로 유지하고 거절한다.
- `crit`은 future feature가 token의 모든 critical extension을 명시적으로 구현하기 전까지 거절한다.
- remote key header `jku`, `jwk`, `x5u`, `x5c`는 기본적으로 거절한다. future trust model에는
  pinned issuer, TLS/server identity rule, key-source allowlist, cache invalidation policy가 필요하다.
- JWE support를 나중에 추가하더라도 `Provider.Parse` 안의 auto-detected extension이 아니라 distinct
  API path여야 한다.

## Future JWE Acceptance Shape

future issue가 `go-jose/go-jose/v4`를 채택하려면 다음을 모두 요구해야 한다.

- JWE를 위한 separate package 또는 constructor boundary. 기존 signed JWT behavior를 바꾸지 않는다.
- `github.com/go-jose/go-jose/v4`를 `v4.1.4` 이상으로 pin한다.
- JWE `alg`와 `enc` combination을 allowlist한다.
- JWE Protected Header 안의 `zip=DEF`만 허용한다.
- maximum compact token size, maximum decompressed size, maximum expansion ratio를 강제한다.
- segment count가 과도한 malformed compact token을 거절한다.
- unsupported `crit` entry를 거절한다.
- explicit pinned trust policy가 설계되고 tested되기 전까지 remote key header를 disabled로 유지한다.
- PBES2보다 service-to-service key management를 선호한다. PBES2가 활성화되면 `p2c` limit와 random
  `p2s`를 강제한다.

## Adopt / Borrow / Skip Decisions

| Decision | Rationale |
|---|---|
| current `jwt` package에는 `golang-jwt/jwt/v5`를 채택 | signed JWT 생성과 검증에 충분하고, narrow/stable/already implemented다. |
| future JWE에만 `go-jose/go-jose/v4`를 borrow | compression이 속하는 standards boundary에 맞고, signed-JWT user 전체에 JOSE-wide dependency를 강제하지 않는다. |
| default dependency로 `jwx/v4`는 skip | active하고 capable하지만 package 범위보다 넓고 현재 `GOEXPERIMENT=jsonv2`를 요구한다. |
| first-party signed-JWT compression은 skip | non-standard shape이고 interop이 약하며 JWE `zip`과 혼동되기 쉽다. |

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
| `golang-jwt/jwt/v5` evaluated | Done | signed JWT only로 유지한다. |
| `go-jose/go-jose/v4` evaluated | Done | preferred future JWE dependency지만 지금 채택하지 않는다. |
| `lestrrat-go/jwx/v4` evaluated | Done | breadth와 `GOEXPERIMENT=jsonv2` 때문에 default helper dependency로 거절한다. |
| Compression decision recorded | Done | signed JWT compression은 non-goal이며 JWE는 future-only다. |
| Security risks recorded | Done | header confusion, remote key headers, decompression limits, compact parsing DoS, PBES2 limits를 기록했다. |
| Follow-up decision | Done | #174가 signed JWT compression을 거절하므로 implementation issue를 열지 않는다. |
