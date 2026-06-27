# Issue 56 Audit Model Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the first Go-native `audit` package with aggregate/event/audit model contracts, validated serialization, ack-based event recording, and history reconstruction basics for issue #56.

**Architecture:** The package is storage-neutral and has no third-party dependencies. Immutable value types and validation functions protect decoded JSON and constructor-built records; only `AggregateRecorder` owns mutable state, guarded by a small mutex scope and proven with stress/race tests.

**Tech Stack:** Go standard library (`encoding/json`, `errors`, `fmt`, `math`, `sort`, `strings`, `sync`, `time`), bluetape-go `testing/concurrency`, bilingual README docs.

---

## File Structure

- Create `audit/doc.go`: package overview, non-goals, JaVers migration table.
- Create `audit/errors.go`: sentinel errors and typed validation error.
- Create `audit/types.go`: `AggregateID`, `Revision`, `Metadata`, constructors, validation helpers.
- Create `audit/event.go`: `DomainEvent`, event options, payload copy/validation, event validation.
- Create `audit/entry.go`: `Entry`, `SnapshotMetadata`, `ChangeMetadata`, JSON decode validation.
- Create `audit/recorder.go`: goroutine-safe recorder with pending snapshot plus explicit ack.
- Create `audit/history.go`: history reconstruction, sort/contiguous validation, duplicate event/idempotency validation.
- Create `audit/*_test.go`: unit, JSON, history, recorder, and concurrency tests.
- Create `audit/README.md` and `audit/README.ko.md`: usage, serialization contract, non-goals, migration table.
- Modify `README.md` and `README.ko.md`: package table and package documentation links.
- Modify `CHANGELOG.md`: add an `[Unreleased]` entry for the public `audit` package.
- Modify `WIP.md`: update or explicitly verify the current release target state for the 0.9.0 audit package work.
- Create `docs/review/2026-06-27-issue-56-audit-model-review.md`: Step 6-R review artifact.
- Create `docs/lessons/2026-06-27-issue-56-audit-model.md`: lesson on ack-based audit handoff.

## Task 1: Add Core Types And Validation Tests

**Complexity:** medium
**Skill:** Apply `$bluetape-go-patterns`.
**Files:**
- Create: `audit/types_test.go`
- Create: `audit/errors.go`
- Create: `audit/types.go`

- [ ] **Step 1: Write failing tests for aggregate IDs, revisions, metadata copies, and zero-value validation**

Add `audit/types_test.go` with these tests:

```go
package audit

import (
	"errors"
	"math"
	"testing"
)

func TestAggregateIDValidationAndString(t *testing.T) {
	id, err := NewAggregateID("order", "ord-1")
	if err != nil {
		t.Fatalf("new aggregate id: %v", err)
	}
	if got := id.String(); got != "order:ord-1" {
		t.Fatalf("string = %q", got)
	}
	if err := id.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	for _, tc := range []struct {
		name string
		typ  string
		id   string
	}{
		{name: "empty type", typ: "", id: "ord-1"},
		{name: "blank type", typ: "   ", id: "ord-1"},
		{name: "empty id", typ: "order", id: ""},
		{name: "blank id", typ: "order", id: "   "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewAggregateID(tc.typ, tc.id); !errors.Is(err, ErrInvalidAggregateID) {
				t.Fatalf("expected invalid aggregate id, got %v", err)
			}
		})
	}

	var zero AggregateID
	if err := zero.Validate(); !errors.Is(err, ErrInvalidAggregateID) {
		t.Fatalf("expected zero aggregate id invalid, got %v", err)
	}
}

func TestRevisionValidationNextAndOverflow(t *testing.T) {
	rev := InitialRevision()
	if rev != 1 {
		t.Fatalf("initial revision = %d", rev)
	}
	next, err := rev.Next()
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if next != 2 {
		t.Fatalf("next = %d", next)
	}
	if err := Revision(0).Validate(); !errors.Is(err, ErrInvalidRevision) {
		t.Fatalf("expected zero revision invalid, got %v", err)
	}
	if _, err := Revision(math.MaxUint64).Next(); !errors.Is(err, ErrInvalidRevision) {
		t.Fatalf("expected overflow invalid, got %v", err)
	}
}

func TestMetadataCloneIsImmutableFromCallerMutation(t *testing.T) {
	metadata := Metadata{"actor": "alice"}
	copied := metadata.Clone()
	metadata["actor"] = "mallory"
	copied["trace"] = "t-1"

	if copied["actor"] != "alice" {
		t.Fatalf("copied actor = %q", copied["actor"])
	}
	if _, ok := metadata["trace"]; ok {
		t.Fatalf("metadata was mutated through clone")
	}
}
```

