# ssm

[한국어](README.ko.md)

`ssm` is a narrow AWS SDK for Go v2 Parameter Store `GetParameter` provider.
It accepts a caller-owned client, exposes SecureString decryption as an
explicit option, returns a redacted `Value`, and optionally uses a caller-owned
positive-TTL `cache.LoadingCache`.

## Usage

```go
provider, err := ssm.New(ssm.Options{
    Client:         client, // caller owns config, credentials, retries, and lifecycle
    Cache:           cache,  // caller owns bounded capacity and eviction policy
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
password := value.Text() // explicit raw-value handoff

secure, err := provider.GetSecure(ctx, "/prod/database/password")
```

`GetSecure` always sends `WithDecryption=true`, regardless of the provider
default. `Value.String()` and `Value.GoString()` return `[REDACTED]`; use
`Text()` or `Bytes()` for an explicit handoff. The zero value is unset, while an
empty parameter remains set.

## Contracts

- Only `GetParameter` is required from the injected client. The provider does
  not create, close, reconfigure, or retry the AWS client.
- `context.Context` is checked before dispatch, after the SDK response, and at
  result publication. Caller cancellation wins over a late successful response.
- `Get` uses `Options.WithDecryption`; `GetSecure` forces decryption. Plain and
  secure lookups use different cache keys and cannot return one another's data.
- `CacheTTL == 0` disables caching. A positive TTL requires the supplied
  `cache.LoadingCache`; the provider never creates an implicit unbounded
  process-local cache. The caller chooses and owns bounded capacity, eviction,
  and invalidation policy. Only successful values are cached. Errors,
  cancellation, and stale values are not cached.
- Names are valid UTF-8, non-blank, and at most 2048 bytes. The exact caller
  string is passed to AWS without trimming or normalization.

## Ownership and security

The caller owns credentials, AWS config/region, endpoint, retry/backoff,
timeouts, client lifecycle, parameter precedence, rotation, and cache
invalidation. Do not log `Value`, raw bytes, parameter identifiers, or unwrapped
provider errors. The package does not implement a configuration framework, IAM
provisioning, or live AWS smoke test.

`Error()` and `%+v` expose only a package sentinel and fixed operation label;
the underlying cause remains available through `errors.Is`/`errors.As`.

## Verification

```bash
go test -count=1 ./ssm
go test -race -count=1 ./ssm
go test -run '^Example' -count=1 ./ssm
```

Tests use deterministic fake clients and do not require AWS credentials or
network access.
