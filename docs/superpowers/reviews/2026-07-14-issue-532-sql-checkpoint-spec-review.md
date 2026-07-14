# Issue #532 PostgreSQL Batch Checkpoint Spec Review

## Scope

- Spec: `docs/superpowers/specs/2026-07-14-issue-532-sql-checkpoint-design.md`
- Reviewed commit: `227d22322054ba6d93f2fb381e373678f81bf1e3`
- Reviewed SHA-256: `3d0208d73a8bc62073a57e60a5607502d53e9c69d0642e2f88351286024ed21e`
- Base: `origin/develop@873555fdd34d66c8cb85c869898017ea0820f1c0`
- Artifact kind: spec

## Initial findings and repairs

| Priority | Lens | Finding | Resolution |
|---|---|---|---|
| P0 | Stability, Developer/API | Output-count chunking could checkpoint past buffered kept output after a later filter or processor skip. | Atomic mode chunks by consumed input and commits pending output with the latest checkpoint. Empty and exact-boundary EOF do not add a revision. |
| P1 | Performance | All-filter and all-skip streams could create one checkpoint transaction per input. | Consumed-input chunking bounds transactions to approximately `ceil(inputs / ChunkSize)` and suppresses the business callback for empty output. |
| P1 | Stability | Missing-row CAS, revision exhaustion, commit cancellation, and competing actors left stale resurrection or ambiguous replay paths. | Create/update CAS statements are split, maximum revision fails before DB access, same-key exclusivity is mandatory, and provider-owned commit unknown is reconciled by a fresh load. |
| P1 | Stability | A callback could issue raw transaction control and allow checkpoint DML outside the original transaction. | A reserved savepoint ownership probe runs on callback return and panic. `25P02` requires a successful `ROLLBACK TO` proof; all unproven paths fail as atomicity unknown before checkpoint DML. |
| P1 | Stability, User/Caller | Guard-release unknown was conflated with provider-owned commit unknown, allowing unsafe automatic replay. | Provider-neutral `ErrAtomicityUnknown` is a stronger recovery barrier requiring quiesce and manual business/checkpoint reconciliation. `AtomicityPanic` preserves the same barrier across panic exits. |
| P1 | Security | Hostile schema objects, excessive privileges, oversized payloads, and diagnostics could break durability or leak data. | Permanent-table/catalog/constraint/rewrite checks, schema/table/column ACL validation, bounded payload projection, redacted typed errors, sensitive panic-value guidance, and hostile tests are required. |
| P1 | Operator/Ops | Legacy cutover, retention, pool cancellation, HA promotion, and unknown recovery lacked measurable gates. | Quiesced reconciliation, isolated canary, recovery drills, bounded ownership probe, pool drain assertions, writable-primary/RPO checks, and autovacuum/replication/dead-tuple runbook gates are specified. |
| P1 | Developer/API | Adding `AtomicWriter` to exported `StepOptions` would break external unkeyed composite literals. | Existing `StepOptions` and `NewStep` remain unchanged; the atomic path uses additive `AtomicStepOptions` and `NewAtomicStep`. |
| P1 | User/Caller | Callback misuse, generic panic restart, and provider switching could cause writes outside the atomic boundary or unsafe replay. | The spec forbids captured DB/external side effects/raw transaction control, adds a top-level recovery example, and requires explicit rollout/rollback reconciliation. |

## Final rerun results

All lanes reviewed the exact spec hash recorded above. The three available review agents were reused
across two bounded waves; each perspective ran as a separate read-only role-scoped task. The main
session owns integration and the final verdict.

| Perspective | P0 | P1 | P2 | P3 | Result |
|---|---:|---:|---:|---:|---|
| Performance | 0 | 0 | 0 | 2 | PASS. Hot-path round trips, chunk amplification, one-second probe, connection drain, and deferred issue #560 measurements are explicit. |
| Stability | 0 | 0 | 0 | 0 | PASS. Pending output, CAS, ownership proof, cancellation, panic, restart, and unknown-recovery boundaries converge. |
| Security | 0 | 0 | 0 | 0 | PASS. Bootstrap order, catalog/ACL checks, payload/codec limits, KeyID, savepoint, and panic redaction are fail closed. |
| Operator/Ops | 0 | 0 | 0 | 2 | PASS. Cutover/rollback, recovery drills, HA/RPO, timeouts, shutdown, retention, and storage-pressure operations are testable. |
| Developer/API | 0 | 0 | 0 | 0 | PASS. The API is additive, preserves keyed and unkeyed legacy callers, and makes lifecycle and error ownership explicit. |
| User/Caller | 0 | 0 | 0 | 0 | PASS. Setup, misuse prevention, restart, manual reconciliation, supervisor recovery, docs, and diagram requirements form one coherent journey. |

The remaining P3 notes are implementation-plan details: fix a deterministic scheduler tolerance for
the pool-drain test, measure savepoint cost under issue #560, and choose workload-specific alert and
vacuum thresholds in the runbook.

## Main-session integration verdict

The design preserves the legacy batch API while adding a database-agnostic atomic core contract and
a PostgreSQL provider whose business write and checkpoint CAS share one transaction. It does not
claim automatic safety when callback code breaks transaction ownership: those paths carry an
explicit atomicity-unknown barrier and prohibit automatic replay. Schema ownership, least privilege,
payload handling, cancellation, panic, rollout, recovery, bilingual documentation, and the required
sequence diagram are acceptance-tested obligations. No P0 or P1 design decision remains before
implementation planning.

P0=0 P1=0 P2=0 P3=4
