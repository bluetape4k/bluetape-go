// Package mongoleader provides MongoDB-backed leader election.
//
// The package implements leader.Elector, leader.GroupElector, and
// leader.StrategicElector with separate document models for single leases,
// bounded group slots, and strategy candidate registries.
//
// Callers own the MongoDB client, database, collection, indexes, write concern,
// and cleanup. Lease validity is decided by the lease_until field in normal
// reads and writes; TTL indexes are optional cleanup support only.
package mongoleader
