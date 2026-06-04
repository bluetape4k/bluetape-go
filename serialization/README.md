# serialization

`serialization` defines small serializer contracts for storage, cache, and
message payloads. The package favors explicit formats and safe decoding over
unsafe object deserialization.

## Import

```go
import "github.com/bluetape4k/bluetape-go/serialization"
```

## Usage

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
- `VersionedSerializer` wraps payloads in a `BTGS` envelope with format and
  version metadata.

## Test

```bash
go test -count=1 ./serialization
```
