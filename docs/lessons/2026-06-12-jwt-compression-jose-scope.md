# JWT Compression과 JOSE Scope 교훈

Issue #174는 JWT compression 문제를 compression-library 선택이 아니라 standards
boundary로 정리했다.

## 교훈

- signed JWT/JWS helper에 `zip` support를 추가하지 않는다. RFC `zip=DEF`
  compression은 JWE의 plaintext-before-encryption 영역이다.
- 좁은 signed JWT parsing, validation, signing에는 `golang-jwt/jwt/v5`를 유지한다.
  실제 JWE use case 없이 default `jwt` helper에 full JOSE dependency를 끌어오지
  않는다.
- compressed JWT support가 필요해지면 별도 JWE API boundary를 만들고 patched
  release에 고정한 `go-jose/go-jose/v4`를 우선한다.
- JWE는 새로운 security surface로 다룬다. code 전에 decompression size limit,
  expansion ratio limit, compact-token segment validation, `crit` handling,
  remote key header policy, PBES2 `p2c` bound가 acceptance에 포함되어야 한다.
- `jwx/v4`는 여전히 강한 JOSE library지만, `GOEXPERIMENT=jsonv2` 요구사항과 넓은
  API surface 때문에 작은 helper package의 default dependency로는 부적합하다.

## 증거

- `docs/superpowers/research/2026-06-12-issue-174-jwt-compression-jose-scope.md`
- `docs/superpowers/reviews/2026-06-12-issue-174-jwt-compression-jose-research-review.md`
- `jwt/README.md`
- `jwt/README.ko.md`
