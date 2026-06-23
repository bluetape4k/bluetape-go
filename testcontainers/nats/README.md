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
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
t.Cleanup(cancel)

url := natstestcontainer.Start(ctx, t)
details := map[string]string{
    natstestcontainer.URLKey: url,
}
client, err := nats.Connect(details[natstestcontainer.URLKey], nats.Timeout(5*time.Second))
if err != nil {
    t.Fatalf("connect nats: %v", err)
}
t.Cleanup(client.Close)
```

## Behavior

- Uses `nats:2.10-alpine`.
- Returns the module-provided connection string.
- Registers container termination with `t.Cleanup`.
- Exposes the URL key as `natstestcontainer.URLKey` (`nats.url`).
- Start failures are categorized as Docker unavailable, image pull failure,
  readiness timeout, context cancellation, or wrapper failure.

## Operational Boundaries

- Docker or another Testcontainers-compatible runtime must be available.
- The fixture is for tests and does not expose production NATS configuration.
- Run Docker-backed Testcontainers packages serially when resources or ports are
  shared.
- CI jobs without Docker should skip `./testcontainers/...`; CI jobs that cover
  these helpers should run the packages with `-p 1`.

## Test

```bash
go test -p 1 -count=1 ./testcontainers/nats
```
