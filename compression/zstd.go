package compression

import (
	"bytes"
	"fmt"
	"io"
	"sync"

	"github.com/klauspost/compress/zstd"
)

// Zstd returns a zstd compressor.
func Zstd() Compressor {
	return ZstdLevel(zstd.SpeedDefault)
}

// ZstdLevel returns a zstd compressor with the requested encoder level.
func ZstdLevel(level zstd.EncoderLevel) Compressor {
	compressor := &zstdCompressor{
		level: level,
	}
	compressor.encoders.New = func() any {
		encoder, err := zstd.NewWriter(io.Discard, zstd.WithEncoderLevel(level))
		if err != nil {
			return err
		}
		return encoder
	}
	return compressor
}

type zstdCompressor struct {
	level    zstd.EncoderLevel
	encoders sync.Pool
}

func (c *zstdCompressor) Name() string {
	return "zstd"
}

func (c *zstdCompressor) Compress(data []byte) ([]byte, error) {
	encoder, err := c.encoder()
	if err != nil {
		return nil, err
	}
	defer c.putEncoder(encoder)

	var buffer bytes.Buffer
	encoder.Reset(&buffer)
	if _, err := encoder.Write(data); err != nil {
		return nil, fmt.Errorf("%s compress: %w", c.Name(), err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("%s compress close: %w", c.Name(), err)
	}
	return buffer.Bytes(), nil
}

func (c *zstdCompressor) Decompress(data []byte) ([]byte, error) {
	return streamCompressor{
		name:   c.Name(),
		writer: c.NewWriter,
		reader: c.NewReader,
	}.Decompress(data)
}

func (c *zstdCompressor) NewWriter(writer io.Writer) (io.WriteCloser, error) {
	if writer == nil {
		return nil, fmt.Errorf("%s writer must not be nil", c.Name())
	}
	return zstd.NewWriter(writer, zstd.WithEncoderLevel(c.level))
}

func (c *zstdCompressor) NewReader(reader io.Reader) (io.ReadCloser, error) {
	if reader == nil {
		return nil, fmt.Errorf("%s reader must not be nil", c.Name())
	}
	decoder, err := zstd.NewReader(reader)
	if err != nil {
		return nil, err
	}
	return decoder.IOReadCloser(), nil
}

func (c *zstdCompressor) encoder() (*zstd.Encoder, error) {
	switch value := c.encoders.Get().(type) {
	case *zstd.Encoder:
		return value, nil
	case error:
		return nil, value
	default:
		return nil, fmt.Errorf("%s encoder pool returned %T", c.Name(), value)
	}
}

func (c *zstdCompressor) putEncoder(encoder *zstd.Encoder) {
	encoder.Reset(io.Discard)
	c.encoders.Put(encoder)
}
