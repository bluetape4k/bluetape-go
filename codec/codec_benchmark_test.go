package codec_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/codec"
)

type codecBenchmarkPayload struct {
	name string
	data []byte
}

type codecBenchmarkCase struct {
	name    string
	payload codecBenchmarkPayload
	encode  func([]byte) string
	decode  func(string) ([]byte, error)
}

func BenchmarkCodecEncode(b *testing.B) {
	for _, tc := range codecBenchmarkCases() {
		tc := tc
		b.Run(tc.name+"/"+tc.payload.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(tc.payload.data)))
			b.ResetTimer()

			var encoded string
			for range b.N {
				encoded = tc.encode(tc.payload.data)
			}

			b.StopTimer()
			if encoded == "" && len(tc.payload.data) > 0 {
				b.Fatal("encode returned empty text")
			}
			b.ReportMetric(float64(len(encoded)), "encoded_bytes")
		})
	}
}

func BenchmarkCodecDecode(b *testing.B) {
	for _, tc := range codecBenchmarkCases() {
		tc := tc
		encoded := tc.encode(tc.payload.data)
		decoded, err := tc.decode(encoded)
		if err != nil {
			b.Fatalf("setup decode failed for %s/%s: %v", tc.name, tc.payload.name, err)
		}
		if !bytes.Equal(decoded, tc.payload.data) {
			b.Fatalf("setup decode mismatch for %s/%s", tc.name, tc.payload.name)
		}

		b.Run(tc.name+"/"+tc.payload.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(tc.payload.data)))
			b.ResetTimer()

			var decoded []byte
			for range b.N {
				decoded, err = tc.decode(encoded)
				if err != nil {
					b.Fatalf("decode failed: %v", err)
				}
			}

			b.StopTimer()
			if !bytes.Equal(decoded, tc.payload.data) {
				b.Fatal("decode returned mismatched bytes")
			}
			b.ReportMetric(float64(len(encoded)), "encoded_bytes")
		})
	}
}

func BenchmarkCodecRoundTrip(b *testing.B) {
	for _, tc := range codecBenchmarkCases() {
		tc := tc
		b.Run(tc.name+"/"+tc.payload.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(tc.payload.data)))
			b.ResetTimer()

			var encoded string
			var decoded []byte
			var err error
			for range b.N {
				encoded = tc.encode(tc.payload.data)
				decoded, err = tc.decode(encoded)
				if err != nil {
					b.Fatalf("decode failed: %v", err)
				}
			}

			b.StopTimer()
			if !bytes.Equal(decoded, tc.payload.data) {
				b.Fatal("round trip returned mismatched bytes")
			}
			b.ReportMetric(float64(len(encoded)), "encoded_bytes")
		})
	}
}

func BenchmarkCodecUUIDURL62(b *testing.B) {
	const uuidText = "24738134-9d88-6645-4ec8-d63aa2031015"
	encoded, err := codec.EncodeUUIDURL62(uuidText)
	if err != nil {
		b.Fatalf("setup EncodeUUIDURL62 failed: %v", err)
	}
	decoded, err := codec.DecodeUUIDURL62(encoded)
	if err != nil {
		b.Fatalf("setup DecodeUUIDURL62 failed: %v", err)
	}
	if decoded != uuidText {
		b.Fatalf("setup UUID round trip = %q, want %q", decoded, uuidText)
	}

	b.Run("Encode/serde-uuid-url62-v1", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(16)
		b.ResetTimer()

		var encoded string
		for range b.N {
			encoded, err = codec.EncodeUUIDURL62(uuidText)
			if err != nil {
				b.Fatalf("EncodeUUIDURL62 failed: %v", err)
			}
		}

		b.StopTimer()
		b.ReportMetric(float64(len(encoded)), "encoded_bytes")
	})

	b.Run("Decode/serde-uuid-url62-v1", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(16)
		b.ResetTimer()

		var decoded string
		for range b.N {
			decoded, err = codec.DecodeUUIDURL62(encoded)
			if err != nil {
				b.Fatalf("DecodeUUIDURL62 failed: %v", err)
			}
		}

		b.StopTimer()
		if decoded != uuidText {
			b.Fatalf("DecodeUUIDURL62 = %q, want %q", decoded, uuidText)
		}
		b.ReportMetric(float64(len(encoded)), "encoded_bytes")
	})
}

