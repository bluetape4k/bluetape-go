# Lessons Learned - PostgreSQL Batch Checkpoint (2026-07-14)

**Related issue:** #532
**Affected modules:** `batch`, `batch/sqlcheckpoint`

## L1: Checkpoint progress is a consumed-input boundary

### Problem

Output count is not a safe resume position. A processor may filter an input,
emit multiple outputs, retry internally, or fail after consuming only part of a
chunk. Advancing by written output can replay or skip source items.

### Decision

Capture the checkpoint after each consumed input and commit the last captured
value only with that chunk's business output. Empty final chunks do not commit,
and failed provider calls cannot mutate the caller's retained pending slice.

### Future guard

Batch persistence must test filtered items, partial output, exact-multiple EOF,
empty input, processor retries, and provider mutation attempts before treating
an offset as durable progress.

## L2: Insert and update need different CAS proofs

### Problem

An absent checkpoint has no stored revision to compare, while an existing row
must reject stale writers. Treating both paths as an unconditional upsert can
hide concurrent creation or overwrite a newer resume position.

### Decision

Use an insert that succeeds only for expected revision zero, and an update that
matches the exact expected revision before incrementing it. A zero affected-row
result rolls back the whole transaction as `ErrCheckpointConflict`.

### Future guard

Provider conformance should prove exact winner/loser behavior with independent
business keys and a barrier immediately before checkpoint CAS.

## L3: Callback return does not prove transaction ownership

### Problem

A callback with SQL access can execute raw transaction control, leave the
transaction aborted, or panic after changing session state. A successful Go
return alone cannot prove the provider still owns a usable transaction.

### Decision

Expose a guarded query/exec session, establish a fixed savepoint, then probe
ownership before checkpoint CAS. Recover PostgreSQL `25P02` only with
`ROLLBACK TO SAVEPOINT`; if ownership cannot be proven, stop at
`ErrAtomicityUnknown` and never guess that rollback succeeded.

### Future guard

Test raw BEGIN/COMMIT/ROLLBACK, aborted transactions, savepoint loss,
cancellation, connection loss, panic, and competing actors. Ownership evidence
must fail closed.

## L4: Commit unknown and atomicity unknown require different recovery

### Problem

A lost commit response may mean the provider-owned transaction committed, but
an ownership failure may mean business and checkpoint attribution itself is
unknown. Automatically replaying either case can duplicate business effects.

### Decision

Classify provider commit ambiguity as `ErrCommitUnknown` with stable operation
`OperationCommit`. Classify unproven callback ownership as
`ErrAtomicityUnknown` as well. Both stop the step. The caller keeps the key
quiesced; commit-only unknown uses a bounded fresh atomic-writer `Load`, while
atomicity unknown requires manual business/checkpoint reconciliation.

### Future guard

Public examples must start at `Step.Run`, inspect `Report.Err`, preserve
same-key exclusivity, and prohibit automatic callback replay.

## L5: Panic behavior belongs to the supervisor contract

### Problem

Recovering a callback panic solely to clean up can accidentally replace the
original panic identity, expose its value in an error string, or resume after
an unproven transaction boundary.

### Decision

When ownership and rollback are proven, re-panic the original value exactly.
When they are not, raise a sanitized `AtomicityPanic` whose trusted accessor is
the only path to the original value. Trusted top-level supervision performs
quiesce and reconciliation.

### Future guard

Cover string, error, nil, and typed-nil panic values, and assert both identity
and default-format redaction.

## L6: Source compatibility needs an external unkeyed fixture

### Problem

Adding a field to an exported options struct can compile inside the package yet
break callers that use unkeyed composite literals.

### Decision

Keep the legacy options layout unchanged and add the atomic path through new
types and a new constructor. Compile an external unkeyed `StepOptions` fixture
as part of `make test` and therefore `make ci`.

### Future guard

For additive Go APIs, test compatibility from another package; package-local
tests cannot represent every source-level caller shape.

## L7: Cleanup and pool release need hostile small-pool tests

### Problem

Rollback or cleanup that depends on an already canceled context can strand a
connection. With a large pool the leak is easy to miss; with one connection it
immediately blocks the next operation.

### Decision

Use deterministic transaction cleanup, preserve rollback causes without
rendering secrets, and prove a known callback failure releases a one-connection
pool. Reader close is attempted with cancellation removed and its error is
joined; the caller still owns an outer shutdown deadline.

### Future guard

Every SQL provider should include one-connection failure/recovery tests and
distinguish library cleanup guarantees from caller-owned shutdown budgets.

## L8: `IF NOT EXISTS` needs catalog, ACL, and role-graph proof

### Problem

Migration syntax does not prove that a pre-existing object has the expected
columns, constraints, owner, ACLs, RLS/policy/trigger state, or role topology.
Checking only roles granted to runtime also misses dangerous inbound edges.

### Decision

Make schema application caller-owned and require exact pre-grant and post-grant
validation. Accept only the approved deployer-to-owner membership, reject all
runtime edges, privileged owner/runtime attributes, unrelated grants, column
ACLs, and hostile catalog drift.

### Future guard

Pair every fixed-schema SQL provider with normal and one-property-hostile
catalog fixtures, including both directions of `pg_auth_members`.

## L9: Isolation is a public provider contract

### Problem

Inheriting a database or role default of Repeatable Read changed concurrent CAS
behavior and surfaced a generic transaction failure instead of the documented
checkpoint conflict.

### Decision

Begin provider commits explicitly at Read Committed. Document that ambient
defaults are ignored, callbacks must be correct at that isolation, codecs and
callbacks must be concurrency-safe, and callers must serialize same-key work
when required by their business semantics.

### Future guard

Concurrency integration tests should deliberately change ambient isolation and
still require the provider's documented result.

## L10: Capacity work must remain evidence-based

### Problem

Correctness tests establish atomicity and failure semantics, not production
throughput, hot-key latency, WAL growth, autovacuum behavior, or pool sizing.

### Decision

Keep performance positioning qualitative and defer the benchmark/capacity
matrix to issue #560. The runbook requires deployment-owned canary thresholds
and database telemetry instead of universal numbers.

### Future guard

Do not turn conformance timings into capacity claims. Benchmark the exact
provider workload and topology under the dedicated benchmark issue.
