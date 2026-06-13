package money

import (
	"context"
	"fmt"
	"reflect"
	"time"
)

// ExchangeRateProvider 는 context-aware 환율 source입니다.
type ExchangeRateProvider interface {
	Rate(ctx context.Context, base Currency, target Currency) (ExchangeRateQuote, error)
}

// ExchangeRateQuote 는 provider가 반환한 환율과 source metadata입니다.
type ExchangeRateQuote struct {
	// Rate 는 변환에 사용할 환율입니다.
	Rate ExchangeRate
	// Source 는 환율 source 이름입니다.
	Source string
	// ObservedAt 은 provider가 환율을 관측한 시각입니다.
	ObservedAt time.Time
	// FetchedAt 은 local process가 snapshot을 가져온 시각입니다.
	FetchedAt time.Time
	// ExpiresAt 은 quote freshness가 만료되는 시각입니다.
	ExpiresAt time.Time
	// Stale 은 refresh 실패 후 오래된 snapshot으로 만든 quote인지 나타냅니다.
	Stale bool
	// RefreshError 는 stale fallback을 유발한 refresh 오류입니다.
	RefreshError error
}

// ConvertWithProvider 는 provider에서 환율을 받아 Money를 target 통화로 변환합니다.
func ConvertWithProvider(ctx context.Context, amount Money, target Currency, provider ExchangeRateProvider) (Money, ExchangeRateQuote, error) {
	if err := amount.validate(); err != nil {
		return Money{}, ExchangeRateQuote{}, err
	}
	if err := target.validate(); err != nil {
		return Money{}, ExchangeRateQuote{}, err
	}
	if isNilProvider(provider) {
		return Money{}, ExchangeRateQuote{}, ErrExchangeRateProvider
	}

	quote, err := provider.Rate(normalizeProviderContext(ctx), amount.Currency(), target)
	if err != nil {
		return Money{}, ExchangeRateQuote{}, err
	}
	if err := quote.Rate.validate(); err != nil {
		return Money{}, ExchangeRateQuote{}, err
	}

	converted, err := Convert(amount, quote.Rate)
	if err != nil {
		return Money{}, ExchangeRateQuote{}, err
	}
	if !sameCurrency(converted.Currency(), target) {
		return Money{}, ExchangeRateQuote{}, fmt.Errorf("%w: provider returned %s/%s for target %s",
			ErrCurrencyMismatch, quote.Rate.Base(), quote.Rate.Quote(), target)
	}
	return converted, quote, nil
}

func normalizeProviderContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func isNilProvider(provider ExchangeRateProvider) bool {
	if provider == nil {
		return true
	}
	value := reflect.ValueOf(provider)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
