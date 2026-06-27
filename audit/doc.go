// Package audit provides storage-neutral aggregate event and audit history
// model types.
//
// The package records domain events with stable aggregate identity, monotonic
// revisions, caller-owned event IDs, idempotency keys, and JSON-safe metadata.
// It intentionally does not own repository persistence, outbox publishing,
// SQL schemas, Redis/Kafka/NATS adapters, or object diffing.
package audit
