package redisvalue

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/cache"
	"github.com/bluetape4k/bluetape-go/serialization"
	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
	"github.com/redis/go-redis/v9"
)

func TestTieredCacheSetRaceWithClearLocalDoesNotPublishLateL1(t *testing.T) {
	setEntered := make(chan struct{})
	releaseSet := make(chan struct{})
	client := &fakeCommandClient{set: func(context.Context, string, any, time.Duration) *redis.StatusCmd {
		close(setEntered)
		<-releaseSet
		return redis.NewStatusResult("OK", nil)
	}}
	local := &faultLocal[string]{}
	tiered := newMutationTiered(t, local, client, serialization.StringSerializer{})
	setDone := make(chan error, 1)
	go func() { setDone <- tiered.Set(context.Background(), "item", "late", time.Minute) }()
	<-setEntered
	if err := tiered.ClearLocal(context.Background()); err != nil {
		t.Fatal(err)
	}
	close(releaseSet)
	if err := <-setDone; err != nil {
		t.Fatal(err)
	}
	if len(local.setValues) != 0 {
		t.Fatalf("late local values = %v", local.setValues)
	}
}

func TestTieredCacheMixedStressRetiresState(t *testing.T) {
	remoteClient := newStressCommandClient()
	remote := unitValueCache[int](remoteClient, serialization.JSONSerializer[int]{}, ValueConfig{
		RemoteTTL: time.Hour, MaxValueBytes: 64, ClearBatchSize: 10,
	})
	tiered := mustTieredCache(t, cache.NewMemory[string, int](), remote, nil)
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers: 16, RoundsPerTask: 600, Timeout: 10 * time.Second,
	})
	var sequence atomic.Uint64
	var loadAttempts atomic.Int64
	report, err := tester.Run(context.Background(), func(ctx context.Context) error {
		round := int(sequence.Add(1) - 1)
		key := strconv.Itoa(round % 8)
		switch round % 6 {
		case 0:
			_, err := tiered.Get(ctx, key)
			if err != nil && !errors.Is(err, cache.ErrCacheMiss) {
				return err
			}
			return nil
		case 1:
			loadAttempts.Add(1)
			_, err := tiered.GetOrLoad(ctx, key, time.Minute, func(context.Context, string) (int, error) {
				return round, nil
			})
			return err
		case 2:
			return tiered.Set(ctx, key, round, time.Minute)
		case 3:
			return tiered.Delete(ctx, key)
		case 4:
			return tiered.ClearLocal(ctx)
		default:
			canceled, cancel := context.WithCancel(ctx)
			cancel()
			_, err := tiered.Get(canceled, key)
			if !errors.Is(err, context.Canceled) {
				return fmt.Errorf("canceled get: %w", err)
			}
			return nil
		}
	})
	if err != nil || report.Completed != 600 {
		t.Fatalf("stress report = %+v, %v", report, err)
	}
	if loadAttempts.Load() != 100 {
		t.Fatalf("load attempts = %d", loadAttempts.Load())
	}
	if tiered.coordinators.active() != 0 {
		t.Fatalf("active coordinators = %d", tiered.coordinators.active())
	}
	if tiered.localState.phaseValue() != phaseHealthy {
		t.Fatalf("local phase = %v", tiered.localState.phaseValue())
	}
}

