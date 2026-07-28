// Package money bluetape-go의 money 기능을 제공한다.
package money

import "errors"

var (
	// ErrInvalidCurrency 통화 코드나 통화 값이 유효하지 않을 때 반환됩니다.
	ErrInvalidCurrency = errors.New("money: invalid currency")
	// ErrInvalidMoney 금액 값이 유효하지 않을 때 반환됩니다.
	ErrInvalidMoney = errors.New("money: invalid money")
	// ErrInvalidAmount 금액 문자열이나 스칼라 값이 유효하지 않을 때 반환됩니다.
	ErrInvalidAmount = errors.New("money: invalid amount")
	// ErrCurrencyMismatch 서로 다른 통화를 섞은 연산에서 반환됩니다.
	ErrCurrencyMismatch = errors.New("money: currency mismatch")
	// ErrDivideByZero 0으로 나누는 금액 연산에서 반환됩니다.
	ErrDivideByZero = errors.New("money: divide by zero")
	// ErrOverflow 금액이나 환율 연산 결과가 표현 범위를 넘을 때 반환됩니다.
	ErrOverflow = errors.New("money: overflow")
	// ErrInvalidExchangeRate 환율 값이 유효하지 않을 때 반환됩니다.
	ErrInvalidExchangeRate = errors.New("money: invalid exchange rate")
	// ErrExchangeRateProvider 환율 provider 설정이나 실행 오류에서 반환됩니다.
	ErrExchangeRateProvider = errors.New("money: exchange rate provider")
	// ErrExchangeRateUnavailable 은 provider 환율을 가져올 수 없을 때 반환됩니다.
	ErrExchangeRateUnavailable = errors.New("money: exchange rate unavailable")
	// ErrExchangeRateStale 은 허용되지 않는 오래된 환율 snapshot에서 반환됩니다.
	ErrExchangeRateStale = errors.New("money: exchange rate stale")
	// ErrUnsupportedExchangeRate provider가 통화쌍을 지원하지 않을 때 반환됩니다.
	ErrUnsupportedExchangeRate = errors.New("money: unsupported exchange rate")
)
