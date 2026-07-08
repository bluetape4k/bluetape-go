package redisbloom

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestPutAndMightContainHaveNoFalseNegative(t *testing.T) {
	ctx := redisTestContext(t)
	client := newRedisClient(t)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)
	filter, err := NewStringBloomFilter(ctx, client, namespace, testConfig(t, 1000))
	if err != nil {
		t.Fatalf("NewStringBloomFilter failed: %v", err)
	}

	changed, err := filter.Put(ctx, "alpha")
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}
	if !changed {
		t.Fatal("first Put should change bits")
	}
	changed, err = filter.Put(ctx, "alpha")
	if err != nil {
		t.Fatalf("second Put failed: %v", err)
	}
	if changed {
		t.Fatal("second Put should not change bits")
	}

	ok, err := filter.MightContain(ctx, "alpha")
	if err != nil {
		t.Fatalf("MightContain inserted failed: %v", err)
	}
	if !ok {
		t.Fatal("inserted value produced false negative")
	}

	ok, err = filter.MightContain(ctx, "definitely-missing")
	if err != nil {
		t.Fatalf("MightContain missing failed: %v", err)
	}
	if ok {
		t.Fatal("unexpected positive for definitely missing value")
	}
}

func TestClearPreservesConfigAndClearsBitmap(t *testing.T) {
	ctx := redisTestContext(t)
	client := newRedisClient(t)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)
	filter, err := NewStringBloomFilter(ctx, client, namespace, testConfig(t, 1000))
	if err != nil {
		t.Fatalf("NewStringBloomFilter failed: %v", err)
	}
	changed, err := filter.Put(ctx, "alpha")
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}
	if !changed {
		t.Fatal("Put should change bits")
	}

	if err := filter.Clear(ctx); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	empty, err := filter.IsEmpty(ctx)
	if err != nil {
		t.Fatalf("IsEmpty failed: %v", err)
	}
	if !empty {
		t.Fatal("expected filter to be empty")
	}

	keys, err := buildKeys(namespace)
	if err != nil {
		t.Fatalf("buildKeys failed: %v", err)
	}
	if exists := client.Exists(ctx, keys.config).Val(); exists != 1 {
		t.Fatalf("config key exists = %d, want 1", exists)
	}
}

func TestExternalBitmapDeletionCreatesEmptyState(t *testing.T) {
	ctx := redisTestContext(t)
	client := newRedisClient(t)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)
	filter, err := NewStringBloomFilter(ctx, client, namespace, testConfig(t, 1000))
	if err != nil {
		t.Fatalf("NewStringBloomFilter failed: %v", err)
	}
	changed, err := filter.Put(ctx, "alpha")
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}
	if !changed {
		t.Fatal("Put should change bits")
	}

	keys, err := buildKeys(namespace)
	if err != nil {
		t.Fatalf("buildKeys failed: %v", err)
	}
	if err := client.Del(ctx, keys.bits).Err(); err != nil {
		t.Fatalf("delete bitmap: %v", err)
	}

	ok, err := filter.MightContain(ctx, "alpha")
	if err != nil {
		t.Fatalf("MightContain failed: %v", err)
	}
	if ok {
		t.Fatal("deleted bitmap should read empty")
	}
}

func TestOperationsRejectChangedConfigBeforeBitmapTouch(t *testing.T) {
	ctx := redisTestContext(t)
	client := newRedisClient(t)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)
	filter, err := NewStringBloomFilter(ctx, client, namespace, testConfig(t, 1000))
	if err != nil {
		t.Fatalf("NewStringBloomFilter failed: %v", err)
	}

	keys, err := buildKeys(namespace)
	if err != nil {
		t.Fatalf("buildKeys failed: %v", err)
	}
	if err := client.HSet(ctx, keys.config, "fingerprint", "changed").Err(); err != nil {
		t.Fatalf("change fingerprint: %v", err)
	}

	changed, err := filter.Put(ctx, "alpha")
	if changed {
		t.Fatal("Put should not change bitmap on config mismatch")
	}
	if !errors.Is(err, ErrConfigMismatch) {
		t.Fatalf("expected ErrConfigMismatch, got %v", err)
	}
	ok, err := filter.MightContain(ctx, "alpha")
	if ok {
		t.Fatal("MightContain should be false on config mismatch")
	}
	if !errors.Is(err, ErrConfigMismatch) {
		t.Fatalf("expected ErrConfigMismatch, got %v", err)
	}
	if size := client.StrLen(ctx, keys.bits).Val(); size != 0 {
		t.Fatalf("bitmap strlen = %d, want 0", size)
	}
}

