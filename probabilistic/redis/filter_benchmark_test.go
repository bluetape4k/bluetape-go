package redisbloom

import (
	"context"
	"strconv"
	"testing"
)

func newBenchmarkStringBloomFilter(b *testing.B, expected uint64) BloomFilter[string] {
	b.Helper()
	ctx := context.Background()
	client := newRedisClient(b)
	namespace := testNamespace(b)
	cleanupNamespace(b, client, namespace)
	filter, err := NewStringBloomFilter(ctx, client, namespace, testConfig(b, expected))
	if err != nil {
		b.Fatalf("NewStringBloomFilter failed: %v", err)
	}
	return filter
}

func BenchmarkRedisBloomPut(b *testing.B) {
	ctx := context.Background()
	filter := newBenchmarkStringBloomFilter(b, uint64(max(b.N, 1))*2)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := filter.Put(ctx, "value:"+strconv.Itoa(i)); err != nil {
			b.Fatalf("Put failed: %v", err)
		}
	}
}

func BenchmarkRedisBloomMightContain(b *testing.B) {
	ctx := context.Background()
	filter := newBenchmarkStringBloomFilter(b, uint64(max(b.N, 1))*2)
	for i := 0; i < 1024; i++ {
		if _, err := filter.Put(ctx, "value:"+strconv.Itoa(i)); err != nil {
			b.Fatalf("seed Put failed: %v", err)
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := filter.MightContain(ctx, "value:"+strconv.Itoa(i%1024)); err != nil {
			b.Fatalf("MightContain failed: %v", err)
		}
	}
}

func BenchmarkRedisBloomOffsets(b *testing.B) {
	filter := newBenchmarkStringBloomFilter(b, uint64(max(b.N, 1))*2)
	concrete, ok := filter.(*bloomFilter[string])
	if !ok {
		b.Fatalf("unexpected filter type %T", filter)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := concrete.offsets("value:" + strconv.Itoa(i)); err != nil {
			b.Fatalf("offsets failed: %v", err)
		}
	}
}
