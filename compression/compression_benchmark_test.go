package compression_test

import (
	"bytes"
	"math/rand/v2"
	"testing"

	"github.com/bluetape4k/bluetape-go/compression"
)

func benchmarkPayloads() map[string][]byte {
	return map[string][]byte{
		"repeated-text": bytes.Repeat([]byte("bluetape-go compression benchmark payload "), 1024),
		"random-bytes":  deterministicRandomBytes(32 * 1024),
	}
}

func deterministicRandomBytes(size int) []byte {
	random := rand.New(rand.NewPCG(0x0b71e7a9e, 0x04c0ffee))
	payload := make([]byte, size)
	for index := range payload {
		payload[index] = byte(random.Uint32())
	}
	return payload
}

func BenchmarkCompressorsCompress(b *testing.B) {
	for payloadName, payload := range benchmarkPayloads() {
		payloadName, payload := payloadName, payload
		b.Run(payloadName, func(b *testing.B) {
			for _, compressor := range compression.All() {
				compressor := compressor
				b.Run(compressor.Name(), func(b *testing.B) {
					b.ReportAllocs()
					b.SetBytes(int64(len(payload)))
					b.ResetTimer()

					var compressed []byte
					for range b.N {
						var err error
						compressed, err = compressor.Compress(payload)
						if err != nil {
							b.Fatalf("Compress failed: %v", err)
						}
					}

					b.StopTimer()
					reportCompressionMetrics(b, len(payload), len(compressed))
				})
			}
		})
	}
}

func BenchmarkCompressorsDecompress(b *testing.B) {
	for payloadName, payload := range benchmarkPayloads() {
		payloadName, payload := payloadName, payload
		b.Run(payloadName, func(b *testing.B) {
			for _, compressor := range compression.All() {
				compressor := compressor
				compressed, err := compressor.Compress(payload)
				if err != nil {
					b.Fatalf("setup Compress failed: %v", err)
				}

				b.Run(compressor.Name(), func(b *testing.B) {
					b.ReportAllocs()
					b.SetBytes(int64(len(payload)))
					reportCompressionMetrics(b, len(payload), len(compressed))
					b.ResetTimer()

					for range b.N {
						decompressed, err := compressor.Decompress(compressed)
						if err != nil {
							b.Fatalf("Decompress failed: %v", err)
						}
						if len(decompressed) != len(payload) {
							b.Fatalf("decompressed size %d, want %d", len(decompressed), len(payload))
						}
					}
				})
			}
		})
	}
}

func reportCompressionMetrics(b *testing.B, originalSize int, compressedSize int) {
	b.Helper()
	b.ReportMetric(float64(compressedSize), "compressed_bytes")
	if originalSize > 0 {
		ratio := float64(compressedSize) / float64(originalSize)
		b.ReportMetric(ratio, "compressed/original")
	}
}
