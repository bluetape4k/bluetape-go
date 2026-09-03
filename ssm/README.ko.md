# ssm

[English](README.md)

`ssm`은 AWS SDK for Go v2 Parameter Store `GetParameter`만 사용하는 좁은
provider입니다. caller-owned client를 주입하고 SecureString 복호화를 명시적
option으로 제공하며, redacted `Value`와 positive TTL `cache.LoadingCache`를
선택적으로 사용합니다.

## 사용 예

```go
provider, err := ssm.New(ssm.Options{
    Client:         client, // config, credential, retry, lifecycle은 caller 소유
    WithDecryption: true,
    CacheTTL:       5 * time.Minute,
})
if err != nil {
    return err
}
value, err := provider.Get(ctx, "/prod/database/password")
if err != nil {
    return err
}
password := value.Text() // raw value를 명시적으로 전달

secure, err := provider.GetSecure(ctx, "/prod/database/password")
```

`GetSecure`는 provider 기본값과 관계없이 항상 `WithDecryption=true`를 전송합니다.
`Value.String()`과 `Value.GoString()`은 `[REDACTED]`를 반환하므로 raw 값이
필요할 때만 `Text()` 또는 `Bytes()`를 호출하십시오. zero value는 unset이고,
빈 parameter도 set 상태로 유지됩니다.

## 계약

- 주입하는 client에는 `GetParameter`만 필요합니다. Provider가 AWS client를
  생성, 종료, 재구성하거나 retry하지 않습니다.
- `context.Context`를 dispatch 전, SDK response 후, 결과 반환 직전에 검사합니다.
  늦게 도착한 성공 response보다 caller cancellation이 우선합니다.
- `Get`은 `Options.WithDecryption`을 사용하고 `GetSecure`는 복호화를 강제합니다.
  Plain과 secure lookup은 서로 다른 cache key를 사용하므로 결과가 충돌하지 않습니다.
- `CacheTTL == 0`이면 cache를 사용하지 않습니다. Positive TTL에서는 전달받은
  `cache.LoadingCache`를 사용하고, 없으면 process-local cache를 사용합니다.
  성공값만 저장하며 오류, cancellation, stale 값은 cache에 저장하지 않습니다.
- 이름은 valid UTF-8, non-blank, 최대 2048 byte여야 합니다. caller 문자열은
  trim 또는 normalization 없이 AWS request에 그대로 전달합니다.

## 소유권과 보안

Credential, AWS config/region, endpoint, retry/backoff, timeout, client lifecycle,
parameter precedence, rotation과 cache invalidation은 caller가 소유합니다.
`Value`, raw bytes, parameter identifier와 unwrap한 provider error를 로그에
남기지 마십시오. 이 package는 configuration framework, IAM provisioning과
live AWS smoke test를 구현하지 않습니다.

`Error()`와 `%+v`에는 package sentinel과 고정 operation label만 나타나며,
원인은 `errors.Is`/`errors.As`로만 확인할 수 있습니다.

## 검증

```bash
go test -count=1 ./ssm
go test -race -count=1 ./ssm
go test -run '^Example' -count=1 ./ssm
```

Test는 deterministic fake client만 사용하므로 AWS credential이나 network가
필요하지 않습니다.
