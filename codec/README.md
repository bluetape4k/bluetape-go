# codec

[English](README.md) | [한국어](README.ko.md)

`codec` contains small string and byte encoders used by bluetape-go packages.
Base64 and hex wrap the Go standard library, while Base58 and Base62 cover
identifier and key formats that are not provided by the standard library.

![codec encoding surface map](../docs/images/readme-diagrams/codec-encoding-surface-map.png)

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
- Base58 uses the same Bitcoin alphabet as bluetape4k-core and preserves
  leading zero bytes as `1`.
- Base62 uses the same `0-9A-Z-a-z` alphabet as bluetape4k-core. Go exposes a
  byte-oriented API, so leading zero bytes are preserved; Kotlin's
  `Base62`/`Url62` helpers are `BigInteger`/UUID-oriented and do not carry
  extra byte-array leading zeros.
- `EncodeURL62` uses the same alphabet as Base62 because it is already URL-safe.
  Kotlin `Url62` UUID vectors are compatible for minimal big-endian numeric UUID
  bytes. UUIDs with high-order zero bytes remain a Go byte-API divergence
  because Go preserves those bytes.
- String helpers convert between UTF-8 strings and byte encodings without
  adding serialization metadata.

## bluetape4k-core compatibility

| Surface | Compatible | Notes |
|---|---|---|
| Base58 alphabet | Yes | Uses the Bitcoin alphabet `123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz`. |
| Base58 leading zeros | Yes | Leading zero bytes encode as leading `1` characters. |
| Base62 alphabet | Yes | Uses `0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz`. |
| Base62 numeric vectors | Yes | Big-endian numeric bytes match Kotlin `BigInteger` vectors, for example `123456789 -> 8M0kX`. |
| URL62 UUID vectors | Conditional | Minimal big-endian numeric UUID bytes match Kotlin `Url62`; high-order zero UUID bytes are preserved by Go but normalized by Kotlin numeric helpers. |
| Extra byte-array leading zeros | Go-specific | Go preserves them because the API is byte-oriented; Kotlin `BigInteger`/UUID helpers normalize them away. |
| Empty input decode | Go-specific | Go decodes `""` to empty bytes for byte round trips; Kotlin rejects blank text inputs. |
| Base62 decode bit limit | Go-specific | Go decodes arbitrary byte payloads and does not enforce Kotlin's default 128-bit `BigInteger`/UUID limit. |

## Test

```bash
go test -count=1 ./codec
```
