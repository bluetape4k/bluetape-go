package money

import (
	"fmt"
	"math"
	"strings"

	"github.com/govalues/decimal"
	gmoney "github.com/govalues/money"
)

// Money는 struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Money struct {
	amount gmoney.Amount
	valid  bool
}

// New는 New 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - amount: New가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//   - currency: New 동작에 필요한 currency 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// NewFromInt64는 NewFromInt64 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - units: NewFromInt64 동작에 필요한 units 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - currency: NewFromInt64 동작에 필요한 currency 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// NewFromFloat64는 NewFromFloat64 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - amount: NewFromFloat64 동작에 필요한 amount 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - currency: NewFromFloat64 동작에 필요한 currency 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// Zero는 Zero 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - currency: Zero 동작에 필요한 currency 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func Zero(currency Currency) (Money, error) {
	return New("0", currency)
}

// NewMinor는 NewMinor 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - units: NewMinor 동작에 필요한 units 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - currency: NewMinor 동작에 필요한 currency 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// Parse는 Parse 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - s: Parse가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// KRWAmount는 KRWAmount 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - amount: KRWAmount가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func KRWAmount(amount string) (Money, error) {
	return New(amount, KRW)
}

// USDAmount는 USDAmount 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - amount: USDAmount가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func USDAmount(amount string) (Money, error) {
	return New(amount, USD)
}

// EURAmount는 EURAmount 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - amount: EURAmount가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func EURAmount(amount string) (Money, error) {
	return New(amount, EUR)
}

// CNYAmount는 CNYAmount 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - amount: CNYAmount가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func CNYAmount(amount string) (Money, error) {
	return New(amount, CNY)
}

// JPYAmount는 JPYAmount 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - amount: JPYAmount가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func JPYAmount(amount string) (Money, error) {
	return New(amount, JPY)
}

// Currency는 Currency 공개 API의 동작을 수행한다.
func (m Money) Currency() Currency {
	if !m.valid {
		return Currency{}
	}
	return Currency{curr: m.amount.Curr()}
}

// String는 String 공개 API의 동작을 수행한다.
func (m Money) String() string {
	if !m.valid {
		return ""
	}
	return m.amount.String()
}

// Amount는 Amount 공개 API의 동작을 수행한다.
func (m Money) Amount() string {
	if !m.valid {
		return ""
	}
	return m.amount.Decimal().String()
}

// MinorUnits는 MinorUnits 공개 API의 동작을 수행한다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// Float64는 Float64 공개 API의 동작을 수행한다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// IsZero는 IsZero 공개 API의 동작을 수행한다.
func (m Money) IsZero() bool {
	return !m.valid
}

// Round는 Round 공개 API의 동작을 수행한다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (m Money) Round() (Money, error) {
	if err := m.validate(); err != nil {
		return Money{}, err
	}
	return Money{amount: m.amount.RoundToCurr(), valid: true}, nil
}

// RoundTo는 RoundTo 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - scale: RoundTo 동작에 필요한 scale 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (m Money) RoundTo(scale int) (Money, error) {
	if err := m.validate(); err != nil {
		return Money{}, err
	}
	if scale < 0 {
		return Money{}, fmt.Errorf("%w: negative scale %d", ErrInvalidAmount, scale)
	}
	return Money{amount: m.amount.Round(scale), valid: true}, nil
}

// Add는 Add 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - other: Add 동작에 필요한 other 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// Sub는 Sub 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - other: Sub 동작에 필요한 other 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// Neg는 Neg 공개 API의 동작을 수행한다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// Abs는 Abs 공개 API의 동작을 수행한다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (m Money) Abs() (Money, error) {
	if err := m.validate(); err != nil {
		return Money{}, err
	}
	if m.amount.IsNeg() {
		return m.Neg()
	}
	return m, nil
}

// Cmp는 Cmp 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - other: Cmp 동작에 필요한 other 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// Equal는 Equal 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - other: Equal 동작에 필요한 other 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func (m Money) Equal(other Money) bool {
	cmp, err := m.Cmp(other)
	return err == nil && cmp == 0
}

// Mul는 Mul 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - factor: Mul가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// Quo는 Quo 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - divisor: Quo가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// Sum는 Sum 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - currency: Sum 동작에 필요한 currency 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - values: Sum 동작에 필요한 values 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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
