package measure

import (
	"fmt"
	"math"
)

// Measure 패키지에서 공개하는 구조체다.
type Measure[D any] struct {
	amount float64
	unit   Unit[D]
}

// New 값 인스턴스를 생성한다.
//
// 매개변수:
//   - amount: New에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - unit: New에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func New[D any](amount float64, unit Unit[D]) (Measure[D], error) {
	if !finite(amount) {
		return Measure[D]{}, fmt.Errorf("%w: amount must be finite", ErrInvalidMeasure)
	}
	if err := unit.validate(); err != nil {
		return Measure[D]{}, fmt.Errorf("%w: %w", ErrInvalidMeasure, err)
	}
	return Measure[D]{amount: amount, unit: unit}, nil
}

// Must 결과 값이 오류이면 panic하고 성공 값을 반환한다.
//
// 매개변수:
//   - amount: Must에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - unit: Must에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
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

// Amount 금액 값을 반환한다.
func (m Measure[D]) Amount() float64 {
	return m.amount
}

// Unit 값에 연결된 단위 정보를 반환한다.
func (m Measure[D]) Unit() Unit[D] {
	return m.unit
}

// BaseAmount 해당 통화 금액을 생성한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// In 값을 대상 단위나 표현으로 변환한다.
//
// 매개변수:
//   - unit: In에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// As 측정값을 대상 단위로 변환한다.
//
// 매개변수:
//   - unit: As에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func (m Measure[D]) As(unit Unit[D]) (Measure[D], error) {
	value, err := m.In(unit)
	if err != nil {
		return Measure[D]{}, err
	}
	return New(value, unit)
}

// Add 현재 값에 입력 값을 더한 결과를 반환한다.
//
// 매개변수:
//   - other: Add에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// Sub 현재 값에서 입력 값을 뺀 결과를 반환한다.
//
// 매개변수:
//   - other: Sub에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// MulScalar 측정값에 scalar를 곱한다.
//
// 매개변수:
//   - scalar: MulScalar에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func (m Measure[D]) MulScalar(scalar float64) (Measure[D], error) {
	if err := m.validate(); err != nil {
		return Measure[D]{}, err
	}
	if !finite(scalar) {
		return Measure[D]{}, fmt.Errorf("%w: scalar must be finite", ErrInvalidMeasure)
	}
	return New(m.amount*scalar, m.unit)
}

// DivScalar 측정값을 scalar로 나눈다.
//
// 매개변수:
//   - scalar: DivScalar에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// Neg 부호를 반전한 값을 반환한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func (m Measure[D]) Neg() (Measure[D], error) {
	if err := m.validate(); err != nil {
		return Measure[D]{}, err
	}
	return New(-m.amount, m.unit)
}

// Compare 두 값을 정렬 순서로 비교한다.
//
// 매개변수:
//   - other: Compare에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// ToNearest 값을 대상 단위나 표현으로 변환한다.
//
// 매개변수:
//   - nearest: ToNearest에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func (m Measure[D]) ToNearest(nearest float64) (Measure[D], error) {
	if err := m.validate(); err != nil {
		return Measure[D]{}, err
	}
	if !finite(nearest) || nearest <= 0 {
		return Measure[D]{}, fmt.Errorf("%w: nearest must be finite and positive", ErrInvalidMeasure)
	}
	return New(math.Round(m.amount/nearest)*nearest, m.unit)
}

// Format 값을 지정한 형식의 문자열로 변환한다.
//
// 매개변수:
//   - unit: Format에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func (m Measure[D]) Format(unit Unit[D]) (string, error) {
	value, err := m.In(unit)
	if err != nil {
		return "", err
	}
	return formatValue(value, unit), nil
}

// Human 측정값을 사람이 읽기 쉬운 문자열로 변환한다.
//
// 매개변수:
//   - candidates: Human에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
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

// String 값을 사람이 읽을 수 있는 문자열로 반환한다.
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
