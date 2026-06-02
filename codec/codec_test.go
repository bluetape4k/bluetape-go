package codec_test

import (
	"bytes"
	"testing"

	"github.com/bluetape4k/bluetape-go/codec"
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
	for _, value := range []string{"0", "O", "I", "l", "!"} {
		if _, err := codec.DecodeBase58(value); err == nil {
			t.Fatalf("expected error for %q", value)
		}
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
	if _, err := codec.DecodeBase62("abc-123"); err == nil {
		t.Fatal("expected invalid Base62 input to fail")
	}
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
