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

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

const imfXML = `<?xml version="1.0" encoding="UTF-8"?>
<message:StructureSpecificData xmlns:message="http://www.sdmx.org/resources/sdmxml/schemas/v2_1/message">
  <message:DataSet>
    <Series COUNTRY="KOR" INDICATOR="XDC_USD" TYPE_OF_TRANSFORMATION="EOP_RT" FREQUENCY="M">
      <Obs TIME_PERIOD="2026-M01" OBS_VALUE="1427"/>
      <Obs TIME_PERIOD="2026-M02" OBS_VALUE="1424.5"/>
      <Obs TIME_PERIOD="2026-M03" OBS_VALUE="1513.4"/>
    </Series>
  </message:DataSet>
</message:StructureSpecificData>`

const imfXDCToEURXML = `<?xml version="1.0" encoding="UTF-8"?>
<message:StructureSpecificData xmlns:message="http://www.sdmx.org/resources/sdmxml/schemas/v2_1/message">
  <message:DataSet>
    <Series COUNTRY="KOR" INDICATOR="XDC_EUR" TYPE_OF_TRANSFORMATION="EOP_RT" FREQUENCY="M">
      <Obs TIME_PERIOD="2026-M01" OBS_VALUE="1700.8413"/>
      <Obs TIME_PERIOD="2026-M02" OBS_VALUE="1681.62225"/>
      <Obs TIME_PERIOD="2026-M03" OBS_VALUE="1740.10732"/>
    </Series>
  </message:DataSet>
</message:StructureSpecificData>`

const imfEURToXDCXML = `<?xml version="1.0" encoding="UTF-8"?>
<message:StructureSpecificData xmlns:message="http://www.sdmx.org/resources/sdmxml/schemas/v2_1/message">
  <message:DataSet>
    <Series COUNTRY="KOR" INDICATOR="EUR_XDC" TYPE_OF_TRANSFORMATION="EOP_RT" FREQUENCY="M">
      <Obs TIME_PERIOD="2026-M01" OBS_VALUE="0.0005879443308438007"/>
      <Obs TIME_PERIOD="2026-M02" OBS_VALUE="0.0005946638729357916"/>
      <Obs TIME_PERIOD="2026-M03" OBS_VALUE="0.0005746771986454261"/>
    </Series>
  </message:DataSet>
</message:StructureSpecificData>`