func TestTieredCacheHealthyL1PathDoesNotCallDependenciesOrAllocate(t *testing.T) {
	local := cache.NewMemory[string, string]()
	if err := local.Set(context.Background(), "hot", "value", time.Hour); err != nil {
		t.Fatal(err)
	}
	panicCall := func() { panic("healthy L1 path called Redis or serializer") }
	client := &fakeCommandClient{
		getRange: func(context.Context, string, int64, int64) *redis.StringCmd { panicCall(); return nil },
		exists:   func(context.Context, ...string) *redis.IntCmd { panicCall(); return nil },
		set:      func(context.Context, string, any, time.Duration) *redis.StatusCmd { panicCall(); return nil },
		del:      func(context.Context, ...string) *redis.IntCmd { panicCall(); return nil },
		scan:     func(context.Context, uint64, string, int64) *redis.ScanCmd { panicCall(); return nil },
		unlink:   func(context.Context, ...string) *redis.IntCmd { panicCall(); return nil },
	}
	serializer := &serializerFuncs[string]{
		marshal:   func(string) ([]byte, error) { panicCall(); return nil, nil },
		unmarshal: func([]byte) (string, error) { panicCall(); return "", nil },
	}
	remote := unitValueCache[string](client, serializer, ValueConfig{RemoteTTL: time.Hour, MaxValueBytes: 32, ClearBatchSize: 2})
	tiered := mustTieredCache(t, local, remote, nil)
	ctx := context.Background()
	allocs := testing.AllocsPerRun(1000, func() {
		value, err := tiered.Get(ctx, "hot")
		if err != nil || value != "value" {
			panic("unexpected healthy L1 result")
		}
	})
	if allocs != 0 {
		t.Fatalf("healthy L1 allocations = %f", allocs)
	}
}

type stressCommandClient struct {
	mu     sync.RWMutex
	values map[string][]byte
}

func newStressCommandClient() *stressCommandClient {
	return &stressCommandClient{values: make(map[string][]byte)}
}

func (c *stressCommandClient) GetRange(ctx context.Context, key string, start, end int64) *redis.StringCmd {
	if err := ctx.Err(); err != nil {
		return redis.NewStringResult("", err)
	}
	c.mu.RLock()
	value, ok := c.values[key]
	copyOfValue := append([]byte(nil), value...)
	c.mu.RUnlock()
	if !ok {
		return redis.NewStringResult("", redis.Nil)
	}
	if start >= int64(len(copyOfValue)) {
		return redis.NewStringResult("", nil)
	}
	if end >= int64(len(copyOfValue)) {
		end = int64(len(copyOfValue)) - 1
	}
	return redis.NewStringResult(string(copyOfValue[start:end+1]), nil)
}

func (c *stressCommandClient) Exists(ctx context.Context, keys ...string) *redis.IntCmd {
	if err := ctx.Err(); err != nil {
		return redis.NewIntResult(0, err)
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	var count int64
	for _, key := range keys {
		if _, ok := c.values[key]; ok {
			count++
		}
	}
	return redis.NewIntResult(count, nil)
}

func (c *stressCommandClient) Set(ctx context.Context, key string, value any, _ time.Duration) *redis.StatusCmd {
	if err := ctx.Err(); err != nil {
		return redis.NewStatusResult("", err)
	}
	encoded := append([]byte(nil), value.([]byte)...)
	c.mu.Lock()
	c.values[key] = encoded
	c.mu.Unlock()
	return redis.NewStatusResult("OK", nil)
}

func (c *stressCommandClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	if err := ctx.Err(); err != nil {
		return redis.NewIntResult(0, err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	var deleted int64
	for _, key := range keys {
		if _, ok := c.values[key]; ok {
			delete(c.values, key)
			deleted++
		}
	}
	return redis.NewIntResult(deleted, nil)
}

func (c *stressCommandClient) Scan(ctx context.Context, _ uint64, _ string, _ int64) *redis.ScanCmd {
	if err := ctx.Err(); err != nil {
		return redis.NewScanCmdResult(nil, 0, err)
	}
	c.mu.RLock()
	keys := make([]string, 0, len(c.values))
	for key := range c.values {
		keys = append(keys, key)
	}
	c.mu.RUnlock()
	sort.Strings(keys)
	return redis.NewScanCmdResult(keys, 0, nil)
}

func (c *stressCommandClient) Unlink(ctx context.Context, keys ...string) *redis.IntCmd {
	return c.Del(ctx, keys...)
}
