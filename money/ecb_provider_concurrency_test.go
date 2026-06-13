package money

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

func TestECBProviderGoroutineStressTesterConcurrentRates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(ecbXML))
	}))
	defer server.Close()

	provider := newTestECBProvider(t, ECBProviderOptions{Endpoint: server.URL})
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       8,
		RoundsPerTask: 24,
		Timeout:       5 * time.Second,
	})
	tester.RunT(t,
		func(ctx context.Context) error {
			_, err := provider.Rate(ctx, EUR, USD)
			return err
		},
		func(ctx context.Context) error {
			_, err := provider.Rate(ctx, USD, EUR)
			return err
		},
		func(ctx context.Context) error {
			_, err := provider.Rate(ctx, USD, KRW)
			return err
		},
		func(ctx context.Context) error {
			_, err := provider.Rate(ctx, KRW, USD)
			return err
		},
		func(ctx context.Context) error {
			_, err := provider.Rate(ctx, USD, USD)
			return err
		},
	)
}

func TestECBProviderGoroutineStressTesterConcurrentStaleRefresh(t *testing.T) {
	var fail atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if fail.Load() {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(ecbXML))
	}))
	defer server.Close()

	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	provider := newTestECBProvider(t, ECBProviderOptions{
		Endpoint:          server.URL,
		CacheTTL:          time.Nanosecond,
		MaxStale:          time.Hour,
		AllowStaleOnError: true,
		Now: func() time.Time {
			return now
		},
	})
	if _, err := provider.Rate(context.Background(), EUR, USD); err != nil {
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
			return errors.New("expected stale quote with refresh error")
		}
		return nil
	})
}

func TestECBProviderAsyncJobTesterCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	provider := newTestECBProvider(t, ECBProviderOptions{
		Endpoint: server.URL,
		Timeout:  time.Second,
	})
	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers:       4,
		RoundsPerTask: 8,
		Timeout:       5 * time.Second,
	})
	tester.RunT(t, func(ctx context.Context) error {
		callCtx, cancel := context.WithCancel(ctx)
		cancel()
		_, err := provider.Rate(callCtx, EUR, USD)
		if !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	})
}
