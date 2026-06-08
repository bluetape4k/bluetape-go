# Issue 32 ID Generator Foundation Plan

Issue: #32
Milestone: 0.6.0
Spec: `docs/superpowers/specs/2026-06-08-issue-32-id-generator-foundation-spec.md`
Spec review: `docs/superpowers/reviews/2026-06-08-issue-32-id-generator-foundation-spec-review.md`
Included child issues: #164, #165, #167
Deferred child issue: #166

## Execution Boundary

Implement only the 0.6.0 `id` package foundation:

- common Go-native generator contracts;
- UUID v4 and UUID v7;
- random and monotonic ULID;
- Snowflake numeric IDs;
- package docs, README pair, root README package promotion, WIP/CHANGELOG notes;
- tests, stress/race coverage, benchmark smoke, and examples.

Do not implement KSUID, Flake, Hashids, a centralized ID service, Redis-backed
machine ID allocation, or a distributed coordination layer in this issue.

Apply `$bluetape-go-patterns` to every Go API, Go test, example, README, and
review task. Keep public APIs narrow, typed-error compatible, and independent
from Kotlin-shaped family singletons.

## Current Evidence

- `github.com/google/uuid v1.6.0` is already present as an indirect dependency
  and local module source contains `NewV7`, `NewV7FromReader`, `NewRandom`,
  `NewRandomFromReader`, `UUID`, and `Parse`.
- `go list -m -versions github.com/oklog/ulid/v2` reports latest stable
  `v2.1.1`.
- `codec` already has Base62 helpers; ID code must not add duplicate Base62
  encoding.
- `testing/concurrency` provides `GoroutineStressTester` and `AsyncJobTester`.
- Root `README.md` and `README.ko.md` release-state text is stale relative to
  `CHANGELOG.md` and `WIP.md`; the id documentation pass must fix adjacent
  release-state drift when promoting the package index.

## Task Plan