func TestNewIMFProviderValidatesOptions(t *testing.T) {
	cases := []struct {
		name    string
		options IMFProviderOptions
	}{
		{name: "negative timeout", options: IMFProviderOptions{Endpoint: "https://example.com/imf", Timeout: -time.Second}},
		{name: "negative cache ttl", options: IMFProviderOptions{Endpoint: "https://example.com/imf", CacheTTL: -time.Second}},
		{name: "negative max stale", options: IMFProviderOptions{Endpoint: "https://example.com/imf", MaxStale: -time.Second}},
		{name: "negative retry count", options: IMFProviderOptions{Endpoint: "https://example.com/imf", RetryCount: -1}},
		{name: "negative retry backoff", options: IMFProviderOptions{Endpoint: "https://example.com/imf", RetryBackoff: -time.Second}},
		{name: "empty endpoint", options: IMFProviderOptions{Endpoint: "   "}},
		{name: "bad scheme", options: IMFProviderOptions{Endpoint: "file:///tmp/imf"}},
		{name: "bad frequency", options: IMFProviderOptions{Endpoint: "https://example.com/imf", Frequency: "W"}},
		{name: "bad rate family", options: IMFProviderOptions{Endpoint: "https://example.com/imf", RateFamily: "SPOT"}},
		{name: "half period window", options: IMFProviderOptions{Endpoint: "https://example.com/imf", StartPeriod: "2026-01"}},
		{name: "bad currency code", options: IMFProviderOptions{Endpoint: "https://example.com/imf", CountryCodes: map[string]string{"BAD": "KOR"}}},
		{name: "empty country code", options: IMFProviderOptions{Endpoint: "https://example.com/imf", CountryCodes: map[string]string{"KRW": ""}}},
		{name: "country path separator", options: IMFProviderOptions{Endpoint: "https://example.com/imf", CountryCodes: map[string]string{"KRW": "K/R"}}},
		{name: "country query separator", options: IMFProviderOptions{Endpoint: "https://example.com/imf", CountryCodes: map[string]string{"KRW": "KO?"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewIMFProvider(tc.options); !errors.Is(err, ErrExchangeRateProvider) {
				t.Fatalf("expected ErrExchangeRateProvider, got %v", err)
			}
		})
	}
}

func TestNewIMFProviderAppliesSafeDefaults(t *testing.T) {
	provider, err := NewIMFProvider(IMFProviderOptions{})
	if err != nil {
		t.Fatalf("NewIMFProvider failed: %v", err)
	}
	if provider.client == nil || provider.now == nil || provider.endpoint != defaultIMFEndpoint {
		t.Fatalf("defaults not applied: %+v", provider)
	}
	if provider.frequency != IMFFrequencyMonthly || provider.rateFamily != IMFRateEndOfPeriod {
		t.Fatalf("unexpected default family/frequency: %s %s", provider.rateFamily, provider.frequency)
	}
	if provider.timeout <= 0 || provider.cacheTTL <= 0 || provider.maxStale <= 0 || provider.retryBackoff <= 0 {
		t.Fatalf("duration defaults should be positive")
	}
	if provider.countryCodes["KRW"] != "KOR" {
		t.Fatalf("expected default KRW IMF country code, got %q", provider.countryCodes["KRW"])
	}
}

func TestIMFProviderRateSupportsDomesticPivotPairs(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/data/ER/KOR.XDC_USD.EOP_RT.M" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.URL.Query().Get("startPeriod") != "2026-M01" || r.URL.Query().Get("endPeriod") != "2026-M03" {
			t.Fatalf("unexpected query %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(imfXML))
	}))
	defer server.Close()

	provider := newTestIMFProvider(t, IMFProviderOptions{
		Endpoint:    server.URL,
		StartPeriod: "2026-M01",
		EndPeriod:   "2026-M03",
	})
	quote, err := provider.Rate(context.Background(), USD, KRW)
	if err != nil {
		t.Fatalf("USD->KRW failed: %v", err)
	}
	if quote.Rate.Base() != USD || quote.Rate.Quote() != KRW || quote.Rate.Rate() != "1513.4" {
		t.Fatalf("unexpected IMF rate: %s/%s %s", quote.Rate.Base(), quote.Rate.Quote(), quote.Rate.Rate())
	}
	if quote.Source != "IMF ER:XDC_USD:EOP_RT:M" {
		t.Fatalf("unexpected source %q", quote.Source)
	}
	if quote.ObservedAt.Format(time.DateOnly) != "2026-03-31" {
		t.Fatalf("unexpected observed date %s", quote.ObservedAt.Format(time.DateOnly))
	}

	_, err = provider.Rate(context.Background(), USD, KRW)
	if err != nil {
		t.Fatalf("cached USD->KRW failed: %v", err)
	}
	if requests.Load() != 1 {
		t.Fatalf("fresh cache should avoid repeated HTTP requests, got %d", requests.Load())
	}
}

func TestIMFProviderRateSupportsReverseDomesticPivotPairs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/data/ER/KOR.USD_XDC.PA_RT.M" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		body := strings.ReplaceAll(imfXML, "XDC_USD", "USD_XDC")
		body = strings.ReplaceAll(body, "EOP_RT", "PA_RT")
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	provider := newTestIMFProvider(t, IMFProviderOptions{
		Endpoint:    server.URL,
		RateFamily:  IMFRatePeriodAverage,
		StartPeriod: "2026-M01",
		EndPeriod:   "2026-M03",
	})
	quote, err := provider.Rate(context.Background(), KRW, USD)
	if err != nil {
		t.Fatalf("KRW->USD failed: %v", err)
	}
	if quote.Rate.Base() != KRW || quote.Rate.Quote() != USD || quote.Source != "IMF ER:USD_XDC:PA_RT:M" {
		t.Fatalf("unexpected quote: %+v", quote)
	}
}

