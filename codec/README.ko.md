# codec

[English](README.md) | [한국어](README.ko.md)

`codec`은 bluetape-go 패키지에서 사용하는 작은 string/byte encoder를 제공합니다. Base64와 hex는 Go standard library를 감싸고, Base58과 Base62는 standard library가 제공하지 않는 identifier/key format을 다룹니다.

## 가져오기

```go
import "github.com/bluetape4k/bluetape-go/codec"
```

## 사용 예

```go
encoded := codec.EncodeBase62String("bluetape-go")
decoded, err := codec.DecodeBase62String(encoded)
if err != nil {
    return err
}

token := codec.EncodeBase64URL([]byte{251, 255, 255})
payload, err := codec.DecodeBase64URL(token)
```

## 동작

- Decode helper는 input alphabet을 검증하고 malformed data에 대해 error를 반환합니다.
- Base58은 bluetape4k-core와 같은 Bitcoin alphabet을 사용하며 leading zero
  byte를 `1`로 보존합니다.
- Base62는 bluetape4k-core와 같은 `0-9A-Z-a-z` alphabet을 사용합니다. Go는
  byte-oriented API이므로 leading zero byte를 보존합니다. Kotlin
  `Base62`/`Url62` helper는 `BigInteger`/UUID 중심이라 추가 byte-array leading
  zero를 보존하지 않습니다.
- `EncodeURL62`는 Base62 alphabet이 이미 URL-safe이므로 같은 alphabet을
  사용합니다. Minimal big-endian numeric UUID byte는 Kotlin `Url62` UUID
  vector와 호환됩니다. High-order zero UUID byte는 Go byte API가 보존하므로
  Kotlin numeric helper와 다른 Go-specific 동작입니다.
- String helper는 UTF-8 string과 byte encoding 사이를 변환하며 serialization metadata를 추가하지 않습니다.

## bluetape4k-core 호환성

| Surface | Compatible | Notes |
|---|---|---|
| Base58 alphabet | Yes | Bitcoin alphabet `123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz`를 사용합니다. |
| Base58 leading zeros | Yes | Leading zero byte는 leading `1` 문자로 인코딩됩니다. |
| Base62 alphabet | Yes | `0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz`를 사용합니다. |
| Base62 numeric vectors | Yes | Big-endian numeric byte는 Kotlin `BigInteger` vector와 일치합니다. 예: `123456789 -> 8M0kX`. |
| URL62 UUID vectors | Conditional | Minimal big-endian numeric UUID byte는 Kotlin `Url62` 출력과 일치합니다. High-order zero UUID byte는 Go가 보존하지만 Kotlin numeric helper는 정규화합니다. |
| Extra byte-array leading zeros | Go-specific | Go API는 byte-oriented라 보존합니다. Kotlin `BigInteger`/UUID helper는 정규화 과정에서 제거합니다. |
| Empty input decode | Go-specific | Go는 byte round trip을 위해 `""`를 empty bytes로 디코딩합니다. Kotlin은 blank text input을 거부합니다. |
| Base62 decode bit limit | Go-specific | Go는 arbitrary byte payload를 디코딩하며 Kotlin의 기본 128-bit `BigInteger`/UUID 제한을 적용하지 않습니다. |

## 테스트

```bash
go test -count=1 ./codec
```
