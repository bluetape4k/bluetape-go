// Package mongoleader provides MongoDB-backed single leader election.
//
// The package implements only leader.Elector. MongoDB GroupElector and
// StrategicElector require separate document models and are intentionally not
// part of this package.
//
// Callers own the MongoDB client, database, collection, indexes, write concern,
// and cleanup. Lease validity is decided by the lease_until field in normal
// reads and writes; TTL indexes are optional cleanup support only.
package mongoleader
