package money

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const ecbXML = `<?xml version="1.0" encoding="UTF-8"?>
<gesmes:Envelope xmlns:gesmes="http://www.gesmes.org/xml/2002-08-01" xmlns="http://www.ecb.int/vocabulary/2002-08-01/eurofxref">
  <Cube>
    <Cube time="2026-06-12">
      <Cube currency="USD" rate="1.25"/>
      <Cube currency="KRW" rate="1300"/>
      <Cube currency="JPY" rate="180"/>
    </Cube>
  </Cube>
</gesmes:Envelope>`

func TestNewECBProviderValidatesOptions(t *testing.T) {
	cases := []struct {
		name    string
		options ECBProviderOptions
	}{
		{name: "negative timeout", options: ECBProviderOptions{Endpoint: "https://example.com/ecb.xml", Timeout: -time.Second}},
		{name: "negative cache ttl", options: ECBProviderOptions{Endpoint: "https://example.com/ecb.xml", CacheTTL: -time.Second}},
		{name: "negative max stale", options: ECBProviderOptions{Endpoint: "https://example.com/ecb.xml", MaxStale: -time.Second}},
		{name: "negative retry count", options: ECBProviderOptions{Endpoint: "https://example.com/ecb.xml", RetryCount: -1}},
		{name: "negative retry backoff", options: ECBProviderOptions{Endpoint: "https://example.com/ecb.xml", RetryBackoff: -time.Second}},
		{name: "negative max body bytes", options: ECBProviderOptions{Endpoint: "https://example.com/ecb.xml", MaxBodyBytes: -1}},
		{name: "empty endpoint", options: ECBProviderOptions{Endpoint: "   "}},
		{name: "bad scheme", options: ECBProviderOptions{Endpoint: "file:///tmp/ecb.xml"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewECBProvider(tc.options); !errors.Is(err, ErrExchangeRateProvider) {
				t.Fatalf("expected ErrExchangeRateProvider, got %v", err)
			}
		})
	}
}

func TestNewECBProviderAppliesSafeDefaults(t *testing.T) {
	provider, err := NewECBProvider(ECBProviderOptions{})
	if err != nil {
		t.Fatalf("NewECBProvider failed: %v", err)
	}
	if provider.client == nil || provider.now == nil || provider.endpoint != defaultECBEndpoint {
		t.Fatalf("defaults not applied: %+v", provider)
	}
	if provider.timeout <= 0 || provider.cacheTTL <= 0 || provider.maxStale <= 0 || provider.retryBackoff <= 0 {
		t.Fatalf("duration defaults should be positive")
	}
	if provider.maxBodyBytes != defaultECBMaxBodyBytes {
		t.Fatalf("max body bytes = %d, want %d", provider.maxBodyBytes, defaultECBMaxBodyBytes)
	}
}

func TestECBProviderRateSupportsDirectReverseAndCrossRates(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte(ecbXML))
	}))
	defer server.Close()

	provider := newTestECBProvider(t, ECBProviderOptions{Endpoint: server.URL})

	eurToUSD, err := provider.Rate(context.Background(), EUR, USD)
	if err != nil {
		t.Fatalf("EUR->USD failed: %v", err)
	}
	if eurToUSD.Rate.Base() != EUR || eurToUSD.Rate.Quote() != USD || eurToUSD.Rate.Rate() != "1.25" {
		t.Fatalf("unexpected EUR->USD rate: %s/%s %s", eurToUSD.Rate.Base(), eurToUSD.Rate.Quote(), eurToUSD.Rate.Rate())
	}

	usdToEUR, err := provider.Rate(context.Background(), USD, EUR)
	if err != nil {
		t.Fatalf("USD->EUR failed: %v", err)
	}
	usd, _ := New("1.25", USD)
	converted, err := Convert(usd, usdToEUR.Rate)
	if err != nil {
		t.Fatalf("reverse conversion failed: %v", err)
	}
	if converted.String() != "EUR 1.00" {
		t.Fatalf("reverse converted = %q", converted.String())
	}

	usdToKRW, err := provider.Rate(context.Background(), USD, KRW)
	if err != nil {
		t.Fatalf("USD->KRW failed: %v", err)
	}
	if usdToKRW.Rate.Base() != USD || usdToKRW.Rate.Quote() != KRW || usdToKRW.Rate.Rate() != "1040" {
		t.Fatalf("unexpected cross rate: %s/%s %s", usdToKRW.Rate.Base(), usdToKRW.Rate.Quote(), usdToKRW.Rate.Rate())
	}
	if requests.Load() != 1 {
		t.Fatalf("fresh cache should avoid repeated HTTP requests, got %d", requests.Load())
	}
}

