package compression

import (
	"bytes"
	"fmt"
	"io"
)

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
