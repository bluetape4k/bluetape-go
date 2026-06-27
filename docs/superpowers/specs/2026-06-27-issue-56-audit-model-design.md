# Issue 56 Audit Model Design

## Problem

Issue #56 starts the 0.9.0 audit/event package track. The package needs stable
Go model contracts inspired by `bluetape4k-javers`, but it must not depend on
JaVers internals, JVM framework behavior, hidden persistence, or a full event
sourcing runtime.

## Current Evidence

- GitHub issue #56 is a P0 0.9.0 task for aggregate IDs, aggregate roots,
  domain events, revisions, audit entries, snapshot/change metadata,
  serialization expectations, event recording, and history reconstruction
  basics.
- `docs/research/2026-06-25-issue-41-audit-scope.md` says #56 should define the
  core model first: aggregate ID/type, revision, domain event, audit entry,
  snapshot metadata, author, occurred/recorded timestamps, idempotency key, and
  serialization rules.
- The same research note defers repository interfaces, outbox semantics, SQL,
  Redis, Kafka, NATS, JaVers object graph diffing, shadow reconstruction, and
  framework parity to later issues.
- `bluetape4k-javers/javers-ddd` provides useful source concepts:
  `AggregateRoot` exposes stable aggregate identity, `DomainEvent` carries
  aggregate ID, occurrence time, and string metadata, and `AggregateRepository`
  publishes events only after source persistence and audit commit succeed.
- Existing bluetape-go packages use small package-local APIs, sentinel errors,
  `doc.go`, bilingual README pairs, table-driven tests, and
  `testing/concurrency` stress helpers when mutable shared state is introduced.

## Design Goals

- Define a storage-neutral `audit` package with explicit model values and no
  new third-party dependencies.
- Make JSON serialization stable through explicit field names, schema version,
  aggregate identity fields, event identity fields, revision, timestamps,
  metadata, and raw payload bytes.
- Keep decoded JSON records subject to the same validation as constructor-built
  values so storage, API, or outbox inputs cannot bypass invariants.
- Keep diffing explicit and out of scope. Callers can record changed fields or
  raw snapshots, but this package will not compare object graphs.
- Provide a small aggregate event recorder that assigns monotonic revisions and
  returns immutable event copies for later repository/outbox issues without
  clearing uncommitted events until the caller explicitly acknowledges durable
  persistence.
- Provide history reconstruction basics that validate one aggregate, sort by
  revision, reject duplicate/non-positive/non-contiguous revisions, and expose
  current head metadata without replaying domain state.

## Non-Goals

- No JaVers clone, CDO snapshot model, object graph diff engine, or shadow
  reconstruction.
- No event sourcing framework and no domain-state replay API.
- No repository, SQL schema, Redis store, Kafka/NATS publisher, durable outbox,
  or hidden ORM persistence in #56.
- No framework hooks for Spring, Ktor, Exposed, transactions, or application
  event buses.

## Approach Options

### Option A: Minimal Model-Only Values

Create only structs for aggregate IDs, domain events, audit entries, metadata,
and history validation helpers. This is simple and low risk, but it leaves event
recording and revision assignment entirely to callers, so it misses part of the
#56 acceptance criteria.

### Option B: Model Values Plus Aggregate Recorder

Create immutable value contracts plus a goroutine-safe aggregate recorder that
records events, assigns next revisions, snapshots pending events, and clears
them only after a caller acknowledges durable persistence. This remains
storage-neutral, gives #57 a clean repository input, and provides a real target
for stress/race coverage.

### Option C: Repository-Ready Audit Runtime

Create model values, recorder, repository interfaces, in-memory repository, and
publisher hooks. This would accelerate later issues, but it crosses #57 and #58
boundaries and risks designing persistence/outbox contracts before their review.

## Chosen Design

Use Option B.

The new `audit` package will include:

- `AggregateID` with `Type` and `ID`, constructor validation, `String`, and
  stable JSON field names.
- `Revision` as a positive `uint64` revision value with `Initial` and `Next`
  helpers. `Next` must return an error instead of wrapping when the current
  revision is `math.MaxUint64`.
- `Metadata` as `map[string]string` with clone/copy helpers so caller mutation
  cannot rewrite recorded events.
