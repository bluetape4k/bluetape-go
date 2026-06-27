# Issue 58 Audit Outbox Design Review

Scope: #58 research/design artifacts, audit README boundary updates, and the
follow-up SQL outbox implementation issue.

Baseline: `de878226904b8b83ec3a4678478983ce81d41c50`

## 7-Tier Review

| Tier | Perspective | P0 | P1 | Verdict | Evidence |
|---|---:|---:|---:|---|---|
| 1 | Security | 0 | 0 | PASS | Design keeps payload redaction, PII policy, tenant isolation, broker auth/TLS, and byte limits application-owned; untrusted JSON must be bounded before `audit.DecodeEntryJSON`. |
| 2 | Architecture | 0 | 0 | PASS | SQL outbox is selected as the first durable boundary; transport adapters are deferred until durable retry/idempotency semantics exist. |
| 3 | Data correctness | 0 | 0 | PASS | At-least-once delivery, duplicate publishes, event ID/idempotency key uniqueness, and per-aggregate ordering limits are explicit. |
| 4 | Go API | 0 | 0 | PASS | Contract roles stay small: store, record, publisher, relay; no broad event-sourcing framework or Kotlin/JVM-shaped surface is introduced. |
| 5 | Operations | 0 | 0 | PASS | Retry, next availability, dead-letter, redacted failure state, migration ownership, and broker topology are visible responsibilities. |
| 6 | Testing | 0 | 0 | PASS | Follow-up #346 requires unit, PostgreSQL Testcontainers, cancellation, dead-letter, and stress/race coverage for async relay behavior. |
| 7 | User/caller | 0 | 0 | PASS | README and research note separate audit history users from publisher/relay users and document application-owned transaction choreography. |

## Integration Verdict

P0=0 P1=0

The selected SQL outbox issue is narrow enough for the next implementation
slice and does not prematurely commit the repository to Kafka, NATS, Redis
Streams, or direct Redis audit storage.