- [ ] **Step 2: Verify tests fail before implementation**

Run:

```bash
go test -count=1 ./audit
```

Expected: FAIL because package `audit` or symbols are undefined.

- [ ] **Step 3: Implement sentinel errors, typed validation error, aggregate IDs, revisions, and metadata**

Implement:

- `ErrInvalidAggregateID`, `ErrInvalidRevision`, `ErrInvalidEvent`, `ErrInvalidEntry`, `ErrMixedAggregate`, `ErrRevisionConflict`.
- `ValidationError` with `Kind`, `Field`, `Value`, `Cause`, `Error`, `Unwrap`, and `Is`.
- `AggregateID{Type string, ID string}` with `NewAggregateID`, `Validate`, `String`.
- `Revision uint64` with `InitialRevision`, `Validate`, and `Next() (Revision, error)`.
- `Metadata map[string]string` with `Clone` and internal validation for non-blank keys.

- [ ] **Step 4: Verify Task 1**

Run:

```bash
go test -count=1 ./audit -run 'Test(AggregateID|Revision|Metadata)'
```

Expected: PASS.

## Task 2: Add Domain Events, Metadata Shapes, And JSON Decode Validation

**Complexity:** high
**Skill:** Apply `$bluetape-go-patterns`.
**Files:**
- Create: `audit/event_test.go`
- Create: `audit/entry_test.go`
- Create: `audit/event.go`
- Create: `audit/entry.go`

- [ ] **Step 1: Write failing tests for event construction and immutable payloads**

Add tests covering:

- `NewDomainEvent(EventOptions)` requires event ID, event type, aggregate ID,
  revision, occurred/recorded timestamps, idempotency key, and valid JSON
  payload. Public construction must use named fields; no broad positional
  constructor may accept same-type identity strings/timestamps.
- Empty payload normalizes to `{}`.
- Caller mutation of metadata and payload after construction does not mutate the event.
- Recorder does not generate event IDs or idempotency keys.

Run:

```bash
go test -count=1 ./audit -run 'TestDomainEvent'
```

Expected: FAIL because event APIs are missing.

- [ ] **Step 2: Implement `DomainEvent` and event validation**

Implement:

- `type EventID string`
- `type EventType string`
- `type DomainEvent struct`
- `type EventOptions struct` and `NewDomainEvent(EventOptions) (DomainEvent, error)`.
- Payload normalization/copy with `json.Valid`.
- `Validate` rejecting zero-value, empty IDs/types, zero timestamps, invalid aggregate/revision, empty idempotency key, invalid metadata, and malformed payload.

- [ ] **Step 3: Write failing tests for `SnapshotMetadata`, `ChangeMetadata`, and audit-entry JSON**

Add tests covering:

- `SnapshotMetadata{format, schema_version, payload}` requires non-blank format/schema version and valid copied JSON payload.
- `SnapshotMetadata{}` and decoded `{"snapshot":{}}` reject with an `ErrInvalidEntry`-compatible error when snapshot metadata is present.
- `ChangeMetadata{changed_fields, summary, attributes}` removes duplicates, rejects blank field names, sorts fields, copies attributes, and rejects an empty `changed_fields` set when change metadata is present.
- `ChangeMetadata{}` and decoded `{"change":{}}` reject with an `ErrInvalidEntry`-compatible error when change metadata is present.
- `Entry` validates schema version `1`, entry/event aggregate and revision match, author is non-blank, and nested event is valid.
- JSON round trip preserves `schema_version`, `aggregate`, `revision`, `event`, `snapshot`, and `change`.
- Decode rejects unsupported `schema_version`, missing required fields, zero values, malformed payloads, entry/event mismatch, and invalid nested metadata while ignoring unknown fields.
- Direct `json.Unmarshal` into `Entry` rejects the same invalid payloads as `DecodeEntryJSON`; invalid decoded structs must not rely on callers remembering to call `Validate`.

Run:

```bash
go test -count=1 ./audit -run 'Test(SnapshotMetadata|ChangeMetadata|Entry)'
```

Expected: FAIL because entry APIs are missing.

- [ ] **Step 4: Implement metadata shapes, `Entry`, validation, and JSON decode helper**

Implement:

