# Issue 57 Audit Repository Lessons

## Context

Issue #57 builds on the #56 audit model by adding storage-neutral repository and
history query contracts before durable adapters.

## Lessons

- Missing audit history should be a boolean result, not a sentinel error, so
  callers can distinguish absent data from invalid input or storage failure.
- Keep `History` full and contiguous from initial revision. Partial history
  queries should return `[]Entry` so repository filters do not weaken history
  reconstruction invariants.
- In-memory repository ordering needs a documented contract. Append-order by
  default with `NewestFirst` reversal is simple, deterministic, and adapter
  friendly.
- Reusable adapter conformance belongs in an importable `audit/audittest`
  package instead of an `_test.go` helper inside `audit`; future adapter
  packages cannot import helpers compiled only for `audit` tests.
- Shared-state repository code needs both targeted behavior tests and
  `GoroutineStressTester` plus `go test -race`; no async job helper was added,
  so `AsyncJobTester` is not applicable to #57.

## Follow-Up Guardrails

- Durable SQL/Redis/Kafka/NATS adapters must run
  `audittest.RunRepositoryConformance` before adding backend-specific behavior.
- Outbox/source transaction semantics are still separate from repository query
  semantics and must not be implied by `MemoryRepository`.
