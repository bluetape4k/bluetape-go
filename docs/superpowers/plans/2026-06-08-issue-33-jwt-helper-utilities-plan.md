# Issue 33 JWT Helper Utilities Plan

## 분류

- 작업 유형: Type A - Full Feature.
- 근거: JWT helper package, signing/parsing API, security-sensitive tests, docs/examples를 포함한다.
- 범위: local JWT provider와 keychain helper를 first-party로 구현한다.

## 목표

`github.com/golang-jwt/jwt/v5` 위에 bluetape-go가 사용할 작은 helper layer를 제공한다. 공개 API는 raw key material 노출을 최소화하고, parsing hardening과 key rotation boundary를 명확히 한다.

## 순서

1. #33 research와 JWT library behavior를 확인한다.
2. provider options, claims handling, keychain lifecycle, parser hardening을 spec에 고정한다.
3. signing, parsing, expiration, invalid algorithm, rotation tests를 먼저 작성한다.
4. `jwt` package provider와 private repository를 구현한다.
5. examples와 README locale pair에 safe usage와 caveats를 기록한다.
6. #173 distributed repository가 확장할 package-private boundary를 확인한다.

## 리뷰 게이트

- public API가 raw HMAC/RSA private key material을 불필요하게 노출하지 않는지 확인한다.
- algorithm confusion 방어가 테스트되는지 확인한다.
- time/clock injection으로 expiration tests가 deterministic한지 확인한다.
- errors가 sensitive token/key 값을 노출하지 않는지 확인한다.
- distributed JWT 후속 작업과 package boundary가 충돌하지 않는지 확인한다.

## 검증 게이트

- `go test -count=1 ./jwt/...`
- `go test -race -count=1 ./jwt/...`
- `go test -count=1 ./...`
- `go vet ./...`
- `make fmt-check`
- `git diff --check`
