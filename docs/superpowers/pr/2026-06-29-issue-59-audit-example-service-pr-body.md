Closes #59.

## Summary

- Add a runnable `examples/audit` order-service recipe that records aggregate command changes through the `audit.Repository` boundary.
- Demonstrate audit history queries, in-memory source-state lookup, and history replay into a small in-memory outbox fixture.
- Document that the example is not full event sourcing or a JaVers-style diff engine, and point durable delivery use cases at `audit/sqloutbox`.

## Validation

- RED: `go test -count=1 ./examples/audit` failed on the missing example-service APIs before implementation.
- `go test -count=1 ./examples/audit ./audit ./audit/audittest ./audit/sqloutbox`
- `go test -race -count=1 ./examples/audit`
- `go vet ./examples/audit ./audit ./audit/audittest ./audit/sqloutbox`
- `make ci`
- `git diff --check`

## DoD Status

- [x] Tests-first coverage exists for history queries, repository-boundary rollback, concurrent commands, and cancellation-aware outbox replay.
- [x] `GoroutineStressTester` and `AsyncJobTester` are used in the example tests.
- [x] Public example behavior is documented in English and Korean README files.
- [x] Lesson, spec, plan, and review artifacts were added for the issue.
