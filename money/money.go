package money

import (
	"fmt"
	"math"
	"strings"

	"github.com/govalues/decimal"
	gmoney "github.com/govalues/money"
)

// Money 패키지에서 공개하는 구조체다.
type Money struct {
	amount gmoney.Amount
	valid  bool
}

// New 값 인스턴스를 생성한다.
//
// 매개변수:
//   - amount: New가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//   - currency: New에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// NewFromInt64 FromInt64 인스턴스를 생성한다.
//
// 매개변수:
//   - units: NewFromInt64에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - currency: NewFromInt64에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// NewFromFloat64 FromFloat64 인스턴스를 생성한다.
//
// 매개변수:
//   - amount: NewFromFloat64에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - currency: NewFromFloat64에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// Zero 타입 T의 zero value를 반환한다.
//
// 매개변수:
//   - currency: Zero에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func Zero(currency Currency) (Money, error) {
	return New("0", currency)
}

// NewMinor Minor 인스턴스를 생성한다.
//
// 매개변수:
//   - units: NewMinor에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - currency: NewMinor에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// Parse 문자열 입력을 도메인 값으로 해석한다.
//
// 매개변수:
//   - s: Parse가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// KRWAmount 해당 통화 금액을 생성한다.
//
// 매개변수:
//   - amount: KRWAmount가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func KRWAmount(amount string) (Money, error) {
	return New(amount, KRW)
}

// USDAmount 해당 통화 금액을 생성한다.
//
// 매개변수:
//   - amount: USDAmount가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func USDAmount(amount string) (Money, error) {
	return New(amount, USD)
}

// EURAmount 해당 통화 금액을 생성한다.
//
// 매개변수:
//   - amount: EURAmount가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func EURAmount(amount string) (Money, error) {
	return New(amount, EUR)
}

// CNYAmount 해당 통화 금액을 생성한다.
//
// 매개변수:
//   - amount: CNYAmount가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func CNYAmount(amount string) (Money, error) {
	return New(amount, CNY)
}

// JPYAmount 해당 통화 금액을 생성한다.
//
// 매개변수:
//   - amount: JPYAmount가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func JPYAmount(amount string) (Money, error) {
	return New(amount, JPY)
}

// Currency 금액의 통화를 반환한다.
func (m Money) Currency() Currency {
	if !m.valid {
		return Currency{}
	}
	return Currency{curr: m.amount.Curr()}
}

// String 값을 사람이 읽을 수 있는 문자열로 반환한다.
func (m Money) String() string {
	if !m.valid {
		return ""
	}
	return m.amount.String()
}

// Amount 금액 값을 반환한다.
func (m Money) Amount() string {
	if !m.valid {
		return ""
	}
	return m.amount.Decimal().String()
}

// MinorUnits 통화의 소수 자리 수를 반환한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// Float64 금액을 float64 값으로 반환한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// IsZero 값이 zero value인지 반환한다.
func (m Money) IsZero() bool {
	return !m.valid
}

// Round 측정값을 지정한 단위에 맞춰 반올림한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func (m Money) Round() (Money, error) {
	if err := m.validate(); err != nil {
		return Money{}, err
	}
	return Money{amount: m.amount.RoundToCurr(), valid: true}, nil
}

// RoundTo 금액을 지정한 소수 자리로 반올림한다.
//
// 매개변수:
//   - scale: RoundTo에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func (m Money) RoundTo(scale int) (Money, error) {
	if err := m.validate(); err != nil {
		return Money{}, err
	}
	if scale < 0 {
		return Money{}, fmt.Errorf("%w: negative scale %d", ErrInvalidAmount, scale)
	}
	return Money{amount: m.amount.Round(scale), valid: true}, nil
}

// Add 현재 값에 입력 값을 더한 결과를 반환한다.
//
// 매개변수:
//   - other: Add에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// Sub 현재 값에서 입력 값을 뺀 결과를 반환한다.
//
// 매개변수:
//   - other: Sub에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// Neg 부호를 반전한 값을 반환한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// Abs 금액의 절댓값을 반환한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func (m Money) Abs() (Money, error) {
	if err := m.validate(); err != nil {
		return Money{}, err
	}
	if m.amount.IsNeg() {
		return m.Neg()
	}
	return m, nil
}

// Cmp 두 금액을 비교한다.
//
// 매개변수:
//   - other: Cmp에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// Equal 두 값이 같은지 반환한다.
//
// 매개변수:
//   - other: Equal에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func (m Money) Equal(other Money) bool {
	cmp, err := m.Cmp(other)
	return err == nil && cmp == 0
}

// Mul 현재 값에 입력 값을 곱한 결과를 반환한다.
//
// 매개변수:
//   - factor: Mul가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// Quo 현재 값을 입력 값으로 나눈 결과를 반환한다.
//
// 매개변수:
//   - divisor: Quo가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// Sum 측정값 목록의 합계를 반환한다.
//
// 매개변수:
//   - currency: Sum에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - values: Sum에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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
