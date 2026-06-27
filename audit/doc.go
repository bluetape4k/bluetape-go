// Package audit provides storage-neutral aggregate event, audit history, and
// repository contracts.
//
// The package records domain events with stable aggregate identity, monotonic
// revisions, caller-owned event IDs, idempotency keys, and JSON-safe metadata.
// It includes a non-durable in-memory repository for tests and examples. It
// intentionally does not own durable SQL schemas, Redis/Kafka/NATS adapters,
// outbox publishing, source transaction choreography, or object diffing.
package audit
