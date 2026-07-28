package compression

import (
	"io"

	"github.com/pierrec/lz4/v4"
)

// LZ4 LZ4 공개 API의 동작을 수행한다.
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
