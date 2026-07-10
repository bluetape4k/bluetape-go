package redisratelimit

import (
	"context"
	"errors"
	"strings"
	"testing"

	btredis "github.com/bluetape4k/bluetape-go/redis"
	"github.com/redis/go-redis/v9"
)

func TestLimiterAllowReturnsTypedRedactedOperationError(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	if err := client.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}

	const namespace = "redacted-namespace-marker"
	const key = " redacted-key-marker:with:delimiters "
	limiter, err := New(Options{
		Client:        client,
		Namespace:     namespace,
		RatePerSecond: 1,
		Burst:         1,
	})
	if err != nil {
		t.Fatalf("new limiter: %v", err)
	}

	_, err = limiter.Allow(context.Background(), key, 1)
	if !errors.Is(err, redis.ErrClosed) {
		t.Fatalf("expected redis.ErrClosed, got %v", err)
	}

	var opErr *btredis.OpError
	if !errors.As(err, &opErr) {
		t.Fatalf("expected *btredis.OpError, got %T", err)
	}
	if got, want := opErr.Family(), "rate limiter"; got != want {
		t.Fatalf("family = %q, want %q", got, want)
	}
	if got, want := opErr.Operation(), "consume"; got != want {
		t.Fatalf("operation = %q, want %q", got, want)
	}
	if got, want := opErr.KeyID(), btredis.RedactedKeyID(limiter.bucketKey(key)); got != want {
		t.Fatalf("key id = %q, want %q", got, want)
	}

	for _, marker := range []string{namespace, key, limiter.bucketKey(key), "redis: client is closed"} {
		if strings.Contains(err.Error(), marker) {
			t.Fatalf("operation error leaked %q: %v", marker, err)
		}
	}
}

func TestOperationErrorJoinsLateContextWithoutLeakingRawKey(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := operationError(ctx, "consume", "raw-key-marker", redis.ErrClosed)
	if !errors.Is(err, redis.ErrClosed) {
		t.Fatalf("expected redis.ErrClosed, got %v", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	var opErr *btredis.OpError
	if !errors.As(err, &opErr) {
		t.Fatalf("expected *btredis.OpError, got %T", err)
	}
	if strings.Contains(err.Error(), "raw-key-marker") {
		t.Fatalf("operation error leaked raw key: %v", err)
	}
}
