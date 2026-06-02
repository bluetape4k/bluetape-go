// Package compression provides small compressor adapters with streaming support.
//
// The package keeps algorithms explicit. Gzip, zlib, and raw DEFLATE use Go's
// standard library. Zstd, LZ4, and Snappy use focused pure-Go dependencies
// because they are common service payload choices and are not in the standard
// library.
package compression
