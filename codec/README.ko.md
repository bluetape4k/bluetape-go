# codec

[English](README.md) | [한국어](README.ko.md)

`codec`은 bluetape-go 패키지에서 사용하는 작은 string/byte encoder를 제공합니다. Base64와 hex는 Go standard library를 감싸고, Base58과 Base62는 standard library가 제공하지 않는 identifier/key format을 다룹니다.

![codec encoding surface map](../docs/images/readme-diagrams/codec-encoding-surface-map.png)

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

compactID, err := codec.EncodeUUIDURL62("24738134-9d88-6645-4ec8-d63aa2031015")
uuidText, err := codec.DecodeUUIDURL62(compactID)
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
  사용합니다. Byte-oriented alias이므로 leading zero byte를 보존합니다.
- `EncodeUUIDURL62`와 `DecodeUUIDURL62`는 Kotlin `Url62`와 호환되는 compact
  UUID text rendering을 제공합니다. UUID byte를 128-bit big-endian number로
  정규화하므로 compact string에는 high-order zero byte가 남지 않습니다.
- UUID URL62 rendering은 `codec`, UUID generation은 `id`, UUID text
  validation은 `core` 소유입니다.
- String helper는 UTF-8 string과 byte encoding 사이를 변환하며 serialization metadata를 추가하지 않습니다.
- Decode string helper는 UTF-8 text helper이며 decoded byte가 valid UTF-8이
  아니면 `core.ErrInvalidUTF8`을 wrapping한 error를 반환합니다.
- Error를 반환하지 않는 encode string helper는 string을 byte로 변환한 뒤
  encoding하며 invalid UTF-8을 보고할 수 없습니다.
- Binary payload는 `DecodeBase64`, `DecodeBase64URL`, `DecodeHex`,
  `DecodeBase58`, `DecodeBase62`, `DecodeURL62` 같은 byte helper를 사용해야
  합니다.

## bluetape4k-core 호환성

| Surface | Compatible | Notes |
|---|---|---|
| Base58 alphabet | Yes | Bitcoin alphabet `123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz`를 사용합니다. |
| Base58 leading zeros | Yes | Leading zero byte는 leading `1` 문자로 인코딩됩니다. |
| Base62 alphabet | Yes | `0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz`를 사용합니다. |
| Base62 numeric vectors | Yes | Big-endian numeric byte는 Kotlin `BigInteger` vector와 일치합니다. 예: `123456789 -> 8M0kX`. |
| URL62 UUID helpers | Yes | `EncodeUUIDURL62`/`DecodeUUIDURL62`는 high-order zero 정규화를 포함해 Kotlin `Url62` numeric UUID 동작과 일치합니다. |
| URL62 byte helpers | Go-specific | `EncodeURL62`/`DecodeURL62`는 Base62 byte alias이며 leading zero byte를 보존합니다. |
| Extra byte-array leading zeros | Go-specific | Go API는 byte-oriented라 보존합니다. Kotlin `BigInteger`/UUID helper는 정규화 과정에서 제거합니다. |
| Empty input decode | Go-specific | Go는 byte round trip을 위해 `""`를 empty bytes로 디코딩합니다. Kotlin은 blank text input을 거부합니다. |
| Base62 decode bit limit | Go-specific | Go는 arbitrary byte payload를 디코딩하며 Kotlin의 기본 128-bit `BigInteger`/UUID 제한을 적용하지 않습니다. |

## 테스트

```bash
go test -count=1 ./codec
```

## 벤치마크

```bash
go test -run '^$' -bench '^BenchmarkCodec' -benchmem ./codec
```

Base64, Base64URL, hex runner는 object, binary, repeated SerDe fixture shape를
다룹니다. Base58, Base62, URL62 runner는 ID/key surface에 가깝고 large binary
transport codec이 아니므로 small payload와 UUID-sized payload로 의도적으로
제한합니다. Raw output artifact를 수집할 때는
`docs/benchmarks/2026-07-07-issue-400-go-serde-runners.md`를 기준으로 삼으세요.

0.14.0 recommendation matrix도 이 경계를 유지합니다. General byte transport는
Base64, Base64URL, hex를 사용하고, Base58, Base62, URL62는 compact ID/key-sized
값과 UUID rendering에 사용하세요.
`docs/research/2026-07-07-issue-402-cross-repo-serde-recommendation.md`를 참고하세요.
