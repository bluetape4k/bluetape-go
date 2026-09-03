# Issue #539 RDS IAM auth helper lesson

## 결정

RDS IAM authentication은 token signing과 database connection lifecycle을
분리해야 한다. `rds/auth`는 AWS SDK의 `BuildAuthToken`만 검증된 wrapper로
호출하고 credential/config/IAM policy, SQL driver/DSN, pool, retry와 refresh는
caller/operator에게 남긴다.

endpoint는 SDK에 맡기기 전에 scheme/path/query/fragment/userinfo/percent
escape/backslash가 없는 `host:port`인지 확인한다. DNS host는 ASCII label
(63 byte/label, 253 byte/전체)만 허용하고 IPv4와 bracketed IPv6를 지원하며
port는 `1..65535`로 제한한다. region과 username은 valid UTF-8, non-blank,
bounded 값으로만 전달한다.

## 검증에서 확인한 hardening

- SDK signing 전후에 `ctx.Err()`를 확인해 늦은 token보다 caller cancellation을
  우선한다.
- `Token`의 `String`/`GoString`/`%+v`는 항상 `[REDACTED]`이고 raw token은
  명시적 `Text`/`Bytes` handoff에서만 사용한다.
- typed-nil credentials와 malformed request를 SDK 호출 전에 거부한다.
- SDK 오류 원인은 `Unwrap`으로만 보존하고 public error에는 operation과 고정
  sentinel만 남겨 credential/provider payload를 노출하지 않는다.
- token cache, refresh goroutine, database/sql connection을 추가하지 않아
  15-minute signing lifetime과 refresh 책임이 caller에게 명확하다.
- exported GoDoc identifier와 Korean reader-facing comments를 `golangci-lint`와
  terminology audit로 확인했다. 최종 lint 결과는 `0 issues`다.

## 다음 변경자를 위한 지침

- token을 log, metric label, error 문자열, URL query 또는 공용 DSN artifact에
  저장하지 않는다. driver password field 전달 직전에만 `Text()`를 호출한다.
- RDS instance discovery, IAM policy/provisioning, credential refresh와 retry를
  helper API에 추가하지 말고 별도 caller/operator 계층에서 명시한다.
- endpoint 규칙을 완화할 때는 SDK의 URL parser가 허용하는 값과 이 helper의
  security contract를 분리해 검토하고 IPv6/port regression test를 유지한다.
- `make tidy-check`는 dependency 변경을 commit 후 clean tree에서 재실행하며,
  fake-first/compile evidence를 live AWS/RDS proof로 표현하지 않는다.

## 확인된 범위

`go test -count=1 ./...`, 신규 package normal/race/example, `go vet`,
`golangci-lint`, `make fmt-check`, `make vet`, `make lint`와 Korean terminology
audit가 통과했다. AWS credential/network, database driver connection과 remote
CI는 이 lesson의 범위 밖이다.
