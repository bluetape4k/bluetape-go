package core

import "github.com/cespare/xxhash/v2"

// XXH64Bytes returns the deterministic XXH64 digest of value with seed 0.
//
// XXH64 is a fast non-cryptographic hash. Do not use it for signatures,
// passwords, authentication tokens, or attacker-resistant integrity checks.
func XXH64Bytes(value []byte) uint64 {
	return xxhash.Sum64(value)
}

// XXH64String returns the deterministic XXH64 digest of value with seed 0.
//
// XXH64 is a fast non-cryptographic hash. Do not use it for signatures,
// passwords, authentication tokens, or attacker-resistant integrity checks.
func XXH64String(value string) uint64 {
	return xxhash.Sum64String(value)
}
