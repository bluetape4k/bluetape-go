package redissem

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	btredis "github.com/bluetape4k/bluetape-go/redis"
	"github.com/redis/go-redis/v9"
)

func TestNewRejectsInvalidSemaphoreOptions(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	t.Cleanup(func() { _ = client.Close() })
	tests := []struct {
		name   string
		client redis.Cmdable
		opts   Options
		want   error
	}{
		{name: "nil client", opts: Options{Key: "k", Permits: 1, TTL: time.Second}, want: btredis.ErrInvalidKey},
		{name: "blank key", client: client, opts: Options{Key: " ", Permits: 1, TTL: time.Second}, want: btredis.ErrInvalidKey},
		{name: "zero permits", client: client, opts: Options{Key: "k", TTL: time.Second}, want: btredis.ErrInvalidKey},
		{name: "negative permits", client: client, opts: Options{Key: "k", Permits: -1, TTL: time.Second}, want: btredis.ErrInvalidKey},
		{name: "zero ttl", client: client, opts: Options{Key: "k", Permits: 1}, want: btredis.ErrInvalidTTL},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.client, tt.opts)
			if !errors.Is(err, tt.want) {
				t.Fatalf("New() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestBuildKeyUsesStableRedactedHashTag(t *testing.T) {
	keys, err := buildKeys(" caller-owned:permits ")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(keys.leases, "{redis-key:") || !strings.HasSuffix(keys.leases, ":leases") {
		t.Fatalf("unexpected leases key: %q", keys.leases)
	}
}

func TestParseAcquireResult(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		want    bool
		wantErr bool
	}{
		{name: "busy", value: int64(0)},
		{name: "acquired", value: int64(1), want: true},
		{name: "unexpected", value: int64(2), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := redis.NewCmd(context.Background(), "eval")
			cmd.SetVal(tt.value)
			got, err := parseAcquireResult(cmd)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parse error = %v", err)
			}
			if err == nil && got != tt.want {
				t.Fatalf("parse result = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestNilLeaseIsIdempotent(t *testing.T) {
	var lease *Lease
	released, err := lease.Release(context.Background())
	if released || err != nil {
		t.Fatalf("nil Release() = %t, %v", released, err)
	}
}

func TestLeaseAccessorZeroValues(t *testing.T) {
	var lease Lease
	if lease.Key() != "" || lease.OwnerToken().RedisValue() != "" {
		t.Fatal("zero Lease accessors returned non-zero values")
	}
}

func TestTryAcquirePreservesCanceledContext(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	t.Cleanup(func() { _ = client.Close() })
	semaphore, err := New(client, Options{Key: "canceled", Permits: 1, TTL: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := semaphore.TryAcquire(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("TryAcquire() error = %v, want context.Canceled", err)
	}
}

func TestAcquirePreservesDeadline(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	t.Cleanup(func() { _ = client.Close() })
	semaphore, err := New(client, Options{Key: "deadline", Permits: 1, TTL: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	if _, err := semaphore.Acquire(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Acquire() error = %v, want context.DeadlineExceeded", err)
	}
}
