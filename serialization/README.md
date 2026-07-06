# serialization

[English](README.md) | [한국어](README.ko.md)

`serialization` defines small serializer contracts for values that cross
storage, cache, or message boundaries. It keeps format and version checks
explicit before callers trust decoded values.

## Import

```go
import "github.com/bluetape4k/bluetape-go/serialization"
```

## Usage

![serialization envelope flow](../docs/images/readme-diagrams/serialization-envelope-flow.png)

```go
type Account struct {
    ID     string `json:"id"`
    Active bool   `json:"active"`
}

jsonSerializer := serialization.NewJSONSerializer[Account]()
versioned, err := serialization.NewVersionedSerializer[Account](jsonSerializer, 1)
if err != nil {
    return err
}

data, err := versioned.Marshal(Account{ID: "acct-1", Active: true})
if err != nil {
    return err
}
value, err := versioned.Unmarshal(data)
```

## Behavior

- `JSONSerializer` uses `encoding/json` and rejects trailing JSON values.
- `WithDisallowUnknownFields` enables strict object decoding.
- `BytesSerializer` copies byte slices on marshal and unmarshal.
- `StringSerializer` is a UTF-8 text serializer and returns an error wrapping
  `core.ErrInvalidUTF8` for invalid UTF-8 input.
- Binary payloads should use `BytesSerializer`.
- `VersionedSerializer` wraps payloads in a `BTGS` envelope with format and
  version metadata.

## Test

```bash
go test -count=1 ./serialization
```

## Benchmark

```bash
go test -run '^$' -bench '^BenchmarkSerialization' -benchmem ./serialization
```

The benchmark runners use deterministic SerDe fixtures for JSON, raw bytes,
raw strings, `BTGS` versioned envelopes, and serialize-then-compress scenarios.
Use `docs/benchmarks/2026-07-07-issue-400-go-serde-runners.md` when collecting
raw output artifacts for the 0.14.0 cross-repo baseline.

For the 0.14.0 cross-repo recommendation matrix, keep Go serialization guidance
small and wire-format explicit: JSON and the Go-local `BTGS` envelope are safe
Go package choices, while JVM Fory/Kryo and future Rust adapters remain separate
wire formats with their own trust boundaries. See
`docs/research/2026-07-07-issue-402-cross-repo-serde-recommendation.md`.
