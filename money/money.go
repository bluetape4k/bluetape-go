package money

import (
	"fmt"
	"math"
	"strings"

	"github.com/govalues/decimal"
	gmoney "github.com/govalues/money"
)

// Money 는 명시적 통화를 가진 decimal-backed 금액 값입니다.
type Money struct {
	amount gmoney.Amount
	valid  bool
}

// New 는 문자열 금액과 통화로 Money 를 생성합니다.
func New(amount string, currency Currency) (Money, error) {
	if err := currency.validate(); err != nil {
		return Money{}, err
	}
	parsed, err := gmoney.ParseAmount(currency.Code(), strings.TrimSpace(amount))
	if err != nil {
		return Money{}, mapAmountError(err)
	}
	return Money{amount: parsed, valid: true}, nil
}

// NewFromInt64 는 major unit 정수 값으로 Money 를 생성합니다.
func NewFromInt64(units int64, currency Currency) (Money, error) {
	if err := currency.validate(); err != nil {
		return Money{}, err
	}
	amount, err := gmoney.NewAmountFromInt64(currency.Code(), units, 0, 0)
	if err != nil {
		return Money{}, mapAmountError(err)
	}
	return Money{amount: amount, valid: true}, nil
}

// NewFromFloat64 는 float64 금액으로 Money 를 생성합니다.
func NewFromFloat64(amount float64, currency Currency) (Money, error) {
	if math.IsNaN(amount) || math.IsInf(amount, 0) {
		return Money{}, fmt.Errorf("%w: special float %v", ErrInvalidAmount, amount)
	}
	if err := currency.validate(); err != nil {
		return Money{}, err
	}
	created, err := gmoney.NewAmountFromFloat64(currency.Code(), amount)
	if err != nil {
		return Money{}, mapAmountError(err)
	}
	return Money{amount: created, valid: true}, nil
}

// Zero 는 지정 통화의 0 금액을 생성합니다.
func Zero(currency Currency) (Money, error) {
	return New("0", currency)
}

// NewMinor 는 통화 minor unit 정수 값으로 Money 를 생성합니다.
func NewMinor(units int64, currency Currency) (Money, error) {
	if err := currency.validate(); err != nil {
		return Money{}, err
	}
	amount, err := gmoney.NewAmountFromMinorUnits(currency.Code(), units)
	if err != nil {
		return Money{}, mapAmountError(err)
	}
	return Money{amount: amount, valid: true}, nil
}

// Parse 는 `USD 12.34` 형식의 텍스트를 Money 로 변환합니다.
func Parse(s string) (Money, error) {
	code, amount, err := splitText(s)
	if err != nil {
		return Money{}, err
	}
	curr, err := ParseCurrency(code)
	if err != nil {
		return Money{}, err
	}
	return New(amount, curr)
}

// KRWAmount 는 대한민국 원 금액을 생성합니다.
func KRWAmount(amount string) (Money, error) {
	return New(amount, KRW)
}

// USDAmount 는 미국 달러 금액을 생성합니다.
func USDAmount(amount string) (Money, error) {
	return New(amount, USD)
}

// EURAmount 는 유로 금액을 생성합니다.
func EURAmount(amount string) (Money, error) {
	return New(amount, EUR)
}

// CNYAmount 는 중국 위안 금액을 생성합니다.
func CNYAmount(amount string) (Money, error) {
	return New(amount, CNY)
}

// JPYAmount 는 일본 엔 금액을 생성합니다.
func JPYAmount(amount string) (Money, error) {
	return New(amount, JPY)
}

// Currency 는 Money 의 통화를 반환합니다.
func (m Money) Currency() Currency {
	if !m.valid {
		return Currency{}
	}
	return Currency{curr: m.amount.Curr()}
}

// String 은 `USD 12.34` 형식의 금액 문자열을 반환합니다.
func (m Money) String() string {
	if !m.valid {
		return ""
	}
	return m.amount.String()
}

// Amount 는 통화를 제외한 decimal 금액 문자열을 반환합니다.
func (m Money) Amount() string {
	if !m.valid {
		return ""
	}
	return m.amount.Decimal().String()
}

// MinorUnits 는 통화 minor unit 정수 값을 반환합니다.
func (m Money) MinorUnits() (int64, error) {
	if err := m.validate(); err != nil {
		return 0, err
	}
	units, ok := m.amount.MinorUnits()
	if !ok {
		return 0, ErrOverflow
	}
	return units, nil
}

// Float64 는 float64 금액을 반환합니다.
func (m Money) Float64() (float64, error) {
	if err := m.validate(); err != nil {
		return 0, err
	}
	value, ok := m.amount.Float64()
	if !ok {
		return 0, ErrOverflow
	}
	return value, nil
}

// IsZero 는 invalid zero-value Money 인지 반환합니다.
func (m Money) IsZero() bool {
	return !m.valid
}

// Round 는 통화 scale에 맞춰 half-even rounding을 적용합니다.
func (m Money) Round() (Money, error) {
	if err := m.validate(); err != nil {
		return Money{}, err
	}
	return Money{amount: m.amount.RoundToCurr(), valid: true}, nil
}

// RoundTo 는 지정 scale에 맞춰 half-even rounding을 적용합니다.
func (m Money) RoundTo(scale int) (Money, error) {
	if err := m.validate(); err != nil {
		return Money{}, err
	}
	if scale < 0 {
		return Money{}, fmt.Errorf("%w: negative scale %d", ErrInvalidAmount, scale)
	}
	return Money{amount: m.amount.Round(scale), valid: true}, nil
}

