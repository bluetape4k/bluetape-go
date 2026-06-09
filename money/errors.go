// Package money 는 통화와 금액을 명시적으로 다루는 작은 값 API를 제공합니다.
package money

import "errors"

var (
	// ErrInvalidCurrency 는 통화 코드나 통화 값이 유효하지 않을 때 반환됩니다.
	ErrInvalidCurrency = errors.New("money: invalid currency")
	// ErrInvalidMoney 는 금액 값이 유효하지 않을 때 반환됩니다.
	ErrInvalidMoney = errors.New("money: invalid money")
	// ErrInvalidAmount 는 금액 문자열이나 스칼라 값이 유효하지 않을 때 반환됩니다.
	ErrInvalidAmount = errors.New("money: invalid amount")
	// ErrCurrencyMismatch 는 서로 다른 통화를 섞은 연산에서 반환됩니다.
	ErrCurrencyMismatch = errors.New("money: currency mismatch")
	// ErrDivideByZero 는 0으로 나누는 금액 연산에서 반환됩니다.
	ErrDivideByZero = errors.New("money: divide by zero")
	// ErrOverflow 는 금액이나 환율 연산 결과가 표현 범위를 넘을 때 반환됩니다.
	ErrOverflow = errors.New("money: overflow")
	// ErrInvalidExchangeRate 는 환율 값이 유효하지 않을 때 반환됩니다.
	ErrInvalidExchangeRate = errors.New("money: invalid exchange rate")
)