func TestECBProviderSameCurrencyAvoidsFetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatalf("same-currency rate should not fetch")
	}))
	defer server.Close()

	provider := newTestECBProvider(t, ECBProviderOptions{Endpoint: server.URL})
	quote, err := provider.Rate(nil, USD, USD) //nolint:staticcheck // nil context normalization is the contract under test.
	if err != nil {
		t.Fatalf("same-currency rate failed: %v", err)
	}
	if quote.Rate.Base() != USD || quote.Rate.Quote() != USD || quote.Rate.Rate() != "1.00" {
		t.Fatalf("unexpected same-currency quote: %+v", quote)
	}
}

func TestECBProviderConvertWithProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(ecbXML))
	}))
	defer server.Close()

	provider := newTestECBProvider(t, ECBProviderOptions{Endpoint: server.URL})
	usd, _ := New("2.00", USD)
	converted, quote, err := ConvertWithProvider(context.Background(), usd, KRW, provider)
	if err != nil {
		t.Fatalf("ConvertWithProvider failed: %v", err)
	}
	if converted.String() != "KRW 2080" {
		t.Fatalf("converted = %q", converted.String())
	}
	if quote.Source != ECBSource || quote.ObservedAt.Format("2006-01-02") != "2026-06-12" {
		t.Fatalf("unexpected quote metadata: %+v", quote)
	}
}

func TestECBProviderRejectsUnsupportedAndInvalidCurrencies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(ecbXML))
	}))
	defer server.Close()

	provider := newTestECBProvider(t, ECBProviderOptions{Endpoint: server.URL})
	cny := MustParseCurrency("CNY")
	if _, err := provider.Rate(context.Background(), USD, cny); !errors.Is(err, ErrUnsupportedExchangeRate) {
		t.Fatalf("expected ErrUnsupportedExchangeRate, got %v", err)
	}
	if _, err := provider.Rate(context.Background(), Currency{}, USD); !errors.Is(err, ErrInvalidCurrency) {
		t.Fatalf("expected ErrInvalidCurrency, got %v", err)
	}
}

func TestECBProviderHTTPAndParseFailures(t *testing.T) {
	cases := []struct {
		name string
		body string
		code int
	}{
		{name: "http error", body: "down", code: http.StatusBadGateway},
		{name: "malformed xml", body: "<not-xml", code: http.StatusOK},
		{name: "missing observation", body: "<Envelope><Cube></Cube></Envelope>", code: http.StatusOK},
		{name: "missing rates", body: `<Envelope><Cube><Cube time="2026-06-12"></Cube></Cube></Envelope>`, code: http.StatusOK},
		{name: "malformed rate", body: strings.ReplaceAll(ecbXML, `rate="1.25"`, `rate="bad"`), code: http.StatusOK},
		{name: "duplicate currency", body: strings.ReplaceAll(ecbXML, `</Cube>`+"\n  </Cube>", `<Cube currency="USD" rate="1.26"/></Cube>`+"\n  </Cube>"), code: http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.code)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			provider := newTestECBProvider(t, ECBProviderOptions{Endpoint: server.URL})
			if _, err := provider.Rate(context.Background(), EUR, USD); !errors.Is(err, ErrExchangeRateProvider) && !errors.Is(err, ErrExchangeRateUnavailable) {
				t.Fatalf("expected provider/unavailable error, got %v", err)
			}
		})
	}
}

func TestECBProviderRejectsOversizedResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(ecbXML + strings.Repeat(" ", 64)))
	}))
	defer server.Close()

	provider := newTestECBProvider(t, ECBProviderOptions{
		Endpoint:     server.URL,
		MaxBodyBytes: int64(len(ecbXML)),
	})
	if _, err := provider.Rate(context.Background(), EUR, USD); !errors.Is(err, ErrExchangeRateUnavailable) {
		t.Fatalf("expected unavailable error wrapping oversized body, got %v", err)
	}
}

func TestECBProviderRetryAndStaleFallback(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(ecbXML))
	}))
	defer server.Close()

	provider := newTestECBProvider(t, ECBProviderOptions{
		Endpoint:     server.URL,
		RetryCount:   1,
		RetryBackoff: time.Nanosecond,
	})
	if _, err := provider.Rate(context.Background(), EUR, USD); err != nil {
		t.Fatalf("retry should recover: %v", err)
	}
	if requests.Load() != 2 {
		t.Fatalf("expected one retry, got %d requests", requests.Load())
	}
}

