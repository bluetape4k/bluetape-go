# core

[English](README.md) | [한국어](README.ko.md)

`core`는 bluetape-go 패키지에서 사용하는 좁은 shared helper를 담습니다. Go standard library가 이미 작업을 명확히 표현한다면 standard library를 우선하세요. 이 패키지는 반복되는 validation, pointer, zero/default, string, 작은 numeric check를 위한 것입니다. 명시적인 open/closed boundary가 필요할 때 쓰는 작은 ordered range value도 제공합니다.

![core helper boundary map](../docs/images/readme-diagrams/core-helper-boundary-map.png)

## 가져오기

```go
import "github.com/bluetape4k/bluetape-go/core"
```

## 사용 예

```go
name := core.BlankToDefault(input.Name, "anonymous")
limit, err := core.Clamp(input.Limit, 1, 100)
if err != nil {
    return err
}

owner := core.Ptr("worker-1")
_ = core.ValueOr(owner, "fallback")

window, err := core.ClosedOpenRange(10, 20)
if err != nil {
    return err
}
_ = window.Contains(10) // true
```

## 동작

- Validation helper는 panic 대신 error를 반환합니다.
- `Range` constructor는 `ClosedRange`, `ClosedOpenRange`, `OpenClosedRange`,
  `OpenOpenRange`로 `[lower, upper]`, `[lower, upper)`, `(lower, upper]`,
  `(lower, upper)` 표기를 지원합니다.
- Invalid range와 NaN float endpoint는 거부합니다. zero-value `Range`는
  안전한 empty range이며, non-empty range는 constructor로 만듭니다.
- `Zero`, `IsZero`, `DefaultIfZero`, `FirstNonZero`는 generic fallback 동작을 명시적으로 유지합니다.
- `TruncateUTF8Bytes`는 rune boundary에서 자르고 negative limit 또는 invalid
  UTF-8 input을 거부합니다.
- Hex helper는 prefixed `0x` / `0X` string을 decode하지 않고 validation합니다.
- Kotlin operator overload와 DSL-style range constructor는 Go API에 포함하지 않습니다.

## 테스트

```bash
go test -count=1 ./core
```
