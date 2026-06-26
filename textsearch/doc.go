// Package textsearch provides deterministic multi-pattern text search.
//
// Matchers are immutable after Compile returns. They are safe for concurrent
// use by multiple goroutines as long as callers do not mutate their input
// strings while searching, which is Go's normal string contract.
package textsearch
