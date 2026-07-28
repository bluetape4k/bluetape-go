package compression

// Default 기본 compressor registry를 반환한다.
func Default() Compressor {
	return Zstd()
}

// All 등록된 모든 compressor를 반환한다.
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
