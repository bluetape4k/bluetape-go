package compression

import (
	"io"

	"github.com/golang/snappy"
)

// Snappy returns a framed Snappy compressor.
func Snappy() Compressor {
	return streamCompressor{
		name: "snappy",
		writer: func(writer io.Writer) (io.WriteCloser, error) {
			return snappy.NewBufferedWriter(writer), nil
		},
		reader: func(reader io.Reader) (io.ReadCloser, error) {
			return nopReadCloser{Reader: snappy.NewReader(reader)}, nil
		},
	}
}
