// Package sqloutbox stores audit entries in a PostgreSQL-backed outbox table.
//
// The package is intentionally transaction-neutral: callers pass the
// database/sql session, usually *sql.DB or *sql.Tx, that should own each
// operation. Delivery is at-least-once and publishers must handle duplicate
// attempts by using the stable event ID or idempotency key.
package sqloutbox
