package probabilistic

import "github.com/bluetape4k/bluetape-go/probabilistic/internal/bloomhash"

func indexes(bytes []byte, hashFunctionCount uint64, bitSize uint64) []uint64 {
	return bloomhash.Indexes(bytes, hashFunctionCount, bitSize)
}
