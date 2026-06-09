# jwt

[English](README.md) | [한국어](README.ko.md)

`jwt`는 명시적 algorithm과 repo-owned error를 사용하는 JSON Web Token 생성,
검증, claim 읽기, local key rotation helper를 제공합니다.

## Import

```go
import "github.com/bluetape4k/bluetape-go/jwt"
```

## 선택 가이드

| 필요 | 사용 | 메모 |
|---|---|---|
| 고정 symmetric signing key | `NewFixedHMACProvider` | HS256은 최소 32-byte secret, HS384는 48-byte, HS512는 64-byte가 필요합니다. |
| 고정 asymmetric signing key | `NewFixedRSAProvider` | 검증된 2048-bit 이상 RSA private key로 RS256/384/512, PS256/384/512를 지원합니다. |
| local in-memory key rotation | `NewHMACProvider` 또는 `NewRSAProvider` | in-memory KeyChain repository, `kid` header, TTL, retained key를 사용합니다. |
| distributed key repository | Deferred | context-aware Redis/Mongo 등 repository는 #173에서 추적합니다. |
| JOSE compression, JWE, JWK, JWKS | Deferred | 안전한 dependency 범위와 compression 동작은 #174에서 추적합니다. |
| external provider cache adapter | Deferred | optional cache-backed provider adapter는 #175에서 추적합니다. |

## 사용법

```go
provider, err := jwt.NewFixedHMACProvider(
    jwt.HS256,
    []byte("0123456789abcdef0123456789abcdef"),
)
if err != nil {
    return err
}

token, err := provider.Compose(
    jwt.WithSubject("account-42"),
    jwt.WithAudience("api"),
    jwt.WithExpiresAfter(time.Hour),
    jwt.WithClaim("role", "admin"),
)
if err != nil {
    return err
}

reader, err := provider.Parse(
    token,
    jwt.WithExpectedSubject("account-42"),
    jwt.WithExpectedAudience("api"),
    jwt.WithExpirationRequired(),
)
if err != nil {
    return err
}
role, ok := reader.ClaimString("role")
```

## 동작

- `jwt`는 auth framework가 아닙니다. HTTP middleware, session, OIDC, JWKS,
  authorization rule, user model을 제공하지 않습니다.
- Parse는 항상 `WithValidMethods`로 허용 algorithm을 제한하고, token의 `alg`
  header가 provider algorithm과 일치해야 verification key material을 반환합니다.
- Reader API는 검증된 header와 claim만 노출하며 raw bearer token은 노출하지
  않습니다.
- Fixed provider는 exactly one fixed key일 때만 inbound missing `kid`를
  허용합니다. Rotating provider는 lookup을 위해 `kid`가 필요합니다.
- In-memory rotation은 `kid`별 retained key를 저장하고 repository capacity를
  넘는 오래된 key를 evict합니다. Retained key는 key가 만료되거나 evict되기 전까지
  old token을 검증할 수 있습니다.
- HMAC fixed secret은 선택한 hash size 이상이어야 합니다. 약한 secret은
  `ErrInvalidKey`를 반환합니다.
- RSA provider constructor는 signing을 위한 검증된 2048-bit 이상 private key를
  요구합니다. Provider는 내부 복사본을 저장하고 verification에는 public key
  material을 사용하므로, 생성 후 caller가 원본 key를 mutate해도 provider 동작은
  바뀌지 않습니다.
- Error는 `errors.Is`가 동작하도록 `ErrInvalidToken`, `ErrExpiredToken`,
  `ErrNotYetValid`, `ErrInvalidKey`, `ErrKeyNotFound` 같은 repo-owned sentinel을
  감쌉니다. Error string에는 raw token, HMAC secret, private key를 포함하지
  않습니다.
- Unsupported JOSE/compression header인 `zip`, `crit`, `jku`, `jwk`, `x5u`,
  `x5c`를 포함한 inbound signed token은 #174에서 안전한 동작을 정하기 전까지
  거부합니다.
- 현재 repository는 process-local 전용입니다. Future context-aware distributed
  key storage는 #173에서 진행합니다.

## Rotation 계약

`Rotate`는 만료되지 않은 current key가 있으면 그대로 반환하고 만료된 경우에만 새
key를 생성합니다. `ForcedRotate`는 rotating provider에서 항상 새 key를 만듭니다.
Fixed provider는 rotate하지 않습니다.

Key generation은 설정된 entropy가 있으면 그것을 사용하고, 없으면 `crypto/rand`를
사용합니다. 하나의 provider를 여러 goroutine에서 공유할 때 custom entropy reader,
clock, key ID generator는 caller가 concurrent use safety를 보장해야 합니다.

## Test

```bash
go test -count=1 ./jwt
go test -race -count=1 ./jwt
```