func TestIMFProviderRateSupportsEURPivotPairs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/data/ER/KOR.XDC_EUR.EOP_RT.M":
			_, _ = w.Write([]byte(imfXDCToEURXML))
		case "/data/ER/KOR.EUR_XDC.EOP_RT.M":
			_, _ = w.Write([]byte(imfEURToXDCXML))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	provider := newTestIMFProvider(t, IMFProviderOptions{
		Endpoint:    server.URL,
		StartPeriod: "2026-M01",
		EndPeriod:   "2026-M03",
	})
	quote, err := provider.Rate(context.Background(), EUR, KRW)
	if err != nil {
		t.Fatalf("EUR->KRW failed: %v", err)
	}
	if quote.Rate.Base() != EUR || quote.Rate.Quote() != KRW || quote.Rate.Rate() != "1740.10732" || quote.Source != "IMF ER:XDC_EUR:EOP_RT:M" {
		t.Fatalf("unexpected EUR quote: %+v", quote)
	}

	quote, err = provider.Rate(context.Background(), KRW, EUR)
	if err != nil {
		t.Fatalf("KRW->EUR failed: %v", err)
	}
	if quote.Rate.Base() != KRW || quote.Rate.Quote() != EUR || quote.Rate.Rate() != "0.0005746771986454261" || quote.Source != "IMF ER:EUR_XDC:EOP_RT:M" {
		t.Fatalf("unexpected reverse EUR quote: %+v", quote)
	}
}

func TestIMFProviderSameCurrencyAvoidsFetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatalf("same-currency rate should not fetch")
	}))
	defer server.Close()

	provider := newTestIMFProvider(t, IMFProviderOptions{Endpoint: server.URL})
	quote, err := provider.Rate(nil, USD, USD) //nolint:staticcheck // nil context normalization is the contract under test.
	if err != nil {
		t.Fatalf("same-currency rate failed: %v", err)
	}
	if quote.Rate.Base() != USD || quote.Rate.Quote() != USD || quote.Rate.Rate() != "1.00" {
		t.Fatalf("unexpected same-currency quote: %+v", quote)
	}
}

func TestIMFProviderRejectsUnsupportedPairsAndInvalidCurrencies(t *testing.T) {
	provider := newTestIMFProvider(t, IMFProviderOptions{})
	if _, err := provider.Rate(context.Background(), KRW, JPY); !errors.Is(err, ErrUnsupportedExchangeRate) {
		t.Fatalf("expected domestic cross unsupported error, got %v", err)
	}
	if _, err := provider.Rate(context.Background(), CNY, JPY); !errors.Is(err, ErrUnsupportedExchangeRate) {
		t.Fatalf("expected pivot-less pair unsupported error, got %v", err)
	}
	if _, err := provider.Rate(context.Background(), Currency{}, USD); !errors.Is(err, ErrInvalidCurrency) {
		t.Fatalf("expected ErrInvalidCurrency, got %v", err)
	}
}

func TestIMFProviderHTTPAndParseFailures(t *testing.T) {
	cases := []struct {
		name string
		body string
		code int
	}{
		{name: "http error", body: "down", code: http.StatusBadGateway},
		{name: "malformed xml", body: "<not-xml", code: http.StatusOK},
		{name: "missing observation", body: "<StructureSpecificData><DataSet></DataSet></StructureSpecificData>", code: http.StatusOK},
		{name: "malformed period", body: strings.ReplaceAll(imfXML, `TIME_PERIOD="2026-M03"`, `TIME_PERIOD="bad"`), code: http.StatusOK},
		{name: "malformed rate", body: strings.ReplaceAll(imfXML, `OBS_VALUE="1513.4"`, `OBS_VALUE="bad"`), code: http.StatusOK},
		{name: "zero rate", body: strings.ReplaceAll(imfXML, `OBS_VALUE="1513.4"`, `OBS_VALUE="0"`), code: http.StatusOK},
		{name: "wrong country", body: strings.ReplaceAll(imfXML, `COUNTRY="KOR"`, `COUNTRY="JPN"`), code: http.StatusOK},
		{name: "wrong indicator", body: strings.ReplaceAll(imfXML, `INDICATOR="XDC_USD"`, `INDICATOR="XDC_EUR"`), code: http.StatusOK},
		{name: "wrong family", body: strings.ReplaceAll(imfXML, `TYPE_OF_TRANSFORMATION="EOP_RT"`, `TYPE_OF_TRANSFORMATION="PA_RT"`), code: http.StatusOK},
		{name: "wrong frequency", body: strings.ReplaceAll(imfXML, `FREQUENCY="M"`, `FREQUENCY="Q"`), code: http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.code)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			provider := newTestIMFProvider(t, IMFProviderOptions{
				Endpoint:    server.URL,
				StartPeriod: "2026-M01",
				EndPeriod:   "2026-M03",
			})
			if _, err := provider.Rate(context.Background(), USD, KRW); !errors.Is(err, ErrExchangeRateProvider) && !errors.Is(err, ErrExchangeRateUnavailable) {
				t.Fatalf("expected provider/unavailable error, got %v", err)
			}
		})
	}
}