// Add 는 같은 통화의 금액을 더합니다.
func (m Money) Add(other Money) (Money, error) {
	if err := validatePair(m, other); err != nil {
		return Money{}, err
	}
	amount, err := m.amount.Add(other.amount)
	if err != nil {
		return Money{}, mapAmountError(err)
	}
	return Money{amount: amount, valid: true}, nil
}

// Sub 는 같은 통화의 금액을 뺍니다.
func (m Money) Sub(other Money) (Money, error) {
	if err := validatePair(m, other); err != nil {
		return Money{}, err
	}
	amount, err := m.amount.Sub(other.amount)
	if err != nil {
		return Money{}, mapAmountError(err)
	}
	return Money{amount: amount, valid: true}, nil
}

// Neg 는 부호를 반전합니다.
func (m Money) Neg() (Money, error) {
	if err := m.validate(); err != nil {
		return Money{}, err
	}
	zero, err := Zero(m.Currency())
	if err != nil {
		return Money{}, err
	}
	return zero.Sub(m)
}

// Abs 는 절댓값 금액을 반환합니다.
func (m Money) Abs() (Money, error) {
	if err := m.validate(); err != nil {
		return Money{}, err
	}
	if m.amount.IsNeg() {
		return m.Neg()
	}
	return m, nil
}

// Cmp 는 같은 통화의 금액을 비교합니다.
func (m Money) Cmp(other Money) (int, error) {
	if err := validatePair(m, other); err != nil {
		return 0, err
	}
	cmp, err := m.amount.Cmp(other.amount)
	if err != nil {
		return 0, mapAmountError(err)
	}
	return cmp, nil
}

// Equal 은 같은 통화와 금액인지 반환합니다.
func (m Money) Equal(other Money) bool {
	cmp, err := m.Cmp(other)
	return err == nil && cmp == 0
}

// Mul 은 decimal 문자열 스칼라를 곱합니다.
func (m Money) Mul(factor string) (Money, error) {
	if err := m.validate(); err != nil {
		return Money{}, err
	}
	scalar, err := parseScalar(factor)
	if err != nil {
		return Money{}, err
	}
	amount, err := m.amount.Mul(scalar)
	if err != nil {
		return Money{}, mapAmountError(err)
	}
	return Money{amount: amount, valid: true}, nil
}

// Quo 는 decimal 문자열 스칼라로 나눕니다.
func (m Money) Quo(divisor string) (Money, error) {
	if err := m.validate(); err != nil {
		return Money{}, err
	}
	scalar, err := parseScalar(divisor)
	if err != nil {
		return Money{}, err
	}
	if scalar.IsZero() {
		return Money{}, ErrDivideByZero
	}
	amount, err := m.amount.Quo(scalar)
	if err != nil {
		return Money{}, mapAmountError(err)
	}
	return Money{amount: amount, valid: true}, nil
}

// Sum 은 같은 통화 금액을 합산합니다.
func Sum(currency Currency, values ...Money) (Money, error) {
	total, err := Zero(currency)
	if err != nil {
		return Money{}, err
	}
	for _, value := range values {
		if err := value.validate(); err != nil {
			return Money{}, err
		}
		if !sameCurrency(currency, value.Currency()) {
			return Money{}, ErrCurrencyMismatch
		}
		total, err = total.Add(value)
		if err != nil {
			return Money{}, err
		}
	}
	return total, nil
}

func (m Money) validate() error {
	if !m.valid || m.amount.Curr() == gmoney.XXX {
		return ErrInvalidMoney
	}
	return nil
}

func validatePair(left, right Money) error {
	if err := left.validate(); err != nil {
		return err
	}
	if err := right.validate(); err != nil {
		return err
	}
	if !sameCurrency(left.Currency(), right.Currency()) {
		return ErrCurrencyMismatch
	}
	return nil
}

func sameCurrency(left, right Currency) bool {
	return !left.IsZero() && !right.IsZero() && left.raw() == right.raw()
}

func parseScalar(value string) (decimal.Decimal, error) {
	scalar, err := decimal.Parse(strings.TrimSpace(value))
	if err != nil {
		message := strings.ToLower(err.Error())
		if strings.Contains(message, "overflow") || strings.Contains(message, "integer part") {
			return decimal.Decimal{}, fmt.Errorf("%w: %w", ErrOverflow, err)
		}
		return decimal.Decimal{}, fmt.Errorf("%w: %w", ErrInvalidAmount, err)
	}
	return scalar, nil
}

func splitText(s string) (string, string, error) {
	fields := strings.Fields(s)
	if len(fields) != 2 {
		return "", "", fmt.Errorf("%w: expected '<currency> <amount>'", ErrInvalidMoney)
	}
	return fields[0], fields[1], nil
}

func mapAmountError(err error) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "currency"):
		return fmt.Errorf("%w: %w", ErrCurrencyMismatch, err)
	case strings.Contains(message, "division by zero") || strings.Contains(message, "divisor is zero"):
		return fmt.Errorf("%w: %w", ErrDivideByZero, err)
	case strings.Contains(message, "overflow") || strings.Contains(message, "integer part"):
		return fmt.Errorf("%w: %w", ErrOverflow, err)
	default:
		return fmt.Errorf("%w: %w", ErrInvalidAmount, err)
	}
}
