package money

import (
	"errors"
	"fmt"
	"strings"

	gmoney "github.com/govalues/money"
	xcurrency "golang.org/x/text/currency"
	"golang.org/x/text/language"
)

// Currency 패키지에서 공개하는 구조체다.
type Currency struct {
	curr gmoney.Currency
}

var (
	// KRW 대한민국 원입니다.
	KRW = MustParseCurrency("KRW")
	// USD 미국 달러입니다.
	USD = MustParseCurrency("USD")
	// EUR 유로입니다.
	EUR = MustParseCurrency("EUR")
	// CNY 중국 위안입니다.
	CNY = MustParseCurrency("CNY")
	// JPY 일본 엔입니다.
	JPY = MustParseCurrency("JPY")
)

// ParseCurrency 문자열 입력을 도메인 값으로 해석한다.
//
// 매개변수:
//   - code: ParseCurrency가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// MustParseCurrency 문자열 입력을 도메인 값으로 해석하고 실패하면 panic한다.
//
// 매개변수:
//   - code: MustParseCurrency가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
func MustParseCurrency(code string) Currency {
	curr, err := ParseCurrency(code)
	if err != nil {
		panic(err)
	}
	return curr
}

// IsCurrency 값이 조건을 만족하는지 반환한다.
//
// 매개변수:
//   - code: IsCurrency가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
func IsCurrency(code string) bool {
	_, err := ParseCurrency(code)
	return err == nil
}

// CurrencyByLocale locale tag에 맞는 기본 통화를 찾는다.
//
// 매개변수:
//   - tag: CurrencyByLocale가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// Code 통화 코드를 반환한다.
func (c Currency) Code() string {
	if c.IsZero() {
		return ""
	}
	return c.curr.Code()
}

// Num ISO 4217 numeric code를 반환한다.
func (c Currency) Num() string {
	if c.IsZero() {
		return ""
	}
	return c.curr.Num()
}

// Scale 측정값에 배율을 적용한다.
func (c Currency) Scale() int {
	if c.IsZero() {
		return 0
	}
	return c.curr.Scale()
}

// String 값을 사람이 읽을 수 있는 문자열로 반환한다.
func (c Currency) String() string {
	return c.Code()
}

// IsZero 값이 zero value인지 반환한다.
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
