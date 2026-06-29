# Issue 57 Audit Repository Plan

## Goal

Add storage-neutral audit repository/history query contracts, in-memory
implementation, reusable conformance tests, and documentation for issue #57.

## Files

- Create `audit/repository.go`: interfaces, `Query`, query validation,
  `ErrInvalidQuery`.
- Create `audit/memory_repository.go`: goroutine-safe in-memory repository.
- Create `audit/audittest/conformance.go`: reusable conformance helper for
  this package and later adapter packages.
- Create `audit/memory_repository_test.go`: memory repository coverage and
  conformance invocation.
- Modify `audit/README.md`, `audit/README.ko.md`, `CHANGELOG.md`, `WIP.md`.
- Add Step 6-R review, lesson, and PR body artifacts.

## Tasks

### Task 1: Query and Interface Contract

1. Write failing tests for zero-value query ordering, invalid revision/time/limit
   ranges, aggregate type trimming, and context cancellation.
2. Implement `ErrInvalidQuery`, `Query.Validate`, `Repository`, and
   `HistoryReader`.
3. Verify:

```bash
go test -count=1 ./audit -run 'TestQuery|TestRepositoryInterface'
```

### Task 2: Memory Repository Append Semantics

1. Write failing tests for append/load full history, append all-or-nothing,
   non-contiguous revision continuation, duplicate event IDs, duplicate
   idempotency keys, mixed aggregate batches, and defensive copies.
2. Implement `MemoryRepository.Append` and `LoadHistory(ctx, aggregate) (History, bool, error)`.
3. Verify:

```bash
go test -count=1 ./audit -run 'TestMemoryRepository(Append|Load|Conflict|Copies)'
```

### Task 3: History Query Semantics

1. Write failing tests for `Find` by aggregate, aggregate type, revision range,
   recorded-time range, newest-first, limit, missing aggregate, and combined
   filters.
2. Implement filtering and stable append-order ordering.
3. Verify:

```bash
go test -count=1 ./audit -run 'TestMemoryRepositoryFind'
```

### Task 4: Latest and Snapshot Helpers

1. Write failing tests for `Latest`, `LatestSnapshot`, `PreviousSnapshot`, and
   no-snapshot/no-entry behavior.
2. Implement helper methods on memory repository through the query engine.
3. Verify:

```bash
go test -count=1 ./audit -run 'TestMemoryRepository(Latest|Snapshot)'
```

### Task 5: Reusable Conformance and Stress

1. Add `audittest.RunRepositoryConformance(t, factory)` covering append, query,
   snapshots, missing aggregates, conflict behavior, and defensive copies.
2. Run the helper against `NewMemoryRepository`.
3. Add `GoroutineStressTester` coverage for concurrent append/query operations.
   `AsyncJobTester` is N/A unless a cancellable async helper is introduced.
4. Verify:

```bash
go test -count=1 ./audit -run 'TestRepositoryConformance|TestMemoryRepositoryConcurrent'
go test -race -count=1 ./audit ./audit/audittest
```

### Task 6: Docs, Release Notes, Review, PR

1. Update README pair with repository API, memory repository caveat, query
   examples, conformance helper, and adapter deferrals.
2. Update CHANGELOG/WIP for #57.
3. Run final verification:

```bash
go test -count=1 ./audit
go test -race -count=1 ./audit ./audit/audittest
make lint
make ci
git diff --check
```

4. Run Step 6-R 7-tier review and fix all P0/P1.
5. Commit, push, open PR with `Closes #57`, issue labels/milestone/assignee, and
   final `## DoD Status`.

## Risk Controls

- Keep `History` full-contiguous-from-initial; use `[]Entry` for partial query
  results.
- Validate a full append batch before mutating memory repository state.
- Return defensive copies from every read path.
- Keep in-memory repository explicitly non-durable.
- Defer SQL/Redis/Kafka/NATS adapters and outbox semantics to later issues.