| Task | Complexity | Expected files | Actions | Verification |
|---|---|---|---|---|
| T0 - Pre-implementation dependency and API decision | high | `docs/superpowers/reviews/2026-06-08-issue-32-id-generator-preimplementation-risk.md`, `go.mod`, `go.sum` if dependencies change | Apply `$bluetape-go-patterns`. Record direct dependency decisions before source code: make `github.com/google/uuid v1.6.0` direct if used for UUID v4/v7; evaluate `github.com/oklog/ulid/v2 v2.1.1` for ULID maintenance signal, parse/format API shape, monotonic entropy behavior, deterministic testability hooks, and fallback/rejection rationale. Require UUID/ULID random defaults to use `crypto/rand` or a dependency-proven crypto entropy default; deterministic readers are test hooks only. Decide public return types: prefer repo-owned factories and string/byte parse boundaries; expose dependency concrete types only if the risk note proves they are idiomatic interoperability surfaces. | Risk artifact records adopt/defer decision, version, maintenance signal, entropy default, deterministic hook boundary, public API exposure decision, and fallback. `go list -m -versions github.com/google/uuid github.com/oklog/ulid/v2`; `rg -n "github.com/google/uuid|github.com/oklog/ulid" go.mod go.sum`. |
| T1 - Package scaffold and common contracts | medium | `id/doc.go`, `id/errors.go`, `id/generator.go`, `id/errors_test.go` | Define `Generator[T]`, `StringGenerator`, `Int64Generator`, typed/sentinel errors, option validation helpers, and documented zero-value policy. Prefer unexported concrete generator types returned by constructors. No `Must*` helpers. Add common error-contract tests for `errors.Is`/`errors.As`, invalid options, invalid encoded IDs, unsupported algorithm/version, and zero-value failure behavior. | `go test -count=1 ./id` compiles after tests exist; common tests prove `errors.Is`/`errors.As` and zero-value behavior for shared contracts. |
| T2 - UUID v4/v7 implementation | high | `id/uuid.go`, `id/uuid_test.go`, `go.mod`, `go.sum` | Add UUID v4 and v7 generation using the selected dependency or first-party fallback. Add parse/format wrappers without leaking dependency types unless T0 approves. Use crypto-grade default entropy; support deterministic/random reader hooks only for tests. Defer v6/v1/v5/name-based/Base62 with explicit docs. Document and, where deterministic hooks allow, test wall-clock rollback behavior for UUID v7: ordering may degrade, generation must not hang, and uniqueness must rely on entropy/state rather than monotonic clock assumptions. | Unit tests for v4 generation, v7 generation, canonical parse/format, invalid parse, dependency parse error wrapping, deterministic-reader UUID v4/v7 cases, failing-reader entropy errors with `%w`, no zero UUID with nil error on entropy failure, v4 non-sortability docs, v7 ordering and rollback behavior where deterministic hooks allow. |
| T3 - ULID random and monotonic implementation | high | `id/ulid.go`, `id/ulid_test.go`, `go.mod`, `go.sum` | Add random ULID and monotonic/stateful monotonic generator with concurrency-safe state or explicit caller-synchronized contract. Prefer concurrency-safe default. Provide canonical string parse/format and timestamp extraction. Use crypto-grade default entropy; keep deterministic entropy source injectable only for tests. Document and, where deterministic hooks allow, test wall-clock rollback behavior: ordering may degrade, generation must not hang, and uniqueness must rely on entropy/state rather than monotonic clock assumptions. | Unit tests for random generation, canonical 26-char round trip, invalid Crockford input, parse error wrapping, deterministic-reader random/monotonic ULID cases, failing-reader entropy errors with `%w`, no zero ULID with nil error on entropy failure, timestamp extraction, same-millisecond monotonic ordering, rollback behavior where deterministic hooks allow, zero-value behavior. |
| T4 - Snowflake implementation | high | `id/snowflake.go`, `id/snowflake_test.go` | First-party Snowflake with 63-bit non-negative IDs, millisecond timestamp, 10-bit caller-provided machine ID, 12-bit sequence, typed rollback and sequence-exhausted errors, parse/decode helpers, no global public singleton. Snowflake uniqueness is scoped to unique caller-assigned machine IDs per live generator/process/deployment; no allocator is provided; duplicate machine IDs across concurrent processes or same-millisecond restarts can duplicate IDs. Do not auto-discover MAC/env/host identity or random process-local machine IDs unless a later review explicitly approves it. Use deterministic clock hooks for tests. Avoid unbounded busy-waiting. | Unit tests for bit layout, machine ID range validation, invalid machine IDs fail closed, decode exposes configured machine ID, parse/decode, invalid parse/decode for negative/non-63-bit IDs and malformed string/base36 input if string rendering is implemented, ordering, sequence rollover/exhaustion, rollback, zero-value behavior, and `errors.Is`/`errors.As`. |
| T5 - Stress, race, and cancellation applicability tests | high | `id/id_concurrency_test.go`, package tests, `docs/superpowers/reviews/2026-06-08-issue-32-id-generator-concurrency-notes.md` if cancellation is N/A | Use `GoroutineStressTester` for concurrent uniqueness/state safety across UUID, ULID, and Snowflake. Use `AsyncJobTester` only if T1 adds a context-aware batch helper; otherwise record the exact N/A rationale in T5's own concurrency notes: "AsyncJobTester N/A: single generation has no caller-observable cancellation boundary." | Branch A if a context-aware batch helper exists: named cancellation test using `AsyncJobTester`. Branch B if omitted: `rg -n "AsyncJobTester N/A: single generation has no caller-observable cancellation boundary" docs/superpowers/reviews/2026-06-08-issue-32-id-generator-concurrency-notes.md`. Always run `go test -race -count=1 ./id`. |
| T6 - Benchmarks and allocation smoke | medium | `id/id_benchmark_test.go` | Add `Benchmark*` coverage for Snowflake `NextInt64`, UUID v4/v7 generation, random ULID, and monotonic ULID. Include `b.RunParallel` benchmarks for stateful hot paths: Snowflake `NextInt64` and monotonic ULID generation. Keep benchmarks package-local, not a new benchmark module. | `go test -run '^$' -bench . -benchmem ./id`. |
| T7 - Examples and package documentation | medium | `id/id_example_test.go`, `id/README.md`, `id/README.ko.md`, `id/doc.go` | Add compile-checked examples for entity IDs, request/correlation IDs, monotonic string IDs, and Snowflake parse/decode. README pair includes selection guide, security caveats, Snowflake metadata exposure, caller-provided machine ID range, operator contract that machine IDs must be unique per live generator/process/deployment, duplicate-machine-ID collision risk, no allocator, no automatic MAC/env/host identity discovery, UUID v7/ULID wall-clock rollback ordering caveats, zero-value policy, URL-safe support/defer docs, deterministic/name-based defer docs, and `KSUID (#166)`, Flake, and Hashids deferred notes. State IDs are not authentication tokens, authorization secrets, or a standalone security boundary. | `go test -count=1 ./id -run Example`; `rg -n "UUID v7|ULID|Snowflake|authentication|authorization|secret|standalone security boundary|unique.*machine ID|duplicate.*machine ID|wall-clock rollback|KSUID.*#166|Flake|Hashids|zero-value|name-based" id/README.md id/README.ko.md id/doc.go`. |
| T8 - Root docs and release notes | medium | `README.md`, `README.ko.md`, `CHANGELOG.md`, `WIP.md` | Promote `id` from planned to active package in root README pair. Refresh adjacent stale release/current-status text against `CHANGELOG.md` and `WIP.md`. Add unreleased or milestone notes for `id`. Keep English/Korean package lists synchronized. | `rg -n "id|0.5.1|0.6.0|v0.5.1|v0.5.0" README.md README.ko.md CHANGELOG.md WIP.md`; manual check of package table order. |
| T9 - Targeted validation | medium | changed files | Run formatter and targeted validations before review. Keep Testcontainers out of scope. Run repo configured local CI with `make ci`; if local CI or full `go test ./...` is environment-blocked, record the exact command, package, and error, then keep targeted `./id` evidence primary with a fallback rationale. | `gofmt -w id`; `go test -count=1 ./id`; `go test -race -count=1 ./id`; `go test -run '^$' -bench . -benchmem ./id`; `go test -count=1 ./...`; `make ci`; `git diff --check`. |
| T10 - Verifier and Step 6-R subagent 7-Tier code review | high | `docs/superpowers/reviews/2026-06-08-issue-32-id-generator-verifier.md`, `docs/superpowers/reviews/2026-06-08-issue-32-id-generator-code-review.md` | Read Step 6-R references. Verify spec/plan acceptance mapping and run subagent-based 7-Tier code review. Use at least the baseline code-reviewer/verifier plus risk lanes for security, SRE, performance, and library-user docs. P0/P1 block PR work and require affected-tier reruns. | Review artifacts record subagent lanes, P0/P1/P2/P3 counts, fixes, reruns, and final `P0=0 P1=0`. |
| T11 - Commit, PR, PR metadata, PR review, CI, and DoD | medium | git/GitHub state, PR body | Commit with Lore trailers after validation/review passes. Push branch and create PR with `Fixes #32`, child issue references, and deferred `KSUID (#166)` note. Set PR assignee, milestone, and labels to match #32. Verify live PR body ends with `## DoD Status`, compare issue #32 metadata against PR metadata, run Step 7-R subagent PR review, check CI, and do not merge without user request. | `git status --short`; `gh issue view 32 --json assignees,labels,milestone,title,state`; `gh pr view --json body,assignees,labels,milestone,url,state`; metadata comparison recorded in DoD/PR review artifact; `gh pr checks`; final Step DoD table. |

