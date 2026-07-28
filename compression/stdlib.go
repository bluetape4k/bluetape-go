package compression

import (
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"io"
)

// Gzip Gzip 공개 API의 동작을 수행한다.
func Gzip() Compressor {
	return GzipLevel(gzip.DefaultCompression)
}

// GzipLevel GzipLevel 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - level: GzipLevel 동작에 필요한 level 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func GzipLevel(level int) Compressor {
	return streamCompressor{
		name: "gzip",
		writer: func(writer io.Writer) (io.WriteCloser, error) {
			return gzip.NewWriterLevel(writer, level)
		},
		reader: func(reader io.Reader) (io.ReadCloser, error) {
			return gzip.NewReader(reader)
		},
	}
}

// Zlib Zlib 공개 API의 동작을 수행한다.
func Zlib() Compressor {
	return ZlibLevel(flate.DefaultCompression)
}

// ZlibLevel ZlibLevel 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - level: ZlibLevel 동작에 필요한 level 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func ZlibLevel(level int) Compressor {
	return streamCompressor{
		name: "zlib",
		writer: func(writer io.Writer) (io.WriteCloser, error) {
			return zlib.NewWriterLevel(writer, level)
		},
		reader: func(reader io.Reader) (io.ReadCloser, error) {
			return zlib.NewReader(reader)
		},
	}
}

// Deflate Deflate 공개 API의 동작을 수행한다.
func Deflate() Compressor {
	return DeflateLevel(flate.DefaultCompression)
}

// DeflateLevel DeflateLevel 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - level: DeflateLevel 동작에 필요한 level 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func DeflateLevel(level int) Compressor {
	return streamCompressor{
		name: "deflate",
		writer: func(writer io.Writer) (io.WriteCloser, error) {
			return flate.NewWriter(writer, level)
		},
		reader: func(reader io.Reader) (io.ReadCloser, error) {
			return flate.NewReader(reader), nil
		},
	}
}
