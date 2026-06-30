# Graph Backend Adapter Order

Date: 2026-06-30
Issue: #50
Milestone: 0.10.0

## Lesson

Backend adapter work should start with one driver-proven package, not a common
repository/session abstraction. Neo4j is the only evaluated backend with an
official Go driver, active releases, context-aware API shape, and a dedicated
Testcontainers Go module. Memgraph belongs next as compatibility coverage on
that driver path, not as a new abstraction.

AGE, FalkorDB, TinkerPop/TinkerGraph, and Neptune stay deferred or rejected for
the first Go adapter slice because their local-test, driver, or managed-service
shape would force more infrastructure than the current graph API can justify.

## Applied Follow-Ups

- #365 owns the first Neo4j adapter proof.
- #366 owns Memgraph compatibility against the Neo4j-driver path.
- #51 examples should use `graph` and `graph/graphio` first; backend examples
  can wait until #365 has a proven package boundary.

## Guardrail

Do not add backend-independent graph repository, session, schema, transaction,
or query interfaces until at least one backend adapter and one domain example
prove the shared behavior in Go.
