# rds/auth

[English](README.md)

`rds/auth`는 AWS SDK for Go v2 `feature/rds/auth.BuildAuthToken`을 감싼 작은
검증 wrapper입니다. RDS IAM database authentication token을 15분 유효한
redacted `Token`으로 반환하고 credential, SQL driver와 connection lifecycle은
caller에게 남깁니다.

## 사용 예

```go
token, err := auth.BuildAuthToken(ctx, auth.Request{
    Endpoint: "database.example.com:5432",
    Region:   "ap-northeast-2",
    Username: "app_user",
}, credentials) // caller-owned aws.CredentialsProvider
if err != nil {
    return err
}

password := token.Text() // PostgreSQL password field에 명시적으로 전달
```

MySQL도 동일하게 `token.Text()`를 driver password field에 전달하십시오.
이 package는 DSN을 만들거나 `database/sql` connection을 열지 않으며 pool도
관리하지 않습니다.

## 계약

- `Request.Endpoint`는 scheme, path, query, fragment가 없는 정확한 `host:port`
  형식이어야 합니다. Bracketed IPv6를 지원하고 port는 `1..65535` 범위입니다.
- Region과 username은 valid UTF-8, non-blank, bounded 값이어야 합니다. 값은
  trim 또는 normalization 없이 AWS SDK에 전달합니다.
- SDK 호출 전과 signing response 후 `context.Context`를 검사합니다. 늦은 token
  결과보다 caller cancellation이 우선합니다.
- 주입하는 `aws.CredentialsProvider`는 caller가 소유하며 이 signing 호출에만
  사용합니다. Helper가 credential을 만들거나 cache, refresh, zero하지 않습니다.
- `Token.String()`과 `Token.GoString()`은 항상 `[REDACTED]`를 반환합니다. Raw
  token handoff에는 `Text()`와 `Bytes()`를 명시적으로 사용하십시오.

AWS는 token에 15분 lifetime(`X-Amz-Expires=900`)을 서명합니다. Refresh 시점,
credential rotation, IAM policy, TLS/driver 설정과 connection retry는 caller가
소유합니다. 이 package에는 live AWS test나 RDS provisioning 경로가 없습니다.

## 검증

```bash
go test -count=1 ./rds/auth
go test -race -count=1 ./rds/auth
go test -run '^Example' -count=1 ./rds/auth
```

Test는 deterministic credential fake만 사용하므로 AWS credential, network,
database driver와 실행 중인 RDS instance가 필요하지 않습니다.
