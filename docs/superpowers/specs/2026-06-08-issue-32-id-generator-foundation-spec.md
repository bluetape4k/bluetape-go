# Issue 32 ID Generator Foundation Spec

Issue: #32
Milestone: 0.6.0
Included child issues: #164, #165, #167
Deferred child issues: #166

## Problem

0.6.0 needs a Go-native `id` package that ports the useful foundation of
`bluetape4k-idgenerators` without copying Kotlin-shaped inheritance or helper
surfaces. The Kotlin source exposes UUID, ULID, KSUID, Snowflake,
GlobalSnowflake, Flake, and Hashids through one `IdGenerator<T>` family and a
README selection guide. The first Go release must preserve algorithm coverage
awareness and selection guidance while keeping the implementation scope small
enough to review and test.

The previous Snowflake-only direction was too narrow. The 0.6.0 scope is now:

- common API, errors, package docs, and selection guide;
- UUID family foundation (#164);
- ULID family foundation (#165);
- Snowflake numeric ID foundation (#167).

KSUID (#166), Flake, and Hashids remain source-parity candidates, but they are
not 0.6.0 blockers.

## Current Evidence

- GitHub #32 is the parent task for common API and parity tracking.
- GitHub #164 requires UUID v4 and v7, with v6/v1/v5/name-based variants
  implemented or explicitly split/deferred.
- GitHub #165 requires random and monotonic ULID generation, parse/format,
  timestamp extraction, and ordering documentation.
- GitHub #167 requires Snowflake generation, parse/decode, ordering, clock
  rollback errors, machine ID guidance, and optional base36/string rendering.
- `docs/research/2026-06-01-milestone-0.6.0-utilities-research.md` says
  utilities should be independent packages, not a catch-all misc package.
- The Kotlin README provides an algorithm selection guide and documents
  Snowflake bit layout, UUID v7/v4, ULID monotonic strings, KSUID, Flake, and
  Hashids.
- The Kotlin source has concrete implementations and tests under
  `utils/idgenerators/src/main/kotlin` and `src/test/kotlin` for UUID, ULID,
  KSUID, Snowflake, Flake, Hashids, Base62, machine/node IDs, monotonic ULID,
  and Snowflake sequencers.
- `bluetape-go/codec` already has Base62 helpers. Reuse or extend the existing
  `codec` package instead of creating ID-local duplicate encoders.
- `bluetape-go/testing/concurrency` already provides `GoroutineStressTester`
  and `AsyncJobTester` for stress and cancellation-shaped tests.

## Goals

- Add package `id`.
- Define narrow Go-native generator contracts for typed IDs, strings, and
  numeric IDs.
- Add common typed errors for parse failures, invalid options, exhausted
  sequence space, and clock rollback.
- Document zero-value behavior for every public generator type.
- Add a package README selection guide that compares UUID v4, UUID v7, ULID,
  and Snowflake for service use cases.
- Implement UUID v4 and UUID v7.
- Implement random and monotonic ULID.
- Implement Snowflake IDs with deterministic clock tests and machine ID
  guidance.
- Reuse repo-local `codec` and `testing/concurrency` helpers where applicable.
- Keep later KSUID, Flake, and Hashids integration possible without breaking
  the 0.6.0 API.

## Non-Goals

- Do not implement KSUID in 0.6.0.
- Do not implement Flake in 0.6.0.
- Do not implement Hashids in 0.6.0.
- Do not add a centralized ID service, Redis-backed machine ID allocator, or
  distributed coordination layer.
- Do not add context parameters to single local ID generation methods.
- Do not expose Kotlin-style broad inheritance, singleton families, extension
  methods, or `Must*` helpers.
- Do not hide security-sensitive randomness or node identity choices behind
  undocumented defaults.

## API Direction

The public API should be package-level factories plus small generator
interfaces. Names may be refined during implementation, but the shape should
stay close to this:

```go
package id

type Generator[T any] interface {
    Next() (T, error)
}

type StringGenerator interface {
    NextString() (string, error)
}

type Int64Generator interface {
    NextInt64() (int64, error)
}
```

Single ID generation is local CPU/entropy work and should not take
`context.Context`. If a batch helper is added because examples or benchmarks
need it, the batch helper may be context-aware:

```go
func NextN[T any](ctx context.Context, gen Generator[T], count int) ([]T, error)
```

The batch helper is optional. If omitted, the implementation review must record
why there is no cancellation-observable path and mark `AsyncJobTester` coverage
as not applicable for that package.

Public API types should avoid exposing third-party concrete UUID or ULID types
as the stable bluetape-go API unless the plan proves that the dependency type is
the idiomatic Go interoperability surface. Prefer repo-owned factory functions,
interfaces, and string/byte parse boundaries so dependencies can be replaced
without breaking callers.

## Package Layout

Expected package shape:

```text
id/
  doc.go
  errors.go
  generator.go
  uuid.go
  uuid_test.go
  ulid.go
  ulid_test.go
  snowflake.go
  snowflake_test.go
  snowflake_concurrency_test.go
  id_example_test.go
  README.md
  README.ko.md
```

Implementation may split files further if it keeps review clearer, but package
boundaries should stay narrow. Do not add subpackages until the API needs a
real isolation boundary.

## Zero-Value Contract

Prefer unexported concrete generator types returned by constructor/factory
functions. If a concrete generator type is exported, its zero value must be
explicitly documented and tested:

- stateless generators may be zero-value usable only when all options have safe
  documented defaults;
- stateful generators such as Snowflake and monotonic ULID must either be
  zero-value usable with safe defaults or return an `errors.Is`-compatible
  invalid-options error from `Next*`;
- zero-value behavior must never return an ambiguous zero ID with nil error.

Option/config structs may use documented zero-value defaults, but the spec and
README must name those defaults.

## UUID Contract

0.6.0 minimum:

- UUID v4 random generation.
- UUID v7 Unix-epoch sortable generation.
- Parse/format round trips for canonical UUID strings.
- Documentation that UUID v4 is random and not sortable.
- Documentation that UUID v7 is time-sortable and suitable for database
  primary keys when clock behavior is acceptable.

Deferred unless dependency evidence makes them low-risk:

- UUID v6.
- UUID v1, because node/MAC privacy implications must be explicit.
- UUID v5/name-based helpers.
- compact Base62 UUID rendering.

Dependency direction:

- Prefer a maintained Go UUID dependency if it supports the required UUID
  versions and parse/format behavior with less correctness risk than a
  first-party implementation.
- `github.com/google/uuid` is already present indirectly in `go.mod`, but the
  plan must verify whether the current version supports UUID v7 and whether it
  should become a direct dependency.
- If the selected dependency lacks UUID v7, compare `github.com/gofrs/uuid/v5`
  and first-party v7 generation before implementation.

## ULID Contract

0.6.0 minimum:

- random ULID generation;
- monotonic ULID generation within the same millisecond;
- canonical 26-character Crockford Base32 string parse/format;
- timestamp extraction;
- ordering documentation.

Concurrency contract:

- A stateful monotonic generator must either be concurrency-safe or clearly
  documented as caller-synchronized. Prefer concurrency-safe behavior for
  library ergonomics.
- Randomness source must be injectable or otherwise testable.

Dependency direction:

- Prefer a maintained ULID dependency if it avoids encoding and monotonicity
  bugs.
- `github.com/oklog/ulid/v2` is the default candidate from prior research, but
  the plan must verify maintenance, API shape, monotonic entropy behavior, and
  testability before adding it.

## Snowflake Contract

0.6.0 minimum:

- 63-bit non-negative integer IDs with timestamp, machine ID, and sequence
  fields.
- Default bit layout compatible with the Kotlin README: timestamp in
  milliseconds, 10-bit machine ID, 12-bit sequence.
- Caller-owned machine ID assignment with documented valid range.
- Deterministic parsing/decomposition into timestamp, machine ID, and sequence.
- Strict typed clock rollback errors.
- Sequence overflow behavior that is documented and tested.
- Optional base36/string rendering if it fits the common string API without
  creating Snowflake-only naming.

Implementation direction:

- Implement Snowflake first-party. The algorithm is small, and first-party code
  avoids adopting a dependency whose API might force package-wide naming.
- Expose a `Clock` or unexported deterministic clock hook if needed for tests.
- Avoid global mutable singleton state in the public API.
- Do not use unbounded busy-waiting when the per-millisecond sequence is
  exhausted. Return a typed sequence exhaustion error or use an explicitly
  documented bounded wait strategy selected during implementation review.
- Do not implement `GlobalSnowflake` as a distributed service. If a centralized
  generator can be represented as a process-local generator with a documented
  machine ID, include it; otherwise defer it.

## Error Contract

Errors must be caller-visible and compatible with `errors.Is` or `errors.As`
where useful.

Expected sentinel or typed errors:

- invalid options;
- invalid machine ID;
- invalid encoded or parsed ID;
- unsupported algorithm/version;
- clock rollback;
- sequence exhausted.

Errors should wrap causal dependency errors with `%w` when returning external
parse or randomness failures. Do not return ambiguous zero IDs with nil errors.

## Concurrency And Cancellation Contract

- Generators that own mutable state, such as Snowflake sequence state or
  stateful monotonic ULID entropy, must be safe for concurrent `Next*` calls or
  explicitly documented as not safe. 0.6.0 should prefer safe-by-default
  generators.
- Stress tests must use `GoroutineStressTester` for concurrent uniqueness,
  ordering where applicable, and absence of data races.
- `go test -race -count=1 ./id` must pass.
- Cancellation tests with `AsyncJobTester` are required only for
  context-aware batch helpers or other long-running paths. If no such API is
  added, the verifier must record `AsyncJobTester` as N/A with the reason that
  single generation has no caller-observable cancellation boundary.

## Documentation Requirements

- Add `id/doc.go`.
- Add `id/README.md` and `id/README.ko.md`.
- README selection guide must cover:
  - DB primary key with sorting: UUID v7 or Snowflake depending on numeric vs
    UUID storage needs.
  - request/correlation ID: UUID v4 or UUID v7.
  - monotonic string ID: ULID.
  - distributed numeric entity ID: Snowflake.
  - future URL-safe second precision ID: KSUID deferred.
  - future 128-bit sortable byte/string ID: Flake deferred.
  - future short obfuscation: Hashids deferred and not security.
- README examples must mark URL-safe ID and deterministic/name-based ID
  scenarios as supported or deferred. For 0.6.0, URL-safe string IDs are covered
  by ULID, while deterministic/name-based UUID helpers are deferred unless they
  are explicitly implemented.
- README must state that generated IDs are identifiers, not authentication
  tokens or authorization secrets. UUID v4 can use cryptographic randomness,
  but callers must not rely on any package ID as a standalone security boundary.
- README must state that Snowflake exposes approximate creation time and
  machine ID bits by design.
- Root `README.md` and `README.ko.md` package indexes must be promoted from
  planned to active package status when `id` ships.
- Root README release/status text must also be refreshed when it is stale
  against `CHANGELOG.md` or `WIP.md`; do not add `id` to a stale package index
  without fixing adjacent release-state drift in the same documentation pass.
- Release notes must update `CHANGELOG.md` and `WIP.md` according to
  `docs/release/release-guide.md`.
- Each implemented family documents length, sortability, entropy source,
  collision scope, ordering guarantees, and parse/format behavior.
- Examples must be compile-checked and cover realistic entity IDs, request IDs,
  and monotonic string IDs.

## Test Requirements

- Unit tests cover successful generation, parse/format round trips, invalid
  parse input, invalid options, and zero-value behavior.
- UUID tests cover v4 non-ordering documentation examples and v7 timestamp
  ordering where dependency hooks allow deterministic checks.
- ULID tests cover random generation, same-millisecond monotonic ordering,
  canonical string validation, invalid Crockford input, and timestamp
  extraction.
- Snowflake tests cover bit layout, machine ID validation, parse/decode,
  ordering, sequence rollover/exhaustion, and clock rollback.
- Stress tests use `GoroutineStressTester` for UUID/ULID/Snowflake concurrent
  uniqueness or state safety.
- Race validation runs `go test -race -count=1 ./id`.
- Benchmark smoke tests cover Snowflake `NextInt64`, UUID v4/v7 generation,
  random ULID generation, and monotonic ULID generation. Run at least
  `go test -run '^$' -bench . -benchmem ./id` before release readiness.
- Dependency tests must avoid probabilistic assertions that can flake. Use
  deterministic entropy/clock injection wherever possible.

## Risks And Mitigations

| Risk | Mitigation |
|---|---|
| API becomes Kotlin-shaped | Keep small Go interfaces and factories; no broad family singleton surface. |
| One algorithm forces inconsistent naming | Design common generator vocabulary before family implementation. |
| UUID/ULID dependency lacks needed version or hooks | Verify dependency support in plan before adding direct dependencies. |
| Snowflake clock rollback silently creates bad IDs | Return typed rollback errors and test deterministic rollback. |
| Monotonic ULID or Snowflake state races | Use mutex/atomic design, `GoroutineStressTester`, and race validation. |
| Tests rely on chance collisions or wall clock timing | Inject entropy/clock where practical; keep wall-clock assertions bounded. |
| Hot-path allocation or lock regression goes unnoticed | Add package benchmarks with `-benchmem` smoke validation for implemented generators. |
| Base62 helper duplication | Reuse or extend `codec` instead of adding ID-local encoding. |
| Deferred algorithms get forgotten | README and issue references keep KSUID, Flake, and Hashids as explicit follow-ups. |

## Acceptance Criteria

- `id` package spec and plan are reviewed with P0=0 and P1=0 before
  implementation.
- Common API, errors, package docs, and README selection guide are implemented.
- UUID v4 and UUID v7 are implemented.
- Random and monotonic ULID are implemented.
- Snowflake generation, parse/decode, rollback errors, and machine ID guidance
  are implemented.
- KSUID, Flake, and Hashids are explicitly deferred and not required for 0.6.0
  closure.
- Unit, stress, race, benchmark smoke, README, release-note, and example
  validations pass for `./id`.
- Step 6-R code review reaches `P0=0 P1=0`.
