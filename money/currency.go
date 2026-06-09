package money

import (
	"fmt"
	"strings"

	gmoney "github.com/govalues/money"
)

// Currency 는 ISO 4217 통화를 나타내는 값입니다.
type Currency struct {
	curr gmoney.Currency
}

var (
	// KRW 는 대한민국 원입니다.
	KRW = MustParseCurrency("KRW")
	// USD 는 미국 달러입니다.
	USD = MustParseCurrency("USD")
	// EUR 는 유로입니다.
	EUR = MustParseCurrency("EUR")
	// CNY 는 중국 위안입니다.
	CNY = MustParseCurrency("CNY")
	// JPY 는 일본 엔입니다.
	JPY = MustParseCurrency("JPY")
)

// ParseCurrency 는 ISO 4217 alphabetic/numeric currency code를 Currency 로 변환합니다.
func ParseCurrency(code string) (Currency, error) {
	normalized := strings.ToUpper(strings.TrimSpace(code))
	if normalized == "" || normalized == "XXX" || normalized == "999" {
		return Currency{}, fmt.Errorf("%w: %q", ErrInvalidCurrency, code)
	}
	curr, err := gmoney.ParseCurr(normalized)
	if err != nil {
		return Currency{}, fmt.Errorf("%w: %w", ErrInvalidCurrency, err)
	}
	if curr == gmoney.XXX {
		return Currency{}, fmt.Errorf("%w: %q", ErrInvalidCurrency, code)
	}
	return Currency{curr: curr}, nil
}

// MustParseCurrency 는 ParseCurrency 와 같지만 실패하면 panic 합니다.
func MustParseCurrency(code string) Currency {
	curr, err := ParseCurrency(code)
	if err != nil {
		panic(err)
	}
	return curr
}

// IsCurrency 는 code가 #35 public wrapper에서 지원하는 유효 통화인지 반환합니다.
func IsCurrency(code string) bool {
	_, err := ParseCurrency(code)
	return err == nil
}

// CurrencyByLocale 은 제한된 BCP47-like locale tag에서 현재 지역 통화를 반환합니다.
func CurrencyByLocale(tag string) (Currency, error) {
	region, ok := localeRegion(tag)
	if !ok {
		return Currency{}, fmt.Errorf("%w: %q", ErrInvalidCurrency, tag)
	}
	code, ok := regionCurrencies[region]
	if !ok {
		return Currency{}, fmt.Errorf("%w: unsupported locale %q", ErrInvalidCurrency, tag)
	}
	return ParseCurrency(code)
}

// Code 는 3-letter ISO 4217 currency code를 반환합니다.
func (c Currency) Code() string {
	if c.IsZero() {
		return ""
	}
	return c.curr.Code()
}

// Num 는 ISO 4217 numeric currency code를 반환합니다.
func (c Currency) Num() string {
	if c.IsZero() {
		return ""
	}
	return c.curr.Num()
}

// Scale 은 통화의 기본 minor unit scale을 반환합니다.
func (c Currency) Scale() int {
	if c.IsZero() {
		return 0
	}
	return c.curr.Scale()
}

// String 은 통화 코드를 반환합니다.
func (c Currency) String() string {
	return c.Code()
}

// IsZero 는 zero-value 또는 no-currency 값을 invalid 통화로 판정합니다.
func (c Currency) IsZero() bool {
	return c.curr == gmoney.XXX
}

func (c Currency) validate() error {
	if c.IsZero() {
		return ErrInvalidCurrency
	}
	return nil
}

func (c Currency) raw() gmoney.Currency {
	return c.curr
}

func localeRegion(tag string) (string, bool) {
	normalized := strings.TrimSpace(tag)
	if normalized == "" {
		return "", false
	}
	normalized = strings.ReplaceAll(normalized, "_", "-")
	parts := strings.Split(normalized, "-")
	if len(parts) < 2 {
		return "", false
	}
	region := strings.ToUpper(parts[len(parts)-1])
	if len(region) != 2 {
		return "", false
	}
	return region, true
}

var regionCurrencies = map[string]string{
	"KR": "KRW",
	"US": "USD",
	"JP": "JPY",
	"CN": "CNY",
	"AT": "EUR",
	"BE": "EUR",
	"DE": "EUR",
	"ES": "EUR",
	"FI": "EUR",
	"FR": "EUR",
	"IE": "EUR",
	"IT": "EUR",
	"NL": "EUR",
	"PT": "EUR",
}