## Acceptance Mapping

| Spec acceptance | Plan coverage |
|---|---|
| Spec and plan reviewed with P0=0 and P1=0 before implementation. | Step 2-R artifact; Step 3-R review after this plan; T10 for code review. |
| Common API, errors, package docs, and README selection guide. | T1, T7 |
| UUID v4 and UUID v7 implemented. | T2, T5, T6, T7 |
| Random and monotonic ULID implemented. | T3, T5, T6, T7 |
| Snowflake generation, parse/decode, rollback errors, and machine ID guidance. | T4, T5, T6, T7 |
| KSUID, Flake, and Hashids deferred outside 0.6.0 closure. | T7, T8 |
| Unit, stress, race, benchmark smoke, README, release-note, and example validations. | T5, T6, T7, T8, T9 |
| Step 6-R code review reaches P0=0 P1=0. | T10 |

## Ordering And Recheck Points

1. Commit spec, spec review, plan, and plan review before implementation.
2. Run T0 before adding source files; do not let dependency concrete types leak
   into public API without the T0 decision.
3. Implement common contracts before family-specific generators.
4. Implement UUID and ULID before Snowflake docs, so README selection examples
   can use stable names.
5. Add family unit tests before stress/race tests; stress tests should assert
   stable semantics, not discover them.
