package money

import (
	"context"
	"fmt"
	"reflect"
	"time"
)

// ExchangeRateProvider interface 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type ExchangeRateProvider interface {
	Rate(ctx context.Context, base Currency, target Currency) (ExchangeRateQuote, error)
}

// ExchangeRateQuote struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type ExchangeRateQuote struct {
	// Rate 변환에 사용할 환율입니다.
	Rate ExchangeRate
	// Source 환율 source 이름입니다.
	Source string
	// ObservedAt 은 provider가 환율을 관측한 시각입니다.
	ObservedAt time.Time
	// FetchedAt 은 local process가 snapshot을 가져온 시각입니다.
	FetchedAt time.Time
	// ExpiresAt 은 quote freshness가 만료되는 시각입니다.
	ExpiresAt time.Time
	// Stale 은 refresh 실패 후 오래된 snapshot으로 만든 quote인지 나타냅니다.
	Stale bool
	// RefreshError stale fallback을 유발한 refresh 오류입니다.
	RefreshError error
}

// ConvertWithProvider ConvertWithProvider 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - amount: ConvertWithProvider 동작에 필요한 amount 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - target: ConvertWithProvider 동작에 필요한 target 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - provider: ConvertWithProvider 동작에 필요한 provider 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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
