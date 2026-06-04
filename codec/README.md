# codec

`codec` contains small string and byte encoders used by bluetape-go packages.
Base64 and hex wrap the Go standard library, while Base58 and Base62 cover
identifier and key formats that are not provided by the standard library.

## Import

```go
import "github.com/bluetape4k/bluetape-go/codec"
```

## Usage

```go
encoded := codec.EncodeBase62String("bluetape-go")
decoded, err := codec.DecodeBase62String(encoded)
if err != nil {
    return err
}

token := codec.EncodeBase64URL([]byte{251, 255, 255})
payload, err := codec.DecodeBase64URL(token)
```

## Behavior

- Decode helpers validate input alphabets and return errors for malformed data.
- Base58 and Base62 preserve leading zero bytes.
- `EncodeURL62` uses the same alphabet as Base62 because it is already URL-safe.
- String helpers convert between UTF-8 strings and byte encodings without
  adding serialization metadata.

## Test

```bash
go test -count=1 ./codec
```
