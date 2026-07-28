package money

import (
	"errors"
	"fmt"
	"strings"

	gmoney "github.com/govalues/money"
	xcurrency "golang.org/x/text/currency"
	"golang.org/x/text/language"
)

// Currency struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
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

// ParseCurrency ParseCurrency 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - code: ParseCurrency가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// MustParseCurrency MustParseCurrency 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - code: MustParseCurrency가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
func MustParseCurrency(code string) Currency {
	curr, err := ParseCurrency(code)
	if err != nil {
		panic(err)
	}
	return curr
}

// IsCurrency IsCurrency 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - code: IsCurrency가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
func IsCurrency(code string) bool {
	_, err := ParseCurrency(code)
	return err == nil
}

// CurrencyByLocale CurrencyByLocale 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - tag: CurrencyByLocale가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// Code Code 공개 API의 동작을 수행한다.
func (c Currency) Code() string {
	if c.IsZero() {
		return ""
	}
	return c.curr.Code()
}

// Num Num 공개 API의 동작을 수행한다.
func (c Currency) Num() string {
	if c.IsZero() {
		return ""
	}
	return c.curr.Num()
}

// Scale Scale 공개 API의 동작을 수행한다.
func (c Currency) Scale() int {
	if c.IsZero() {
		return 0
	}
	return c.curr.Scale()
}

// String String 공개 API의 동작을 수행한다.
func (c Currency) String() string {
	return c.Code()
}

// IsZero IsZero 공개 API의 동작을 수행한다.
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
