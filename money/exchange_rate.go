package money

import (
	"fmt"
	"strings"

	gmoney "github.com/govalues/money"
)

// ExchangeRate 패키지에서 공개하는 구조체다.
type ExchangeRate struct {
	rate  gmoney.ExchangeRate
	valid bool
}

// NewExchangeRate ExchangeRate 인스턴스를 생성한다.
//
// 매개변수:
//   - base: NewExchangeRate에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - quote: NewExchangeRate에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - rate: NewExchangeRate가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func NewExchangeRate(base Currency, quote Currency, rate string) (ExchangeRate, error) {
	if err := base.validate(); err != nil {
		return ExchangeRate{}, err
	}
	if err := quote.validate(); err != nil {
		return ExchangeRate{}, err
	}
	normalized := strings.TrimSpace(rate)
	if normalized == "" {
		return ExchangeRate{}, ErrInvalidExchangeRate
	}
	parsed, err := gmoney.ParseExchRate(base.Code(), quote.Code(), normalized)
	if err != nil {
		return ExchangeRate{}, mapExchangeRateError(err)
	}
	if parsed.IsZero() || !parsed.IsPos() {
		return ExchangeRate{}, ErrInvalidExchangeRate
	}
	if base.raw() == quote.raw() && !parsed.IsOne() {
		return ExchangeRate{}, ErrInvalidExchangeRate
	}
	return ExchangeRate{rate: parsed, valid: true}, nil
}

// Convert 금액을 대상 통화로 변환한다.
//
// 매개변수:
//   - amount: Convert에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - rate: Convert에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func Convert(amount Money, rate ExchangeRate) (Money, error) {
	if err := amount.validate(); err != nil {
		return Money{}, err
	}
	if err := rate.validate(); err != nil {
		return Money{}, err
	}
	if !rate.canConvert(amount) {
		return Money{}, ErrCurrencyMismatch
	}
	converted, err := rate.rate.Conv(amount.amount)
	if err != nil {
		return Money{}, mapAmountError(err)
	}
	return Money{amount: converted.RoundToCurr(), valid: true}, nil
}

// Valid 값이 유효한지 반환한다.
func (r ExchangeRate) Valid() bool {
	return r.valid && !r.rate.IsZero() && r.rate.IsPos() &&
		r.rate.Base() != gmoney.XXX && r.rate.Quote() != gmoney.XXX
}

// Base 환율의 기준 통화를 반환한다.
func (r ExchangeRate) Base() Currency {
	if !r.Valid() {
		return Currency{}
	}
	return Currency{curr: r.rate.Base()}
}

// Quote 환율의 상대 통화를 반환한다.
func (r ExchangeRate) Quote() Currency {
	if !r.Valid() {
		return Currency{}
	}
	return Currency{curr: r.rate.Quote()}
}

// Rate 기준 통화와 대상 통화 사이의 환율을 반환한다.
func (r ExchangeRate) Rate() string {
	if !r.Valid() {
		return ""
	}
	return r.rate.Decimal().String()
}

// IsZero 값이 zero value인지 반환한다.
func (r ExchangeRate) IsZero() bool {
	return !r.Valid()
}

func (r ExchangeRate) canConvert(amount Money) bool {
	if !r.Valid() || amount.validate() != nil {
		return false
	}
	curr := amount.Currency().raw()
	return curr == r.rate.Base() || curr == r.rate.Quote()
}

func (r ExchangeRate) validate() error {
	if !r.Valid() {
		return ErrInvalidExchangeRate
	}
	return nil
}

func mapExchangeRateError(err error) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "currency"):
		return fmt.Errorf("%w: %w", ErrInvalidCurrency, err)
	case strings.Contains(message, "overflow"):
		return fmt.Errorf("%w: %w", ErrOverflow, err)
	default:
		return fmt.Errorf("%w: %w", ErrInvalidExchangeRate, err)
	}
}
