# Issue #56 Audit Model Lessons

Date: 2026-06-27

## Lessons

- Keep raw byte cloning neutral. Event payload defaults can be event-specific,
  but shared clone helpers must not convert nil payloads into valid JSON because
  snapshot and decode paths need to reject missing required fields.
- Validation errors for audit data should preserve sentinel and field evidence
  without echoing caller values. Audit payloads, metadata, authors, and
  idempotency keys are likely to contain sensitive operational data.
- In-memory pending events are a retry aid, not durability. Recorder examples
  must say that source writes and audit commits need a shared durable
  transaction, rollback, outbox, or reconciliation path.
- New package APIs should be made Go-idiomatic before repository/outbox packages
  depend on them. `audit.Entry`, `audit.NewEntry`, `audit.DecodeEntryJSON`, and
  `audit.ErrInvalidEntry` avoid freezing stuttered names.
