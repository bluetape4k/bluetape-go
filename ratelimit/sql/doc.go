// Package sqlratelimit provides a PostgreSQL-backed keyed token-bucket limiter.
//
// Callers own the database pool, schema migration, and cleanup scheduling.
package sqlratelimit