- `const SchemaVersion = 1`
- `SnapshotMetadata` constructor/validation.
- `ChangeMetadata` constructor/validation with sorted changed fields.
- `Entry` constructor/validation.
- `DecodeEntryJSON([]byte) (Entry, error)` that unmarshals, validates, and rejects unsupported schema versions or missing required fields.
- Validating `UnmarshalJSON` for `Entry` and nested metadata value types where direct struct unmarshal would otherwise bypass invariants.
- A documented precondition that `DecodeEntryJSON` accepts already bounded bytes only; #57/#58 repository/API/outbox adapters must enforce byte/depth limits before reading or decoding untrusted input.
- Single-entry validation covers only per-record invariants. Duplicate
  revisions, duplicate event IDs, and duplicate idempotency keys are batch
  invariants checked by `NewHistory` or future batch decode helpers, not hidden
  mutable state in `Entry.Validate`.

- [ ] **Step 5: Verify Task 2**

Run:

```bash
go test -count=1 ./audit -run 'Test(DomainEvent|SnapshotMetadata|ChangeMetadata|Entry)'
```

Expected: PASS.

## Task 3: Add Aggregate Recorder With Peek-Plus-Ack Lifecycle

**Complexity:** high
**Skill:** Apply `$bluetape-go-patterns`.
**Files:**
- Create: `audit/recorder_test.go`
- Create: `audit/recorder_concurrency_test.go`
- Create: `audit/recorder.go`

- [ ] **Step 1: Write failing recorder lifecycle tests**

Add tests covering:

- `NewAggregateRecorder(id)` starts at head `0` and first event revision is `1`.
- `NewAggregateRecorderFromHead(id, 7)` records the next event at revision `8`.
- `PendingEvents` returns immutable copies and does not clear pending events.
- A simulated downstream persistence failure can call `PendingEvents` again and receive the same event IDs/revisions.
- `AckThrough(1)` clears only events at or below revision 1 and keeps later events.
- `AckThrough` rejects zero revision and revisions beyond the current head with `ErrInvalidRevision`.
- `AggregateRecorder.Record(EventRecord)` uses a named input struct with event
  ID, event type, occurred timestamp, idempotency key, metadata, and payload.
  No broad positional `Record` API may accept same-type identity strings or
  timestamps.
- Invalid `Record` inputs, including empty event ID, empty event type, zero
  occurred timestamp, empty idempotency key, invalid metadata, malformed
  payload, and `math.MaxUint64` restored/current head, return an error without
  advancing head or changing pending events. The next valid record must receive
  the original next revision.
- `AckThrough` is idempotent for already-acknowledged positive revisions
  `<= head`; repeated `AckThrough(1)` succeeds without clearing later events,
  and invalid `AckThrough(head+1)` leaves pending unchanged.

Run:

```bash
go test -count=1 ./audit -run 'TestAggregateRecorder'
```

Expected: FAIL because recorder APIs are missing.

- [ ] **Step 2: Implement recorder with narrow lock scope**

Implement:

- `AggregateRecorder` with `mu sync.Mutex`, aggregate ID, head revision, and pending event slice.
- Constructor validation for aggregate ID and restored head.
- `type EventRecord struct` and `Record(EventRecord) (DomainEvent, error)` that validates/copies metadata/payload before locking when possible, locks only to assign revision and append pending event, and returns an immutable event copy.
- `PendingEvents() []DomainEvent` locks only long enough to snapshot the pending event slice/state, releases the mutex, then deep-copies metadata/payload outside the lock before returning.
- `AckThrough(revision Revision) error` clears acknowledged pending events after validation, zeroes acknowledged slots, and compacts/copies survivors into a fresh backing slice so acknowledged large payloads are not retained.

- [ ] **Step 3: Write failing stress/race tests for the public recorder contract**

Use:

```go
import concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
```

with `concurrencytest.NewGoroutineStressTester` to assert:

- concurrent `Record` produces no duplicate or skipped revisions;
- returned events remain immutable;
- repeated `PendingEvents` and `AckThrough` under concurrent record operations do not race or lose acknowledged boundaries;
- concurrent `Record`, `PendingEvents`, and `AckThrough` over non-trivial payloads/backlogs remains race-clean and does not expose acknowledged events through `PendingEvents`.
- bounded stress options use nonzero `Workers`, `RoundsPerTask`, and `Timeout`,
  and tests assert `report.Completed == report.Scheduled`.
- concrete ack interleaving: capture a pending snapshot through revision `N`,
  concurrently record later events, call `AckThrough(N)`, then assert revisions
  `> N` remain pending and all observed revisions are unique and contiguous.

Run:

```bash
go test -count=1 ./audit -run 'TestAggregateRecorder.*Stress|TestAggregateRecorder.*Concurrent'
```

Expected: FAIL until concurrency behavior is implemented correctly.

- [ ] **Step 4: Verify Task 3**

