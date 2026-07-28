package compression

// Default는 Default 공개 API의 동작을 수행한다.
func Default() Compressor {
	return Zstd()
}

// All는 All 공개 API의 동작을 수행한다.
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
