# Issue #545 JWKS provider lesson

## 결정

`jwt/jwks`는 `go-jose/v4`를 JWK decode/conversion 경계에만 사용하고, fetch,
TTL, rotation, cooldown, single-flight와 caller context 수명은 provider가
소유한다. RSA/ECDSA/EdDSA 공개키만 허용하며 HMAC과 JWE는 별도 범위로 남겼다.

## 실제 복구와 검증

- 초기 unknown-`kid` 경로는 cold fetch 성공 뒤에도 다음 miss에서 즉시 forced
  fetch를 반복했다. 모든 성공 snapshot에 cooldown anchor를 기록해 같은
  generation의 burst를 한 번으로 합쳤다.
- leader context가 취소된 뒤 delayed transport가 결과를 반환할 수 있으므로
  flight identity와 현재 owner 검사를 유지했다. takeover 이후 늦은 결과는
  publication을 덮어쓰지 않는다.
- `go-jose`가 제공한 ECDSA raw coordinate를 직접 mutate하는 코드는 Go 1.26
  deprecation 경고를 만들었다. `PublicKey.Bytes`와
  `ParseUncompressedPublicKey`로 validation/defensive copy를 고정했다.
- 신규 public package의 Korean Go doc은 exported identifier 뒤 공백으로
  시작해야 `revive` 규칙을 통과한다. `make lint`는 최종 0 issues다.
- `make ci`는 커밋 전 `tidy-check`가 의도한 direct dependency 정규화 diff를
  발견해 중단했다. 구현 commit 뒤 clean tree에서 동일 gate를 재실행해야 한다.

## 다음 변경자를 위한 지침

- `KeyFunc`는 signature key lookup만 담당한다. issuer, audience, expiration,
  `nbf`와 authorization policy를 provider 안으로 끌어들이지 않는다.
- caller가 주입한 HTTP client는 TLS/proxy/DNS/redirect trust boundary를
  완화할 수 있으므로 README와 운영 event에서 caller-owned임을 유지한다.
- TTL 만료 refresh 실패에서 stale snapshot을 반환하지 않는다. 운영 복구는
  bounded `Refresh`와 known `kid` readiness 검증 뒤 traffic을 재개한다.
- cached key material과 `kid`를 log/metric label에 직접 넣지 않는다. event에는
  operation, bounded fetch class, outcome, bounded status만 남긴다.

- custom `RoundTripper`는 `Request.Context()` 취소를 request와 response body
  수명 전체에 전파해야 한다. 비협조 transport의 orphan worker는 provider가
  강제로 회수할 수 없으므로 caller-owned trust boundary로 문서화하고,
  context-aware transport에서는 반복 취소 뒤 active worker가 0으로 돌아오는
  회귀 테스트를 유지한다.
- `http.DefaultTransport`를 복제할 때도 전역 `DialTLSContext`, `DialTLS`,
  `TLSClientConfig`, `TLSNextProto`, `Proxy`를 상속하지 않아야 한다. 기본
  client의 SSRF와 TLS 경계는 프로세스 전역 mutable transport 설정과
  독립적으로 유지한다.