func TestOperationsRejectMissingOrCorruptConfig(t *testing.T) {
	for _, tc := range []struct {
		name   string
		damage func(context.Context, *redis.Client, redisKeys) error
	}{
		{
			name: "missing config",
			damage: func(ctx context.Context, client *redis.Client, keys redisKeys) error {
				return client.Del(ctx, keys.config).Err()
			},
		},
		{
			name: "missing fingerprint",
			damage: func(ctx context.Context, client *redis.Client, keys redisKeys) error {
				return client.HDel(ctx, keys.config, "fingerprint").Err()
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := redisTestContext(t)
			client := newRedisClient(t)
			namespace := testNamespace(t)
			cleanupNamespace(t, client, namespace)
			filter, err := NewStringBloomFilter(ctx, client, namespace, testConfig(t, 1000))
			if err != nil {
				t.Fatalf("NewStringBloomFilter failed: %v", err)
			}
			if _, err := filter.Put(ctx, "seed"); err != nil {
				t.Fatalf("seed Put failed: %v", err)
			}
			keys, err := buildKeys(namespace)
			if err != nil {
				t.Fatalf("buildKeys failed: %v", err)
			}
			before := client.Dump(ctx, keys.bits).Val()
			if err := tc.damage(ctx, client, keys); err != nil {
				t.Fatalf("damage config: %v", err)
			}

			if changed, err := filter.Put(ctx, "alpha"); changed || !errors.Is(err, ErrConfigCorrupt) {
				t.Fatalf("Put changed=%v err=%v, want ErrConfigCorrupt without change", changed, err)
			}
			if ok, err := filter.MightContain(ctx, "seed"); ok || !errors.Is(err, ErrConfigCorrupt) {
				t.Fatalf("MightContain ok=%v err=%v, want ErrConfigCorrupt", ok, err)
			}
			if err := filter.Clear(ctx); !errors.Is(err, ErrConfigCorrupt) {
				t.Fatalf("Clear err=%v, want ErrConfigCorrupt", err)
			}
			if _, err := filter.BitCount(ctx); !errors.Is(err, ErrConfigCorrupt) {
				t.Fatalf("BitCount err=%v, want ErrConfigCorrupt", err)
			}
			if _, err := filter.IsEmpty(ctx); !errors.Is(err, ErrConfigCorrupt) {
				t.Fatalf("IsEmpty err=%v, want ErrConfigCorrupt", err)
			}
			after := client.Dump(ctx, keys.bits).Val()
			if before != after {
				t.Fatal("bitmap changed after corrupt config operations")
			}
		})
	}
}

func TestConcurrentPutAndMightContain(t *testing.T) {
	ctx := redisTestContext(t)
	client := newRedisClient(t)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)
	filter, err := NewStringBloomFilter(ctx, client, namespace, testConfig(t, 10000))
	if err != nil {
		t.Fatalf("NewStringBloomFilter failed: %v", err)
	}

	var wg sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				value := strconv.Itoa(worker) + ":" + strconv.Itoa(i)
				if _, err := filter.Put(ctx, value); err != nil {
					t.Errorf("Put failed: %v", err)
					return
				}
				ok, err := filter.MightContain(ctx, value)
				if err != nil {
					t.Errorf("MightContain failed: %v", err)
					return
				}
				if !ok {
					t.Errorf("inserted value missing: %s", value)
					return
				}
			}
		}(worker)
	}
	wg.Wait()
}

