// Package redisbloom provides Redis-backed Bloom filters and HyperLogLog
// cardinality estimates using ordinary Redis commands.
//
// Filters in this package store shared distributed state. A false result means
// a value is not present unless Redis keys were cleared, deleted, evicted, or
// overwritten after insertion. A true result may be a false positive.
//
// HyperLogLog values estimate cardinality only. They do not answer membership
// questions and should not be used as Bloom or Cuckoo filters.
package redisbloom
