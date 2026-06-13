package redisbloom

import (
	"context"
	"strconv"
	"testing"

	"github.com/bluetape4k/bluetape-go/probabilistic"
)

func benchmarkRedisBloomConfigs(b *testing.B, expected uint64) []struct {
	name   string
	config probabilistic.Config
} {
	b.Helper()
	return []struct {
		name   string
		config probabilistic.Config
	}{
		{name: "fpp_0.100", config: benchmarkConfig(b, expected, 0.100)},
		{name: "fpp_0.010", config: benchmarkConfig(b, expected, 0.010)},
		{name: "fpp_0.001", config: benchmarkConfig(b, expected, 0.001)},
	}
}

func benchmarkConfig(b *testing.B, expected uint64, fpp float64) probabilistic.Config {
	b.Helper()
	cfg, err := probabilistic.NewConfig(expected, fpp)
	if err != nil {
		b.Fatalf("NewConfig failed: %v", err)
	}
	return cfg
}

func newBenchmarkStringBloomFilter(b *testing.B, cfg probabilistic.Config) BloomFilter[string] {
	b.Helper()
	ctx := context.Background()
	client := newRedisClient(b)
	namespace := testNamespace(b)
	cleanupNamespace(b, client, namespace)
	filter, err := NewStringBloomFilter(ctx, client, namespace, cfg)
	if err != nil {
		b.Fatalf("NewStringBloomFilter failed: %v", err)
	}
	return filter
}

func BenchmarkRedisBloomPut(b *testing.B) {
	for _, bc := range benchmarkRedisBloomConfigs(b, uint64(max(b.N, 1))*2) {
		b.Run(bc.name, func(b *testing.B) {
			ctx := context.Background()
			filter := newBenchmarkStringBloomFilter(b, bc.config)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := filter.Put(ctx, "value:"+strconv.Itoa(i)); err != nil {
					b.Fatalf("Put failed: %v", err)
				}
			}
		})
	}
}

func BenchmarkRedisBloomMightContain(b *testing.B) {
	for _, bc := range benchmarkRedisBloomConfigs(b, uint64(max(b.N, 1))*2) {
		b.Run(bc.name, func(b *testing.B) {
			ctx := context.Background()
			filter := newBenchmarkStringBloomFilter(b, bc.config)
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
		})
	}
}

func BenchmarkRedisBloomOffsets(b *testing.B) {
	for _, bc := range benchmarkRedisBloomConfigs(b, uint64(max(b.N, 1))*2) {
		b.Run(bc.name, func(b *testing.B) {
			filter := newBenchmarkStringBloomFilter(b, bc.config)
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
		})
	}
}
