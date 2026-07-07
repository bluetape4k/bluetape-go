# Issue #406 Review: Audit Publisher Contract

## Scope

- `audit/sqloutbox.Publisher` contract documentation.
- `Relay.RunOnce` caller cancellation handling.
- PostgreSQL-backed cancellation, duplicate retry, and concurrent relay stress
  tests.
- English/Korean README updates and issue traceability docs.

## Subagent Note

Native subagent spawning was not used because the available subagent tool
surface is gated to explicit user subagent requests. The main session performed
the six independent review lanes and integration verdict as fallback.

## Lane Findings

### Performance

P0: 0
P1: 0

- `isCallerCancellation` is a constant-time check on an already-returned
  publisher error.
- The new stress test adds package test cost but remains scoped to
  `./audit/sqloutbox`.

### Stability

P0: 0
P1: 0

- Caller cancellation now returns without converting shutdown into retry or
  dead-letter state.
- Non-cancellation publisher errors still use the existing `MarkFailed` path.
- Stress coverage exercises concurrent `RunOnce` claim/publish/mark behavior.

### Security

P0: 0
P1: 0

- No new secrets, network clients, or broker credentials are introduced.
- README now states returned publisher errors should be bounded and redacted
  because failure text is persisted.

### Operator/Ops

P0: 0
P1: 0

- Shutdown semantics are explicit: caller context cancellation leaves claimed
  rows for lease-based recovery instead of writing false failure state.
- Adapter-owned topology, TLS, retention, replay, logging, metrics, and
  redaction responsibilities are documented.

### Developer/API

P0: 0
P1: 0

- Public API shape stays stable: `Publisher.Publish(context.Context, Record)
  error`.
- The doc comment now describes at-least-once duplicates, dedupe keys, and
  cancellation behavior.
- No generic message-bus abstraction was introduced.

### User/Caller

P0: 0
P1: 0

- README examples remain valid.
- English and Korean README files are synchronized for the new contract.
- Existing class and sequence diagrams remain visually sound and still cover
  the public participants and retry branch; no new diagram is needed.

## Integration Verdict

P0: 0
P1: 0

The change is narrow and contract-preserving. It fixes an observable shutdown
semantics gap, adds regression coverage for cancellation, duplicate retry
envelope stability, and concurrent relay execution, and records diagram
judgment without redrawing unchanged visuals.

## Evidence

- `go test -count=1 ./audit/sqloutbox`
- `go test -race -count=1 ./audit/sqloutbox`
- `go test -count=1 ./audit/sqloutbox -run 'RelayRunOnce(PublisherContextCancellationDoesNotRetry|RetriesDuplicatePublishWithStableEnvelope|ConcurrentStressPublishesEachRecordOnce)'`
- `golangci-lint cache clean && golangci-lint run ./...`
- `make ci`
- `git diff --check`
- Visual inspection:
  - `docs/images/readme-diagrams/audit-sqloutbox-class-contract-map.png`
  - `docs/images/readme-diagrams/audit-sqloutbox-relay-sequence.png`
