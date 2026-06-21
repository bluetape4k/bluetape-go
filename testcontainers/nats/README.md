# testcontainers/nats

[English](README.md) | [한국어](README.ko.md)

`testcontainers/nats` starts a NATS container for integration tests and returns
the client connection URL.

![testcontainers helper flow](../../docs/images/readme-diagrams/testcontainers-helper-flow.png)

## Import

```go
import natstestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/nats"
```

## Usage

```go
url := natstestcontainer.Start(context.Background(), t)
client, err := nats.Connect(url, nats.Timeout(5*time.Second))
if err != nil {
    t.Fatalf("connect nats: %v", err)
}
t.Cleanup(client.Close)
```

## Behavior

- Uses `nats:2.10-alpine`.
- Returns the module-provided connection string.
- Registers container termination with `t.Cleanup`.
- Fatal test failures are reported through the supplied `testing.T`.

## Operational Boundaries

- Docker or another Testcontainers-compatible runtime must be available.
- The fixture is for tests and does not expose production NATS configuration.

## Test

```bash
go test -count=1 ./testcontainers/nats
```
