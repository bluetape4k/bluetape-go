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
- Base58과 Base62는 leading zero byte를 보존합니다.
- `EncodeURL62`는 Base62 alphabet이 이미 URL-safe이므로 같은 alphabet을 사용합니다.
- String helper는 UTF-8 string과 byte encoding 사이를 변환하며 serialization metadata를 추가하지 않습니다.

## 테스트

```bash
go test -count=1 ./codec
```
