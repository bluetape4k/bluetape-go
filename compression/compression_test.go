package compression_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/compression"
	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
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

func TestZstdCompressMatchesStreamWriter(t *testing.T) {
	compressor := compression.Zstd()
	payloads := [][]byte{
		{},
		[]byte("bluetape-go zstd payload"),
		[]byte(strings.Repeat(`{"service":"bluetape-go","kind":"compression","region":"region-01"}`, 4096)),
	}

	for _, payload := range payloads {
		compressed, err := compressor.Compress(payload)
		if err != nil {
			t.Fatalf("Compress failed: %v", err)
		}

		var streamed bytes.Buffer
		writer, err := compressor.NewWriter(&streamed)
		if err != nil {
			t.Fatalf("NewWriter failed: %v", err)
		}
		if _, err := writer.Write(payload); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
		if err := writer.Close(); err != nil {
			t.Fatalf("Close failed: %v", err)
		}

		if !bytes.Equal(compressed, streamed.Bytes()) {
			t.Fatalf("Compress output differs from stream writer for payload length %d", len(payload))
		}
	}
}

func TestZstdCompressConcurrentStress(t *testing.T) {
	compressor := compression.Zstd()
	payload := []byte(strings.Repeat(`{"service":"bluetape-go","kind":"compression","region":"region-01"}`, 512))
	expected, err := compressor.Compress(payload)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       8,
		RoundsPerTask: 32,
		Timeout:       5 * time.Second,
	})
	tester.RunT(t, func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		compressed, err := compressor.Compress(payload)
		if err != nil {
			return err
		}
		if !bytes.Equal(compressed, expected) {
			return errors.New("concurrent zstd compress output differs from baseline")
		}

		decompressed, err := compressor.Decompress(compressed)
		if err != nil {
			return err
		}
		if !bytes.Equal(decompressed, payload) {
			return errors.New("concurrent zstd round-trip mismatch")
		}
		return nil
	})
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

func TestDecompressLimitRejectsOversizedOutput(t *testing.T) {
	payload := testPayload()

	for _, compressor := range compression.All() {
		t.Run(compressor.Name(), func(t *testing.T) {
			compressed, err := compressor.Compress(payload)
			if err != nil {
				t.Fatalf("Compress failed: %v", err)
			}

			decompressed, err := compression.DecompressLimit(compressor, compressed, int64(len(payload)))
			if err != nil {
				t.Fatalf("DecompressLimit exact limit failed: %v", err)
			}
			if !bytes.Equal(decompressed, payload) {
				t.Fatalf("got %q, want %q", decompressed, payload)
			}

			_, err = compression.DecompressLimit(compressor, compressed, int64(len(payload)-1))
			if !errors.Is(err, compression.ErrDecompressedSizeExceeded) {
				t.Fatalf("DecompressLimit oversized error = %v, want ErrDecompressedSizeExceeded", err)
			}
		})
	}
}

func TestDecompressLimitRejectsInvalidArguments(t *testing.T) {
	if _, err := compression.DecompressLimit(nil, nil, 0); err == nil {
		t.Fatal("nil compressor should fail")
	}
	if _, err := compression.DecompressLimit(compression.Gzip(), nil, -1); err == nil {
		t.Fatal("negative maxBytes should fail")
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
