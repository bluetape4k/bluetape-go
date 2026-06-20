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
client := redis.NewClient(&redis.Options{Addr: addr})
t.Cleanup(func() {
    _ = client.Close()
})
```

## Behavior

- Uses `redis:7.4-alpine`.
- Waits for Redis readiness before returning.
- Registers container termination with `t.Cleanup`.
- Fatal test failures are reported through the supplied `testing.T`.

## Operational Boundaries

- Docker or another Testcontainers-compatible runtime must be available.
- The helper is intended for tests and does not expose production Redis
  configuration.

## Test

```bash
go test -count=1 ./testcontainers/redis
```
