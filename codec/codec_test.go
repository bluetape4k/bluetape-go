package codec_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/bluetape4k/bluetape-go/codec"
	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

func TestBase58RoundTrip(t *testing.T) {
	input := []byte("Hello, World!")
	encoded := codec.EncodeBase58(input)
	if encoded != "72k1xXWG59fYdzSNoA" {
		t.Fatalf("unexpected Base58 encoding: %s", encoded)
	}

	decoded, err := codec.DecodeBase58(encoded)
	if err != nil {
		t.Fatalf("DecodeBase58 failed: %v", err)
	}
	if !bytes.Equal(decoded, input) {
		t.Fatalf("got %q, want %q", decoded, input)
	}
}

func TestBase58KotlinCompatibilityVectors(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		encoded string
	}{
		{
			name:    "Bitcoin alphabet sample without comma",
			input:   []byte("Hello World!"),
			encoded: "2NEpo7TZRRrLZSi2U",
		},
		{
			name:    "Kotlin algorithm with comma",
			input:   []byte("Hello, World!"),
			encoded: "72k1xXWG59fYdzSNoA",
		},
		{
			name:    "leading zero vector",
			input:   []byte{0, 0, 1, 2, 3},
			encoded: "11Ldp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := codec.EncodeBase58(tt.input)
			if encoded != tt.encoded {
				t.Fatalf("EncodeBase58() = %q, want %q", encoded, tt.encoded)
			}

			decoded, err := codec.DecodeBase58(tt.encoded)
			if err != nil {
				t.Fatalf("DecodeBase58(%q) failed: %v", tt.encoded, err)
			}
			if !bytes.Equal(decoded, tt.input) {
				t.Fatalf("DecodeBase58(%q) = %v, want %v", tt.encoded, decoded, tt.input)
			}
		})
	}
}

func TestBase58PreservesLeadingZeros(t *testing.T) {
	input := []byte{0, 0, 1, 2, 3}
	encoded := codec.EncodeBase58(input)
	if encoded[:2] != "11" {
		t.Fatalf("expected leading zeroes to encode as 1, got %q", encoded)
	}

	decoded, err := codec.DecodeBase58(encoded)
	if err != nil {
		t.Fatalf("DecodeBase58 failed: %v", err)
	}
	if !bytes.Equal(decoded, input) {
		t.Fatalf("got %v, want %v", decoded, input)
	}
}

func TestBase58RejectsInvalidInput(t *testing.T) {
	for _, value := range []string{"0", "O", "I", "l", "!", "안녕"} {
		if _, err := codec.DecodeBase58(value); err == nil {
			t.Fatalf("expected error for %q", value)
		}
	}
}

func TestDecodeEmptyInputReturnsEmptyBytes(t *testing.T) {
	for name, decode := range map[string]func(string) ([]byte, error){
		"Base58": codec.DecodeBase58,
		"Base62": codec.DecodeBase62,
		"URL62":  codec.DecodeURL62,
	} {
		t.Run(name, func(t *testing.T) {
			decoded, err := decode("")
			if err != nil {
				t.Fatalf("decode empty input failed: %v", err)
			}
			if len(decoded) != 0 {
				t.Fatalf("decode empty input = %v, want empty bytes", decoded)
			}
		})
	}
}

func TestDecodeBlankWhitespaceInputFails(t *testing.T) {
	for name, decode := range map[string]func(string) ([]byte, error){
		"Base58": codec.DecodeBase58,
		"Base62": codec.DecodeBase62,
		"URL62":  codec.DecodeURL62,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decode(" \t "); err == nil {
				t.Fatal("expected blank whitespace input to fail")
			}
		})
	}
}

func TestBase62RoundTrip(t *testing.T) {
	values := [][]byte{
		{},
		{0},
		{0, 0, 42},
		[]byte("bluetape-go"),
		{255, 254, 253, 252},
	}

	for _, input := range values {
		encoded := codec.EncodeBase62(input)
		decoded, err := codec.DecodeBase62(encoded)
		if err != nil {
			t.Fatalf("DecodeBase62(%q) failed: %v", encoded, err)
		}
		if !bytes.Equal(decoded, input) {
			t.Fatalf("Base62 round trip got %v, want %v", decoded, input)
		}
	}
}

