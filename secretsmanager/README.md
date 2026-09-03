# secretsmanager

[한국어](README.ko.md)

`secretsmanager` is a narrow AWS SDK for Go v2 `GetSecretValue` provider. It
accepts a caller-owned client, returns string or binary data in a redacted
`Value`, and optionally uses a positive-TTL `cache.LoadingCache`.

## Usage

```go
provider, err := secretsmanager.New(secretsmanager.Options{
    Client: client, // caller owns config, credentials, retries, and lifecycle
    CacheTTL: 5 * time.Minute,
})
if err != nil {
    return err
}
value, err := provider.Get(ctx, "prod/database/password")
if err != nil {
    return err
}
password := value.Text() // explicit raw-value handoff
```

`Value.String()` and `Value.GoString()` always return `[REDACTED]`. Use
`Text()` for `SecretString` or `Bytes()` for `SecretBinary`; `Bytes()` returns
an independent copy. The zero value is unset, while an empty AWS value remains
set and can be distinguished with `IsSet()`.

## Contracts

- Only `GetSecretValue` is required from the injected client. The provider does
  not create, close, reconfigure, or retry the AWS client.
- `context.Context` is checked before dispatch, after the SDK response, and at
  result publication. Caller cancellation wins over a late successful response.
- `SecretString` and `SecretBinary` must be mutually exclusive. A missing or
  malformed response is not cached and returns a typed sentinel.
- `CacheTTL == 0` disables caching. A positive TTL uses the supplied
  `cache.LoadingCache`; if no cache is supplied, the provider creates a
  process-local cache. Only successful values are cached. Errors, cancellation,
  and stale values are never returned from this provider's cache path.
- Names are valid UTF-8, non-blank, and at most 2048 bytes. The exact caller
  string is passed to AWS without trimming or normalization.

## Ownership and security

The caller owns credentials, AWS config/region, endpoint, retry/backoff,
timeouts, client lifecycle, secret rotation, cache invalidation policy, and
application precedence. Do not log `Value`, raw bytes, secret identifiers, or
unwrapped provider errors. The package does not implement a configuration
framework, KMS envelope, IAM provisioning, or live AWS smoke test.

`Error()` and `%+v` expose only a package sentinel and a fixed operation label;
the underlying cause remains available through `errors.Is`/`errors.As`.

## Verification

```bash
go test -count=1 ./secretsmanager
go test -race -count=1 ./secretsmanager
go test -run '^Example' -count=1 ./secretsmanager
```

Tests use deterministic fake clients and do not require AWS credentials or
network access.
