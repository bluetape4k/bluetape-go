# core

[English](README.md) | [한국어](README.ko.md)

`core`는 bluetape-go 패키지에서 사용하는 좁은 shared helper를 담습니다. Go standard library가 이미 작업을 명확히 표현한다면 standard library를 우선하세요. 이 패키지는 반복되는 validation, pointer, zero/default, string, 작은 numeric check를 위한 것입니다. 명시적인 open/closed boundary가 필요할 때 쓰는 작은 ordered range value도 제공합니다.
반복되는 filter/key 용도를 위한 wildcard 및 XXH64 helper, quarter/date-iteration workflow를 위한 작은 calendar helper도 포함합니다.

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

matched, err := core.MatchWildcard("order-*.json", "order-2026.json")
if err != nil {
    return err
}
_ = matched // true

pathMatched, err := core.MatchWildcardPath("configs/**/*.yaml", `configs\prod\app.yaml`)
if err != nil {
    return err
}
_ = pathMatched // true

cacheKeyHash := core.XXH64String("customer:42")
_ = cacheKeyHash

reportingQuarter, err := core.ParseYearQuarter("2026-Q3")
if err != nil {
    return err
}
periodStart, err := reportingQuarter.Start(time.UTC)
if err != nil {
    return err
}
periodEnd, err := reportingQuarter.End(time.UTC)
if err != nil {
    return err
}

for reportDate := range core.DatesUntil(periodStart, periodStart.AddDate(0, 0, 3)) {
    _ = reportDate.Format(time.DateOnly)
}
_ = periodEnd
```

## 동작

- Validation helper는 panic 대신 `ErrInvalidArgument`를 감싼 error를 반환합니다.
  specialized helper는 `ErrInvalidTime`, `ErrInvalidUTF8` 같은 sentinel도 유지합니다.
- `Range` constructor는 `ClosedRange`, `ClosedOpenRange`, `OpenClosedRange`,
  `OpenOpenRange`로 `[lower, upper]`, `[lower, upper)`, `(lower, upper]`,
  `(lower, upper)` 표기를 지원합니다.
- Invalid range와 NaN float endpoint는 거부합니다. zero-value `Range`는
  안전한 empty range이며, non-empty range는 constructor로 만듭니다.
- `Zero`, `IsZero`, `DefaultIfZero`, `IfZeroOrDefault`, `FirstNonZero`는 generic fallback 동작을 명시적으로 유지합니다.
- `TruncateUTF8Bytes`는 rune boundary에서 자르고 negative limit 또는 invalid
  UTF-8 input을 거부합니다.
- Hex helper는 prefixed `0x` / `0X` string을 decode하지 않고 validation합니다.
- `MatchWildcard`는 case-sensitive이며 `*`, `?`, consecutive star,
  `\*`, `\?`, `\\` 같은 escaped literal을 지원합니다. trailing escape는
  `ErrMalformedWildcardPattern`을 반환합니다.
- `MatchWildcardPath`는 lexical matching입니다. `/`와 `\`를 input separator로
  받고, full segment `**`를 zero-or-more path segment로 처리합니다.
  slash-separated pattern segment 안에서는 `*`, `?`, `\` escaped literal도
  지원합니다. 모든 OS에서 case-sensitive이며 filesystem을 읽거나 path를 clean하지 않습니다.
- `XXH64Bytes`와 `XXH64String`은 seed 0의 deterministic XXH64 값을 반환합니다.
  XXH64는 non-cryptographic hash입니다. signature, password, token,
  attacker-resistant integrity check에는 `crypto/*` 또는 keyed MAC을 사용하세요.
- `Quarter`와 `YearQuarter` helper는 quarter 값을 validation하고 year `0`을
  거부합니다. Quarter end는 exclusive라서 reporting window를 `[start, end)`
  형태로 표현할 수 있습니다.
- `DatesUntil`과 `DatesInclusive`는 start location 기준의 calendar date를
  순회하고 midnight 값을 반환합니다. End boundary는 date 비교 전에 start
  location으로 변환합니다.
- Date iteration은 `time.Time.AddDate`로 진행하므로 daylight-saving 전환에서도
  하루를 항상 24시간으로 가정하지 않고 calendar date를 보존합니다.
- Kotlin operator overload와 DSL-style range constructor는 Go API에 포함하지 않습니다.
- Kotlin numeric duration DSL, broad duration parser/formatter wrapper,
  Java-time temporal type mirror, period/calendar framework, JVM classpath
  resource loading, system-property wrapper, shutdown hook, generic object
  hashing, temp/output/env helper, broad string/byte alias는 의도적으로
  제외합니다. Go caller는 `time`, `os`, `io/fs`, `runtime`, `context`,
  명시적 encoding을 직접 사용해야 합니다.

## 테스트

```bash
go test -count=1 ./core
```
