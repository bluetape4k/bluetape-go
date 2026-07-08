package redisbloom

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/probabilistic"
	"github.com/redis/go-redis/v9"
)

func TestHyperLogLogAddCountAndMerge(t *testing.T) {
	ctx := redisTestContext(t)
	client := newRedisClient(t)
	destinationNamespace := testNamespace(t) + ":dest"
	sourceNamespace := testNamespace(t) + ":source"
	cleanupNamespace(t, client, destinationNamespace)
	cleanupNamespace(t, client, sourceNamespace)

	destination, err := NewStringHyperLogLog(client, destinationNamespace)
	if err != nil {
		t.Fatalf("NewStringHyperLogLog destination failed: %v", err)
	}
	source, err := NewStringHyperLogLog(client, sourceNamespace)
	if err != nil {
		t.Fatalf("NewStringHyperLogLog source failed: %v", err)
	}

	changed, err := destination.Add(ctx, "alpha", "beta")
	if err != nil {
		t.Fatalf("destination Add failed: %v", err)
	}
	if !changed {
		t.Fatal("first Add should change HLL state")
	}
	changed, err = destination.Add(ctx, "alpha", "beta")
	if err != nil {
		t.Fatalf("duplicate Add failed: %v", err)
	}
	if changed {
		t.Fatal("duplicate Add should not change HLL state")
	}
	if _, err := source.Add(ctx, "gamma"); err != nil {
		t.Fatalf("source Add failed: %v", err)
	}

	count, err := destination.Count(ctx)
	if err != nil {
		t.Fatalf("destination Count failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("destination Count = %d, want 2", count)
	}

	if err := destination.Merge(ctx, sourceNamespace); err != nil {
		t.Fatalf("Merge failed: %v", err)
	}
	count, err = destination.Count(ctx)
	if err != nil {
		t.Fatalf("merged Count failed: %v", err)
	}
	if count != 3 {
		t.Fatalf("merged Count = %d, want 3", count)
	}
}

func TestHyperLogLogBytesAndCustomHasher(t *testing.T) {
	ctx := redisTestContext(t)
	client := newRedisClient(t)
	bytesNamespace := testNamespace(t) + ":bytes"
	customNamespace := testNamespace(t) + ":custom"
	cleanupNamespace(t, client, bytesNamespace)
	cleanupNamespace(t, client, customNamespace)

	bytesHLL, err := NewBytesHyperLogLog(client, bytesNamespace)
	if err != nil {
		t.Fatalf("NewBytesHyperLogLog failed: %v", err)
	}
	if _, err := bytesHLL.Add(ctx, []byte("alpha"), []byte("beta")); err != nil {
		t.Fatalf("bytes Add failed: %v", err)
	}
	count, err := bytesHLL.Count(ctx)
	if err != nil {
		t.Fatalf("bytes Count failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("bytes Count = %d, want 2", count)
	}

	hasher, err := probabilistic.NewHasher("hll:int:v1", func(value int) []byte {
		return []byte{byte(value)}
	})
	if err != nil {
		t.Fatalf("NewHasher failed: %v", err)
	}
	custom, err := NewHyperLogLog(HyperLogLogOptions[int]{
		Client:    client,
		Namespace: customNamespace,
		Hasher:    hasher,
	})
	if err != nil {
		t.Fatalf("NewHyperLogLog failed: %v", err)
	}
	if custom.HasherKey() != "hll:int:v1" {
		t.Fatalf("HasherKey = %q", custom.HasherKey())
	}
	if _, err := custom.Add(ctx, 1, 2, 3); err != nil {
		t.Fatalf("custom Add failed: %v", err)
	}
	count, err = custom.Count(ctx)
	if err != nil {
		t.Fatalf("custom Count failed: %v", err)
	}
	if count != 3 {
		t.Fatalf("custom Count = %d, want 3", count)
	}
}

func TestHyperLogLogRejectsInvalidOptions(t *testing.T) {
	t.Parallel()

	hasher, err := probabilistic.NewHasher("hll:string:v1", func(value string) []byte {
		return []byte(value)
	})
	if err != nil {
		t.Fatalf("NewHasher failed: %v", err)
	}
	for _, tc := range []struct {
		name    string
		options HyperLogLogOptions[string]
	}{
		{
			name: "nil client",
			options: HyperLogLogOptions[string]{
				Namespace: "tenant-a:hll",
				Hasher:    hasher,
			},
		},
		{
			name: "typed nil client",
			options: HyperLogLogOptions[string]{
				Client:    (*redis.Client)(nil),
				Namespace: "tenant-a:hll",
				Hasher:    hasher,
			},
		},
		{
			name: "invalid namespace",
			options: HyperLogLogOptions[string]{
				Client:    stubCmdable{},
				Namespace: "tenant@example.test",
				Hasher:    hasher,
			},
		},
		{
			name: "empty hasher",
			options: HyperLogLogOptions[string]{
				Client:    stubCmdable{},
				Namespace: "tenant-a:hll",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewHyperLogLog(tc.options)
			if !errors.Is(err, ErrInvalidOptions) {
				t.Fatalf("expected ErrInvalidOptions, got %v", err)
			}
		})
	}
}

func TestHyperLogLogRejectsInvalidMergeSources(t *testing.T) {
	ctx := redisTestContext(t)
	client := newRedisClient(t)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)
	hll, err := NewStringHyperLogLog(client, namespace)
	if err != nil {
		t.Fatalf("NewStringHyperLogLog failed: %v", err)
	}

	if changed, err := hll.Add(ctx); changed || err != nil {
		t.Fatalf("empty Add changed=%v err=%v, want no-op", changed, err)
	}
	if err := hll.Merge(ctx); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("empty Merge err=%v, want ErrInvalidOptions", err)
	}
	if err := hll.Merge(ctx, "raw@email.test"); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("unsafe source Merge err=%v, want ErrInvalidOptions", err)
	}
}