func TestIMFProviderBoundsHTTPResponseBodies(t *testing.T) {
	t.Run("oversized success body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(strings.Repeat("x", defaultIMFMaxBodyBytes+1)))
		}))
		defer server.Close()

		provider := newTestIMFProvider(t, IMFProviderOptions{
			Endpoint:    server.URL,
			StartPeriod: "2026-M01",
			EndPeriod:   "2026-M03",
		})
		if _, err := provider.Rate(context.Background(), USD, KRW); !errors.Is(err, ErrExchangeRateProvider) {
			t.Fatalf("expected bounded body provider error, got %v", err)
		}
	})

	t.Run("bounded error body diagnostic", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("bad request " + strings.Repeat("x", 4096)))
		}))
		defer server.Close()

		provider := newTestIMFProvider(t, IMFProviderOptions{
			Endpoint:    server.URL,
			StartPeriod: "2026-M01",
			EndPeriod:   "2026-M03",
		})
		_, err := provider.Rate(context.Background(), USD, KRW)
		if !errors.Is(err, ErrExchangeRateProvider) {
			t.Fatalf("expected HTTP provider error, got %v", err)
		}
		if len(err.Error()) > 700 {
			t.Fatalf("HTTP error diagnostic should be bounded, got %d bytes", len(err.Error()))
		}
	})
}

func TestIMFProviderRetryClassification(t *testing.T) {
	t.Run("retries server errors", func(t *testing.T) {
		var requests atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if requests.Add(1) == 1 {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			_, _ = w.Write([]byte(imfXML))
		}))
		defer server.Close()

		provider := newTestIMFProvider(t, IMFProviderOptions{
			Endpoint:     server.URL,
			RetryCount:   1,
			RetryBackoff: time.Nanosecond,
			StartPeriod:  "2026-M01",
			EndPeriod:    "2026-M03",
		})
		if _, err := provider.Rate(context.Background(), USD, KRW); err != nil {
			t.Fatalf("retry should recover server error: %v", err)
		}
		if requests.Load() != 2 {
			t.Fatalf("expected one retry, got %d requests", requests.Load())
		}
	})

	t.Run("does not retry client errors or malformed XML", func(t *testing.T) {
		cases := []struct {
			name string
			code int
			body string
		}{
			{name: "client error", code: http.StatusBadRequest, body: "bad"},
			{name: "malformed xml", code: http.StatusOK, body: "<not-xml"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				var requests atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					requests.Add(1)
					w.WriteHeader(tc.code)
					_, _ = w.Write([]byte(tc.body))
				}))
				defer server.Close()

				provider := newTestIMFProvider(t, IMFProviderOptions{
					Endpoint:     server.URL,
					RetryCount:   2,
					RetryBackoff: time.Nanosecond,
					StartPeriod:  "2026-M01",
					EndPeriod:    "2026-M03",
				})
				if _, err := provider.Rate(context.Background(), USD, KRW); !errors.Is(err, ErrExchangeRateProvider) {
					t.Fatalf("expected provider error, got %v", err)
				}
				if requests.Load() != 1 {
					t.Fatalf("expected no retry, got %d requests", requests.Load())
				}
			})
		}
	})
}

