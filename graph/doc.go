// Package graph provides model-only graph values for I/O helpers and examples.
//
// The package intentionally contains no graph database client, query DSL,
// schema manager, repository, session, transaction, algorithm engine, or backend
// adapter. Follow-up packages own those contracts after concrete I/O and example
// work proves shared behavior.
//
// Properties are shallow-copied at map boundaries. Nested mutable values remain
// caller-owned and should be deep-copied by future adapters before trust
// boundaries.
//
// Path validates step values and weight only. It does not prove endpoint
// continuity, alternating vertex/edge order, or traversal correctness. Future
// algorithms or backend adapters own those stricter invariants.
package graph
