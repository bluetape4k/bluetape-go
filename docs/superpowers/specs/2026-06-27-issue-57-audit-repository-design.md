# Issue 57 Audit Repository Design

## Problem

Issue #57 follows the #56 audit model with storage-neutral repository and
history query contracts. The package needs reusable interfaces and an in-memory
implementation that prove ordering, filtering, validation, and conformance
before SQL, Redis, Kafka/NATS, or example-service adapters are designed.

## Evidence

- Issue #57 asks for storage-neutral repository interfaces, in-memory
  implementation, reusable conformance tests, and history queries by aggregate
  ID/type, time/revision, newest/previous snapshots where useful.
- `docs/research/2026-06-25-issue-41-audit-scope.md` says #57 should implement
  repository/history query interfaces with in-memory conformance first and defer
  concrete SQL/Redis/Kafka/NATS stores.
- #56 added `Entry`, `DomainEvent`, `AggregateID`, `Revision`, `History`, and
  validated decode helpers. #57 should build on those values rather than
  re-open the model contract.
- #56 lessons require durable transaction/outbox/reconciliation semantics before
  acknowledging recorded events; #57 can define repository save/query behavior
  but must not claim to solve outbox delivery or source write atomicity.

## Goals

- Add `Repository` and `HistoryReader` contracts to package `audit`.
- Add `Query` values that support aggregate identity, aggregate type,
  revision range, recorded-time range, newest-first ordering, and limit.
- Add snapshot helpers for newest and previous snapshot-bearing entries.
- Add an in-memory repository for tests, examples, and conformance.
- Add reusable conformance tests that later SQL/Redis/file adapters can call.
- Preserve validation, copy/immutability, and `errors.Is` behavior.
- Use `context.Context` on repository methods.

## Non-Goals

- No SQL, Redis, Kafka, NATS, MongoDB, filesystem, or outbox adapter in #57.
- No source-of-truth transaction manager or event publication semantics.
- No JaVers object graph diffing, shadow reconstruction, CDO internals, or
  framework lifecycle hooks.
- No partial history reconstruction that relaxes #56 contiguous-from-initial
  `History` semantics. Query result slices can be partial; `History` remains a
  full contiguous aggregate history.

## Public API

```go
type Repository interface {
    Append(ctx context.Context, entries ...Entry) error
    HistoryReader
}

type HistoryReader interface {
    Find(ctx context.Context, query Query) ([]Entry, error)
    LoadHistory(ctx context.Context, aggregate AggregateID) (History, bool, error)
    Latest(ctx context.Context, aggregate AggregateID) (Entry, bool, error)
    LatestSnapshot(ctx context.Context, aggregate AggregateID) (Entry, bool, error)
    PreviousSnapshot(ctx context.Context, aggregate AggregateID, before Revision) (Entry, bool, error)
}
```

`Append` is an all-or-nothing operation. It validates the whole batch before
mutating storage, rejects mixed aggregates inside one append, rejects duplicate
event IDs and duplicate idempotency keys, rejects non-contiguous revision
continuation for an existing aggregate, and returns `ErrRevisionConflict`,
`ErrMixedAggregate`, or `ErrInvalidEntry` through `errors.Is`.

`Query` should be a value type:

```go
type Query struct {
    Aggregate *AggregateID
    AggregateType string
    FromRevision Revision
    ToRevision Revision
    FromRecordedAt time.Time
    ToRecordedAt time.Time
    NewestFirst bool
    Limit int
}
```

Zero-value `Query` returns all entries ordered by repository append order
oldest-first. `NewestFirst` reverses that append order. A positive `Limit` caps
results after filtering and ordering. `FromRevision`/`ToRevision` are inclusive
when non-zero. Recorded-time bounds are inclusive. Invalid ranges return
`ErrInvalidQuery`.

## In-Memory Implementation

`NewMemoryRepository() *MemoryRepository` provides a goroutine-safe repository.
It stores cloned entries indexed by aggregate and global append order. It must
not retain caller-owned mutable payload/metadata slices or maps, and returned
entries must be defensive copies.

The memory implementation is for tests, examples, and conformance. It is not a
durable audit store and should say so in docs.

## Conformance

Add reusable test helpers in package `audit/audittest` so later adapter
packages can import and run the same contract:

```go
type RepositoryFactory func(testing.TB) audit.Repository
func RunRepositoryConformance(t *testing.T, factory RepositoryFactory)
```

The helper should cover:

- append/load full history;
- query by aggregate, type, revision range, recorded-time range, newest-first,
  and limit;
- latest and snapshot helpers;
- missing aggregate behavior;
- append validation failures are all-or-nothing;
- duplicate event ID/idempotency key conflicts;
- reusable conformance behavior against `MemoryRepository`;
- goroutine stress for concurrent appends and queries.

## Error Handling

Add `ErrInvalidQuery` for invalid query values. Preserve existing sentinels:
`ErrInvalidEntry`, `ErrMixedAggregate`, and `ErrRevisionConflict`. Missing
aggregate/history should be non-exceptional for `Find`, `Latest`, and snapshot
helpers. `LoadHistory` returns `ok=false` when the aggregate has no history and
reserves errors for invalid input, cancellation, or corrupted repository state.

## Docs

Update `audit/README.md` and `audit/README.ko.md` with repository contracts,
memory repository caveat, query examples, and adapter follow-up decisions. Root
README can remain package-level unless package status text needs a repository
mention. Add CHANGELOG/WIP bullets for #57.

## Acceptance Mapping

| Issue #57 requirement | Design coverage |
|---|---|
| Storage-neutral repository interfaces | `Repository`, `HistoryReader`, `Query` |
| In-memory implementation | `MemoryRepository` |
| Query history by aggregate/type/time/revision/newest snapshots | `Find`, `Latest`, `LatestSnapshot`, `PreviousSnapshot` |
| Adapter follow-up decisions | README and WIP document SQL/Redis/Kafka/NATS deferral |
| Tests | Reusable conformance, memory tests, stress/race, query validation |
