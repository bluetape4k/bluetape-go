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
writer := &kafka.Writer{
    Addr:  kafka.TCP(brokers...),
    Topic: "bluetape-test",
}
t.Cleanup(func() {
    _ = writer.Close()
})
```

## Behavior

- Uses `confluentinc/confluent-local:7.5.0`.
- Configures cluster ID `bluetape-test-cluster`.
- Returns the broker list from the Testcontainers Kafka module.
- Fails the test if no broker address is returned.
- Registers container termination with `t.Cleanup`.

## Operational Boundaries

- Docker or another Testcontainers-compatible runtime must be available.
- Kafka startup can be slower than smaller fixtures; use an explicit test
  timeout around the start context.

## Test

```bash
go test -count=1 ./testcontainers/kafka
```
