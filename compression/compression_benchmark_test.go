package compression_test

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/compression"
)

type benchmarkPayload struct {
	name string
	data []byte
}

func benchmarkPayloads() []benchmarkPayload {
	return []benchmarkPayload{
		{name: "json/small", data: deterministicJSONPayload(1 * 1024)},
		{name: "json/medium", data: deterministicJSONPayload(48 * 1024)},
		{name: "json/large", data: deterministicJSONPayload(768 * 1024)},
		{name: "text/small", data: deterministicTextPayload(1 * 1024)},
		{name: "text/medium", data: deterministicTextPayload(48 * 1024)},
		{name: "text/large", data: deterministicTextPayload(768 * 1024)},
		{name: "binary/small", data: deterministicBinaryPayload(1 * 1024)},
		{name: "binary/medium", data: deterministicBinaryPayload(48 * 1024)},
		{name: "binary/large", data: deterministicBinaryPayload(768 * 1024)},
		{name: "random/small", data: deterministicRandomBytes(1 * 1024)},
		{name: "random/medium", data: deterministicRandomBytes(48 * 1024)},
		{name: "random/large", data: deterministicRandomBytes(768 * 1024)},
	}
}

func deterministicJSONPayload(size int) []byte {
	var builder strings.Builder
	builder.Grow(size + 1024)
	builder.WriteString(`{"service":"bluetape-go","kind":"compression","events":[`)

	for index := 0; builder.Len() < size+256; index++ {
		if index > 0 {
			builder.WriteByte(',')
		}
		fmt.Fprintf(
			&builder,
			`{"id":%d,"region":"region-%02d","level":"info","message":"%s","latency_ms":%d}`,
			index,
			index%17,
			strings.Repeat("payload-", 1+(index%5)),
			7+(index%97),
		)
	}

	builder.WriteString(`]}`)
	return []byte(builder.String())
}

func deterministicTextPayload(size int) []byte {
	lines := []string{
		"2026-06-12T09:00:00Z INFO compression request completed service=bluetape-go route=/payloads status=200",
		"2026-06-12T09:00:01Z INFO cache hit namespace=benchmark key=payload-matrix latency_ms=3",
		"2026-06-12T09:00:02Z WARN retry scheduled component=compressor attempt=1 reason=transient-backpressure",
		"Compression benchmarks need deterministic UTF-8 text so local snapshots remain comparable across runs.",
	}
	var builder strings.Builder
	builder.Grow(size + len(lines[0]))
	for index := 0; builder.Len() < size; index++ {
		builder.WriteString(lines[index%len(lines)])
		builder.WriteString(" sequence=")
		builder.WriteString(strconv.Itoa(index))
		builder.WriteByte('\n')
	}
	return []byte(builder.String()[:size])
}

func deterministicBinaryPayload(size int) []byte {
	payload := make([]byte, size)
	for index := range payload {
		// 낮은 엔트로피와 반복 구간을 섞어 구조화 binary payload를 흉내낸다.
		switch {
		case index%64 < 8:
			payload[index] = 0
		case index%64 < 16:
			payload[index] = 0xff
		default:
			payload[index] = byte((index*31 + index/7) % 251)
		}
	}
	return payload
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
	for _, payload := range benchmarkPayloads() {
		payload := payload
		b.Run(payload.name, func(b *testing.B) {
			for _, compressor := range compression.All() {
				compressor := compressor
				b.Run(compressor.Name(), func(b *testing.B) {
					b.ReportAllocs()
					b.SetBytes(int64(len(payload.data)))
					b.ResetTimer()

					var compressed []byte
					for range b.N {
						var err error
						compressed, err = compressor.Compress(payload.data)
						if err != nil {
							b.Fatalf("Compress failed: %v", err)
						}
					}

					b.StopTimer()
					reportCompressionMetrics(b, len(payload.data), len(compressed))
				})
			}
		})
	}
}

func BenchmarkCompressorsDecompress(b *testing.B) {
	for _, payload := range benchmarkPayloads() {
		payload := payload
		b.Run(payload.name, func(b *testing.B) {
			for _, compressor := range compression.All() {
				compressor := compressor
				compressed, err := compressor.Compress(payload.data)
				if err != nil {
					b.Fatalf("setup Compress failed: %v", err)
				}
				decompressed, err := compressor.Decompress(compressed)
				if err != nil {
					b.Fatalf("setup Decompress failed: %v", err)
				}
				if !bytes.Equal(decompressed, payload.data) {
					b.Fatalf("setup roundtrip mismatch for %s/%s", payload.name, compressor.Name())
				}

				b.Run(compressor.Name(), func(b *testing.B) {
					b.ReportAllocs()
					b.SetBytes(int64(len(payload.data)))
					b.ResetTimer()

					for range b.N {
						decompressed, err := compressor.Decompress(compressed)
						if err != nil {
							b.Fatalf("Decompress failed: %v", err)
						}
						if len(decompressed) != len(payload.data) {
							b.Fatalf("decompressed size %d, want %d", len(decompressed), len(payload.data))
						}
					}

					b.StopTimer()
					reportCompressionMetrics(b, len(payload.data), len(compressed))
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
