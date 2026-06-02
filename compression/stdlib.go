package compression

import (
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"io"
)

// Gzip returns a gzip compressor using the standard library.
func Gzip() Compressor {
	return GzipLevel(gzip.DefaultCompression)
}

// GzipLevel returns a gzip compressor with the requested compression level.
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

// Zlib returns a zlib compressor using the standard library.
func Zlib() Compressor {
	return ZlibLevel(flate.DefaultCompression)
}

// ZlibLevel returns a zlib compressor with the requested compression level.
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

// Deflate returns a raw DEFLATE compressor using the standard library.
func Deflate() Compressor {
	return DeflateLevel(flate.DefaultCompression)
}

// DeflateLevel returns a raw DEFLATE compressor with the requested level.
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
