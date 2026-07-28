package compression

import (
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"io"
)

// Gzip 해당 형식의 compressor를 생성한다.
func Gzip() Compressor {
	return GzipLevel(gzip.DefaultCompression)
}

// GzipLevel 지정한 압축 level을 사용하는 compressor를 생성한다.
//
// 매개변수:
//   - level: GzipLevel에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
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

// Zlib 해당 형식의 compressor를 생성한다.
func Zlib() Compressor {
	return ZlibLevel(flate.DefaultCompression)
}

// ZlibLevel 지정한 압축 level을 사용하는 compressor를 생성한다.
//
// 매개변수:
//   - level: ZlibLevel에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
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

// Deflate 해당 형식의 compressor를 생성한다.
func Deflate() Compressor {
	return DeflateLevel(flate.DefaultCompression)
}

// DeflateLevel 지정한 압축 level을 사용하는 compressor를 생성한다.
//
// 매개변수:
//   - level: DeflateLevel에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
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
