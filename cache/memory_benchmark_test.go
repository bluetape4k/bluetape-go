package cache

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type benchmarkClock struct {
	now time.Time
}

func newBenchmarkClock() *benchmarkClock {
	return &benchmarkClock{now: time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)}
}

func (c *benchmarkClock) Now() time.Time {
	return c.now
}

func (c *benchmarkClock) Advance(duration time.Duration) {
	c.now = c.now.Add(duration)
}

func benchmarkKeys(prefix string, size int) []string {
	keys := make([]string, size)
	for i := range keys {
		keys[i] = prefix + "-" + strconv.Itoa(i)
	}
	return keys
}

func BenchmarkMemoryGetHit(b *testing.B) {
	ctx := context.Background()
	c := NewMemory[string, int]()
	if err := c.Set(ctx, "hit", 42, time.Minute); err != nil {
		b.Fatalf("set hit: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		value, err := c.Get(ctx, "hit")
		if err != nil {
			b.Fatalf("get hit: %v", err)
		}
		if value != 42 {
			b.Fatalf("value = %d, want 42", value)
		}
	}
}

func BenchmarkMemoryGetMiss(b *testing.B) {
	ctx := context.Background()
	c := NewMemory[string, int]()

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		if _, err := c.Get(ctx, "missing"); !errors.Is(err, ErrCacheMiss) {
			b.Fatalf("get miss: %v", err)
		}
	}
}

func BenchmarkMemorySet(b *testing.B) {
	ctx := context.Background()
	c := NewMemory[string, int]()

	b.ReportAllocs()
	b.ResetTimer()

	for i := range b.N {
		if err := c.Set(ctx, "set", i, time.Minute); err != nil {
			b.Fatalf("set: %v", err)
		}
	}
}

func BenchmarkMemoryDeleteExisting(b *testing.B) {
	ctx := context.Background()
	c := NewMemory[string, int]()
	keys := benchmarkKeys("delete", 1024)
	for i, key := range keys {
		if err := c.Set(ctx, key, i, time.Minute); err != nil {
			b.Fatalf("seed delete key: %v", err)
		}
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := range b.N {
		key := keys[i&1023]
		if err := c.Delete(ctx, key); err != nil {
			b.Fatalf("delete: %v", err)
		}
		if err := c.Set(ctx, key, i, time.Minute); err != nil {
			b.Fatalf("restore deleted key: %v", err)
		}
	}
}

func BenchmarkMemoryTTLExpiredGet(b *testing.B) {
	ctx := context.Background()
	clock := newBenchmarkClock()
	c := newMemoryWithClock[string, int](clock.Now)
	keys := benchmarkKeys("expired", 1024)

	b.ReportAllocs()
	b.ResetTimer()

	for i := range b.N {
		key := keys[i&1023]
		if err := c.Set(ctx, key, i, time.Nanosecond); err != nil {
			b.Fatalf("set expiring key: %v", err)
		}
		clock.Advance(2 * time.Nanosecond)
		if _, err := c.Get(ctx, key); !errors.Is(err, ErrCacheMiss) {
			b.Fatalf("expired get: %v", err)
		}
	}
}

func BenchmarkMemoryGetOrLoadHot(b *testing.B) {
	ctx := context.Background()
	c := NewMemory[string, int]()
	if _, err := c.GetOrLoad(ctx, "hot", time.Minute, func(context.Context, string) (int, error) {
		return 42, nil
	}); err != nil {
		b.Fatalf("prime hot key: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		value, err := c.GetOrLoad(ctx, "hot", time.Minute, func(context.Context, string) (int, error) {
			b.Fatal("hot loader should not run")
			return 0, nil
		})
		if err != nil {
			b.Fatalf("get hot key: %v", err)
		}
		if value != 42 {
			b.Fatalf("value = %d, want 42", value)
		}
	}
}

func BenchmarkMemoryGetOrLoadCold(b *testing.B) {
	ctx := context.Background()
	c := NewMemory[string, int]()
	keys := benchmarkKeys("cold", 1024)
	var loads int64

	b.ReportAllocs()
	b.ResetTimer()

	for i := range b.N {
		key := keys[i&1023] + "-" + strconv.Itoa(i/1024)
		value, err := c.GetOrLoad(ctx, key, time.Minute, func(context.Context, string) (int, error) {
			atomic.AddInt64(&loads, 1)
			return i, nil
		})
		if err != nil {
			b.Fatalf("get cold key: %v", err)
		}
		if value != i {
			b.Fatalf("value = %d, want %d", value, i)
		}
	}

	b.StopTimer()
	b.ReportMetric(float64(loads)/float64(b.N), "loads/op")
}

func BenchmarkMemoryGetOrLoadSameKeyConcurrent(b *testing.B) {
	const workers = 16
	ctx := context.Background()
	var totalLoads int64

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		c := NewMemory[string, int]()
		loads, err := runConcurrentLoads(ctx, workers, func() (int, error) {
			return c.GetOrLoad(ctx, "shared", time.Minute, func(context.Context, string) (int, error) {
				atomic.AddInt64(&totalLoads, 1)
				return 42, nil
			})
		})
		if err != nil {
			b.Fatalf("same-key concurrent load: %v", err)
		}
		if loads != 42*workers {
			b.Fatalf("sum = %d, want %d", loads, 42*workers)
		}
	}

	b.StopTimer()
	b.ReportMetric(float64(totalLoads)/float64(b.N), "loads/op")
}

func BenchmarkMemoryGetOrLoadDifferentKeysConcurrent(b *testing.B) {
	const workers = 16
	ctx := context.Background()
	var totalLoads int64

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		c := NewMemory[string, int]()
		loads, err := runConcurrentLoadsByWorker(ctx, workers, func(worker int) func() (int, error) {
			return func() (int, error) {
				key := "key-" + strconv.Itoa(worker)
				return c.GetOrLoad(ctx, key, time.Minute, func(context.Context, string) (int, error) {
					atomic.AddInt64(&totalLoads, 1)
					return worker, nil
				})
			}
		})
		if err != nil {
			b.Fatalf("different-key concurrent load: %v", err)
		}
		if loads != workers*(workers-1)/2 {
			b.Fatalf("sum = %d, want %d", loads, workers*(workers-1)/2)
		}
	}

	b.StopTimer()
	b.ReportMetric(float64(totalLoads)/float64(b.N), "loads/op")
}

func runConcurrentLoads(ctx context.Context, workers int, load func() (int, error)) (int, error) {
	return runConcurrentLoadsByWorker(ctx, workers, func(int) func() (int, error) {
		return load
	})
}

func runConcurrentLoadsByWorker(
	ctx context.Context,
	workers int,
	loadForWorker func(int) func() (int, error),
) (int, error) {
	start := make(chan struct{})
	results := make(chan int, workers)
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	wg.Add(workers)

	for worker := range workers {
		worker := worker
		go func() {
			defer wg.Done()
			select {
			case <-start:
			case <-ctx.Done():
				errs <- ctx.Err()
				return
			}
			value, err := loadForWorker(worker)()
			if err != nil {
				errs <- err
				return
			}
			results <- value
		}()
	}

	close(start)
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		if err != nil {
			return 0, err
		}
	}

	sum := 0
	for value := range results {
		sum += value
	}
	return sum, nil
}
