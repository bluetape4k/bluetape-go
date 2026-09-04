// Package redislock provides a Redis-backed fenced lock with an expiring
// owner lease and a persistent, monotonically increasing fencing token.
//
// The lock is a single Redis-instance primitive. A fencing token only protects
// an external resource when that resource stores the greatest accepted token
// and rejects older tokens. Lease expiry can otherwise allow an old holder and
// a new holder to overlap, so callers must keep critical sections bounded by
// the lease TTL and use a cleanup context when the work context is canceled.
package redislock
