# Audit Research Ordering

For audit work, do not begin by porting JaVers or by choosing Kafka, Redis, or
SQL adapters first. The first Go value is a small audit/event model with
explicit aggregate IDs, revisions, metadata, serialization rules, and history
query semantics.

Storage should be proven through an in-memory conformance implementation before
any durable adapter. SQL is the likely durable history source, but it should
align with the relational SQL boundary. Kafka and NATS are publisher/outbox
adapters, not history query stores. Redis is a projection or explicit audit
store only when replay and head semantics are defined.
