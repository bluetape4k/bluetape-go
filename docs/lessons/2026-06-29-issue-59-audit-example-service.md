# Issue #59 Lesson: Audit Example Service

## Decision

The runnable Go example should show audit boundaries with a plain order service
instead of copying Ktor, Spring, Exposed, Kafka, or Redis wiring.

## Lessons

- The useful source parity is the source-write versus audit-history boundary,
  not the JVM framework container.
- Example code should remain under `examples/` so it demonstrates composition
  without becoming a production helper API.
- In-memory outbox replay is enough for a service-free example when the README
  clearly points durable delivery to `audit/sqloutbox`.

## Follow-ups

- Workshop examples can later compose `examples/audit` concepts with a real SQL
  outbox or broker adapter.
- Keep future HTTP examples explicit about whether current reads come from the
  source model, audit history, or a projection.
