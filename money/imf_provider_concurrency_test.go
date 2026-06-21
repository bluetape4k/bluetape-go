package money

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

func TestIMFProviderGoroutineStressTesterConcurrentRates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := imfXML
		switch r.URL.Path {
		case "/data/ER/KOR.XDC_USD.EOP_RT.M":
			body = imfXML
		case "/data/ER/KOR.USD_XDC.EOP_RT.M":
			body = strings.ReplaceAll(imfXML, "XDC_USD", "USD_XDC")
		case "/data/ER/KOR.XDC_EUR.EOP_RT.M":
			body = imfXDCToEURXML
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	provider := newTestIMFProvider(t, IMFProviderOptions{
		Endpoint:    server.URL,
		StartPeriod: "2026-M01",
		EndPeriod:   "2026-M03",
	})
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       8,
		RoundsPerTask: 24,
		Timeout:       5 * time.Second,
	})
	tester.RunT(t,
		func(ctx context.Context) error {
			_, err := provider.Rate(ctx, USD, KRW)
			return err
		},
		func(ctx context.Context) error {
			_, err := provider.Rate(ctx, KRW, USD)
			return err
		},
		func(ctx context.Context) error {
			_, err := provider.Rate(ctx, EUR, KRW)
			return err
		},
		func(ctx context.Context) error {
			_, err := provider.Rate(ctx, USD, USD)
			return err
		},
	)
}

func TestIMFProviderGoroutineStressTesterConcurrentStaleRefresh(t *testing.T) {
	var fail atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if fail.Load() {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(imfXML))
	}))
	defer server.Close()

	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
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
	now = now.Add(time.Second)
	fail.Store(true)

	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       6,
		RoundsPerTask: 12,
		Timeout:       5 * time.Second,
	})
	tester.RunT(t, func(ctx context.Context) error {
		quote, err := provider.Rate(ctx, USD, KRW)
		if err != nil {
			return err
		}
		if !quote.Stale || quote.RefreshError == nil {
			return errors.New("expected stale IMF quote with refresh error")
		}
		return nil
	})
}

func TestIMFProviderAsyncJobTesterCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	provider := newTestIMFProvider(t, IMFProviderOptions{
		Endpoint:    server.URL,
		Timeout:     time.Second,
		StartPeriod: "2026-M01",
		EndPeriod:   "2026-M03",
	})
	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers:       4,
		RoundsPerTask: 8,
		Timeout:       5 * time.Second,
	})
	tester.RunT(t, func(ctx context.Context) error {
		callCtx, cancel := context.WithCancel(ctx)
		cancel()
		_, err := provider.Rate(callCtx, USD, KRW)
		if !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	})
}
