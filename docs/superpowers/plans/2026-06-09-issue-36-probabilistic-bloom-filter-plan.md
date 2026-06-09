# Issue #36 Probabilistic Bloom Filter Plan

Spec: `docs/superpowers/specs/2026-06-09-issue-36-probabilistic-bloom-filter-spec.md`
Issue: #36
Branch: `issue-36-probabilistic`

## Step 1-R Research Summary

- Kotlin `utils/probabilistic` current source is in-memory Bloom only.
- Kotlin `infra/lettuce` Redis Bloom/Cuckoo/HLL exists but is tracked by #182.
- Use first-party Go code and standard library only.
- Use explicit `Hasher[T]` for generic values instead of reflection or
  serialization. Hasher compatibility uses an explicit comparable key because
  Go function values cannot be compared for identity.
- Strengthen Go behavior beyond Kotlin by making the filter goroutine-safe with
  `sync.RWMutex`, source snapshotting for `PutAll`, self-merge no-op handling,
  and stress + race tests covering `Put`, `MightContain`, `PutAll`, `Clear`,
  and metadata reads.

## Step 2-R Spec Review Plan

Review the spec against the 7-Tier frame:

1. Security: no unbounded untrusted parsing, no serialization, no external
   input trust boundary beyond caller-owned hasher bytes.
2. Ops/SRE: predictable memory from config, no goroutines/IO/resources.
3. Structural: package boundary is `probabilistic`, Redis is #182.
4. Go quality: concrete type, explicit errors, no Kotlin-shaped DSL.
5. Tests/types: config, behavior, merge, false-positive, stress/race.
6. Performance: fixed hash count, SHA-256 tradeoff, mutex scope.
7. Docs/release: README bilingual, WIP/CHANGELOG/root README, #182 deferral.

Exit: integrated findings show `P0=0 P1=0`.

## Step 3-R Plan Review Plan

Review this plan before implementation:

- Verify all source parity requirements are mapped to files/tests.
- Verify stress/race and `AsyncJobTester` N/A rationale are explicit.
- Verify release docs and PR metadata tasks are present.
- Verify no Redis work is accidentally included in #36.

Exit: integrated findings show `P0=0 P1=0`.

## Step 4-T Implementation Tasks

### T1. Package skeleton and errors

Files:

- `probabilistic/doc.go`
- `probabilistic/errors.go`
- `probabilistic/config.go`
- `probabilistic/hasher.go`

Tasks:

- Define package docs and sentinel errors.
- Implement `Config`, defaults, validation, bit-size/hash-count math, expected
  FPP, and approximate count helpers.
- Implement `Hasher[T]` with `NewHasher(key, sum)` validation,
  `ErrNilHasher`, and `ErrEmptyHasherKey`.
- Keep overflow and unsupported bitset checks deterministic.

Validation:

- `go test -count=1 ./probabilistic -run Config`

### T2. Hashing and Bloom filter implementation

Files:

- `probabilistic/hash.go`
- `probabilistic/bloom_filter.go`

Tasks:

- Implement stable string and byte-slice hashers with built-in compatibility
  keys.
- Implement SHA-256 double hashing index calculation.
- Implement exported `BloomFilter[T]` interface backed by an unexported concrete
  type so callers cannot construct an invalid zero-value filter.
- Implement `Put`, `MightContain`, `PutAll`, `Clear`, metadata reads, FPP, and
  approximate count.
- Implement compatibility checks including config and hasher key.
- Implement `PutAll` source snapshot under the source read lock, target OR
  merge under the target write lock, and self-merge no-op behavior.

Validation:

- `go test -count=1 ./probabilistic -run 'Bloom|Hash|Merge'`

### T3. Tests and examples

Files:

- `probabilistic/config_test.go`
- `probabilistic/bloom_filter_test.go`
- `probabilistic/bloom_filter_concurrency_test.go`
- `probabilistic/bloom_filter_example_test.go`

Tasks:

- Add table-driven config validation tests.
- Add behavior, clear, merge, incompatible merge, nil, and FPP tests.
- Add stress test using `GoroutineStressTester` with
  `max(32, runtime.GOMAXPROCS(0)*4)` workers and 512 rounds.
- Add stress/race cases for concurrent `Put`, `MightContain`, `PutAll`,
  reciprocal merge, self-merge, `Clear`, and metadata reads. Define the
  no-false-negative guarantee as applying to successful inserts not followed by
  a completed `Clear`.
- Use deterministic FPP corpus sizes: insert 10,000 values into a 1% filter and
  query 20,000 disjoint missing values with an upper bound of 3%.