- `DomainEvent` with `EventID`, `EventType`, `AggregateID`, `Revision`,
  `OccurredAt`, `RecordedAt`, `IdempotencyKey`, `Metadata`, and raw JSON
  `Payload`. Event IDs and idempotency keys are required caller-supplied opaque
  values; the recorder does not generate either value.
- `AuditEntry` with schema version, aggregate identity, revision, author,
  optional `SnapshotMetadata`, optional `ChangeMetadata`, and the recorded
  event. Validation must reject entries where the entry aggregate/revision does
  not match the nested event aggregate/revision.
- `Validate` methods for decoded aggregate IDs, events, entries, metadata, and
  history inputs. JSON unmarshalling must call the same validation path or the
  documented decode helper must reject invalid decoded records before use.
- `AggregateRecorder` as a small concurrency-safe recorder for one aggregate.
  It can be created from zero or a known head revision, assigns revisions
  monotonically after that head, validates event type and timestamps, copies
  metadata/payload, exposes immutable `PendingEvents`, and clears pending events
  only through an explicit acknowledgement such as `AckThrough(revision)`.
- `History` reconstruction helpers that accept audit entries for one aggregate,
  sort ascending by revision, reject invalid/mixed/duplicate/non-contiguous
  revisions, and expose head revision and entries.

The recorder handoff contract is peek-plus-ack, not clear-on-read. A caller can
read pending events repeatedly after a repository/outbox failure and must only
acknowledge through a revision once the corresponding audit entries are durable.
This avoids silently losing audit events between model recording and future #57
or #58 persistence/outbox steps.

Go code should not define a JaVers-style `AggregateRoot` interface in #56. The
Go-native aggregate root contract is the stable `AggregateID` plus a caller-owned
domain object. `AggregateRecorder` is a helper for recording events around that
domain object; it is not a base class, repository, or persistence owner.

## Error Handling

The package will expose sentinel errors compatible with `errors.Is`:

- `ErrInvalidAggregateID`
- `ErrInvalidRevision`
- `ErrInvalidEvent`
- `ErrInvalidAuditEntry`
- `ErrMixedAggregate`
- `ErrRevisionConflict`

Typed error values will include the invalid field or aggregate/revision context
without hiding the sentinel kind.

## Serialization Contract

The first schema version is `1`. Public JSON fields will remain explicit and
lower snake-style in struct tags, for example `schema_version`, `aggregate`,
`revision`, `event_id`, `event_type`, `occurred_at`, `recorded_at`,
`idempotency_key`, `metadata`, `payload`, `snapshot`, and `change`.

`Payload` is `json.RawMessage`. Constructors and JSON decode paths must copy it
and reject invalid JSON payloads except an empty payload, which is normalized to
`{}`. This keeps payload compatibility caller-owned while still preventing
malformed audit records from entering the model.

Decoded records must reject unsupported schema versions, empty aggregate type or
ID, zero revisions, empty event IDs or event types, zero timestamps, missing
idempotency keys, malformed payload JSON, mixed aggregate identity, and
duplicate revisions before they can be used for history reconstruction.

Unknown JSON fields are ignored for forward compatibility, but unsupported
`schema_version` values are rejected with an `ErrInvalidAuditEntry`-compatible
error. Missing required fields are rejected instead of defaulting to zero
values. This gives rolled-back v1 readers a deterministic failure mode when
future producers write incompatible versions.

Zero-value public structs are invalid until constructed or decoded through the
package validation path. Constructors, `Validate`, JSON decode helpers, and
history reconstruction must return `errors.Is`-compatible sentinel errors for
invalid zero values instead of treating them as empty records.

`SnapshotMetadata` has the stable shape `{format, schema_version, payload}`:
`format` and `schema_version` are required non-blank strings and `payload` is a
validated copied `json.RawMessage`. It stores caller-provided state snapshots
without diffing or replay semantics.

`ChangeMetadata` has the stable shape `{changed_fields, summary, attributes}`:
`changed_fields` is a copied sorted set of non-blank field names, `summary` is
optional caller text, and `attributes` is copied string metadata. It records
explicit caller-known change hints only; the package does not compute diffs.

