# Issue #538 Secrets Manager 및 SSM provider lesson

## 결정

AWS secret 조회 helper는 credential/config/retry/lifecycle를 생성하지 않고
caller-owned SDK method subset만 받아야 한다. SecretString, SecretBinary와
SSM SecureString은 서로 다른 response 계약이므로 `secretsmanager`와 `ssm`을
분리하고, 원문은 immutable `Value`의 명시적 `Bytes`/`Text` 경로로만 전달한다.

positive TTL cache가 필요할 때는 기존 `cache.LoadingCache` 계약을 재사용한다.
성공 결과만 저장하고 오류, cancellation, stale value는 저장하지 않으며, SSM
plain/decrypted 조회는 cache key namespace를 분리해야 한다.

## 검증에서 확인한 hardening

- SDK response 직후 `ctx.Err()`를 다시 검사해 늦은 성공이 caller cancellation을
  가리지 않게 했다.
- `String`/`GoString`/`%+v`와 typed `Error`는 raw secret, parameter name,
  provider payload를 출력하지 않는다. 원인은 `Unwrap`으로만 보존한다.
- `SecretString`과 `SecretBinary`의 exact-one 규칙, non-nil empty binary,
  SSM의 nil `Parameter`/`Value`를 fake-first test로 고정했다.
- `Value.Bytes()`와 provider/cache 경계에서 defensive copy를 사용해 caller
  buffer mutation이 cache에 저장된 값을 바꾸지 않게 했다.
- positive TTL concurrent load는 기존 Memory cache의 single-flight에 맡기고,
  provider가 global cache나 refresh goroutine을 만들지 않는다.
- GoDoc exported identifier 규칙을 `golangci-lint`로 확인하려면 한국어 조사와
  identifier를 공백 또는 `-`로 분리해야 한다. 최종 lint 결과는 `0 issues`다.

## 다음 변경자를 위한 지침

- AWS credential chain, IAM policy, KMS envelope, environment precedence와
  rotation/refresh를 provider 내부로 끌어들이지 않는다.
- caller-owned custom cache가 오류 또는 stale 값을 저장하지 않는지 구현 계약을
  유지하고, provider 오류 문자열에 lookup key를 추가하지 않는다.
- 새로운 secret source를 추가할 때도 response shape, decryption mode와 redaction
  규칙을 별도 package/API contract로 먼저 고정한다.
- `make tidy-check`는 dependency 변경을 commit 후 clean tree에서 다시 실행하고,
  live AWS smoke가 없다는 사실을 CI 성공으로 과장하지 않는다.

## 확인된 범위

`go test -count=1 ./...`, 신규 package normal/race/example, `go vet`,
`golangci-lint`, `make fmt-check`, `make vet`, `make lint`와 Korean terminology
audit가 통과했다. AWS credential/network와 remote CI는 이 lesson의 범위 밖이다.
