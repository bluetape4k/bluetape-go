package compression

// Default returns the recommended general-purpose compressor.
func Default() Compressor {
	return Zstd()
}

// All returns the foundation compressors supported by bluetape-go.
func All() []Compressor {
	return []Compressor{
		Gzip(),
		Zlib(),
		Deflate(),
		Zstd(),
		LZ4(),
		Snappy(),
	}
}