Idempotency keys are opaque caller-supplied values in #56. The recorder will not
derive predictable keys from aggregate IDs, revisions, event types, or
timestamps. Later persistence/outbox issues may define optional generator
helpers, but deduplication semantics must not depend on parseable key contents.
Within one reconstructed history, duplicate idempotency keys for different
event IDs are invalid because they would make retry/deduplication ambiguous for
later repository or outbox layers.

The package does not inspect, classify, redact, truncate, or encrypt metadata,
payload, snapshot, change, or author fields. Callers must redact secrets and
PII before constructing audit records and must enforce payload size/depth limits
at trust boundaries that accept untrusted input.

Recorder implementation should validate and copy metadata/payload before taking
the recorder mutex when possible. The lock should cover only revision assignment
and pending-slice mutation/snapshot/ack operations, so large payload validation
does not unnecessarily serialize independent callers.

## Test Requirements

- Event recording assigns revision 1, then 2, preserves aggregate identity, and
  returns immutable copies of metadata and payload.
- Recorder construction from a restored head revision appends at head + 1 and
  does not produce duplicate revisions after history reconstruction.
- Pending-event tests prove repeated `PendingEvents` calls do not clear events,
  failed downstream persistence can repull the same events, and ack clears only
  revisions at or below the acknowledged durable revision.
- Revision ordering rejects zero, duplicate, non-contiguous, and mixed-aggregate
  audit entries, and history reconstruction sorts valid entries.
- JSON serialization round trips a canonical audit entry and preserves the
  expected field names and schema version.
- Negative JSON/decode tests reject unsupported schema versions, empty aggregate
  IDs, zero revisions, empty event IDs/types, zero timestamps, missing
  idempotency keys, malformed payloads, mixed aggregate identity, and duplicate
  revisions.
- Audit-entry validation tests reject entry/event aggregate or revision
  mismatches, invalid zero-value structs, duplicate event IDs, duplicate
  idempotency keys, and revision overflow.
- Snapshot/change metadata tests cover required fields, copied payloads,
  changed-field normalization, and caller-owned diffing semantics.
- Idempotency key tests prove caller-supplied opaque keys are preserved and no
  predictable key is generated by the recorder.
- History reconstruction exposes the expected head revision and entries without
  replaying domain state.
- `AggregateRecorder` concurrent recording uses
  `testing/concurrency.GoroutineStressTester`; no `AsyncJobTester` is required
  because #56 introduces no asynchronous or context-cancelable operation.
- Race verification must include `go test -race -count=1 ./audit` and a bounded
  stress test that asserts no duplicate or skipped revisions, immutable returned
  events, and atomic `PendingEvents`/ack behavior under concurrent `Record`.

## Documentation Impact

- Add `audit/README.md` and `audit/README.ko.md` with package purpose, examples,
  serialization contract, and non-goals.
- Document that #56 is not durable audit storage or an outbox. Production
  callers must make source persistence, audit commit, outbox enqueue,
  observability, replay/head ownership, and rollback behavior explicit in #57
  and #58 adapters.
- Add `audit/doc.go` with package-level Go documentation and a JaVers migration
  expectation table that maps `AggregateRoot`, DomainEvent metadata, JaVers
  snapshots, repositories, object diffing, and publishers to the #56 Go-native
  contract or later issues.
- Update root `README.md` and `README.ko.md` package lists and documentation
  links.
- Add a review artifact under `docs/review/` and a lesson under
  `docs/lessons/` after implementation.

## Acceptance Criteria Mapping

| Issue #56 requirement | Design coverage |
|---|---|
| Aggregate ID/root/domain event/revision/audit entry/metadata | `AggregateID`, `AggregateRecorder`, `DomainEvent`, `Revision`, `AuditEntry`, `SnapshotMetadata`, `ChangeMetadata` |
| Serialization compatibility expectations | Schema version 1, explicit JSON tags, raw JSON payload validation, validated decode, and copy behavior |
| Diffing decision | Explicit non-goal; callers may record changed fields only |
| Event recording | `AggregateRecorder` with monotonic revisions, head restart, peek-plus-ack pending-event lifecycle |
| History reconstruction basics | `NewHistory` validation, sorting, head revision, one-aggregate invariant, contiguous revisions |
| Tests | Unit, serialization, history, `GoroutineStressTester`, and explicit race verification |
