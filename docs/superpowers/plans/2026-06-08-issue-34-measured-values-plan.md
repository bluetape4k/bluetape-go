# Issue #34 Measured Values Plan

Issue: #34
Milestone: 0.6.0
Spec: `docs/superpowers/specs/2026-06-08-issue-34-measured-values-spec.md`
Package: `measure`

## Scope

Implement a first-party Go package for typed measured values and units. The
package must cover the source family breadth named by #34 while staying
Go-native: generic `Unit[D]`/`Measure[D]`, immutable built-in unit variables,
family-scoped parsing, deterministic formatting, temperature as affine values,
and typed compound helpers.

No new direct dependency is planned. `github.com/docker/go-units` is already an
indirect dependency through infrastructure tooling, but #34 must not import it.

## Task Plan

| Task | Risk | Files | Work | Tests / Evidence |
|---|---|---|---|---|
| T0 - Preimplementation source/API check | medium | docs review artifact | Re-read current `bluetape4k-projects/utils/measured` source files and current Go repo patterns. Run CodeGraph context on package/readme/test conventions before code. Record fallback decisions: first-party, no direct external units dependency, package `measure`, no context API, and explicit `github.com/docker/go-units` rejection evidence. | `docs/superpowers/reviews/2026-06-08-issue-34-measured-preimplementation-risk.md`; `go list -deps ./measure | rg 'github.com/docker/go-units'` expects no match after implementation; `git diff --check`. |
| T1 - Core errors and immutable unit model | high | `measure/errors.go`, `measure/unit.go`, `measure/unit_test.go` | Add sentinel errors and typed wrappers. Implement `Unit[D]`, `UnitOption`, `NewUnit`, `MustUnit`, immutable fields, suffix spacing, finite positive ratio validation, zero-value invalid behavior, `Name`, `Suffix`, `Ratio`, and amount formatting helper. | Tests for blank name/suffix, zero/negative/NaN/Inf ratio, zero-value unit methods, `errors.Is`, suffix spacing, and built-in immutable value behavior. |
| T2 - Measure core operations | high | `measure/measure.go`, `measure/measure_test.go` | Implement `Measure[D]`, `New`, `Must`, `Amount`, `Unit`, `In`, `As`, `Add`, `Sub`, `MulScalar`, `DivScalar`, `Neg`, `Compare`, `ToNearest`, `Format`, `String`, and mandatory tolerance helpers for tests/docs. Reject NaN/Inf. Return typed errors for invalid zero-value measure/unit and divide by zero. Keep `String()` no-panic and best-effort with a stable invalid marker for zero-value measures. | Tests for conversions, same-family arithmetic, scalar operations, compare, nearest, finite validation, divide by zero, invalid nearest zero/negative/NaN/Inf, zero-value methods returning typed errors, `String()` invalid marker, absolute/relative tolerance helpers, and no ambiguous zero/nil success. |
| T3 - Built-in simple unit families | high | `measure/units.go`, family tests | Add dimension markers and built-in package variables/constructors for length, time, mass, area, volume, storage, binary size, frequency, energy, power, pressure, angle, graphics length, velocity, and acceleration. Model velocity and acceleration as ratio-dimension aliases, not separate incompatible marker types. Keep names unambiguous (`Meter`, `Meters`, `Kibibyte`, `Kilobytes10` where needed). | Table tests for every family ratio listed in spec, constructor units, negative/zero/large values, storage-vs-binary-size distinction, canonical base-unit conversion per family for future DB adapters, large storage/binary tolerance expectations, and `Div(Length, Time)` interop with velocity helpers. |
| T4 - Registry and parsing | high | `measure/registry.go`, `measure/parse.go`, `measure/parse_test.go` | Add immutable `Registry[D]`, `NewRegistry`, `MustRegistry`, copy-on-build suffix lookup, duplicate suffix validation, generic `Parse`, and family parser functions. Reject zero-value registries, malformed number, missing suffix, unknown suffix, NaN/Inf, and unsupported compound expressions beyond built-in `m/s`, `km/hr`, and `m/s^2` family parsers. | Success/failure parse tables for every parser: length, time, mass, area, volume, storage, binary size, frequency, energy, power, pressure, angle, graphics length, temperature, temperature delta, velocity, and acceleration. Include whitespace, unknown suffix, malformed number, NaN/Inf, duplicate suffix, zero-value registry, `errors.Is(ErrInvalidParse)`, and suffix metadata checks. |
| T5 - Human formatting | medium | `measure/format.go`, `measure/format_test.go` | Implement deterministic numeric renderer: round to 9 fractional digits, trim trailing zeroes, keep one fractional digit. Implement family `Human` helpers, explicit `Format`, angle normalization, suffix spacing, and README-aligned output. | Formatting tables for below-1, exactly-1, large, negative, angle degree spacing/normalization, binary size SI/IEC, storage, pressure, and rounding. |
| T6 - Temperature and delta | high | `measure/temperature.go`, `measure/temperature_test.go` | Implement `Temperature`, `TemperatureDelta`, and `TemperatureUnit` as affine concrete value types outside `Measure[D]`. Add Kelvin/Celsius/Fahrenheit constructors, conversions, add/sub/delta, compare, and formatting. Preserve source behavior for numeric negative Kelvin unless plan review changes it. | Tests for absolute K/C/F conversions, F delta `5/9`, delta formatting, absolute-minus-absolute, add/sub delta, comparison, negative Kelvin behavior, and parse/format. |
| T7 - Compound helpers | high | `measure/compound.go`, `measure/compound_test.go` | Implement generic `ProductUnit`, `RatioUnit`, `InverseUnit`, `Mul`, `Div`, and specialized helpers for area, volume, velocity, length-from-velocity-time, power-from-energy-time, and energy-from-power-time. Keep returned known units stable. | Tests for generic product/ratio/inverse unit construction, generic `Mul`/`Div`, zero/invalid unit inputs, typed error wrapping with `errors.Is`, length*length -> area, area*length -> volume, volume inverse helpers, length/time -> velocity, velocity*time -> length, power*time -> energy, energy/time -> power, acceleration formatting/conversion, divide-by-zero. |
| T8 - Stress, race, examples | high | `measure/measure_concurrency_test.go`, `measure/measure_example_test.go` | Add `GoroutineStressTester` over shared built-in units/registries and `AsyncJobTester` harness-only test that respects test context before local operations. Add compile-checked examples for conversion, parsing, temperature, velocity, and energy. | `go test -race -count=1 ./measure`; `go test -count=1 ./measure -run Example`; assertions that stress jobs cover conversion/parse/format/arithmetic/compound operations with no failure or panic; evidence that `AsyncJobTester` jobs respect context before local work; split grep checks for `GoroutineStressTester` and `AsyncJobTester`. |
| T9 - Docs and release metadata | medium | `measure/README.md`, `measure/README.ko.md`, `measure/doc.go`, root README pair, `CHANGELOG.md`, `WIP.md` | Add package docs and README coverage table. Update root package index and 0.6.0 status. Document non-goals: DB adapter, decimal/money exactness, locale formatting, arbitrary expression parser. | README/source-name grep, `git diff --check`, package examples. |
| T10 - Verification and Step 6-R review | high | docs review artifacts | Run validation commands, create verifier/concurrency notes if needed, run Step 6-R 7-Tier review with subagents when available or local fallback if thread-limited, fix P0/P1 and rerun affected lanes. | `go test -count=1 ./measure`, `go test -race -count=1 ./measure`, `go test -count=1 ./...`, `golangci-lint config verify`, `golangci-lint run ./measure`, `make ci`, Step 6-R artifact `P0=0 P1=0`. |
| T11 - PR and post-PR gate | medium | PR body, PR review artifact | Commit with Lore trailers, push branch, create PR with `Fixes #34`, mirror issue assignee/labels/milestone, verify PR body ends with `## DoD Status`, run Step 7-R PR review, fix P0/P1, wait for CI. | PR #, GitHub CI pass, PR review/comment evidence, metadata match. |

