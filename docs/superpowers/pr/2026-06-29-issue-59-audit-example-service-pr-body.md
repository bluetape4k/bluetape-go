Closes #59.

## Summary

- Add a runnable `examples/audit` order-service recipe that records aggregate command changes through the `audit.Repository` boundary.
- Demonstrate audit history queries, in-memory source-state lookup, and history replay into a small in-memory outbox fixture.
- Add a README diagram and beginner-oriented explanation for the source-state, audit-history, and outbox boundaries.
- Document that the example is not full event sourcing or a JaVers-style diff engine, and point durable delivery use cases at `audit/sqloutbox`.

## Validation

- RED: `go test -count=1 ./examples/audit` failed on the missing example-service APIs before implementation.
- `go test -count=1 ./examples/audit ./audit ./audit/audittest ./audit/sqloutbox`
- `go test -race -count=1 ./examples/audit`
- `go vet ./examples/audit ./audit ./audit/audittest ./audit/sqloutbox`
- `make ci`
- `python3 -c "import xml.etree.ElementTree as ET; ET.parse('docs/images/readme-diagrams/audit-example-service-flow.svg')"`
- `~/.local/bin/cairosvg docs/images/readme-diagrams/audit-example-service-flow.svg -o docs/images/readme-diagrams/audit-example-service-flow.png -s 2`
- rendered PNG inspection and marker-color audit
- `git diff --check`

## DoD Status

- [x] Tests-first coverage exists for history queries, repository-boundary rollback, concurrent commands, and cancellation-aware outbox replay.
- [x] `GoroutineStressTester` and `AsyncJobTester` are used in the example tests.
- [x] Public example behavior is documented in English and Korean README files with a source-backed SVG/PNG diagram.
- [x] Lesson, spec, plan, and review artifacts were added for the issue.
