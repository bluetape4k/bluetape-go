package compression

import (
	"io"

	"github.com/golang/snappy"
)

// Snappy는 Snappy 공개 API의 동작을 수행한다.
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