func codecBenchmarkCases() []codecBenchmarkCase {
	var cases []codecBenchmarkCase
	byteCodecs := []struct {
		name   string
		encode func([]byte) string
		decode func(string) ([]byte, error)
	}{
		{name: "Base64", encode: codec.EncodeBase64, decode: codec.DecodeBase64},
		{name: "Base64URL", encode: codec.EncodeBase64URL, decode: codec.DecodeBase64URL},
		{name: "Hex", encode: codec.EncodeHex, decode: codec.DecodeHex},
	}
	for _, payload := range codecBenchmarkPayloads() {
		for _, c := range byteCodecs {
			cases = append(cases, codecBenchmarkCase{
				name:    c.name,
				payload: payload,
				encode:  c.encode,
				decode:  c.decode,
			})
		}
	}

	alphabetPayloads := []codecBenchmarkPayload{
		codecSmallObjectPayload(),
		codecUUIDPayload(),
	}
	alphabetCodecs := []struct {
		name   string
		encode func([]byte) string
		decode func(string) ([]byte, error)
	}{
		{name: "Base58", encode: codec.EncodeBase58, decode: codec.DecodeBase58},
		{name: "Base62", encode: codec.EncodeBase62, decode: codec.DecodeBase62},
		{name: "URL62", encode: codec.EncodeURL62, decode: codec.DecodeURL62},
	}
	for _, payload := range alphabetPayloads {
		for _, c := range alphabetCodecs {
			cases = append(cases, codecBenchmarkCase{
				name:    c.name,
				payload: payload,
				encode:  c.encode,
				decode:  c.decode,
			})
		}
	}
	return cases
}

func codecBenchmarkPayloads() []codecBenchmarkPayload {
	return []codecBenchmarkPayload{
		codecSmallObjectPayload(),
		{name: "serde-medium-nested-v1", data: codecJSONPayload(48 * 1024)},
		{name: "serde-binary-payload-v1", data: codecBinaryPayload(96 * 1024)},
		{name: "serde-repeated-collection-v1", data: []byte(codecTextPayload(160 * 1024))},
	}
}

func codecSmallObjectPayload() codecBenchmarkPayload {
	return codecBenchmarkPayload{
		name: "serde-small-object-v1",
		data: []byte(fmt.Sprintf(`{"id":"acct-20260707-0001","status":"active","active":true,"balance_minor":120045,"currency":"KRW","created_at":"2026-07-07T00:00:00Z","attributes":{"tenant":"blue","region":"ap-northeast-2","segment":"benchmark","risk":"low","notes":"%s"}}`, strings.Repeat("small-object-low-cardinality-note-", 18))),
	}
}

func codecUUIDPayload() codecBenchmarkPayload {
	return codecBenchmarkPayload{
		name: "serde-uuid-url62-v1",
		data: []byte{0x24, 0x73, 0x81, 0x34, 0x9d, 0x88, 0x66, 0x45, 0x4e, 0xc8, 0xd6, 0x3a, 0xa2, 0x03, 0x10, 0x15},
	}
}

func codecJSONPayload(size int) []byte {
	var builder strings.Builder
	builder.Grow(size + 512)
	builder.WriteString(`{"service":"bluetape-go","fixture":"serde-medium-nested-v1","items":[`)
	for index := 0; builder.Len() < size+128; index++ {
		if index > 0 {
			builder.WriteByte(',')
		}
		fmt.Fprintf(
			&builder,
			`{"sequence":%d,"tenant_id":"tenant-%02d","kind":"codec.measurement","message":"%s"}`,
			index,
			index%17,
			strings.Repeat("deterministic codec benchmark text ", 3+index%5),
		)
	}
	builder.WriteString(`]}`)
	return []byte(builder.String())
}

func codecBinaryPayload(size int) []byte {
	payload := make([]byte, size)
	copy(payload, []byte("BTGB"))
	payload[4] = 1
	for index := 5; index < len(payload)-4; index++ {
		switch {
		case index%128 < 16:
			payload[index] = 0
		case index%128 < 32:
			payload[index] = 0xff
		default:
			payload[index] = byte((index*41 + index/13) % 251)
		}
	}
	binary.BigEndian.PutUint32(payload[len(payload)-4:], crc32.ChecksumIEEE(payload[:len(payload)-4]))
	return payload
}

func codecTextPayload(size int) string {
	lines := []string{
		"2026-07-07T00:00:00Z INFO codec fixture emitted service=bluetape-go status=ok",
		"2026-07-07T00:00:01Z INFO encoded payload accepted route=/benchmark/codec tenant=blue",
		"Codec benchmark text remains deterministic across locale and timezone defaults.",
	}
	var builder strings.Builder
	builder.Grow(size + len(lines[0]))
	for index := 0; builder.Len() < size; index++ {
		builder.WriteString(lines[index%len(lines)])
		builder.WriteString(" sequence=")
		fmt.Fprintf(&builder, "%06d", index)
		builder.WriteByte('\n')
	}
	return builder.String()[:size]
}
