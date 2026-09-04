package redislock

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	btredis "github.com/bluetape4k/bluetape-go/redis"
	"github.com/redis/go-redis/v9"
)

func TestNewRejectsInvalidOptions(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	t.Cleanup(func() { _ = client.Close() })
	tests := []struct {
		name   string
		client redis.Cmdable
		opts   Options
		want   error
	}{
		{name: "nil client", opts: Options{Key: "k", TTL: time.Second}, want: btredis.ErrInvalidKey},
		{name: "blank key", client: client, opts: Options{Key: "  ", TTL: time.Second}, want: btredis.ErrInvalidKey},
		{name: "zero ttl", client: client, opts: Options{Key: "k"}, want: btredis.ErrInvalidTTL},
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

func TestBuildKeysUsesSameRedactedHashTag(t *testing.T) {
	keys, err := buildKeys(" caller-owned:orders ")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(keys.owner, "{redis-key:") || !strings.Contains(keys.counter, "{redis-key:") {
		t.Fatalf("keys must use redacted hash tags: %#v", keys)
	}
	ownerStart := strings.IndexByte(keys.owner, '{')
	ownerEnd := strings.IndexByte(keys.owner, '}')
	counterStart := strings.IndexByte(keys.counter, '{')
	counterEnd := strings.IndexByte(keys.counter, '}')
	if ownerStart < 0 || ownerEnd < 0 || counterStart < 0 || counterEnd < 0 {
		t.Fatalf("keys must contain hash tags: %#v", keys)
	}
	ownerTag := keys.owner[ownerStart : ownerEnd+1]
	counterTag := keys.counter[counterStart : counterEnd+1]
	if ownerTag != counterTag {
		t.Fatalf("hash tags differ: owner=%q counter=%q", ownerTag, counterTag)
	}
}

func TestParseAcquireResult(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		acquired bool
		fence    uint64
		wantErr  bool
	}{
		{name: "busy", value: []int64{0, 0}},
		{name: "acquired", value: []int64{1, 42}, acquired: true, fence: 42},
		{name: "wrong length", value: []int64{1}, wantErr: true},
		{name: "zero fence", value: []int64{1, 0}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := redis.NewCmd(context.Background(), "eval")
			cmd.SetVal(tt.value)
			acquired, fence, err := parseAcquireResult(cmd)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parse error = %v", err)
			}
			if err == nil && (acquired != tt.acquired || fence != tt.fence) {
				t.Fatalf("parse result = %t/%d", acquired, fence)
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
	if lease.Key() != "" || lease.OwnerToken().RedisValue() != "" || lease.FencingToken() != 0 {
		t.Fatal("zero Lease accessors returned non-zero values")
	}
}

func TestTryAcquirePreservesCanceledContext(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	t.Cleanup(func() { _ = client.Close() })
	lock, err := New(client, Options{Key: "canceled", TTL: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := lock.TryAcquire(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("TryAcquire() error = %v, want context.Canceled", err)
	}
}

func TestAcquirePreservesDeadline(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	t.Cleanup(func() { _ = client.Close() })
	lock, err := New(client, Options{Key: "deadline", TTL: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	if _, err := lock.Acquire(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Acquire() error = %v, want context.DeadlineExceeded", err)
	}
}