func TestBase62KotlinNumericVectors(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		encoded string
	}{
		{
			name:    "zero",
			input:   []byte{0},
			encoded: "0",
		},
		{
			name:    "big integer sample",
			input:   []byte{0x07, 0x5b, 0xcd, 0x15},
			encoded: "8M0kX",
		},
		{
			name:    "Url62 UUID sample",
			input:   []byte{0x24, 0x73, 0x81, 0x34, 0x9d, 0x88, 0x66, 0x45, 0x4e, 0xc8, 0xd6, 0x3a, 0xa2, 0x03, 0x10, 0x15},
			encoded: "16mVan3wbAXR6tQwIbfS5d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := codec.EncodeBase62(tt.input)
			if encoded != tt.encoded {
				t.Fatalf("EncodeBase62() = %q, want %q", encoded, tt.encoded)
			}

			decoded, err := codec.DecodeBase62(tt.encoded)
			if err != nil {
				t.Fatalf("DecodeBase62(%q) failed: %v", tt.encoded, err)
			}
			if !bytes.Equal(decoded, tt.input) {
				t.Fatalf("DecodeBase62(%q) = %v, want %v", tt.encoded, decoded, tt.input)
			}
		})
	}
}

func TestBase62PreservesByteAPIDivergenceFromKotlinBigInteger(t *testing.T) {
	input := []byte{0, 0, 0x07, 0x5b, 0xcd, 0x15}
	encoded := codec.EncodeBase62(input)
	if encoded != "008M0kX" {
		t.Fatalf("expected byte API to preserve leading zeros, got %q", encoded)
	}

	decoded, err := codec.DecodeBase62(encoded)
	if err != nil {
		t.Fatalf("DecodeBase62 failed: %v", err)
	}
	if !bytes.Equal(decoded, input) {
		t.Fatalf("got %v, want %v", decoded, input)
	}
}

func TestBase62DecodesOverKotlinDefaultBitLimit(t *testing.T) {
	input := []byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef, 0x10, 0x32, 0x54, 0x76, 0x98, 0xba, 0xdc, 0xfe, 0x11}
	encoded := codec.EncodeBase62(input)

	decoded, err := codec.DecodeBase62(encoded)
	if err != nil {
		t.Fatalf("DecodeBase62 failed: %v", err)
	}
	if !bytes.Equal(decoded, input) {
		t.Fatalf("got %v, want %v", decoded, input)
	}
}

func TestBase62PreservesHighOrderZeroUUIDBytes(t *testing.T) {
	input := []byte{0, 0, 0x81, 0x34, 0x9d, 0x88, 0x66, 0x45, 0x4e, 0xc8, 0xd6, 0x3a, 0xa2, 0x03, 0x10, 0x15}
	encoded := codec.EncodeBase62(input)
	if encoded[:2] != "00" {
		t.Fatalf("expected high-order zero UUID bytes to stay visible, got %q", encoded)
	}

	decoded, err := codec.DecodeBase62(encoded)
	if err != nil {
		t.Fatalf("DecodeBase62 failed: %v", err)
	}
	if !bytes.Equal(decoded, input) {
		t.Fatalf("got %v, want %v", decoded, input)
	}
}

func TestURL62IsBase62Alias(t *testing.T) {
	input := []byte("safe/id?value=1")
	encoded := codec.EncodeURL62(input)
	if encoded != codec.EncodeBase62(input) {
		t.Fatalf("URL62 should use Base62 alphabet")
	}
	for _, char := range encoded {
		if !isBase62Char(char) {
			t.Fatalf("URL62 produced non URL-safe character %q in %q", char, encoded)
		}
	}

	decoded, err := codec.DecodeURL62(encoded)
	if err != nil {
		t.Fatalf("DecodeURL62 failed: %v", err)
	}
	if !bytes.Equal(decoded, input) {
		t.Fatalf("got %q, want %q", decoded, input)
	}
}

func isBase62Char(char rune) bool {
	return ('0' <= char && char <= '9') || ('A' <= char && char <= 'Z') || ('a' <= char && char <= 'z')
}

func TestBase62RejectsInvalidInput(t *testing.T) {
	for _, value := range []string{"abc-123", "abc_123", "안녕"} {
		if _, err := codec.DecodeBase62(value); err == nil {
			t.Fatalf("expected invalid Base62 input %q to fail", value)
		}
	}
}

