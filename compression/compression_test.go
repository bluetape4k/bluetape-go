package compression_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/compression"
)

func testPayload() []byte {
	return []byte(strings.Repeat("bluetape-go compression payload ", 64))
}

func TestCompressorsRoundTripBytes(t *testing.T) {
	payload := testPayload()

	for _, compressor := range compression.All() {
		t.Run(compressor.Name(), func(t *testing.T) {
			compressed, err := compressor.Compress(payload)
			if err != nil {
				t.Fatalf("Compress failed: %v", err)
			}
			if bytes.Equal(compressed, payload) {
				t.Fatalf("expected compressed data to differ from input")
			}

			decompressed, err := compressor.Decompress(compressed)
			if err != nil {
				t.Fatalf("Decompress failed: %v", err)
			}
			if !bytes.Equal(decompressed, payload) {
				t.Fatalf("got %q, want %q", decompressed, payload)
			}
		})
	}
}

func TestCompressorsRoundTripStreams(t *testing.T) {
	payload := testPayload()

	for _, compressor := range compression.All() {
		t.Run(compressor.Name(), func(t *testing.T) {
			var compressed bytes.Buffer
			writer, err := compressor.NewWriter(&compressed)
			if err != nil {
				t.Fatalf("NewWriter failed: %v", err)
			}
			if _, err := writer.Write(payload); err != nil {
				t.Fatalf("Write failed: %v", err)
			}
			if err := writer.Close(); err != nil {
				t.Fatalf("Close failed: %v", err)
			}

			reader, err := compressor.NewReader(&compressed)
			if err != nil {
				t.Fatalf("NewReader failed: %v", err)
			}
			defer func() {
				if err := reader.Close(); err != nil {
					t.Fatalf("reader close failed: %v", err)
				}
			}()

			decompressed, err := io.ReadAll(reader)
			if err != nil {
				t.Fatalf("ReadAll failed: %v", err)
			}
			if !bytes.Equal(decompressed, payload) {
				t.Fatalf("got %q, want %q", decompressed, payload)
			}
		})
	}
}

func TestCompressorsRejectCorruptInput(t *testing.T) {
	corrupt := []byte("not compressed")

	for _, compressor := range compression.All() {
		t.Run(compressor.Name(), func(t *testing.T) {
			if _, err := compressor.Decompress(corrupt); err == nil {
				t.Fatal("expected corrupt input to fail")
			}
		})
	}
}

func TestCompressorRejectsNilStreams(t *testing.T) {
	compressor := compression.Gzip()

	if _, err := compressor.NewWriter(nil); err == nil {
		t.Fatal("expected nil writer to fail")
	}
	if _, err := compressor.NewReader(nil); err == nil {
		t.Fatal("expected nil reader to fail")
	}
}

func TestDefaultIsZstd(t *testing.T) {
	if compression.Default().Name() != "zstd" {
		t.Fatalf("expected zstd default, got %s", compression.Default().Name())
	}
}

func TestAllIncludesExpectedAlgorithms(t *testing.T) {
	got := make(map[string]bool)
	for _, compressor := range compression.All() {
		got[compressor.Name()] = true
	}

	for _, name := range []string{"gzip", "zlib", "deflate", "zstd", "lz4", "snappy"} {
		if !got[name] {
			t.Fatalf("missing compressor %s in All()", name)
		}
	}
}
