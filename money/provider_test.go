package money

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeRateProvider struct {
	quote ExchangeRateQuote
	err   error
	seen  context.Context
}

func (p *fakeRateProvider) Rate(ctx context.Context, _ Currency, _ Currency) (ExchangeRateQuote, error) {
	p.seen = ctx
	return p.quote, p.err
}

func TestConvertWithProviderReturnsConvertedMoneyAndQuote(t *testing.T) {
	rate, err := NewExchangeRate(USD, KRW, "1300")
	if err != nil {
		t.Fatalf("NewExchangeRate failed: %v", err)
	}
	observedAt := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)
	quote := ExchangeRateQuote{
		Rate:       rate,
		Source:     ECBSource,
		ObservedAt: observedAt,
		FetchedAt:  observedAt.Add(16 * time.Hour),
		ExpiresAt:  observedAt.Add(40 * time.Hour),
	}
	provider := &fakeRateProvider{quote: quote}
	usd, _ := New("2.00", USD)

	converted, used, err := ConvertWithProvider(nil, usd, KRW, provider) //nolint:staticcheck // nil context normalization is the contract under test.
	if err != nil {
		t.Fatalf("ConvertWithProvider failed: %v", err)
	}
	if converted.String() != "KRW 2600" {
		t.Fatalf("converted = %q", converted.String())
	}
	if used.Source != ECBSource || !used.ObservedAt.Equal(observedAt) || used.Rate.Rate() != "1300" {
		t.Fatalf("unexpected quote: %+v", used)
	}
	if provider.seen == nil {
		t.Fatalf("nil context should be normalized before provider call")
	}
}

func TestConvertWithProviderRejectsNilProviders(t *testing.T) {
	usd, _ := New("1.00", USD)
	if _, _, err := ConvertWithProvider(context.Background(), usd, KRW, nil); !errors.Is(err, ErrExchangeRateProvider) {
		t.Fatalf("expected ErrExchangeRateProvider for nil provider, got %v", err)
	}

	var provider *fakeRateProvider
	if _, _, err := ConvertWithProvider(context.Background(), usd, KRW, provider); !errors.Is(err, ErrExchangeRateProvider) {
		t.Fatalf("expected ErrExchangeRateProvider for typed nil provider, got %v", err)
	}
}

func TestConvertWithProviderPropagatesProviderError(t *testing.T) {
	cause := errors.New("provider down")
	provider := &fakeRateProvider{err: fmtExchangeProviderError(cause)}
	usd, _ := New("1.00", USD)

	converted, quote, err := ConvertWithProvider(context.Background(), usd, KRW, provider)
	if !errors.Is(err, ErrExchangeRateProvider) || !errors.Is(err, cause) {
		t.Fatalf("expected wrapped provider error, got %v", err)
	}
	if !converted.IsZero() || quote.Rate.Valid() {
		t.Fatalf("provider error should not return valid values: %v %+v", converted, quote)
	}
}

func TestConvertWithProviderRejectsInvalidQuote(t *testing.T) {
	provider := &fakeRateProvider{quote: ExchangeRateQuote{Source: ECBSource}}
	usd, _ := New("1.00", USD)

	if _, _, err := ConvertWithProvider(context.Background(), usd, KRW, provider); !errors.Is(err, ErrInvalidExchangeRate) {
		t.Fatalf("expected ErrInvalidExchangeRate, got %v", err)
	}
}

func TestConvertWithProviderValidatesInput(t *testing.T) {
	rate, _ := NewExchangeRate(USD, KRW, "1300")
	provider := &fakeRateProvider{quote: ExchangeRateQuote{Rate: rate, Source: ECBSource}}

	if _, _, err := ConvertWithProvider(context.Background(), Money{}, KRW, provider); !errors.Is(err, ErrInvalidMoney) {
		t.Fatalf("expected ErrInvalidMoney, got %v", err)
	}

	usd, _ := New("1.00", USD)
	if _, _, err := ConvertWithProvider(context.Background(), usd, Currency{}, provider); !errors.Is(err, ErrInvalidCurrency) {
		t.Fatalf("expected ErrInvalidCurrency, got %v", err)
	}
}

func TestConvertWithProviderRejectsWrongTargetQuote(t *testing.T) {
	rate, _ := NewExchangeRate(USD, EUR, "0.9")
	provider := &fakeRateProvider{quote: ExchangeRateQuote{Rate: rate, Source: ECBSource}}
	usd, _ := New("1.00", USD)

	if _, _, err := ConvertWithProvider(context.Background(), usd, KRW, provider); !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("expected ErrCurrencyMismatch, got %v", err)
	}
}

func fmtExchangeProviderError(cause error) error {
	return &wrappedProviderError{cause: cause}
}

type wrappedProviderError struct {
	cause error
}

func (e *wrappedProviderError) Error() string {
	return "provider failed: " + e.cause.Error()
}

func (e *wrappedProviderError) Unwrap() error {
	return errors.Join(ErrExchangeRateProvider, e.cause)
}
