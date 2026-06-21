// Package bloomhash contains shared Bloom filter hash index derivation helpers.
package bloomhash

import (
	"crypto/sha256"
	"encoding/binary"
)

const hash2Fallback = uint64(0x9e3779b97f4a7c15)

// Indexes returns Bloom bit offsets using SHA-256 double hashing.
func Indexes(bytes []byte, hashFunctionCount uint64, bitSize uint64) []uint64 {
	hash1, hash2 := hashes(bytes)
	result := make([]uint64, hashFunctionCount)
	for i := range hashFunctionCount {
		result[i] = (hash1 + i*hash2) % bitSize
	}
	return result
}

// AppendIndexes appends Bloom bit offsets to dst without allocating an indexes slice.
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
