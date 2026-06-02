package compression

import (
	"io"

	"github.com/klauspost/compress/zstd"
)

// Zstd returns a zstd compressor.
func Zstd() Compressor {
	return ZstdLevel(zstd.SpeedDefault)
}

// ZstdLevel returns a zstd compressor with the requested encoder level.
func ZstdLevel(level zstd.EncoderLevel) Compressor {
	return streamCompressor{
		name: "zstd",
		writer: func(writer io.Writer) (io.WriteCloser, error) {
			return zstd.NewWriter(writer, zstd.WithEncoderLevel(level))
		},
		reader: func(reader io.Reader) (io.ReadCloser, error) {
			decoder, err := zstd.NewReader(reader)
			if err != nil {
				return nil, err
			}
			return decoder.IOReadCloser(), nil
		},
	}
}
