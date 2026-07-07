# serialization

[English](README.md) | [한국어](README.ko.md)

`serialization`은 storage, cache, message 경계를 지나는 값을 위한 작은
serializer 계약을 정의합니다. 디코딩된 값을 신뢰하기 전에 format과 version을
명시적으로 확인하는 흐름을 유지합니다.

## 가져오기

```go
import "github.com/bluetape4k/bluetape-go/serialization"
```

## 사용 예

![serialization envelope 흐름](../docs/images/readme-diagrams/serialization-envelope-flow.png)

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

- `JSONSerializer`는 `encoding/json`을 사용하며 뒤따르는 JSON 값을 거부합니다.
  기본 decode path는 `json.Unmarshal`을 사용하고,
  `WithDisallowUnknownFields`는 unknown object field를 거부하기 위해 decoder를
  사용합니다.
- `WithDisallowUnknownFields`는 엄격한 object decoding을 활성화합니다.
- `BytesSerializer`는 marshal/unmarshal 시 byte slice를 복사합니다.
- `StringSerializer`는 UTF-8 text serializer이며 invalid UTF-8 input에는
  `core.ErrInvalidUTF8`을 wrapping한 error를 반환합니다.
- Binary payload는 `BytesSerializer`를 사용해야 합니다.
- `VersionedSerializer`는 payload를 format/version metadata가 있는 `BTGS` envelope로 감쌉니다.

## 테스트

```bash
go test -count=1 ./serialization
```

## 벤치마크

```bash
go test -run '^$' -bench '^BenchmarkSerialization' -benchmem ./serialization
```

Benchmark runner는 JSON, raw bytes, raw strings, `BTGS` versioned envelope,
serialize-then-compress scenario에 결정적 SerDe fixture를 사용합니다. 0.14.0
cross-repo baseline의 raw output artifact를 수집할 때는
`docs/benchmarks/2026-07-07-issue-400-go-serde-runners.md`를 기준으로 삼으세요.

0.14.0 cross-repo recommendation matrix 기준으로 Go serialization guidance는
작고 wire format이 명확해야 합니다. JSON과 Go-local `BTGS` envelope는 안전한
Go package 선택지이지만, JVM Fory/Kryo와 future Rust adapter는 각자의 trust
boundary를 가진 별도 wire format입니다.
`docs/research/2026-07-07-issue-402-cross-repo-serde-recommendation.md`를 참고하세요.

Issue #456은 repeated-collection JSON decode allocation profile을
`docs/research/outputs/issue-456/` 아래에 보존합니다. 좁은 기본 decode 최적화는
strict trailing-value rejection을 유지하면서 unknown-field rejection을 요청하지
않은 경우 추가 `json.Decoder` buffer copy를 피합니다.
