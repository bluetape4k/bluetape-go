package compression

// Default Default 공개 API의 동작을 수행한다.
func Default() Compressor {
	return Zstd()
}

// All All 공개 API의 동작을 수행한다.
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
