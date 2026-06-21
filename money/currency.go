package money

import (
	"errors"
	"fmt"
	"strings"

	gmoney "github.com/govalues/money"
	xcurrency "golang.org/x/text/currency"
	"golang.org/x/text/language"
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

// CurrencyByLocale 은 BCP47 locale tag의 명시적 현재 지역 통화를 반환합니다.
func CurrencyByLocale(tag string) (Currency, error) {
	normalized := normalizeLocaleTag(tag)
	if normalized == "" {
		return Currency{}, fmt.Errorf("%w: %q", ErrInvalidCurrency, tag)
	}
	region, ok := explicitLocaleRegion(normalized)
	if !ok {
		return Currency{}, fmt.Errorf("%w: locale %q has no explicit region", ErrInvalidCurrency, tag)
	}
	if _, err := language.Parse(normalized); err != nil {
		var valueErr language.ValueError
		if !errors.As(err, &valueErr) {
			return Currency{}, fmt.Errorf("%w: invalid locale %q: %w", ErrInvalidCurrency, tag, err)
		}
	}
	return currencyByRegion(region, tag)
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

func normalizeLocaleTag(tag string) string {
	return strings.ReplaceAll(strings.TrimSpace(tag), "_", "-")
}

func explicitLocaleRegion(tag string) (language.Region, bool) {
	parts := strings.Split(tag, "-")
	if len(parts) < 2 {
		return language.Region{}, false
	}
	for _, part := range parts[1:] {
		if len(part) == 1 {
			return language.Region{}, false
		}
		if len(part) != 2 && len(part) != 3 {
			continue
		}
		region, err := language.ParseRegion(part)
		if err == nil {
			return region, true
		}
	}
	return language.Region{}, false
}

func currencyByRegion(region language.Region, originalTag string) (Currency, error) {
	iter := xcurrency.Query(xcurrency.Region(region))
	var code string
	count := 0
	for iter.Next() {
		if !iter.IsTender() {
			continue
		}
		count++
		if count == 1 {
			code = iter.Unit().String()
		}
	}
	if count != 1 {
		return Currency{}, fmt.Errorf("%w: locale %q maps to %d current tender currencies", ErrInvalidCurrency, originalTag, count)
	}
	curr, err := ParseCurrency(code)
	if err != nil {
		return Currency{}, fmt.Errorf("%w: locale %q maps to invalid currency %q: %w", ErrInvalidCurrency, originalTag, code, err)
	}
	return curr, nil
}
