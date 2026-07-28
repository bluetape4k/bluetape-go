package measure

import (
	"fmt"
	"math"
)

// Measure는 struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Measure[D any] struct {
	amount float64
	unit   Unit[D]
}

// New는 New 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - amount: New 동작에 필요한 amount 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - unit: New 동작에 필요한 unit 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func New[D any](amount float64, unit Unit[D]) (Measure[D], error) {
	if !finite(amount) {
		return Measure[D]{}, fmt.Errorf("%w: amount must be finite", ErrInvalidMeasure)
	}
	if err := unit.validate(); err != nil {
		return Measure[D]{}, fmt.Errorf("%w: %w", ErrInvalidMeasure, err)
	}
	return Measure[D]{amount: amount, unit: unit}, nil
}

// Must는 Must 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - amount: Must 동작에 필요한 amount 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - unit: Must 동작에 필요한 unit 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func Must[D any](amount float64, unit Unit[D]) Measure[D] {
	measure, err := New(amount, unit)
	if err != nil {
		panic(err)
	}
	return measure
}

func (m Measure[D]) validate() error {
	if !finite(m.amount) {
		return fmt.Errorf("%w: amount must be finite", ErrInvalidMeasure)
	}
	if err := m.unit.validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidMeasure, err)
	}
	return nil
}

// Amount는 Amount 공개 API의 동작을 수행한다.
func (m Measure[D]) Amount() float64 {
	return m.amount
}

// Unit는 Unit 공개 API의 동작을 수행한다.
func (m Measure[D]) Unit() Unit[D] {
	return m.unit
}

// BaseAmount는 BaseAmount 공개 API의 동작을 수행한다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (m Measure[D]) BaseAmount() (float64, error) {
	if err := m.validate(); err != nil {
		return 0, err
	}
	value := m.amount * m.unit.ratio
	if !finite(value) {
		return 0, fmt.Errorf("%w: base amount overflow", ErrInvalidMeasure)
	}
	return value, nil
}

// In는 In 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - unit: In 동작에 필요한 unit 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (m Measure[D]) In(unit Unit[D]) (float64, error) {
	if err := m.validate(); err != nil {
		return 0, err
	}
	if err := unit.validate(); err != nil {
		return 0, fmt.Errorf("%w: %w", ErrInvalidUnit, err)
	}
	value := m.amount * (m.unit.ratio / unit.ratio)
	if !finite(value) {
		return 0, fmt.Errorf("%w: conversion overflow", ErrInvalidMeasure)
	}
	return value, nil
}

// As는 As 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - unit: As 동작에 필요한 unit 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (m Measure[D]) As(unit Unit[D]) (Measure[D], error) {
	value, err := m.In(unit)
	if err != nil {
		return Measure[D]{}, err
	}
	return New(value, unit)
}

// Add는 Add 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - other: Add 동작에 필요한 other 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (m Measure[D]) Add(other Measure[D]) (Measure[D], error) {
	if err := m.validate(); err != nil {
		return Measure[D]{}, err
	}
	if err := other.validate(); err != nil {
		return Measure[D]{}, err
	}
	unit := minUnit(m.unit, other.unit)
	left, err := m.In(unit)
	if err != nil {
		return Measure[D]{}, err
	}
	right, err := other.In(unit)
	if err != nil {
		return Measure[D]{}, err
	}
	return New(left+right, unit)
}

// Sub는 Sub 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - other: Sub 동작에 필요한 other 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (m Measure[D]) Sub(other Measure[D]) (Measure[D], error) {
	if err := m.validate(); err != nil {
		return Measure[D]{}, err
	}
	if err := other.validate(); err != nil {
		return Measure[D]{}, err
	}
	unit := minUnit(m.unit, other.unit)
	left, err := m.In(unit)
	if err != nil {
		return Measure[D]{}, err
	}
	right, err := other.In(unit)
	if err != nil {
		return Measure[D]{}, err
	}
	return New(left-right, unit)
}

