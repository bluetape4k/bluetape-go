# Issue #408 Audit Publisher Adoption Lesson

Date: 2026-07-07
Scope: `examples/audit`, `audit/sqloutbox`

## Lesson

Audit publisher adoption belongs in the example boundary documentation, not in
a new abstraction. The durable path should read as
`Store.Enqueue -> Relay.RunOnce/Run -> Publisher.Publish -> downstream
dedupe`, with transaction ownership and duplicate delivery visible.

## Pattern

- Keep `examples/audit` small and runnable.
- Use `EntrySink` as the seam between the example replay path and production
  `Store.Enqueue`.
- Document relay lifecycle choices: `RunOnce` for scheduler-owned polling,
  `Run` for service-owned workers.
- Preserve stable `Record.EventID` and `Record.IdempotencyKey` in any publisher
  adapter prose.
- Link workshop follow-up issues when runnable cross-repo examples are needed.
- When a README diagram already owns the reader question, update that single
  asset and rerender it instead of adding parallel diagrams.

## Follow-Up

Durable broker adapters still need separate packages and operator runbooks for
topology, authentication/TLS, retention, replay, redaction, and consumer
idempotency.
