// Package btredis provides small Redis safety primitives shared by Redis-backed
// bluetape-go packages.
//
// The package intentionally does not wrap the go-redis client and does not own
// Redis connections, logging, metrics, retries, or tenant isolation. Callers own
// Redis clients, deadlines, access control, and package-specific key contracts.
//
// Owner tokens and exact Redis keys are sensitive lease inputs. Use Lease,
// OwnerToken, and redacted key formatting for diagnostics; do not log RedisValue
// output or raw keys.
package btredis
