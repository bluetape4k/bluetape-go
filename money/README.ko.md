# money

[English](README.md) | [한국어](README.ko.md)

`money`는 ISO 4217 통화, decimal-backed 금액, 합산, 직렬화, caller-supplied
환율 변환을 위한 Go-native value API를 제공합니다.

## Import

```go
import "github.com/bluetape4k/bluetape-go/money"
```

## 선택 가이드

| 필요 | 사용 | 메모 |
|---|---|---|
| 통화 코드 파싱 | `ParseCurrency` 또는 `MustParseCurrency` | ISO alphabetic/numeric code를 받지만 no-currency `XXX`/`999`는 거부합니다. |
| 결정적 금융 입력 | `New` | `"12.34"` 같은 decimal string을 우선 사용합니다. |
| major-unit 정수 입력 | `NewFromInt64` | `NewFromInt64(12, USD)`는 `USD 12.00`, `NewFromInt64(12, KRW)`는 `KRW 12`입니다. |
| minor-unit 정수 입력 | `NewMinor` | `NewMinor(12, USD)`는 `USD 0.12`, `NewMinor(12, JPY)`는 `JPY 12`입니다. |
| 합산 | `Sum` | 입력이 비어 있으면 요청한 통화의 valid zero amount를 반환합니다. |
| 환율 변환 | `NewExchangeRate`와 `Convert` | 환율은 caller가 제공합니다. provider-backed 조회는 #178에서 진행합니다. |
| 전체 locale mapping | Deferred | 현재는 제한된 common locale lookup만 제공하며 전체 mapping은 #179에서 추적합니다. |
| Long-backed FastMoney | Deferred | benchmark 기반 평가는 #180에서 추적합니다. |

## 사용법

```go
price, err := money.New("12.34", money.USD)
if err != nil {
    return err
}
tax, err := money.New("0.66", money.USD)
if err != nil {
    return err
}
total, err := price.Add(tax)
if err != nil {
    return err
}

payload, err := json.Marshal(total)
```

## 동작

- `money`는 전체 회계 시스템이 아닙니다. Ledger, posting rule, tax policy,
  financial calendar, provider-backed FX lookup, jurisdiction-specific rounding
  policy는 제공하지 않습니다.
- 정밀도 모델은 `github.com/govalues/money`와 `github.com/govalues/decimal`에
  기반합니다. 값은 immutable decimal-backed value지만 무제한 arbitrary precision
  숫자는 아닙니다.
- Public API type은 bluetape-go wrapper입니다. Caller는 public signature에서
  upstream concrete type에 의존하지 않습니다.
- Zero-value `Money{}`, `Currency{}`, `ExchangeRate{}`는 invalid sentinel
  value입니다. 유효한 0 금액은 `Zero(currency)`를 사용합니다.
- `ParseCurrency("XXX")`, `ParseCurrency("999")`, no-currency 값으로 생성하는
  작업은 `ErrInvalidCurrency`를 반환합니다.
- 산술은 같은 통화끼리만 허용합니다. 통화 불일치는 `errors.Is`로 확인 가능한
  `ErrCurrencyMismatch`를 반환합니다.
- `Round`는 통화 scale 기준 half-even 반올림을 사용합니다. `RoundTo`는 요청한
  scale에 half-even 반올림을 적용하되 통화의 최소 scale은 유지합니다.
- `MinorUnits`는 통화 scale 기준 half-even 반올림 후 minor-unit 정수를
  반환합니다. `int64` 범위를 넘으면 `ErrOverflow`를 반환합니다.
- JSON 직렬화 shape은 `{"amount":"12.34","currency":"USD"}`입니다. Text
  직렬화 shape은 `USD 12.34`입니다.
- Parse와 unmarshal helper는 제한된 money shape을 검증하지만 HTTP request size,
  body size, streaming limit는 caller가 소유합니다.
- `NewFromFloat64`는 편의를 위해 제공하지만 결정적 금융 입력에는 string 또는
  minor-unit constructor를 우선합니다.
- Money value는 immutable하며 goroutine 사이에 전달해도 안전합니다. #35는
  대표 연산을 `testing/concurrency.GoroutineStressTester`와 `go test -race`로
  검증합니다.

## Test

```bash
go test -count=1 ./money
go test -race -count=1 ./money
```
