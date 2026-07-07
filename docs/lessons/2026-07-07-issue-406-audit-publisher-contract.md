# Issue #406 Lesson: Audit Publisher Contract

## Decision

`audit/sqloutbox.Publisher` remains a small `context.Context` plus `Record`
contract. It is not a generic message-bus abstraction.

## Lessons

- Treat caller context cancellation as shutdown, not publisher failure. Relay
  must return the cancellation error and leave the claimed row for lease-based
  recovery.
- Treat all other publish errors as retry/dead-letter state through
  `MarkFailed`.
- Prove duplicate publish attempts with a stable event ID and idempotency key
  before adding helper or broker adapters.
- Use `AsyncJobTester` for cancellation lifecycle coverage and
  `GoroutineStressTester` for concurrent relay/store paths when the package
  owns goroutine-sensitive behavior.
- Do not add or redraw diagrams unless the public participants or sequence
  shape changes. README prose is enough for contract refinements when existing
  class and sequence diagrams remain visually sound.

## Follow-Ups

- #407 can add the deterministic test/discard publisher helpers against this
  contract.
- #408 should adopt the contract language in examples and avoid presenting
  transport adapters as audit history stores.
