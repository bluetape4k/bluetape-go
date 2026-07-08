# Issue #439 Audit Outbox Benchmark Lesson

Audit and outbox benchmark work is complete only when raw output, result tables,
charts, and written interpretation all move together.

- Preserve benchmark commands and raw output under `docs/research/outputs/` so
  follow-up adapter issues can link measured evidence instead of summary-only
  claims.
- Keep in-memory audit repository rows separate from PostgreSQL/Testcontainers
  outbox rows. They measure different boundaries and should not be ranked as
  interchangeable delivery options.
- Database-backed benchmarks need an explicit opt-in env flag, serial command,
  fixture image/version, and bounded operation contexts.
- Treat impossible Testcontainers rows such as single-digit `ns/op` as a timer
  hygiene failure. Setup, reset, and seed work may be excluded, but the SQL
  operation itself must be inside `StartTimer` / `StopTimer`.
- A local benchmark snapshot is regression and planning evidence, not a
  production throughput guarantee. Do not use it to change delivery semantics,
  retry ownership, idempotency, or dead-letter behavior without a separate
  design issue.
