# rds/auth

[한국어](README.ko.md)

`rds/auth` is a small validation wrapper around AWS SDK for Go v2
`feature/rds/auth.BuildAuthToken`. It returns the 15-minute RDS IAM database
authentication token in a redacted `Token` value and leaves credentials, SQL
drivers, and connection lifecycle with the caller.

## Usage

```go
token, err := auth.BuildAuthToken(ctx, auth.Request{
    Endpoint: "database.example.com:5432",
    Region:   "ap-northeast-2",
    Username: "app_user",
}, credentials) // caller-owned aws.CredentialsProvider
if err != nil {
    return err
}

password := token.Text() // pass explicitly to the PostgreSQL password field
```

For MySQL, pass the same explicit `token.Text()` value to the driver's password
field. The package does not construct a DSN, open a `database/sql` connection,
or manage a pool.

## Contracts

- `Request.Endpoint` must be an exact `host:port` value without a scheme, path,
  query, fragment, userinfo, percent escape, or backslash. DNS hosts use ASCII
  labels (each at most 63 bytes, total at most 253 bytes); IPv4 and bracketed
  IPv6 literals are supported; ports are `1..65535`.
- Region and username must be valid UTF-8, non-blank, and bounded. Values are
  passed to the AWS SDK without trimming or normalization.
- `context.Context` is checked before the SDK call and after the signing
  response. Caller cancellation wins over a late token result.
- The injected `aws.CredentialsProvider` is caller-owned and is used only for
  this signing call. The helper does not create, cache, refresh, or zero the
  provider's credentials.
- `Token.String()` and `Token.GoString()` always return `[REDACTED]`.
  `Text()` and `Bytes()` are explicit raw-token handoff methods.

AWS signs tokens with a 15-minute lifetime (`X-Amz-Expires=900`). The caller
owns refresh timing, credential rotation, IAM policy, TLS/driver settings, and
connection retries. The package has no live AWS test or RDS provisioning path.

## Verification

```bash
go test -count=1 ./rds/auth
go test -race -count=1 ./rds/auth
go test -run '^Example' -count=1 ./rds/auth
```

Tests use deterministic credential fakes and do not require AWS credentials,
network access, a database driver, or a running RDS instance.
