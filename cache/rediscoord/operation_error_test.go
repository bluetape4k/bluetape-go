package rediscoord

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/cache"
	btredis "github.com/bluetape4k/bluetape-go/redis"
	"github.com/redis/go-redis/v9"
)

func TestDirectRedisFailuresAreTypedAndRedacted(t *testing.T) {
	const (
		namespace = "namespace: key-marker"
		key       = "caller-key: key-marker"
		token     = "opaque-owner-token-marker"
		payload   = "payload-marker"
	)

	tests := []struct {
		name      string
		operation string
		rawKey    func(*StampedeCache[string]) string
		invoke    func(context.Context, *StampedeCache[string]) error
	}{
		{
			name:      "result get",
			operation: "result-get",
			rawKey:    func(coord *StampedeCache[string]) string { return coord.resultKey(key) },
			invoke: func(ctx context.Context, coord *StampedeCache[string]) error {
				_, _, err := coord.readOwnerResult(ctx, key, time.Second, token)
				return err
			},
		},
		{
			name:      "owner get",
			operation: "owner-get",
			rawKey:    func(coord *StampedeCache[string]) string { return coord.lockKey(key) },
			invoke: func(ctx context.Context, coord *StampedeCache[string]) error {
				_, _, err := coord.ownerToken(ctx, key)
				return err
			},
		},
		{
			name:      "result set",
			operation: "result-set",
			rawKey:    func(coord *StampedeCache[string]) string { return coord.resultKey(key) },
			invoke: func(ctx context.Context, coord *StampedeCache[string]) error {
				return coord.storeResult(ctx, key, token, []byte(payload))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			coord := newClosedCoordinator(t, namespace)
			err := tt.invoke(context.Background(), coord)
			assertCacheCoordinationOpError(t, err, tt.operation, tt.rawKey(coord), namespace, key, token, payload)
		})
	}
}

func TestEnsureOwnerFailureIsTypedAndRedacted(t *testing.T) {
	ctx := context.Background()
	const (
		namespace = "namespace: owner-key-marker"
		key       = "caller-key: owner-key-marker"
	)
	client := redisClient(ctx, t)
	coord := newMemoryCoordinator(t, client, namespace)
	lease, err := coord.tryAcquire(ctx, key)
	if err != nil {
		t.Fatalf("acquire owner lease: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}

	err = coord.ensureOwner(ctx, lease)
	assertCacheCoordinationOpError(t, err, "owner-check", lease.Key(), namespace, key, lease.Token())
}

func TestOperationErrorRetainsProviderCauseAndCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := operationError(ctx, "result-get", "caller-key: context-marker", redis.ErrClosed)
	assertCacheCoordinationOpError(t, err, "result-get", "caller-key: context-marker", "context-marker")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("operation error should retain canceled context: %v", err)
	}
}

func TestStampedeCachePreservesNamespaceAndCallerKeyBytes(t *testing.T) {
	const (
		namespace = " namespace: {cluster} "
		key       = " caller:key {item} "
	)
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	t.Cleanup(func() {
		_ = client.Close()
	})
	coord, err := NewStampedeCache[string](Options[string]{
		Client:    client,
		Cache:     cache.NewMemory[string, string](),
		Namespace: namespace,
		Codec:     JSONCodec[string]{},
	})
	if err != nil {
		t.Fatalf("new coordinator: %v", err)
	}

	if got, want := coord.lockKey(key), defaultKeyPrefix+":"+namespace+":lock:"+key; got != want {
		t.Fatalf("lock key = %q, want %q", got, want)
	}
	if got, want := coord.resultKey(key), defaultKeyPrefix+":"+namespace+":result:"+key; got != want {
		t.Fatalf("result key = %q, want %q", got, want)
	}
}

func TestResultEnvelopePreservesOpaqueToken(t *testing.T) {
	const token = "legacy opaque owner: {token}"

	encoded, err := encodeResult(token, []byte("value"))
	if err != nil {
		t.Fatalf("encode result: %v", err)
	}
	payload, ok, err := decodeResult(encoded, token)
	if err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if !ok || string(payload) != "value" {
		t.Fatalf("opaque token result = %q, ok=%t", payload, ok)
	}
}

func newClosedCoordinator(t *testing.T, namespace string) *StampedeCache[string] {
	t.Helper()

	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	coord, err := NewStampedeCache[string](Options[string]{
		Client:    client,
		Cache:     cache.NewMemory[string, string](),
		Namespace: namespace,
		Codec:     JSONCodec[string]{},
	})
	if err != nil {
		t.Fatalf("new coordinator: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}
	return coord
}

func assertCacheCoordinationOpError(
	t *testing.T,
	err error,
	operation string,
	rawKey string,
	forbidden ...string,
) {
	t.Helper()

	if !errors.Is(err, redis.ErrClosed) {
		t.Fatalf("error should retain redis.ErrClosed: %v", err)
	}
	var opErr *btredis.OpError
	if !errors.As(err, &opErr) {
		t.Fatalf("error should be redis.OpError, got %T: %v", err, err)
	}
	if opErr.Family() != "cache coordination" {
		t.Fatalf("family = %q, want cache coordination", opErr.Family())
	}
	if opErr.Operation() != operation {
		t.Fatalf("operation = %q, want %q", opErr.Operation(), operation)
	}
	if opErr.KeyID() != btredis.RedactedKeyID(rawKey) {
		t.Fatalf("key id = %q, want %q", opErr.KeyID(), btredis.RedactedKeyID(rawKey))
	}
	for _, value := range append(forbidden, rawKey) {
		if value != "" && strings.Contains(err.Error(), value) {
			t.Fatalf("error leaked %q: %v", value, err)
		}
	}
}
