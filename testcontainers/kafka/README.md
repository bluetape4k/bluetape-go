# testcontainers/kafka

[English](README.md) | [한국어](README.ko.md)

`testcontainers/kafka` starts a Kafka container for integration tests and
returns at least one broker address.

![testcontainers helper flow](../../docs/images/readme-diagrams/testcontainers-helper-flow.png)

## Import

```go
import kafkatestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/kafka"
```

## Usage

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
t.Cleanup(cancel)

brokers := kafkatestcontainer.Start(ctx, t)
details := map[string][]string{
    kafkatestcontainer.BrokersKey: brokers,
}
writer := &kafka.Writer{
    Addr:  kafka.TCP(details[kafkatestcontainer.BrokersKey]...),
    Topic: "bluetape-test",
}
t.Cleanup(func() {
    _ = writer.Close()
})
```

## Shared Server API

Use `Start(ctx, t)` when a broker slice is enough. Use `StartServer(ctx, t)`
when a test needs the shared Testcontainers server contract: host lookup, mapped
ports, endpoints, connection details, cleanup, manual termination, or explicit
env export.

The example assumes `tcserver` aliases `github.com/bluetape4k/bluetape-go/testcontainers/server`.

```go
srv := kafkatestcontainer.StartServer(ctx, t)
details, err := srv.ConnectionDetails(ctx)
if err != nil {
    t.Fatalf("kafka details: %v", err)
}
if err := tcserver.ExportEnv(t, details, map[string]string{
    kafkatestcontainer.BrokersKey: "BLUETAPE_KAFKA_BROKERS",
}); err != nil {
    t.Fatal(err)
}
```

The generic `kafka.brokers` connection detail is a comma-separated string for
env export and reporting. `Start(ctx, t)` still returns `[]string`.

`tcserver.ExportEnv` uses `testing.TB.Setenv`; do not call it from tests that
use `t.Parallel` or have parallel ancestors.

## Behavior

- Uses `confluentinc/confluent-local:7.5.0`.
- Configures cluster ID `bluetape-test-cluster`.
- Returns the broker list from the Testcontainers Kafka module.
- Fails the test if no broker address is returned.
- Registers container termination with `t.Cleanup`.
- Exposes the broker list key as `kafkatestcontainer.BrokersKey`
  (`kafka.brokers`).
- Start failures are categorized as Docker unavailable, image pull failure,
  readiness timeout, context cancellation, or wrapper failure.

## Operational Boundaries

- Docker or another Testcontainers-compatible runtime must be available.
- Kafka startup can be slower than smaller fixtures; use an explicit test
  timeout around the start context.
- Dynamic host port mapping is the default. Read mapped ports and exported env
  values after the container starts; they point to host ports, not
  container-internal ports.
- Fixed host ports are not exposed by this helper because they can collide in
  parallel local runs and CI jobs.
- Run Docker-backed Testcontainers packages serially when resources or ports are
  shared.
- CI jobs without Docker should skip `./testcontainers/...`; CI jobs that cover
  these helpers should run the packages with `-p 1`.

## Test

```bash
go test -p 1 -count=1 ./testcontainers/kafka
```