func TestIMFProviderStaleFallbackExposesRefreshError(t *testing.T) {
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	var current atomic.Value
	current.Store(imfXML)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		body := current.Load().(string)
		if body == "fail" {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	provider := newTestIMFProvider(t, IMFProviderOptions{
		Endpoint:          server.URL,
		CacheTTL:          time.Second,
		MaxStale:          time.Hour,
		AllowStaleOnError: true,
		StartPeriod:       "2026-M01",
		EndPeriod:         "2026-M03",
		Now: func() time.Time {
			return now
		},
	})
	if _, err := provider.Rate(context.Background(), USD, KRW); err != nil {
		t.Fatalf("initial rate failed: %v", err)
	}
	current.Store("fail")
	now = now.Add(2 * time.Second)

	quote, err := provider.Rate(context.Background(), USD, KRW)
	if err != nil {
		t.Fatalf("stale fallback should succeed: %v", err)
	}
	if !quote.Stale || quote.RefreshError == nil || !errors.Is(quote.RefreshError, ErrExchangeRateProvider) {
		t.Fatalf("stale quote should expose refresh error: %+v", quote)
	}

	now = now.Add(2 * time.Hour)
	if _, err := provider.Rate(context.Background(), USD, KRW); !errors.Is(err, ErrExchangeRateStale) {
		t.Fatalf("expected ErrExchangeRateStale beyond MaxStale, got %v", err)
	}
}

func TestIMFProviderDoesNotHideContextErrorsWithStaleFallback(t *testing.T) {
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	var fail atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if fail.Load() {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(imfXML))
	}))
	defer server.Close()

	provider := newTestIMFProvider(t, IMFProviderOptions{
		Endpoint:          server.URL,
		CacheTTL:          time.Nanosecond,
		MaxStale:          time.Hour,
		AllowStaleOnError: true,
		StartPeriod:       "2026-M01",
		EndPeriod:         "2026-M03",
		Now: func() time.Time {
			return now
		},
	})
	if _, err := provider.Rate(context.Background(), USD, KRW); err != nil {
		t.Fatalf("initial rate failed: %v", err)
	}
	fail.Store(true)
	now = now.Add(time.Second)

	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := provider.Rate(cancelCtx, USD, KRW); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled instead of stale quote, got %v", err)
	}
}

func TestIMFProviderUsesPostRefreshTimeForStaleAge(t *testing.T) {
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	var fail atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if fail.Load() {
			now = now.Add(2 * time.Hour)
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(imfXML))
	}))
	defer server.Close()

	provider := newTestIMFProvider(t, IMFProviderOptions{
		Endpoint:          server.URL,
		CacheTTL:          time.Second,
		MaxStale:          time.Hour,
		AllowStaleOnError: true,
		StartPeriod:       "2026-M01",
		EndPeriod:         "2026-M03",
		Now: func() time.Time {
			return now
		},
	})
	if _, err := provider.Rate(context.Background(), USD, KRW); err != nil {
		t.Fatalf("initial rate failed: %v", err)
	}
	fail.Store(true)
	now = now.Add(2 * time.Second)

	if _, err := provider.Rate(context.Background(), USD, KRW); !errors.Is(err, ErrExchangeRateStale) {
		t.Fatalf("expected post-refresh stale age rejection, got %v", err)
	}
}

func TestIMFProviderCancellationAndTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers: 1,
		Timeout: time.Second,
	})
	report, err := tester.Run(context.Background(), func(ctx context.Context) error {
		provider := newTestIMFProvider(t, IMFProviderOptions{
			Endpoint:    server.URL,
			Timeout:     10 * time.Millisecond,
			StartPeriod: "2026-M01",
			EndPeriod:   "2026-M03",
		})
		if _, err := provider.Rate(ctx, USD, KRW); !errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("expected provider timeout deadline, got %w", err)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("AsyncJobTester provider timeout run failed: report=%+v err=%v", report, err)
	}

	provider := newTestIMFProvider(t, IMFProviderOptions{
		Endpoint:    server.URL,
		Timeout:     time.Second,
		StartPeriod: "2026-M01",
		EndPeriod:   "2026-M03",
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	started := time.Now()
	if _, err := provider.Rate(ctx, USD, KRW); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected caller deadline, got %v", err)
	}
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("caller deadline was weakened, elapsed=%s", elapsed)
	}

	cancelCtx, cancelNow := context.WithCancel(context.Background())
	cancelNow()
	if _, err := provider.Rate(cancelCtx, USD, KRW); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestIMFProviderDefaultMonthlyPeriodWindow(t *testing.T) {
	provider := newTestIMFProvider(t, IMFProviderOptions{
		Now: func() time.Time {
			return time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
		},
	})
	window, err := provider.periodWindow()
	if err != nil {
		t.Fatalf("periodWindow failed: %v", err)
	}
	if window.start != "2024-M12" || window.end != "2026-M06" {
		t.Fatalf("unexpected window %+v", window)
	}
}

func newTestIMFProvider(t *testing.T, options IMFProviderOptions) *IMFProvider {
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
	provider, err := NewIMFProvider(options)
	if err != nil {
		t.Fatalf("NewIMFProvider failed: %v", err)
	}
	return provider
}

func ExampleIMFProvider() {
	provider, err := NewIMFProvider(IMFProviderOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	_ = provider
	fmt.Println("provider ready")
	// Output:
	// provider ready
}
