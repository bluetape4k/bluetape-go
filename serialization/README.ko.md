# serialization

[English](README.md) | [한국어](README.ko.md)

`serialization`은 storage, cache, message payload를 위한 작은 serializer contract를 정의합니다. 이 패키지는 unsafe object deserialization보다 명시적인 format과 safe decoding을 선호합니다.

## 가져오기

```go
import "github.com/bluetape4k/bluetape-go/serialization"
```

## 사용 예

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

## 동작

- `JSONSerializer`는 `encoding/json`을 사용하고 trailing JSON value를 거부합니다.
- `WithDisallowUnknownFields`는 strict object decoding을 활성화합니다.
- `BytesSerializer`는 marshal/unmarshal 시 byte slice를 복사합니다.
- `VersionedSerializer`는 payload를 format/version metadata가 있는 `BTGS` envelope로 감쌉니다.

## 테스트

```bash
go test -count=1 ./serialization
```