Run:

```bash
go test -count=1 ./audit -run 'TestAggregateRecorder'
go test -race -count=1 ./audit -run 'TestAggregateRecorder'
```

Expected: PASS. `AsyncJobTester` is N/A because #56 has no async or context-cancelable operation.

- [ ] **Step 5: Add recorder allocation benchmarks**

Add benchmarks for:

- `BenchmarkAggregateRecorderRecordSmallPayload`
- `BenchmarkAggregateRecorderRecordLargePayload`
- `BenchmarkAggregateRecorderPendingEventsSmallBacklog`
- `BenchmarkAggregateRecorderPendingEventsLargeBacklog`
- `BenchmarkAggregateRecorderAckThroughCompactsSurvivors`

Run:

```bash
go test -run '^$' -bench 'BenchmarkAggregateRecorder(Record|PendingEvents|AckThrough)' -benchmem ./audit
```

Expected: PASS and capture baseline allocation output. No strict threshold is required for #56.

## Task 4: Add History Reconstruction

**Complexity:** medium
**Skill:** Apply `$bluetape-go-patterns`.
**Files:**
- Create: `audit/history_test.go`
- Create: `audit/history.go`

- [ ] **Step 1: Write failing history tests**

Add tests covering:

- unsorted valid entries are sorted ascending by revision;
- `HeadRevision` returns the newest revision;
- `NewHistory(nil)` and `NewHistory([]Entry{})` reject with an
  `ErrInvalidEntry`-compatible error. Empty histories require a future
  explicit constructor that includes an aggregate ID.
- mixed aggregate IDs reject with `ErrMixedAggregate`;
- zero, duplicate, non-contiguous revisions reject with `ErrRevisionConflict` or `ErrInvalidRevision`;
- histories must start at `InitialRevision()`; `[2]`, `[7]`, `[7,8]`, and
  `[1,3]` reject with `ErrRevisionConflict` because #56 does not introduce a
  partial-history/head-only constructor;
- duplicate event IDs and duplicate idempotency keys for different events reject;
- a recorder created from history head appends at head + 1.

Run:

```bash
go test -count=1 ./audit -run 'TestHistory'
```

Expected: FAIL because history APIs are missing.

- [ ] **Step 2: Implement `History` reconstruction**

Implement:

- `History` with aggregate ID, sorted entries, and head revision.
- `NewHistory(entries []Entry) (History, error)`.
- Immutable `Entries() []Entry`, `AggregateID() AggregateID`, and `HeadRevision() Revision`.
- Sorting and validation without replaying domain state.

- [ ] **Step 3: Verify Task 4**

Run:

```bash
go test -count=1 ./audit -run 'TestHistory'
```

Expected: PASS.

## Task 5: Add Documentation And Examples

**Complexity:** medium
**Skill:** Apply `$bluetape-go-patterns`.
**Files:**
- Create: `audit/doc.go`
- Create: `audit/README.md`
- Create: `audit/README.ko.md`
- Create: `audit/audit_example_test.go`
- Modify: `README.md`
- Modify: `README.ko.md`

- [ ] **Step 1: Write compile-checked examples**

Add examples for:

- creating an aggregate ID and recorder;
- recording events with caller-supplied event/idempotency IDs;
- peeking pending events, simulating durable persistence failure, re-reading the
  same pending events, then acknowledging through revision only after simulated
  durable audit commit succeeds;
- validated JSON decode with direct `json.Unmarshal` rejection for invalid
  records and `DecodeEntryJSON` usage for bounded trusted bytes;
- constructing history from audit entries.

Run:

```bash
go test -count=1 ./audit -run '^Example'
```

Expected: FAIL until APIs/docs examples compile.

- [ ] **Step 2: Add package docs and README pair**

Document:

- #56 scope and non-goals;
- serialization schema version and validated decode;
- snapshot/change metadata shapes;
- secret/PII redaction is caller-owned for `author`, `metadata`, `payload`, `snapshot`, `change`, and `idempotency_key`;
- payload size/depth limits are trust-boundary-owned;
- the package never logs, redacts, encrypts, truncates, hashes, or classifies immutable audit fields;
- redaction must happen before constructing audit values;
- recorder is not durable storage or an outbox;
- failure-then-success recorder handoff where pending events are re-read after
  failed persistence and ack happens only after durable audit commit;
- README headings in both locales: Import, Minimal Usage, Validated Decode,
  Recorder Handoff, Error Handling, Non-goals, and JaVers Migration;
