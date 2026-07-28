// Package bloomhash bluetape-go의 bloomhash probabilistic data structure 기능을 제공한다.
// 공개 API 주석은 capacity, false-positive rate, hasher, key, TTL, backend ownership, 오류 계약을 한국어로 확인할 수 있도록 유지한다.
package bloomhash

import (
	"crypto/sha256"
	"encoding/binary"
)

const hash2Fallback = uint64(0x9e3779b97f4a7c15)

// Indexes Indexes 공개 API의 동작을 수행하며 Bloom hash index 계산의 capacity, hash count, deterministic mapping 계약을 보존한다.
//
// 매개변수:
//   - bytes: Indexes 동작에 필요한 bytes 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - hashFunctionCount: hash index를 계산하는 deterministic hasher다. compatibility와 seed 의미는 hasher 계약을 따른다.
//   - bitSize: Indexes 동작에 필요한 bitSize 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func Indexes(bytes []byte, hashFunctionCount uint64, bitSize uint64) []uint64 {
	hash1, hash2 := hashes(bytes)
	result := make([]uint64, hashFunctionCount)
	for i := range hashFunctionCount {
		result[i] = (hash1 + i*hash2) % bitSize
	}
	return result
}

// AppendIndexes AppendIndexes 공개 API의 동작을 수행하며 Bloom hash index 계산의 capacity, hash count, deterministic mapping 계약을 보존한다.
//
// 매개변수:
//   - dst: AppendIndexes 동작에 필요한 dst 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - bytes: AppendIndexes 동작에 필요한 bytes 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - hashFunctionCount: hash index를 계산하는 deterministic hasher다. compatibility와 seed 의미는 hasher 계약을 따른다.
//   - bitSize: AppendIndexes 동작에 필요한 bitSize 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func AppendIndexes(dst []any, bytes []byte, hashFunctionCount uint64, bitSize uint64) []any {
	hash1, hash2 := hashes(bytes)
	for i := range hashFunctionCount {
		dst = append(dst, (hash1+i*hash2)%bitSize)
	}
	return dst
}

func hashes(bytes []byte) (uint64, uint64) {
	sum := sha256.Sum256(bytes)
	hash1 := binary.BigEndian.Uint64(sum[0:8])
	hash2 := binary.BigEndian.Uint64(sum[8:16])
	if hash2 == 0 {
		hash2 = hash2Fallback
	}
	return hash1, hash2
}
