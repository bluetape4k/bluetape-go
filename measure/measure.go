package measure

import (
	"fmt"
	"math"
)

// Measure  수치와 Unit을 함께 보관하는 타입 지정 측정값입니다.
type Measure[D any] struct {
	amount float64
	unit   Unit[D]
}

// New  수치와 단위의 유효성을 검사해 Measure를 생성합니다.
func New[D any](amount float64, unit Unit[D]) (Measure[D], error) {
	if !finite(amount) {
		return Measure[D]{}, fmt.Errorf("%w: amount must be finite", ErrInvalidMeasure)
	}
	if err := unit.validate(); err != nil {
		return Measure[D]{}, fmt.Errorf("%w: %w", ErrInvalidMeasure, err)
	}
	return Measure[D]{amount: amount, unit: unit}, nil
}

// Must  유효하지 않은 측정값이면 panic을 발생시킵니다.
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

// Amount  현재 단위 기준 수치값을 반환합니다.
func (m Measure[D]) Amount() float64 {
	return m.amount
}

// Unit  현재 단위를 반환합니다.
func (m Measure[D]) Unit() Unit[D] {
	return m.unit
}

// BaseAmount  기준 단위 기준 수치값을 반환합니다.
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

// In  대상 단위 기준 수치값을 반환합니다.
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

// As  대상 단위로 변환한 Measure를 반환합니다.
func (m Measure[D]) As(unit Unit[D]) (Measure[D], error) {
	value, err := m.In(unit)
	if err != nil {
		return Measure[D]{}, err
	}
	return New(value, unit)
}

// Add  두 측정값을 더 작은 ratio 단위로 맞춰 더합니다.
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

// Sub  두 측정값을 더 작은 ratio 단위로 맞춰 뺍니다.
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

// MulScalar  측정값을 스칼라로 곱합니다.
func (m Measure[D]) MulScalar(scalar float64) (Measure[D], error) {
	if err := m.validate(); err != nil {
		return Measure[D]{}, err
	}
	if !finite(scalar) {
		return Measure[D]{}, fmt.Errorf("%w: scalar must be finite", ErrInvalidMeasure)
	}
	return New(m.amount*scalar, m.unit)
}

// DivScalar  측정값을 스칼라로 나눕니다.
func (m Measure[D]) DivScalar(scalar float64) (Measure[D], error) {
	if err := m.validate(); err != nil {
		return Measure[D]{}, err
	}
	if !finite(scalar) || scalar == 0 {
		return Measure[D]{}, fmt.Errorf("%w: scalar must be finite and non-zero", ErrInvalidMeasure)
	}
	return New(m.amount/scalar, m.unit)
}

// Neg  측정값의 부호를 반전합니다.
func (m Measure[D]) Neg() (Measure[D], error) {
	if err := m.validate(); err != nil {
		return Measure[D]{}, err
	}
	return New(-m.amount, m.unit)
}

// Compare  두 측정값을 같은 단위로 변환해 비교합니다.
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

// ToNearest  현재 단위 기준 수치를 nearest 배수로 반올림합니다.
func (m Measure[D]) ToNearest(nearest float64) (Measure[D], error) {
	if err := m.validate(); err != nil {
		return Measure[D]{}, err
	}
	if !finite(nearest) || nearest <= 0 {
		return Measure[D]{}, fmt.Errorf("%w: nearest must be finite and positive", ErrInvalidMeasure)
	}
	return New(math.Round(m.amount/nearest)*nearest, m.unit)
}

// Format  대상 단위로 변환한 사람이 읽을 수 있는 문자열을 반환합니다.
func (m Measure[D]) Format(unit Unit[D]) (string, error) {
	value, err := m.In(unit)
	if err != nil {
		return "", err
	}
	return formatValue(value, unit), nil
}

// Human  후보 단위 중 절대값이 1 이상인 가장 큰 단위를 골라 포맷합니다.
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

// String  panic 없는 디버그 문자열을 반환합니다.
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