- Add examples for construction, membership, merge, and introspection.
- Record `AsyncJobTester` N/A in a review note because no context/IO boundary
  exists. The note must contain the exact phrase `AsyncJobTester: N/A` and a
  verification command must grep for it.
- Add small opt-in benchmarks for `Put`, `MightContain`, and `PutAll` so future
  performance work has a baseline without turning benchmarks into a CI gate.

Validation:

- `go test -count=1 ./probabilistic`
- `go test -race -count=1 ./probabilistic`

### T4. Documentation and release notes

Files:

- `probabilistic/README.md`
- `probabilistic/README.ko.md`
- `README.md`
- `README.ko.md`
- `CHANGELOG.md`
- `WIP.md`
- `docs/superpowers/reviews/2026-06-09-issue-36-probabilistic-concurrency-notes.md`
- `docs/superpowers/reviews/2026-06-09-issue-36-probabilistic-testlog.md`

Tasks:

- Document false-positive/no-false-negative/deletion boundaries.
- Document goroutine-safe contract and stress/race validation.
- Document Redis Bloom/Cuckoo/HLL deferral to #182.
- Add root README package table entries and portable utility links.
- Add CHANGELOG `[Unreleased]` item.
- Update WIP so #35 is delivered and #36 closes 0.6.0 implementation.
- Record validation commands and results.
- Verify package documentation policy:
  - `doc.go` purpose and Kotlin compatibility note;
  - README concurrency/context behavior;
  - README error semantics;
  - exported comments start with exported identifiers.
- Verify `AsyncJobTester: N/A` with:
  - `rg -n "AsyncJobTester: N/A" docs/superpowers/reviews/2026-06-09-issue-36-probabilistic-concurrency-notes.md`

Validation:

- `git diff --check`
- README link/path spot checks with `rg`.

### T5. Full local gate

Commands:

- `go test -count=1 ./probabilistic`
- `go test -race -count=1 ./probabilistic`
- `make ci`
- `git diff --check`

Exit: all pass or any failure has a fixed rerun.

## Step 5 Verifier Checklist

- API compiles and examples are runnable.
- Error sentinels work with `errors.Is`.
- Stress test runs in normal `go test`.
- Race detector covers the stress path.
- `PutAll` lock/snapshot behavior is covered by stress and race tests.
- `Clear` concurrency semantics are documented and tested.
- No Redis code is added under #36.
- No `go.mod` dependency is added.
- Docs mention #182 deferral.

## Step 6-R 7-Tier Code Review

Use subagents where available, then integrate:

1. Security.
2. Ops/SRE reliability.
3. Structural impact.
4. Go code quality/API.
5. Tests/types/silent failure.
6. Performance/stability.
7. Docs/release/evidence integrity.

Mandatory evidence:

- reviewed scope and base/head commit;
- P0/P1/P2/P3 table;
- commands run;
- `P0=0 P1=0` before PR.

## Step 7 PR

- Push `issue-36-probabilistic`.
- Create PR against `develop`.
- PR title: `Add probabilistic Bloom filter`
- PR must close #36.
- Set metadata to match #36:
  - assignee: `debop`;
  - milestone: `0.6.0`;
  - labels: `type: task`, `priority: p1`, `area: utilities`.
- PR body must end with a Step DoD table.
- PR body must include `Fixes #36`.
- PR body final heading must be exactly `## DoD Status`.
- Verify PR metadata and CI with:
  - `gh pr view <pr> --json assignees,labels,milestone,closingIssuesReferences`
  - `gh pr checks <pr>`

## Step 8 Merge and Local Sync

After GitHub CI passes:

- merge PR;
- fetch and fast-forward root `develop`;
- remove issue worktree and local branch;
- verify clean root status.

## Step 9 0.6.0 Release Follow-Through

After #36 merge:

- confirm 0.6.0 open issues are only epic #6;
- update/close epic #6 with closure comment;
- close milestone `0.6.0`;
- prepare changelog `## [v0.6.0] - <actual release date>`;
- validate release preflight per `bluetape4k-publish-go`;
- run exact release preflight commands:
  - `git status --short --branch`
  - `git fetch --prune origin main develop --tags`
  - `git log --oneline origin/main..origin/develop`
  - `gh issue list --milestone 0.6.0 --state open`
  - `gh pr list --state open`
  - `git tag --list "v0.6.0"`
  - `git ls-remote --tags origin "refs/tags/v0.6.0*"`
  - `gh release view v0.6.0`
  - `rg -n "## \\[v0.6.0\\]" CHANGELOG.md`
- create release PR `develop -> main`;
- wait CI and merge;
- tag `v0.6.0` on `main`;
- push tag and create GitHub Release;
- local sync and cleanup.
