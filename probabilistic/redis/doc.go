// Package redisbloom provides Redis-backed Bloom filters using ordinary Redis
// bitmap commands and immutable metadata.
//
// Filters in this package store shared distributed state. A false result means
// a value is not present unless Redis keys were cleared, deleted, evicted, or
// overwritten after insertion. A true result may be a false positive.
package redisbloom
