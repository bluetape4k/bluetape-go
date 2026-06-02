package compression

import (
	"io"

	"github.com/pierrec/lz4/v4"
)

// LZ4 returns an LZ4 stream compressor.
func LZ4() Compressor {
	return streamCompressor{
		name: "lz4",
		writer: func(writer io.Writer) (io.WriteCloser, error) {
			return lz4.NewWriter(writer), nil
		},
		reader: func(reader io.Reader) (io.ReadCloser, error) {
			return nopReadCloser{Reader: lz4.NewReader(reader)}, nil
		},
	}
}
