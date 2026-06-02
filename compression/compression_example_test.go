package compression_test

import (
	"bytes"
	"fmt"

	"github.com/bluetape4k/bluetape-go/compression"
)

func ExampleCompressor() {
	compressor := compression.Default()
	payload := []byte("bluetape-go compression payload")

	compressed, err := compressor.Compress(payload)
	if err != nil {
		return
	}
	decompressed, err := compressor.Decompress(compressed)
	if err != nil {
		return
	}

	fmt.Println(compressor.Name())
	fmt.Println(bytes.Equal(payload, decompressed))

	// Output:
	// zstd
	// true
}

func ExampleAll() {
	for _, compressor := range compression.All() {
		fmt.Println(compressor.Name())
	}

	// Output:
	// gzip
	// zlib
	// deflate
	// zstd
	// lz4
	// snappy
}
