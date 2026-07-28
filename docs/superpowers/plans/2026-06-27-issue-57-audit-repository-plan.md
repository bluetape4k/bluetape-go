# Issue 57 Audit Repository Plan

> 한국어 운영 요약: 이 계획 문서는 사용자 협업용 실행 계획이다. 아래 원문에 포함된 명령, 경로, API 이름, issue/PR 번호, branch 이름, code block, test output은 추적성과 재현성을 위해 그대로 보존한다. 작업 순서, 위험, 검증, 롤백 판단은 한국어 독자가 바로 실행 경계를 이해할 수 있도록 이 메모를 우선 적용한다.
> 추가 한국어 요약: 이 문서의 실행 판단은 기존 순서를 따르며, 변경자는 작업 표와 검증 목록을 먼저 확인한 뒤 관련 테스트를 실행한다. 영어로 남은 항목은 코드 식별자 또는 재현 증거다.\n

## 목표

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

## 작업

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
