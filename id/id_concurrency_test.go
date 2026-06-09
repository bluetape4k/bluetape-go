package id

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

func TestGeneratorsAreConcurrentSafe(t *testing.T) {
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       8,
		RoundsPerTask: 32,
		Timeout:       5 * time.Second,
	})

	monotonic, err := NewMonotonicULIDGenerator()
	if err != nil {
		t.Fatalf("NewMonotonicULIDGenerator failed: %v", err)
	}
	snowflake, err := NewSnowflakeGenerator(7)
	if err != nil {
		t.Fatalf("NewSnowflakeGenerator failed: %v", err)
	}

	var mu sync.Mutex
	seen := make(map[string]struct{})
	record := func(kind string, value string) error {
		mu.Lock()
		defer mu.Unlock()
		key := kind + ":" + value
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate %s", key)
		}
		seen[key] = struct{}{}
		return nil
	}

	report, err := tester.Run(context.Background(),
		func(context.Context) error {
			value, err := NewUUIDV4()
			if err != nil {
				return err
			}
			return record("uuid-v4", value)
		},
		func(context.Context) error {
			value, err := NewUUIDV7()
			if err != nil {
				return err
			}
			return record("uuid-v7", value)
		},
		func(context.Context) error {
			value, err := monotonic.NextString()
			if err != nil {
				return err
			}
			return record("ulid-monotonic", value)
		},
		func(context.Context) error {
			value, err := snowflake.NextString()
			if err != nil {
				return err
			}
			return record("snowflake", value)
		},
	)
	if err != nil {
		t.Fatalf("stress run failed: report=%+v err=%v", report, err)
	}
	if report.Completed != 128 {
		t.Fatalf("expected 128 completions, got %+v", report)
	}
}

func TestGUIDGeneratorsStayUniqueAcrossGoroutines(t *testing.T) {
	tests := []struct {
		name string
		new  func() (StringGenerator, error)
	}{
		{
			name: "uuid-v4",
			new: func() (StringGenerator, error) {
				return NewUUIDV4Generator()
			},
		},
		{
			name: "uuid-v7",
			new: func() (StringGenerator, error) {
				return NewUUIDV7Generator()
			},
		},
		{
			name: "ulid-random",
			new: func() (StringGenerator, error) {
				return NewULIDGenerator()
			},
		},
		{
			name: "ulid-monotonic",
			new: func() (StringGenerator, error) {
				return NewMonotonicULIDGenerator()
			},
		},
		{
			name: "ksuid",
			new: func() (StringGenerator, error) {
				return NewKSUIDGenerator()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator, err := tt.new()
			if err != nil {
				t.Fatalf("%s generator failed: %v", tt.name, err)
			}

			const (
				workerGoroutines = 64
				rounds           = 512
				idsPerTask       = 16
			)
			expected := rounds * idsPerTask
			tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
				Workers:       max(workerGoroutines, runtime.GOMAXPROCS(0)*4),
				RoundsPerTask: rounds,
				Timeout:       10 * time.Second,
			})

			var (
				created atomic.Int64
				mu      sync.Mutex
				seen    = make(map[string]struct{}, expected)
			)
			report, err := tester.Run(context.Background(), func(context.Context) error {
				for range idsPerTask {
					value, err := generator.NextString()
					if err != nil {
						return err
					}
					mu.Lock()
					if _, ok := seen[value]; ok {
						mu.Unlock()
						return fmt.Errorf("duplicate %s ID %q", tt.name, value)
					}
					seen[value] = struct{}{}
					mu.Unlock()
					created.Add(1)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("%s stress failed: report=%+v err=%v", tt.name, report, err)
			}
			if report.Completed != rounds {
				t.Fatalf("%s expected %d completions across goroutine workers, got %+v", tt.name, rounds, report)
			}
			if int(created.Load()) != expected || len(seen) != expected {
				t.Fatalf("%s expected %d unique IDs, created=%d seen=%d", tt.name, expected, created.Load(), len(seen))
			}
		})
	}
}