// MulScalar는 MulScalar 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - scalar: MulScalar 동작에 필요한 scalar 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (m Measure[D]) MulScalar(scalar float64) (Measure[D], error) {
	if err := m.validate(); err != nil {
		return Measure[D]{}, err
	}
	if !finite(scalar) {
		return Measure[D]{}, fmt.Errorf("%w: scalar must be finite", ErrInvalidMeasure)
	}
	return New(m.amount*scalar, m.unit)
}

// DivScalar는 DivScalar 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - scalar: DivScalar 동작에 필요한 scalar 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (m Measure[D]) DivScalar(scalar float64) (Measure[D], error) {
	if err := m.validate(); err != nil {
		return Measure[D]{}, err
	}
	if !finite(scalar) {
		return Measure[D]{}, fmt.Errorf("%w: scalar must be finite", ErrInvalidMeasure)
	}
	if scalar == 0 {
		return Measure[D]{}, fmt.Errorf("%w: scalar must be non-zero", ErrDivideByZero)
	}
	return New(m.amount/scalar, m.unit)
}

// Neg는 Neg 공개 API의 동작을 수행한다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (m Measure[D]) Neg() (Measure[D], error) {
	if err := m.validate(); err != nil {
		return Measure[D]{}, err
	}
	return New(-m.amount, m.unit)
}

// Compare는 Compare 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - other: Compare 동작에 필요한 other 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (m Measure[D]) Compare(other Measure[D]) (int, error) {
	if err := m.validate(); err != nil {
		return 0, err
	}
	if err := other.validate(); err != nil {
		return 0, err
	}
	left, err := m.In(other.unit)
	if err != nil {
		return 0, err
	}
	switch {
	case left < other.amount:
		return -1, nil
	case left > other.amount:
		return 1, nil
	default:
		return 0, nil
	}
}

// ToNearest는 ToNearest 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - nearest: ToNearest 동작에 필요한 nearest 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (m Measure[D]) ToNearest(nearest float64) (Measure[D], error) {
	if err := m.validate(); err != nil {
		return Measure[D]{}, err
	}
	if !finite(nearest) || nearest <= 0 {
		return Measure[D]{}, fmt.Errorf("%w: nearest must be finite and positive", ErrInvalidMeasure)
	}
	return New(math.Round(m.amount/nearest)*nearest, m.unit)
}

// Format는 Format 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - unit: Format 동작에 필요한 unit 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (m Measure[D]) Format(unit Unit[D]) (string, error) {
	value, err := m.In(unit)
	if err != nil {
		return "", err
	}
	return formatValue(value, unit), nil
}

// Human는 Human 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - candidates: Human 동작에 필요한 candidates 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (m Measure[D]) Human(candidates ...Unit[D]) (string, error) {
	if len(candidates) == 0 {
		return m.Format(m.unit)
	}
	if err := m.validate(); err != nil {
		return "", err
	}
	best := candidates[0]
	bestValue, err := m.In(best)
	if err != nil {
		return "", err
	}
	for _, candidate := range candidates[1:] {
		value, err := m.In(candidate)
		if err != nil {
			return "", err
		}
		if math.Abs(value) >= 1 && candidate.ratio >= best.ratio {
			best = candidate
			bestValue = value
		}
	}
	if math.Abs(bestValue) < 1 {
		smallest := candidates[0]
		for _, candidate := range candidates[1:] {
			if candidate.ratio < smallest.ratio {
				smallest = candidate
			}
		}
		best = smallest
		bestValue, err = m.In(best)
		if err != nil {
			return "", err
		}
	}
	return formatValue(bestValue, best), nil
}

// String는 String 공개 API의 동작을 수행한다.
func (m Measure[D]) String() string {
	if err := m.validate(); err != nil {
		return "<invalid measure>"
	}
	return formatValue(m.amount, m.unit)
}

func minUnit[D any](first, second Unit[D]) Unit[D] {
	if first.ratio <= second.ratio {
		return first
	}
	return second
}