func TestHyperLogLogContextCancellationVisible(t *testing.T) {
	client := newRedisClient(t)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)
	hll, err := NewStringHyperLogLog(client, namespace)
	if err != nil {
		t.Fatalf("NewStringHyperLogLog failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := hll.Add(ctx, "alpha"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Add err = %v, want context.Canceled", err)
	}
	if _, err := hll.Count(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Count err = %v, want context.Canceled", err)
	}
	if err := hll.Merge(ctx, namespace+":source"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Merge err = %v, want context.Canceled", err)
	}
}

func TestHyperLogLogRedisErrorsAreWrappedAndRedacted(t *testing.T) {
	ctx := redisTestContext(t)
	client := newRedisClient(t)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)
	hll, err := NewStringHyperLogLog(client, namespace)
	if err != nil {
		t.Fatalf("NewStringHyperLogLog failed: %v", err)
	}
	key, err := buildHyperLogLogKey(namespace)
	if err != nil {
		t.Fatalf("buildHyperLogLogKey failed: %v", err)
	}
	if err := client.Set(ctx, key.key, "not-a-hyperloglog", 0).Err(); err != nil {
		t.Fatalf("seed invalid hll payload: %v", err)
	}

	_, err = hll.Count(ctx)
	var redisErr RedisError
	if !errors.As(err, &redisErr) {
		t.Fatalf("expected RedisError, got %v", err)
	}
	if redisErr.Family != "redis hll" {
		t.Fatalf("RedisError family = %q, want redis hll", redisErr.Family)
	}
	for _, sensitive := range []string{namespace, key.key, "not-a-hyperloglog"} {
		if strings.Contains(err.Error(), sensitive) {
			t.Fatalf("error leaked %q: %v", sensitive, err)
		}
	}
}

func TestHyperLogLogDoesNotSendRawValuesToRedis(t *testing.T) {
	ctx := redisTestContext(t)
	client := newRedisClient(t)
	recorder := newCommandPayloadRecorder()
	client.AddHook(recorder)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)
	hll, err := NewStringHyperLogLog(client, namespace)
	if err != nil {
		t.Fatalf("NewStringHyperLogLog failed: %v", err)
	}

	if _, err := hll.Add(ctx, "alice@example.test"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	for _, arg := range recorder.Args() {
		if strings.Contains(arg, "alice@example.test") {
			t.Fatalf("redis command leaked raw value in arg %q", arg)
		}
	}
}

func TestHyperLogLogConcurrentAddAndCount(t *testing.T) {
	ctx := redisTestContext(t)
	client := newRedisClient(t)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)
	hll, err := NewStringHyperLogLog(client, namespace)
	if err != nil {
		t.Fatalf("NewStringHyperLogLog failed: %v", err)
	}

	var wg sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				value := fmt.Sprintf("%d:%d:%s", worker, i, time.Now().Format(time.RFC3339Nano))
				if _, err := hll.Add(ctx, value); err != nil {
					t.Errorf("Add failed: %v", err)
					return
				}
				if _, err := hll.Count(ctx); err != nil {
					t.Errorf("Count failed: %v", err)
					return
				}
			}
		}(worker)
	}
	wg.Wait()
}

type commandPayloadRecorder struct {
	mu   sync.Mutex
	args []string
}

func newCommandPayloadRecorder() *commandPayloadRecorder {
	return &commandPayloadRecorder{}
}

func (r *commandPayloadRecorder) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (r *commandPayloadRecorder) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		r.record(cmd)
		return next(ctx, cmd)
	}
}

func (r *commandPayloadRecorder) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		for _, cmd := range cmds {
			r.record(cmd)
		}
		return next(ctx, cmds)
	}
}

func (r *commandPayloadRecorder) record(cmd redis.Cmder) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, arg := range cmd.Args() {
		r.args = append(r.args, fmt.Sprint(arg))
	}
}

func (r *commandPayloadRecorder) Args() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	copied := make([]string, len(r.args))
	copy(copied, r.args)
	return copied
}
