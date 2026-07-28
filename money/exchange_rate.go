package money

import (
	"fmt"
	"strings"

	gmoney "github.com/govalues/money"
)

// ExchangeRate struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type ExchangeRate struct {
	rate  gmoney.ExchangeRate
	valid bool
}

// NewExchangeRate NewExchangeRate 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - base: NewExchangeRate 동작에 필요한 base 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - quote: NewExchangeRate 동작에 필요한 quote 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - rate: NewExchangeRate가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// Convert Convert 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - amount: Convert 동작에 필요한 amount 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - rate: Convert 동작에 필요한 rate 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// Valid Valid 공개 API의 동작을 수행한다.
func (r ExchangeRate) Valid() bool {
	return r.valid && !r.rate.IsZero() && r.rate.IsPos() &&
		r.rate.Base() != gmoney.XXX && r.rate.Quote() != gmoney.XXX
}

// Base Base 공개 API의 동작을 수행한다.
func (r ExchangeRate) Base() Currency {
	if !r.Valid() {
		return Currency{}
	}
	return Currency{curr: r.rate.Base()}
}

// Quote Quote 공개 API의 동작을 수행한다.
func (r ExchangeRate) Quote() Currency {
	if !r.Valid() {
		return Currency{}
	}
	return Currency{curr: r.rate.Quote()}
}

// Rate Rate 공개 API의 동작을 수행한다.
func (r ExchangeRate) Rate() string {
	if !r.Valid() {
		return ""
	}
	return r.rate.Decimal().String()
}

// IsZero IsZero 공개 API의 동작을 수행한다.
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