6. Add benchmark smoke before Step 6-R so performance reviewers have evidence.
7. Refresh root README release-state text in the same pass that promotes `id`.
8. Run targeted `./id` tests and race before full `./...`.
9. Run Step 6-R only after code, tests, examples, README pair, root docs, and
   release notes are present.

## Risk Controls

| Risk | Control |
|---|---|
| Dependency type becomes stable API accidentally | T0 explicitly decides public return/parse boundary before implementation. |
| Random ID entropy defaults are weak or untestable | T0, T2, and T3 require crypto-grade default entropy, test-only deterministic readers, and failing-reader wrapping tests. |
| UUID v7 dependency support is weaker than assumed | T0 checks local module source and fallback candidates before T2. |
| ULID monotonic state races | T3 requires concurrency-safe default or explicit caller synchronization; T5 race/stress proves it. |
| Snowflake sequence exhaustion busy-waits | T4 requires typed exhaustion or reviewed bounded wait; T6 benchmark smoke catches hot-path regression. |
| Snowflake machine ID is inferred from host identity | T4 and T7 require caller-provided machine IDs and forbid MAC/env/host auto-discovery in this issue. |
| Snowflake duplicate IDs from machine ID reuse | T4 and T7 require the operator contract: unique caller-assigned machine IDs per live generator/process/deployment; no allocator is provided and duplicate machine IDs are documented as collision risk. |
| UUID v7 or ULID ordering assumptions fail on wall-clock rollback | T2, T3, and T7 require rollback behavior tests/docs where deterministic hooks allow: no hang, uniqueness not dependent on monotonic clocks, ordering caveat explicit. |
| Zero-value behavior is ambiguous | T1 and family tests require documented zero-value behavior and no zero-ID nil success. |
| ID examples imply auth/security | T7 requires explicit non-auth-token and Snowflake metadata exposure caveats. |
| Deferred KSUID/Flake/Hashids are lost | T7/T8 README and issue references keep them visible. |
| Release docs drift further | T8 updates root README pair, CHANGELOG, and WIP together. |

## Validation Commands

```bash
gofmt -w id
go test -count=1 ./id
go test -race -count=1 ./id
go test -run '^$' -bench . -benchmem ./id
go test -count=1 ./...
make ci
rg -n "GoroutineStressTester|AsyncJobTester N/A: single generation has no caller-observable cancellation boundary" id docs/superpowers/reviews
rg -n "UUID v7|ULID|Snowflake|authentication|authorization|secret|standalone security boundary|unique.*machine ID|duplicate.*machine ID|wall-clock rollback|KSUID.*#166|Flake|Hashids|zero-value|name-based" id/README.md id/README.ko.md id/doc.go
rg -n "id|0.5.1|0.6.0|v0.5.1|v0.5.0" README.md README.ko.md CHANGELOG.md WIP.md
git diff --check
```

## Step 3 Checklist Completion Report

| Item | Status | Notes |
|------|--------|-------|
| Plan path confirmed inside feature worktree | Done | `docs/superpowers/plans/2026-06-08-issue-32-id-generator-foundation-plan.md`. |
| All tasks have complexity labels | Done | T0-T11 include labels. |
| `$bluetape-go-patterns` applied to code-bearing tasks | Done | Execution boundary and T0-T11 require Go API/test/docs/review checks. |
| Plan code/test snippets conform to `$bluetape-go-patterns` | Done | No implementation snippets beyond API/command shape; context/error/concurrency rules are explicit. |
| Thread/coroutine safety helpers assigned | Done | Go scope uses `GoroutineStressTester`; `AsyncJobTester` is conditional/N/A unless a context-aware batch helper is added. Kotlin helpers are N/A. |
| Tests and verification tasks included | Done | T1-T6, T9, T10. |
| Multilingual README, English contributor docs, and AGENTS.md tasks included | Done | T7/T8 cover README pairs and release docs; AGENTS.md has no planned impact. |
| Risky ordering/dependency assumptions explicit | Done | T0 and ordering/recheck points. |
| Spec + plan committed before implementation | Pending | Commit after Step 3-R passes. |
