package sqlratelimit

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/bluetape4k/bluetape-go/ratelimit"
)

func TestNewValidatesDatabaseAndDoesNotTouchIt(t *testing.T) {
	if _, err := New(nil, Options{RatePerSecond: 1, Burst: 1}); err == nil {
		t.Fatal("New(nil) succeeded")
	}
	db := &sql.DB{}
	limiter, err := New(db, Options{RatePerSecond: 1, Burst: 1})
	if err != nil || limiter == nil {
		t.Fatalf("New() = %v, %v", limiter, err)
	}
	if limiter.db != db {
		t.Fatal("New did not retain caller-owned database")
	}
}

func TestAllowRejectsBeforeDatabaseTraffic(t *testing.T) {
	var nilLimiter *Limiter
	if result, err := nilLimiter.Allow(context.Background(), "key", 1); err == nil || result != (ratelimit.Result{}) {
		t.Fatalf("nil limiter Allow = %+v, %v", result, err)
	}

	db := &sql.DB{}
	limiter, err := New(db, Options{RatePerSecond: 1, Burst: 1, MaxKeyBytes: 4})
	if err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := limiter.Allow(canceled, "key", 1); !errors.Is(err, context.Canceled) || result != (ratelimit.Result{}) {
		t.Fatalf("pre-canceled Allow = %+v, %v", result, err)
	}
	for _, tt := range []struct {
		key    string
		tokens int64
	}{{"  ", 1}, {"12345", 1}, {"key", 0}, {"key", 2}} {
		if result, err := limiter.Allow(context.Background(), tt.key, tt.tokens); err == nil || result != (ratelimit.Result{}) {
			t.Fatalf("invalid Allow(%q,%d) = %+v, %v", tt.key, tt.tokens, result, err)
		}
	}
}

func TestCleanupRejectsBeforeDatabaseTraffic(t *testing.T) {
	var nilLimiter *Limiter
	if count, err := nilLimiter.Cleanup(context.Background(), 1); err == nil || count != 0 {
		t.Fatalf("nil limiter Cleanup = %d, %v", count, err)
	}
	limiter, err := New(&sql.DB{}, Options{RatePerSecond: 1, Burst: 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, limit := range []int{-1, 0, MaxCleanupBatch + 1} {
		if count, err := limiter.Cleanup(context.Background(), limit); err == nil || count != 0 {
			t.Fatalf("Cleanup(%d) = %d, %v", limit, count, err)
		}
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if count, err := limiter.Cleanup(canceled, 1); !errors.Is(err, context.Canceled) || count != 0 {
		t.Fatalf("pre-canceled Cleanup = %d, %v", count, err)
	}
}
