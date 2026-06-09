package probabilistic

import (
	"context"
	"runtime"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

func TestBloomFilterStressConcurrentOperations(t *testing.T) {
	rounds := 512
	workers := max(32, runtime.GOMAXPROCS(0)*4)

	left := newStringFilterForTest(t, 20_000, 0.01)
	right := newStringFilterForTest(t, 20_000, 0.01)
	for i := 0; i < 2_000; i++ {
		right.Put("seed-" + strconv.Itoa(i))
	}

	var operations atomic.Int64
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       workers,
		RoundsPerTask: rounds,
		Timeout:       10 * time.Second,
	})

	report := tester.RunT(
		t,
		func(context.Context) error {
			value := operations.Add(1)
			left.Put("left-" + strconv.FormatInt(value, 10))
			return nil
		},
		func(context.Context) error {
			value := operations.Add(1)
			left.MightContain("left-" + strconv.FormatInt(value, 10))
			return nil
		},
		func(context.Context) error {
			_ = left.BitCount()
			_ = left.ApproximateElementCount()
			_ = left.ExpectedFPP()
			return nil
		},
		func(context.Context) error {
			return left.PutAll(right)
		},
		func(context.Context) error {
			return right.PutAll(left)
		},
		func(context.Context) error {
			return left.PutAll(left)
		},
		func(context.Context) error {
			left.Clear()
			return nil
		},
	)

	expectedCompletions := rounds * 7
	if report.Completed != expectedCompletions {
		t.Fatalf("expected %d completions, got %+v", expectedCompletions, report)
	}

	left.Put("final")
	if !left.MightContain("final") {
		t.Fatal("expected no false negative after final insert not followed by Clear")
	}
}

func TestBloomFilterStressCustomHasher(t *testing.T) {
	cfg, err := NewConfig(10_000, 0.01)
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}
	var hashCalls atomic.Int64
	hasher, err := NewHasher("stress-int-decimal", func(value int) []byte {
		hashCalls.Add(1)
		return []byte(strconv.Itoa(value))
	})
	if err != nil {
		t.Fatalf("NewHasher failed: %v", err)
	}
	filter, err := NewBloomFilter(cfg, hasher)
	if err != nil {
		t.Fatalf("NewBloomFilter failed: %v", err)
	}

	rounds := 256
	workers := max(16, runtime.GOMAXPROCS(0)*2)
	var operations atomic.Int64
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       workers,
		RoundsPerTask: rounds,
		Timeout:       10 * time.Second,
	})

	report := tester.RunT(
		t,
		func(context.Context) error {
			value := int(operations.Add(1))
			filter.Put(value)
			return nil
		},
		func(context.Context) error {
			value := int(operations.Add(1))
			filter.MightContain(value)
			return nil
		},
		func(context.Context) error {
			_ = filter.ExpectedFPP()
			return nil
		},
	)

	expectedCompletions := rounds * 3
	if report.Completed != expectedCompletions {
		t.Fatalf("expected %d completions, got %+v", expectedCompletions, report)
	}
	if hashCalls.Load() == 0 {
		t.Fatal("expected custom hasher to be called")
	}
}
