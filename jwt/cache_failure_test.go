package jwt

import (
	"context"
	"errors"
	"testing"
	"time"

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

func TestCachedProviderContextMatrix(t *testing.T) {
	provider := newTestFixedHMACProvider(t)
	cache := newSpyReaderCache(time.Now)
	cached, err := NewCachedProvider(provider, cache)
	if err != nil {
		t.Fatalf("NewCachedProvider() error = %v", err)
	}
	if _, err := cached.ParseContext(nil, "token"); !errors.Is(err, ErrInvalidOptions) { //nolint:staticcheck // nil context is the public contract under test.
		t.Fatalf("nil context error = %v, want ErrInvalidOptions", err)
	}
	deadline, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if _, err := cached.ParseContext(deadline, "token"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline context error = %v, want DeadlineExceeded", err)
	}
	if err := cached.ClearCache(nil); !errors.Is(err, ErrInvalidOptions) { //nolint:staticcheck // nil context is the public contract under test.
		t.Fatalf("ClearCache(nil) error = %v, want ErrInvalidOptions", err)
	}
	if err := cached.ClearCache(deadline); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("ClearCache(deadline) error = %v, want DeadlineExceeded", err)
	}
}

func TestCachedProviderAsyncCancellationDoesNotCache(t *testing.T) {
	provider := newTestFixedHMACProvider(t)
	cache := newSpyReaderCache(time.Now)
	cache.setBlock = make(chan struct{}, 1)
	cached, err := NewCachedProvider(provider, cache)
	if err != nil {
		t.Fatalf("NewCachedProvider() error = %v", err)
	}
	token, err := provider.Compose(WithExpiresAfter(time.Hour))
	if err != nil {
		t.Fatalf("Compose() error = %v", err)
	}

	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers:       1,
		RoundsPerTask: 2,
		Timeout:       5 * time.Second,
	})
	tester.RunT(t, func(ctx context.Context) error {
		parseCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		done := make(chan error, 1)
		go func() {
			_, err := cached.ParseContext(parseCtx, token)
			done <- err
		}()
		select {
		case <-cache.setBlock:
		case <-ctx.Done():
			return ctx.Err()
		}
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	})
	_, sets, _, _ := cache.snapshot()
	if sets != 0 {
		t.Fatalf("canceled parse must not complete cache set, sets=%d", sets)
	}
	cache.mu.Lock()
	entries := len(cache.values)
	cache.mu.Unlock()
	if entries != 0 {
		t.Fatalf("canceled parse must not leave cached readers, entries=%d", entries)
	}
}
