# Issue #59 Plan: Audit Example Service

## Steps

1. Add failing tests for a runnable order-service audit example.
2. Implement `examples/audit` service and in-memory outbox fixture.
3. Document usage and boundaries in English/Korean package READMEs.
4. Link the example from root READMEs and changelog.
5. Run targeted tests, race tests, vet/lint, and local CI.
6. Record review, lesson, and PR body artifacts.

## Design Decisions

- Keep code under `examples/audit`; do not promote a new production package API.
- Inject `audit.Repository` so the example proves the repository boundary.
- Use `audit.MemoryRepository` and `MemoryOutbox` fixtures to stay service-free.
- Mention `audit/sqloutbox.Store.Enqueue` as the production durable outbox path,
  but do not require PostgreSQL for this example.

## Risk Controls

- Use TDD RED before implementation.
- Use `GoroutineStressTester` and `AsyncJobTester` as required by #59.
- Run the race detector for the example package.