func TestECBProviderStaleFallbackExposesRefreshError(t *testing.T) {
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	var current atomic.Value
	current.Store(ecbXML)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		body := current.Load().(string)
		if body == "fail" {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	provider := newTestECBProvider(t, ECBProviderOptions{
		Endpoint:          server.URL,
		CacheTTL:          time.Second,
		MaxStale:          time.Hour,
		AllowStaleOnError: true,
		Now: func() time.Time {
			return now
		},
	})
	if _, err := provider.Rate(context.Background(), EUR, USD); err != nil {
		t.Fatalf("initial rate failed: %v", err)
	}
	current.Store("fail")
	now = now.Add(2 * time.Second)

	quote, err := provider.Rate(context.Background(), EUR, USD)
	if err != nil {
		t.Fatalf("stale fallback should succeed: %v", err)
	}
	if !quote.Stale || quote.RefreshError == nil || !errors.Is(quote.RefreshError, ErrExchangeRateProvider) {
		t.Fatalf("stale quote should expose refresh error: %+v", quote)
	}

	now = now.Add(2 * time.Hour)
	if _, err := provider.Rate(context.Background(), EUR, USD); !errors.Is(err, ErrExchangeRateStale) {
		t.Fatalf("expected ErrExchangeRateStale beyond MaxStale, got %v", err)
	}
}

func TestECBProviderStaleDisallowedReturnsStaleError(t *testing.T) {
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	var current atomic.Value
	current.Store(ecbXML)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		body := current.Load().(string)
		if body == "fail" {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	provider := newTestECBProvider(t, ECBProviderOptions{
		Endpoint: server.URL,
		CacheTTL: time.Second,
		Now: func() time.Time {
			return now
		},
	})
	if _, err := provider.Rate(context.Background(), EUR, USD); err != nil {
		t.Fatalf("initial rate failed: %v", err)
	}
	current.Store("fail")
	now = now.Add(2 * time.Second)

	if _, err := provider.Rate(context.Background(), EUR, USD); !errors.Is(err, ErrExchangeRateStale) {
		t.Fatalf("expected ErrExchangeRateStale, got %v", err)
	}
}

func TestECBProviderCancellationAndTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	provider := newTestECBProvider(t, ECBProviderOptions{
		Endpoint: server.URL,
		Timeout:  10 * time.Millisecond,
	})
	if _, err := provider.Rate(context.Background(), EUR, USD); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected provider timeout deadline, got %v", err)
	}

	provider = newTestECBProvider(t, ECBProviderOptions{
		Endpoint: server.URL,
		Timeout:  time.Second,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	started := time.Now()
	if _, err := provider.Rate(ctx, EUR, USD); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected caller deadline, got %v", err)
	}
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("caller deadline was weakened, elapsed=%s", elapsed)
	}

	cancelCtx, cancelNow := context.WithCancel(context.Background())
	cancelNow()
	if _, err := provider.Rate(cancelCtx, EUR, USD); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func newTestECBProvider(t *testing.T, options ECBProviderOptions) *ECBProvider {
	t.Helper()
	if options.Now == nil {
		options.Now = func() time.Time {
			return time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
		}
	}
	if options.CacheTTL == 0 {
		options.CacheTTL = time.Hour
	}
	if options.MaxStale == 0 {
		options.MaxStale = time.Hour
	}
	if options.Timeout == 0 {
		options.Timeout = time.Second
	}
	provider, err := NewECBProvider(options)
	if err != nil {
		t.Fatalf("NewECBProvider failed: %v", err)
	}
	return provider
}

func TestECBProviderDuplicateCurrencyFixture(t *testing.T) {
	body := strings.Replace(ecbXML, `      <Cube currency="KRW" rate="1300"/>`, `      <Cube currency="USD" rate="1.26"/>
      <Cube currency="KRW" rate="1300"/>`, 1)
	if _, err := parseECBSnapshot(strings.NewReader(body), time.Now(), time.Hour); !errors.Is(err, ErrExchangeRateProvider) {
		t.Fatalf("expected duplicate currency provider error, got %v", err)
	}
}

func TestECBProviderParseRejectsInvalidCurrency(t *testing.T) {
	body := strings.Replace(ecbXML, `currency="USD"`, `currency="XXX"`, 1)
	if _, err := parseECBSnapshot(strings.NewReader(body), time.Now(), time.Hour); !errors.Is(err, ErrExchangeRateProvider) {
		t.Fatalf("expected invalid currency provider error, got %v", err)
	}
}

func ExampleECBProvider() {
	provider, err := NewECBProvider(ECBProviderOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	_ = provider
	fmt.Println("provider ready")
	// Output:
	// provider ready
}