func TestContextCancellationVisible(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := newRedisClient(t)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)
	filter, err := NewStringBloomFilter(redisTestContext(t), client, namespace, testConfig(t, 1000))
	if err != nil {
		t.Fatalf("NewStringBloomFilter failed: %v", err)
	}

	_, err = filter.MightContain(ctx, "alpha")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestDeadlineExceededVisible(t *testing.T) {
	client := newRedisClient(t)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)
	filter, err := NewStringBloomFilter(redisTestContext(t), client, namespace, testConfig(t, 1000))
	if err != nil {
		t.Fatalf("NewStringBloomFilter failed: %v", err)
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	_, err = filter.MightContain(ctx, "alpha")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
}

type commandRecorder struct {
	mu     sync.Mutex
	counts map[string]int
}

func newCommandRecorder() *commandRecorder {
	return &commandRecorder{counts: make(map[string]int)}
}

func (r *commandRecorder) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (r *commandRecorder) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		r.record(cmd)
		return next(ctx, cmd)
	}
}

func (r *commandRecorder) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		for _, cmd := range cmds {
			r.record(cmd)
		}
		return next(ctx, cmds)
	}
}

func (r *commandRecorder) record(cmd redis.Cmder) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.counts[strings.ToLower(cmd.Name())]++
}

func (r *commandRecorder) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for name := range r.counts {
		delete(r.counts, name)
	}
}

func (r *commandRecorder) Count(name string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.counts[strings.ToLower(name)]
}

func (r *commandRecorder) ScriptDelta() int {
	return r.Count("eval") + r.Count("evalsha")
}

func TestHotPathUsesOneScriptRoundTripPerCall(t *testing.T) {
	ctx := redisTestContext(t)
	client := newRedisClient(t)
	recorder := newCommandRecorder()
	client.AddHook(recorder)
	namespace := testNamespace(t)
	cleanupNamespace(t, client, namespace)
	filter, err := NewStringBloomFilter(ctx, client, namespace, testConfig(t, 1000))
	if err != nil {
		t.Fatalf("NewStringBloomFilter failed: %v", err)
	}

	_, _ = filter.Put(ctx, "warm")
	_, _ = filter.MightContain(ctx, "warm")
	_, _ = filter.BitCount(ctx)
	_, _ = filter.IsEmpty(ctx)
	_ = filter.Clear(ctx)

	assertOneScript := func(name string, op func() error) {
		t.Helper()
		recorder.Reset()
		if err := op(); err != nil {
			t.Fatalf("%s failed: %v", name, err)
		}
		if got := recorder.ScriptDelta(); got != 1 {
			t.Fatalf("%s script commands = %d, want 1", name, got)
		}
		if got := recorder.Count("eval"); got != 0 {
			t.Fatalf("%s eval commands = %d, want 0 after warmup", name, got)
		}
		for _, direct := range []string{"getbit", "setbit", "bitcount", "hget", "hgetall", "hset", "del", "strlen"} {
			if got := recorder.Count(direct); got != 0 {
				t.Fatalf("%s sent direct %s command %d times", name, direct, got)
			}
		}
	}

	assertOneScript("init matched", func() error {
		_, err := NewStringBloomFilter(ctx, client, namespace, testConfig(t, 1000))
		return err
	})
	assertOneScript("put", func() error {
		_, err := filter.Put(ctx, "alpha")
		return err
	})
	assertOneScript("might contain", func() error {
		_, err := filter.MightContain(ctx, "alpha")
		return err
	})
	assertOneScript("bit count", func() error {
		_, err := filter.BitCount(ctx)
		return err
	})
	assertOneScript("is empty", func() error {
		_, err := filter.IsEmpty(ctx)
		return err
	})
	assertOneScript("clear", func() error {
		return filter.Clear(ctx)
	})
}

func TestLuaScriptsAreStaticAndUseKeysArgv(t *testing.T) {
	scripts := map[string]string{
		"init":         initConfigLua,
		"put":          putLua,
		"mightContain": mightContainLua,
		"clear":        clearLua,
		"bitCount":     bitCountLua,
		"isEmpty":      isEmptyLua,
	}

	for name, script := range scripts {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(script, "KEYS[") {
				t.Fatal("script does not reference KEYS")
			}
			if !strings.Contains(script, "ARGV[") {
				t.Fatal("script does not reference ARGV")
			}
			if strings.Contains(script, "{"+"{") || strings.Contains(script, "%s") {
				t.Fatal("script appears to contain runtime interpolation marker")
			}
		})
	}
}
