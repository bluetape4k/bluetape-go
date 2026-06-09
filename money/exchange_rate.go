package money

import (
	"fmt"
	"strings"

	gmoney "github.com/govalues/money"
)

// ExchangeRate 는 두 통화 사이의 caller-supplied 환율 값입니다.
type ExchangeRate struct {
	rate  gmoney.ExchangeRate
	valid bool
}

// NewExchangeRate 는 base/quote 통화와 decimal 문자열 환율로 ExchangeRate 를 생성합니다.
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

// Convert 는 Money 를 ExchangeRate 의 반대쪽 통화로 변환합니다.
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

// Valid 는 환율이 변환 가능한 값인지 반환합니다.
func (r ExchangeRate) Valid() bool {
	return r.valid && !r.rate.IsZero() && r.rate.IsPos() &&
		r.rate.Base() != gmoney.XXX && r.rate.Quote() != gmoney.XXX
}

// Base 는 기준 통화를 반환합니다.
func (r ExchangeRate) Base() Currency {
	if !r.Valid() {
		return Currency{}
	}
	return Currency{curr: r.rate.Base()}
}

// Quote 는 상대 통화를 반환합니다.
func (r ExchangeRate) Quote() Currency {
	if !r.Valid() {
		return Currency{}
	}
	return Currency{curr: r.rate.Quote()}
}

// Rate 는 decimal 환율 문자열을 반환합니다.
func (r ExchangeRate) Rate() string {
	if !r.Valid() {
		return ""
	}
	return r.rate.Decimal().String()
}

// IsZero 는 invalid zero-value ExchangeRate 인지 반환합니다.
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
