# Issue #222 Testing Examples

Issue: #222
Parent Epic: #221
Date: 2026-06-24

## Decision

Add focused compile-checked examples to the existing `testing` package instead
of adding an assertion DSL, faker dependency, or JUnit-style parameter-source
API.

This follows #214's research decision: Go-native table tests, ordinary
subtests, explicit builders, package-local `testdata`, seeded `math/rand/v2`,
`cmp.Diff`, and existing cancellation helpers cover the useful developer
experience without another test framework.

## Implemented Scope

- `testing/patterns_example_test.go`
  - table-driven tests with named subtests;
  - package-local domain builder;
  - `cmp.Diff` structured comparison;
  - golden-file check from `testing/testdata`;
  - generated temp output via `TempOutputPath`;
  - deterministic seeded random data;
  - cancellation assertion examples with `RequireContextCanceled` and
    `RequireCleanupOnCancel`;
  - compile-checked example function.
- `testing/testdata/order.golden.json`
  - canonical expected fixture.
- `testing/README.md` and `testing/README.ko.md`
  - explain the focused patterns and non-goals.
- root README pair
  - points test-support readers to the focused examples.

## Rejected

- General assertion DSL: duplicates standard `testing`, `cmp.Diff`, and
  existing helpers.
- Faker dependency: #214 did not find a concrete consumer that justifies random
  realistic data over explicit builders.
- JUnit-style parameter source: Go table literals remain clearer and
  type-checked.

## Validation

- PASS `go test -count=1 ./testing`
- PASS `go test -race -count=1 ./testing`
- PASS `make fmt-check`
- PASS `make vet`
- PASS `golangci-lint cache clean && make lint`
- PASS `git diff --check`
- PASS staged `make tidy-check`
- PASS `make test`
- PASS `make race`