## Implementation Notes

- Keep files under one `measure/` package; no subpackages in #34.
- Use only standard library packages: `errors`, `fmt`, `math`, `strconv`,
  `strings`, `time` where needed. Do not import `github.com/docker/go-units`.
- Public Go comments must be Korean and start with the exported identifier.
- Built-in units are package variables initialized from `MustUnit`; callers can
  create custom units with `NewUnit`, but built-in parsers only include built-in
  registries unless the caller passes a custom registry.
- `time.Duration` helpers should be explicit, e.g. `Duration(d time.Duration)
  Measure[Time]` and `ToDuration(m Measure[Time]) (time.Duration, error)`.
- Keep `Byte` naming out of the public API if it reads as predeclared `byte`;
  prefer `StorageByte()`, `BinaryByte()`, or similarly unambiguous names if review
  finds collision risk during implementation.

## Validation Commands

Run after implementation and after any Step 6-R P0/P1 fix:

```bash
gofmt -w measure
go test -count=1 ./measure
go test -race -count=1 ./measure
go test -count=1 ./measure -run Example
go test -count=1 ./...
golangci-lint config verify
golangci-lint run ./measure
git diff --check origin/develop --
rg -n "GoroutineStressTester" measure docs/superpowers/reviews
rg -n "AsyncJobTester" measure docs/superpowers/reviews
rg -n "Length|Time|Mass|Temperature|Storage|BinarySize|Frequency|Energy|Power|Velocity|Acceleration|Area|Volume|Pressure|Angle" measure/README.md measure/README.ko.md measure/doc.go
go list -deps ./measure | rg 'github.com/docker/go-units' && exit 1 || true
make ci
```

## Plan Risks

| Risk | Severity | Mitigation |
|---|---|---|
| Generic phantom dimension API becomes too complex for callers. | P1 | Keep constructors/family helpers prominent and document generic helpers as lower-level. Library-user review must check README examples. |
| Unit name collisions (`Byte`, `Time`, `Energy`) reduce Go usability. | P2 | Use unambiguous exported unit names if implementation review flags stutter or predeclared-name confusion. |
| Float formatting churn causes brittle tests/docs. | P1 | T5 fixes the exact renderer before broad family tests. |
| AsyncJobTester is artificial for local CPU-only work. | P2 | Keep it as a harness-only test without production API; Step 6-R may accept N/A only with explicit reviewer approval. |
| All family coverage is broad. | P1 | Implement common core first, then table-driven family constants. If a family is split, create a follow-up issue before PR and preserve API compatibility. |

## Step 3 Checklist Completion Report

| Item | Status | Notes |
|---|---|---|
| Every spec requirement maps to a task | Done | T1-T11 map core, families, parsing, formatting, temperature, compound units, stress, docs, review, PR. |
| Task ordering is implementable | Done | Core errors/units precede measure ops, families, parsing, formatting, special types, and compound helpers. |
| Tests and validation are concrete | Done | Each implementation task has tests and final commands. |
| Public docs and release metadata assigned | Done | T9 covers package/root README pairs, CHANGELOG, WIP, package docs. |