func TestBase58Base62ConcurrentRoundTripStress(t *testing.T) {
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       8,
		RoundsPerTask: 100,
	})

	inputs := [][]byte{
		{},
		{0},
		{0, 0, 1, 2, 3},
		[]byte("Hello, World!"),
		[]byte("안녕, bluetape-go"),
		{0xff, 0xfe, 0xfd, 0xfc, 0xfb, 0xfa},
	}

	tester.RunT(t, func(context.Context) error {
		for _, input := range inputs {
			encoded := codec.EncodeBase58(input)
			decoded, err := codec.DecodeBase58(encoded)
			if err != nil {
				return fmt.Errorf("DecodeBase58(%q): %w", encoded, err)
			}
			if !bytes.Equal(decoded, input) {
				return fmt.Errorf("Base58 round trip = %v, want %v", decoded, input)
			}
		}
		return nil
	}, func(context.Context) error {
		for _, input := range inputs {
			encoded := codec.EncodeBase62(input)
			decoded, err := codec.DecodeBase62(encoded)
			if err != nil {
				return fmt.Errorf("DecodeBase62(%q): %w", encoded, err)
			}
			if !bytes.Equal(decoded, input) {
				return fmt.Errorf("Base62 round trip = %v, want %v", decoded, input)
			}
		}
		return nil
	}, func(context.Context) error {
		for _, input := range inputs {
			encoded := codec.EncodeURL62(input)
			decoded, err := codec.DecodeURL62(encoded)
			if err != nil {
				return fmt.Errorf("DecodeURL62(%q): %w", encoded, err)
			}
			if !bytes.Equal(decoded, input) {
				return fmt.Errorf("URL62 round trip = %v, want %v", decoded, input)
			}
		}
		return nil
	})
}

func TestBase64RoundTrip(t *testing.T) {
	input := []byte("Hello, World!")
	encoded := codec.EncodeBase64(input)
	if encoded != "SGVsbG8sIFdvcmxkIQ==" {
		t.Fatalf("unexpected Base64 encoding: %s", encoded)
	}

	decoded, err := codec.DecodeBase64(encoded)
	if err != nil {
		t.Fatalf("DecodeBase64 failed: %v", err)
	}
	if !bytes.Equal(decoded, input) {
		t.Fatalf("got %q, want %q", decoded, input)
	}
}

func TestBase64URLUsesRawURLEncoding(t *testing.T) {
	input := []byte{251, 255, 255}
	encoded := codec.EncodeBase64URL(input)
	if encoded != "-___" {
		t.Fatalf("unexpected URL-safe Base64 encoding: %s", encoded)
	}
	if bytes.ContainsAny([]byte(encoded), "+/=") {
		t.Fatalf("Base64URL should not contain +, /, or padding: %q", encoded)
	}

	decoded, err := codec.DecodeBase64URL(encoded)
	if err != nil {
		t.Fatalf("DecodeBase64URL failed: %v", err)
	}
	if !bytes.Equal(decoded, input) {
		t.Fatalf("got %v, want %v", decoded, input)
	}
}

func TestBase64RejectsInvalidInput(t *testing.T) {
	if _, err := codec.DecodeBase64("not valid base64"); err == nil {
		t.Fatal("expected invalid Base64 input to fail")
	}
}

func TestHexRoundTrip(t *testing.T) {
	input := []byte("Hello, World!")
	encoded := codec.EncodeHex(input)
	if encoded != "48656c6c6f2c20576f726c6421" {
		t.Fatalf("unexpected Hex encoding: %s", encoded)
	}

	decoded, err := codec.DecodeHex(encoded)
	if err != nil {
		t.Fatalf("DecodeHex failed: %v", err)
	}
	if !bytes.Equal(decoded, input) {
		t.Fatalf("got %q, want %q", decoded, input)
	}
}

func TestHexRejectsInvalidInput(t *testing.T) {
	if _, err := codec.DecodeHex("abc"); err == nil {
		t.Fatal("expected odd-length Hex input to fail")
	}
}

func TestStringHelpers(t *testing.T) {
	input := "안녕, bluetape-go"

	base58, err := codec.DecodeBase58String(codec.EncodeBase58String(input))
	if err != nil {
		t.Fatalf("DecodeBase58String failed: %v", err)
	}
	base62, err := codec.DecodeBase62String(codec.EncodeBase62String(input))
	if err != nil {
		t.Fatalf("DecodeBase62String failed: %v", err)
	}
	base64, err := codec.DecodeBase64String(codec.EncodeBase64String(input))
	if err != nil {
		t.Fatalf("DecodeBase64String failed: %v", err)
	}
	hexed, err := codec.DecodeHexString(codec.EncodeHexString(input))
	if err != nil {
		t.Fatalf("DecodeHexString failed: %v", err)
	}

	for name, got := range map[string]string{
		"base58": base58,
		"base62": base62,
		"base64": base64,
		"hex":    hexed,
	} {
		if got != input {
			t.Fatalf("%s got %q, want %q", name, got, input)
		}
	}
}
