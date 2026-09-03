# secretsmanager

[English](README.md)

`secretsmanager`는 AWS SDK for Go v2 `GetSecretValue`만 사용하는 좁은
provider입니다. caller-owned client를 주입하고 string 또는 binary 결과를
redacted `Value`로 반환하며, positive TTL `cache.LoadingCache`를 선택적으로
사용합니다.

## 사용 예

```go
provider, err := secretsmanager.New(secretsmanager.Options{
    Client: client, // config, credential, retry, lifecycle은 caller 소유
    CacheTTL: 5 * time.Minute,
})
if err != nil {
    return err
}
value, err := provider.Get(ctx, "prod/database/password")
if err != nil {
    return err
}
password := value.Text() // raw value를 명시적으로 전달
```

`Value.String()`과 `Value.GoString()`은 항상 `[REDACTED]`를 반환합니다.
`SecretString`은 `Text()`, `SecretBinary`는 `Bytes()`를 사용하십시오.
`Bytes()`는 독립된 복사본을 반환합니다. zero value는 unset 상태이고, AWS가
반환한 빈 값은 set 상태로 유지되므로 `IsSet()`으로 구분할 수 있습니다.

## 계약

- 주입하는 client에는 `GetSecretValue`만 필요합니다. Provider가 AWS client를
  생성, 종료, 재구성하거나 retry하지 않습니다.
- `context.Context`를 dispatch 전, SDK response 후, 결과 반환 직전에 검사합니다.
  늦게 도착한 성공 response보다 caller cancellation이 우선합니다.
- `SecretString`과 `SecretBinary`는 동시에 존재할 수 없습니다. 값이 없거나
  malformed response이면 typed sentinel을 반환하고 cache에 저장하지 않습니다.
- `CacheTTL == 0`이면 cache를 사용하지 않습니다. Positive TTL에서는 전달받은
  `cache.LoadingCache`를 사용하고, cache가 없으면 provider가 process-local
  cache를 생성합니다. 성공값만 저장하며 오류, cancellation, stale 값은
  cache 경로에서 반환하지 않습니다.
- 이름은 valid UTF-8, non-blank, 최대 2048 byte여야 합니다. caller 문자열은
  trim 또는 normalization 없이 AWS request에 그대로 전달합니다.

## 소유권과 보안

Credential, AWS config/region, endpoint, retry/backoff, timeout, client lifecycle,
secret rotation, cache invalidation policy와 애플리케이션 precedence는 caller가
소유합니다. `Value`, raw bytes, secret identifier와 unwrap한 provider error를
로그에 남기지 마십시오. 이 package는 configuration framework, KMS envelope,
IAM provisioning과 live AWS smoke test를 구현하지 않습니다.

`Error()`와 `%+v`에는 package sentinel과 고정 operation label만 나타납니다.
원인은 `errors.Is`/`errors.As`로만 확인할 수 있습니다.

## 검증

```bash
go test -count=1 ./secretsmanager
go test -race -count=1 ./secretsmanager
go test -run '^Example' -count=1 ./secretsmanager
```

Test는 deterministic fake client만 사용하므로 AWS credential이나 network가
필요하지 않습니다.
