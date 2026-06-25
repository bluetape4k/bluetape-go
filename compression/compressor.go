package compression

import (
	"bytes"
	"errors"
	"fmt"
	"io"
)

// ErrDecompressedSizeExceeded reports that decompressed output crossed the caller-provided limit.
var ErrDecompressedSizeExceeded = errors.New("decompressed size exceeded")

// Compressor compresses and decompresses byte slices and streams.
type Compressor interface {
	Name() string
	Compress([]byte) ([]byte, error)
	Decompress([]byte) ([]byte, error)
	NewWriter(io.Writer) (io.WriteCloser, error)
	NewReader(io.Reader) (io.ReadCloser, error)
}

type streamCompressor struct {
	name   string
	writer func(io.Writer) (io.WriteCloser, error)
	reader func(io.Reader) (io.ReadCloser, error)
}

func (c streamCompressor) Name() string {
	return c.name
}

func (c streamCompressor) Compress(data []byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer, err := c.NewWriter(&buffer)
	if err != nil {
		return nil, err
	}
	if _, err := writer.Write(data); err != nil {
		_ = writer.Close()
		return nil, fmt.Errorf("%s compress: %w", c.name, err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("%s compress close: %w", c.name, err)
	}
	return buffer.Bytes(), nil
}

func (c streamCompressor) Decompress(data []byte) ([]byte, error) {
	reader, err := c.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = reader.Close()
	}()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("%s decompress: %w", c.name, err)
	}
	return decompressed, nil
}

// DecompressLimit decompresses data while retaining at most maxBytes bytes.
//
// Use this helper for untrusted or externally supplied compressed payloads. It
// returns ErrDecompressedSizeExceeded when the expanded stream exceeds maxBytes.
func DecompressLimit(compressor Compressor, data []byte, maxBytes int64) ([]byte, error) {
	if compressor == nil {
		return nil, fmt.Errorf("compressor must not be nil")
	}
	if maxBytes < 0 {
		return nil, fmt.Errorf("maxBytes[%d] must be non-negative", maxBytes)
	}
	reader, err := compressor.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = reader.Close()
	}()

	limited := io.LimitReader(reader, maxBytes+1)
	decompressed, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("%s decompress: %w", compressor.Name(), err)
	}
	if int64(len(decompressed)) > maxBytes {
		return nil, fmt.Errorf("%w: max %d bytes", ErrDecompressedSizeExceeded, maxBytes)
	}
	return decompressed, nil
}

func (c streamCompressor) NewWriter(writer io.Writer) (io.WriteCloser, error) {
	if writer == nil {
		return nil, fmt.Errorf("%s writer must not be nil", c.name)
	}
	return c.writer(writer)
}

func (c streamCompressor) NewReader(reader io.Reader) (io.ReadCloser, error) {
	if reader == nil {
		return nil, fmt.Errorf("%s reader must not be nil", c.name)
	}
	return c.reader(reader)
}

type nopReadCloser struct {
	io.Reader
}

func (nopReadCloser) Close() error {
	return nil
}
