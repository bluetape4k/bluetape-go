package probabilistic

import (
	"strconv"
	"testing"
)

func BenchmarkBloomFilterPut(b *testing.B) {
	filter := newStringFilterForBenchmark(b)
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		filter.Put("value-" + strconv.Itoa(i))
	}
}

func BenchmarkBloomFilterMightContain(b *testing.B) {
	filter := newStringFilterForBenchmark(b)
	for i := 0; i < 10_000; i++ {
		filter.Put("value-" + strconv.Itoa(i))
	}
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		filter.MightContain("value-" + strconv.Itoa(i%10_000))
	}
}

func BenchmarkBloomFilterPutAll(b *testing.B) {
	cfg, err := NewConfig(10_000, 0.01)
	if err != nil {
		b.Fatalf("NewConfig failed: %v", err)
	}
	source, err := NewStringBloomFilter(cfg)
	if err != nil {
		b.Fatalf("NewStringBloomFilter failed: %v", err)
	}
	for i := 0; i < 10_000; i++ {
		source.Put("source-" + strconv.Itoa(i))
	}
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		target, err := NewStringBloomFilter(cfg)
		if err != nil {
			b.Fatalf("NewStringBloomFilter failed: %v", err)
		}
		if err := target.PutAll(source); err != nil {
			b.Fatalf("PutAll failed: %v", err)
		}
	}
}

func newStringFilterForBenchmark(b *testing.B) BloomFilter[string] {
	b.Helper()
	cfg, err := NewConfig(100_000, 0.01)
	if err != nil {
		b.Fatalf("NewConfig failed: %v", err)
	}
	filter, err := NewStringBloomFilter(cfg)
	if err != nil {
		b.Fatalf("NewStringBloomFilter failed: %v", err)
	}
	return filter
}