- JaVers expectation table with columns for JaVers concept, Go replacement,
  explicit non-port, later issue, and user action, mapping AggregateRoot,
  DomainEvent metadata, snapshots, repositories, object diffing, shadow
  reconstruction, framework wiring, Kafka history, Redis write-behind, and
  publishers.

- [ ] **Step 3: Update root README pair**

Add `audit` to:

- the root package table near `sqlkit` / planned data/audit packages in both
  `README.md` and `README.ko.md`;
- the grouped Package Documentation section as a new Audit/history bullet or
  the closest data-access group in both locales.

- [ ] **Step 3b: Update release-facing notes**

Add a `CHANGELOG.md` `[Unreleased]` bullet for the new `audit` package. Update
`WIP.md` to the current 0.9.0 audit target state or record an explicit reason
in the Step 6-R artifact if release target text should not change in this PR.

- [ ] **Step 4: Verify Task 5**

Run:

```bash
go test -count=1 ./audit -run '^Example'
rg -n "\\[`audit`\\]" README.md README.ko.md
rg -n "github.com/bluetape4k/bluetape-go/audit|Import|Minimal Usage|Validated Decode|Recorder Handoff|Error Handling|Non-goals|JaVers Migration" audit/README.md
rg -n "github.com/bluetape4k/bluetape-go/audit|가져오기|최소 사용|검증된 Decode|Recorder Handoff|에러 처리|Non-goals|JaVers Migration" audit/README.ko.md
rg -n "audit" CHANGELOG.md WIP.md README.md README.ko.md audit/README.md audit/README.ko.md
```

Expected: PASS and both root READMEs link the package.

## Task 6: Final Verification, Review Artifact, Lesson, And PR Prep

**Complexity:** medium
**Skill:** Apply `$bluetape-go-patterns` and `verification-before-completion`.
**Files:**
- Create: `docs/review/2026-06-27-issue-56-audit-model-review.md`
- Create: `docs/lessons/2026-06-27-issue-56-audit-model.md`

- [ ] **Step 1: Run targeted and repo checks**

Run:

```bash
go test -count=1 ./audit
go test -race -count=1 ./audit
go test -run '^$' -bench 'BenchmarkAggregateRecorder(Record|PendingEvents|AckThrough)' -benchmem ./audit
go test -race -count=1 ./...
go test -count=1 ./...
make fmt-check
make tidy-check
make vet
make lint
make ci
git diff --check
```

Expected: PASS. If repo-wide race, lint, CI, or other checks fail for
pre-existing reasons, capture exact output and keep targeted `./audit`
race/test evidence.

- [ ] **Step 2: Write Step 6-R review artifact**

Write `docs/review/2026-06-27-issue-56-audit-model-review.md` with:

- P0/P1/P2/P3 table across performance, stability, security, operator, developer/API, user/caller, and integration.
- Evidence commands from Step 1.
- Explicit `AsyncJobTester N/A` rationale.
- Baseline commit, branch/base, changed files, and diff summary.
- Release-readiness evidence: `CHANGELOG.md`, `WIP.md` target-state decision,
  `make ci`, and GitHub CI / `gh pr checks` status after PR creation.
- Nightly/Testcontainers status or explicit N/A rationale for #56 model-only
  changes.
- Repository/outbox handoff text existence: source write vs audit commit
  ordering, ack-after-durable rule, rollback/downgrade behavior,
  replay/head owner, observability expectations, and deferred #57/#58
  responsibilities.
- Exported API doc comment checklist: every exported type, const, error,
  constructor, method, and function has an English Go doc comment.
- Any deferred P2/P3 follow-ups.

- [ ] **Step 3: Write lesson**

Write `docs/lessons/2026-06-27-issue-56-audit-model.md` recording:

- audit event handoff must be peek-plus-ack, not destructive pull;
- decoded JSON must validate the same invariants as constructors;
- Go-native audit roots should use stable IDs and caller-owned domain objects rather than JaVers base interfaces.

- [ ] **Step 4: Commit, push, and create PR**

Use the Lore commit protocol. PR body must end with `## DoD Status` and include
Step DoD evidence, validation commands, release-readiness evidence, review
result P0=0/P1=0, and `Closes #56`. After PR creation, run `gh pr checks` and
record the result before asking for merge.

## Rollback / Re-run Points

- If recorder lifecycle tests show ambiguous ack semantics, stop before implementation commit and revise Task 3 plus docs.
- If JSON decode validation requires custom unmarshal recursion that becomes fragile, prefer an explicit `DecodeEntryJSON` helper and document that repository/outbox code must use it.
- If race tests are flaky, reduce stress concurrency but keep `go test -race -count=1 ./audit` mandatory.
