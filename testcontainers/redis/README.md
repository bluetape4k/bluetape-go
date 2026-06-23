# testcontainers/redis

[English](README.md) | [한국어](README.ko.md)

`testcontainers/redis` starts a Redis container for integration tests and
returns the mapped `host:port` address.

![testcontainers helper flow](../../docs/images/readme-diagrams/testcontainers-helper-flow.png)

## Import

```go
import redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
```

## Usage

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
t.Cleanup(cancel)

addr := redistestcontainer.Start(ctx, t)
details := map[string]string{
    redistestcontainer.AddressKey: addr,
}
client := redis.NewClient(&redis.Options{Addr: details[redistestcontainer.AddressKey]})
t.Cleanup(func() {
    _ = client.Close()
})
```

## Behavior

- Uses `redis:7.4-alpine`.
- Waits for Redis readiness before returning.
- Registers container termination with `t.Cleanup`.
- Exposes the address key as `redistestcontainer.AddressKey`
  (`redis.address`).
- Start failures are categorized as Docker unavailable, image pull failure,
  readiness timeout, context cancellation, or wrapper failure.

## Operational Boundaries

- Docker or another Testcontainers-compatible runtime must be available.
- The helper is intended for tests and does not expose production Redis
  configuration.
- Run Docker-backed Testcontainers packages serially when resources or ports are
  shared.
- CI jobs without Docker should skip `./testcontainers/...`; CI jobs that cover
  these helpers should run the packages with `-p 1`.

## Test

```bash
go test -p 1 -count=1 ./testcontainers/redis
```
