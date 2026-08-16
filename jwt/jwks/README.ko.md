# jwt/jwks

`jwt/jwks`는 `github.com/golang-jwt/jwt/v5` 서명 검증을 위한 선택적이고
caller-owned인 JWKS provider입니다.

RSA, ECDSA, Ed25519 공개키를 허용합니다. 대칭 `oct` key, JWE, OIDC discovery,
package-global cache, background refresh는 의도적으로 범위에서 제외합니다.

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

## 빠른 시작

`New`는 network-free constructor입니다. readiness 단계에서 bounded
`Refresh`를 명시적으로 실행한 뒤 request-scoped `KeyFunc`를 만들고 claims
정책은 JWT parser에 둡니다.

```go
func verifyJWKS(req *http.Request, signedToken string) error {
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
    _, err = parser.ParseWithClaims(signedToken, claims, keyFunc)
    return err
}
```

Provider는 key lookup과 signature algorithm matching만 담당합니다. issuer,
audience, subject, `exp`, `nbf`와 다른 claims는 parser option으로 명시해야
합니다. 이 option을 생략하면 signature 경계만 구성됩니다.

각 request context마다 새 `KeyFunc`를 만드세요. 취소된 context를 캡처한
closure를 재사용하지 마세요.

## Endpoint와 trust boundary

- 기본 endpoint scheme은 HTTPS입니다. loopback HTTP는 test와 local development에만
  허용합니다.
- endpoint에는 host가 있어야 하며 userinfo와 fragment를 포함할 수 없습니다.
- 기본 client는 private, link-local, unspecified 및 다른 non-global dial target을
  거부하고 환경 proxy를 사용하지 않으며 response header를 64 KiB로 제한하고
  redirect를 따라가지 않습니다. HTTP endpoint는 loopback IP literal만 허용합니다.
- `WithHTTPClient`를 사용하면 TLS 검증, proxy, DNS/dial, redirect, allowlist 정책을
  caller가 책임집니다. caller가 의도하고 문서화한 경우가 아니면
  `InsecureSkipVerify`를 사용하지 마세요. custom `RoundTripper`는 request와
  response body 수명 전체에서 `Request.Context()` 취소를 준수해야 합니다. 취소를
  무시하는 transport는 takeover 중에도 이전 refresh 작업을 남길 수 있습니다.
- endpoint는 직접 JWKS JSON URL이어야 합니다. OIDC discovery와 issuer metadata
  자동 조회는 수행하지 않습니다.

## Cache와 rotation

기본 cache TTL은 5분, fetch timeout은 10초, response body 제한은 1 MiB
(hard cap 8 MiB), unknown `kid` refresh cooldown은 1초입니다. warm hit는
I/O를 수행하지 않습니다. TTL 만료와 unknown `kid`는 bounded single-flight
refresh를 유발하며 concurrent caller는 하나의 request를 공유합니다. 명시적
`Refresh`는 cooldown을 우회하지만 background loop를 만들지 않습니다.

성공한 refresh만 immutable snapshot을 교체합니다. 만료 snapshot을 갱신할 수
없으면 stale key material을 반환하지 않고 fail closed 합니다. 반환하는
RSA/ECDSA 값과 Ed25519 byte slice는 defensive copy입니다.

## Key 정책

- RSA는 public key, 최소 2048-bit, representable한 3 이상 odd exponent여야 합니다.
- ECDSA는 P-256/ES256, P-384/ES384, P-521/ES512가 일치해야 합니다.
- EdDSA는 Ed25519이며 32-byte public key가 필요합니다.
- `use`는 비어 있거나 `sig`만 허용하고, `key_ops`가 있으면 정확히 `verify`여야 합니다.
- private material, `oct`, unknown key type, 비어 있거나 중복/잘못된 `kid`, 지원하지 않는
  algorithm은 거부합니다. `kid`는 최대 128 printable ASCII byte이며 set은 최대 256개 key입니다.
- `x5u`/`x5c` metadata는 추가 fetch를 만들지 않습니다. embedded public key도 동일한
  JWK 검증 정책을 통과해야 합니다.

기본 algorithm 집합은 `RS256`, `RS384`, `RS512`, `PS256`, `PS384`, `PS512`,
`ES256`, `ES384`, `ES512`, `EdDSA`입니다. `WithAllowedAlgorithms`는 이 집합을
좁히기만 하며 HMAC 또는 다른 대칭 algorithm을 활성화하지 않습니다. root
`jwt.Algorithm`을 전달할 때는 명시적으로 변환하세요.

```go
jwks.WithAllowedAlgorithms(jwks.Algorithm(rootjwt.RS256))
```

## Error와 운영

| 분류 | `errors.Is` / `errors.As` | 재시도 지침 |
|---|---|---|
| 잘못된 option | `jwt.ErrInvalidOptions` | 설정을 수정하고 같은 입력을 반복하지 않습니다. |
| 잘못되거나 안전하지 않은 set | `jwks.ErrMalformedSet`, `jwt.ErrInvalidKey`, `jwks.SetError` | 해당 payload 사용을 중단하고 endpoint owner에게 page합니다. |
| 지원하지 않는 algorithm | `jwks.ErrUnsupportedAlgorithm` | caller allowlist와 issuer contract를 맞춥니다. HMAC으로 넓히지 않습니다. |
| fetch/status/body/context | `jwks.ErrFetch`, `jwks.FetchError`; context error 보존 | bounded caller context와 service 정책 안에서만 재시도합니다. |
| unknown `kid` | `jwt.ErrKeyNotFound` | bounded rotation refresh를 허용하고 cooldown/page threshold 뒤 조사합니다. |

event field는 `operation`, bounded `FetchClass`, outcome, bounded HTTP status만
사용하세요. endpoint URL, bearer token, raw body, JWK material, raw transport
cause, high-cardinality `kid`는 log에 남기지 않습니다.

권장 runbook은 다음 표를 따릅니다.

| 단계 | 담당 | 조치 | 해제 조건 |
|---|---|---|---|
| Preflight | service/on-call owner | endpoint health, TLS, allowlist와 현재 `FetchClass`를 확인하고 이전 endpoint 설정을 보관합니다. | caller deadline이 있는 bounded `Refresh`를 실행할 수 있습니다. |
| Warning | service/on-call owner | endpoint URL, raw body, token, JWK, transport cause, high-cardinality `kid` 없이 첫 실패를 기록합니다. | 성공 refresh 뒤 failure counter가 초기화됩니다. |
| Page | service/on-call owner | 연속 3회 또는 5분 중 먼저 도달하면 page합니다. | readiness `Refresh`와 known `kid` signature 검증이 성공합니다. |
| Rollback | release owner | 이전 endpoint 설정을 복원하고 새 provider를 생성해 readiness `Refresh`와 known token 검증 뒤 traffic을 재개합니다. | 복원 provider에서 known token 검증이 성공합니다. |
| Rotation | issuer/release owner | 모든 consumer refresh 전까지 overlap key를 유지하고 이후 이전 key를 제거합니다. | 모든 consumer readiness가 통과하고 이전 key lookup이 fail closed 됩니다. |

mixed-version rotation에서는 모든 consumer가 refresh를 완료할 때까지
overlap key를 유지한 뒤 이전 key를 제거합니다.

traffic을 열기 전 readiness에서는 다음과 같이 동일한 bounded 명령을
실행하세요.

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

## Non-goal

이 package는 OIDC discovery, JWE decryption, token claims 정책, endpoint failover,
logging/metrics, retry/backoff, background refresh를 구현하지 않습니다. 이 정책은
provider를 소유한 service가 결정합니다.
