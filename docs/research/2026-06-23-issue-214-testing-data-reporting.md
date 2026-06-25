# Issue #214 testing data and reporting research

Date: 2026-06-23
Milestone: 0.6.4
Issue: #214
Parent: #209

## Decision

Do not add a faker, random-data, parameter-source, or Mermaid reporting
dependency in 0.6.4.

Keep the Go testing surface centered on table-driven tests, fuzz tests,
compile-checked examples, fixtures, golden files, and small deterministic data
builders. Defer any faker dependency to #222, where focused assertion test-data
and fixture examples can prove a concrete consumer.

## Acceptance Criteria Coverage

| Requirement | Decision |
|---|---|
| Compare standard table tests, fuzzing, fixtures, and small data builders before dependencies. | Prefer standard table tests, `testing.F` fuzz targets, package fixtures, examples, and deterministic builders. |
| Faker/random dependency recommendation includes maintenance, license, determinism, and CI notes. | No dependency now. Candidate packages are documented below with maintenance/license/determinism risks for later review. |
| Reporting recommendation explains `go test -json` fit. | Keep `go test -json` as the machine-readable source; do not generate Mermaid as a library feature. |
| Include `testing/assertions`, random/faker support, parameter sources, mock web servers, Spring/Ktor test data patterns. | Assertions/test-data examples route to #222; HTTP/mock servers route to #219/#224; Spring/Ktor-style fixtures become typed Go builders, not reflection injection. |
| Decide later package needs. | Database/audit/graph/AWS/golden needs should start with deterministic builders and checked-in fixtures; randomized text/token data remains opt-in research only. |

## Go Baseline

- Table-driven tests already dominate the repository and fit parameter-source
  needs without a helper API.
- Go fuzz targets (`func FuzzXxx(f *testing.F)`) are the right shape for parser,
  codec, wildcard, and boundary-input coverage when examples or tables are too
  narrow.
- Compile-checked examples already provide caller-facing documentation and
  exact-output verification.
- `go test -json` emits the same information as verbose test output in a
  machine-readable format. Downstream reporting should consume that stream
  rather than adding a custom test runner.
- `testing.T.TempDir`, `testing.T.Setenv`, and the scoped helpers from #212 are
  enough for temp output, env restoration, and golden-file write targets.

## Candidate Dependency Snapshot

Live metadata was collected with `gh repo view` and `go list -m -versions` on
2026-06-23.

| Package | License | Activity / version signal | Determinism and CI notes | Decision |
|---|---|---:|---|---|
| `github.com/brianvoe/gofakeit/v7` | MIT | Active repo; latest observed module line `v7.15.0`; zero dependencies; supports seeded faker instances and custom random sources. | Best candidate if a future package needs broad realistic text/domain data. Use local `Faker` instances only; avoid global seeding in parallel tests. | Defer to #222. |
| `github.com/go-faker/faker/v4` | MIT | Active repo; latest observed module line `v4.8.0`; struct-tag oriented. | Reflection/tag surface is broader than current needs, can panic on unsupported/private fields, and makes fixtures less explicit. | Reject for 0.6.4. |
| `github.com/jaswdr/faker/v2` | MIT | Active repo; latest observed module line `v2.9.1`; zero-dependency faker-style API. | Useful for ad hoc realistic values but includes APIs that can create image/temp files and broad random behavior; determinism must be wrapped carefully. | Defer. |
| `github.com/Pallinder/go-randomdata` | MIT | Older release line `v1.2.0`; last push observed in 2023. | Small API, but weaker maintenance signal and less explicit determinism than local builders. | Reject. |

## Parameter Sources

JUnit-style field-source and parameter-source APIs should not be ported as a
generic Go helper. In Go, table literals are simpler, type-checked, easy to name,
and compose with subtests.

Recommended pattern:

```go
tests := []struct {
	name string
	in   string
	want string
}{
	{name: "empty", in: "", want: ""},
	{name: "trim", in: " value ", want: "value"},
}

for _, tt := range tests {
	t.Run(tt.name, func(t *testing.T) {
		// exercise behavior
	})
}
```

Generics helpers are acceptable only when they remove repeated assertion loops
inside one package. Do not add a public `testing` parameter-source API until at
least three packages repeat the same typed pattern and table literals are no
longer clear.

## Test Data Builders

Add examples before dependencies. The #222 follow-up should start with local,
deterministic builders for:

- database/audit/graph domain fixtures with stable IDs, timestamps, and small
  readable names;
- randomized text/token data only when a fuzz target or parser boundary needs
  more input variety;
- AWS payload fixtures as checked-in JSON or typed builders before emulator
  integration;
- golden-file helpers that build paths with #212 `TempOutputPath` but keep
  canonical expected files in package-local `testdata`.

## Mock Web Servers and Spring/Ktor Patterns

Spring/Ktor test data patterns map to Go as explicit builders, `httptest.Server`
fixtures, and package-local client/server helpers. Do not add a WireMock-style
general dependency in 0.6.4.

HTTP mock and fault-injection work already belongs to #219 and integration
recipe documentation belongs to #224. Those issues should decide whether a
shared mock server wrapper is justified by real package consumers.

## Reporting

Keep reporting outside the public `testing` helper API:

- Use `go test -json` for structured event streams.
- Store coverage and package summaries through existing CI/doc tooling.
- Mermaid/timeline output can be an external script or docs artifact if a future
  issue proves value, but it should not become a library dependency or test
  runner.

Reporting helpers that transform `go test -json` must preserve package, test,
  action, elapsed time, output, and failure text. A diagram-only view is
  insufficient as merge evidence.

## Follow-Up Mapping

| Need | Follow-up |
|---|---|
| Focused assertion test-data and deterministic fixture examples | #222 |
| HTTP mock and fault-injection Testcontainers wrappers | #219 |
| End-to-end integration recipes for corrected 0.6.x packages | #224 |
| Generic parameter-source public API | Non-goal until repeated need is proven. |
| Mermaid/timeline reporting library | Non-goal; use `go test -json` consumers if needed. |

## Sources

- #214 issue requirements.
- `docs/research/2026-06-21-issue-202-source-parity-matrix.md`.
- Go tool help for `go test -json` and fuzz/example test functions.
- `https://github.com/brianvoe/gofakeit`
- `https://github.com/go-faker/faker`
- `https://github.com/jaswdr/faker`
- `https://github.com/Pallinder/go-randomdata`
