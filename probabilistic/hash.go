package probabilistic

import (
	"crypto/sha256"
	"encoding/binary"
)

const hash2Fallback = uint64(0x9e3779b97f4a7c15)

func indexes(bytes []byte, hashFunctionCount uint64, bitSize uint64) []uint64 {
	sum := sha256.Sum256(bytes)
	hash1 := binary.BigEndian.Uint64(sum[0:8])
	hash2 := binary.BigEndian.Uint64(sum[8:16])
	if hash2 == 0 {
		hash2 = hash2Fallback
	}

	result := make([]uint64, hashFunctionCount)
	for i := range hashFunctionCount {
		result[i] = (hash1 + i*hash2) % bitSize
	}
	return result
}
