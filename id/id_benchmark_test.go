package id

import (
	"sync/atomic"
	"testing"
	"time"
)

func BenchmarkUUIDV4(b *testing.B) {
	for b.Loop() {
		if _, err := NewUUIDV4(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUUIDV7(b *testing.B) {
	for b.Loop() {
		if _, err := NewUUIDV7(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkULIDRandom(b *testing.B) {
	generator, err := NewULIDGenerator()
	if err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		if _, err := generator.NextString(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkULIDMonotonicParallel(b *testing.B) {
	generator, err := NewMonotonicULIDGenerator()
	if err != nil {
		b.Fatal(err)
	}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := generator.NextString(); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkKSUIDNextString(b *testing.B) {
	generator, err := NewKSUIDGenerator()
	if err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		if _, err := generator.NextString(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSnowflakeNextInt64(b *testing.B) {
	generator, err := newBenchmarkSnowflake()
	if err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		if _, err := generator.NextInt64(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSnowflakeNextInt64SameMillisecond(b *testing.B) {
	generator, err := newSameMillisecondBenchmarkSnowflake()
	if err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		if _, err := generator.NextInt64(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSnowflakeNextInt64Parallel(b *testing.B) {
	generator, err := newBenchmarkSnowflake()
	if err != nil {
		b.Fatal(err)
	}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := generator.NextInt64(); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func newBenchmarkSnowflake() (SnowflakeGenerator, error) {
	epoch := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var millis atomic.Int64
	return NewSnowflakeGenerator(1, WithSnowflakeEpoch(epoch), WithSnowflakeTime(func() time.Time {
		return epoch.Add(time.Duration(millis.Add(1)) * time.Millisecond)
	}))
}

func newSameMillisecondBenchmarkSnowflake() (SnowflakeGenerator, error) {
	epoch := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var calls atomic.Int64
	return NewSnowflakeGenerator(1, WithSnowflakeEpoch(epoch), WithSnowflakeTime(func() time.Time {
		index := calls.Add(1) - 1
		millis := index / (snowflakeMaxSequence + 1)
		return epoch.Add(time.Duration(millis) * time.Millisecond)
	}))
}
