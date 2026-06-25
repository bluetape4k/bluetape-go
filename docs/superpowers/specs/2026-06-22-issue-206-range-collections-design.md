# Issue #206 Range and Collection Primitives Design

Issue: [#206](https://github.com/bluetape4k/bluetape-go/issues/206)  
Parent Epic: [#204](https://github.com/bluetape4k/bluetape-go/issues/204)  
Milestone: `0.6.3`  
Date: 2026-06-22

## Goal

Add a small Go-native subset of bluetape4k range and collection primitives:

- ordered range values with closed/open boundary semantics;
- bounded stack and ring buffer containers;
- a pagination value type;
- lazy permutation iteration where it remains a compact `iter.Seq` helper.

The goal is selective foundation parity, not Kotlin/JVM API mirroring.

## Current Evidence

- #206 asks for open-open, closed-open, open-closed, and closed-closed range
  semantics, plus `BoundedStack`, `RingBuffer`, `PaginatedList`, and lazy
  permutation iteration when the API remains small.
- `docs/research/2026-06-21-issue-202-source-parity-matrix.md` marks range
  semantics as missing and collections as partial. It explicitly excludes
  Kotlin operator overload semantics, DSL constructors, and Java/Kotlin
  collection extension parity.
- `docs/research/2026-06-01-issue-8-core-support-inventory.md` and
  `docs/research/2026-06-01-issue-9-collections-inventory.md` defer ranges,
  bounded stack, ring buffer, pagination, and permutations until a separate API
  decision exists.
- Current `core` has `Clamp`, `RequireInRange`, and `RequireInOpenRange`, but
  no public range value type.
- Current `collections` has slice/map transforms only. It preserves nil slice
  inputs as nil results and empty non-nil inputs as empty non-nil results.
- The Kotlin source has `io.bluetape4k.ranges.Range` plus four boundary
  implementations, `BoundedStack`, `RingBuffer`, `PaginatedList`, and a large
  lazy `Permutation` hierarchy.
- `go doc iter.Seq` confirms Go's standard iterator shape is
  `func(yield func(V) bool)`, which is enough for a small lazy permutation
  helper without a custom stream/list abstraction.
- No current bluetape-go or bluetape-go-workshop `data`, `graph`, or
  `rule-engine` package consumes pagination/range primitives yet. This branch
  should therefore avoid broad framework-shaped APIs.

Retrieval note: context-mode had no stored #206-specific decision. GNO returned
the issue and parity docs despite a local `node-llama-cpp` Metal warning.

## Brainstorming Options

### Option 1: Minimal Range Helpers Only

Add only constructors and `Contains` checks for ordered ranges, leaving all
collections deferred.

Pros: smallest surface and lowest risk.  
Cons: does not satisfy #206's collection acceptance criteria and leaves the
known #9/#202 collection gap unresolved.

### Option 2: Small Value Types and Iterators

Add a compact range value in `core`, small mutable-but-not-goroutine-safe
containers in `collections`, a simple page value, and an `iter.Seq` permutation
generator.

Pros: satisfies #206, keeps Go API narrow, uses stdlib `cmp` and `iter`, and
avoids new dependencies.  
Cons: introduces several public symbols, so docs, examples, and tests must be
precise about edge cases and goroutine-safety.

### Option 3: Broad Kotlin Parity Port

Port the Kotlin class hierarchy, conversion helpers, stream adapters, sequence
operations, and thread-safe locking behavior.

Pros: closest source parity on paper.  
Cons: Kotlin-shaped, too broad for Go, duplicates standard library behavior,
and conflicts with the parity matrix non-goals.

## Chosen Approach

Use Option 2.

The implementation will add only the primitives that are useful as reusable
foundation helpers:

1. `core.Range[T cmp.Ordered]` value with unexported endpoints and boundary
   flags plus read-only accessors.
2. Constructors for the four boundary combinations.
3. Range methods for `Contains`, `ContainsRange`, `Overlaps`, `Empty`, and
   string formatting.
4. `collections.BoundedStack[T]` with fixed capacity, push/pop/peek/list
   helpers, and explicit non-goroutine-safe documentation.
5. `collections.RingBuffer[T]` with fixed capacity, add/get/list/clear/drop
   helpers, and explicit non-goroutine-safe documentation.
6. `collections.Page[T]` and `PageOf` for 0-based pagination metadata and
   immutable content snapshots.
7. `collections.Permutations[T]` returning `iter.Seq[[]T]`, generating
   permutations lazily and stopping when the caller stops iteration.

No new dependency is needed.

## API Design

### Ranges

`core.Range[T cmp.Ordered]` should be a value type with unexported fields so
callers cannot bypass constructor validation:

```go
type Range[T cmp.Ordered] struct {
    lower T
    upper T
    lowerInclusive bool
    upperInclusive bool
}
```

Factories:

- `ClosedRange(lower, upper T) (Range[T], error)` for `[lower, upper]`
- `ClosedOpenRange(lower, upper T) (Range[T], error)` for `[lower, upper)`
- `OpenClosedRange(lower, upper T) (Range[T], error)` for `(lower, upper]`
- `OpenOpenRange(lower, upper T) (Range[T], error)` for `(lower, upper)`

Invalid ranges return errors instead of panicking. For open/open and half-open
ranges, equal endpoints are invalid because the value set is empty. For closed
ranges, `lower <= upper` is valid.

Methods:

- `Lower() T`
- `Upper() T`
- `LowerInclusive() bool`
- `UpperInclusive() bool`
- `Contains(value T) bool`
- `ContainsRange(other Range[T]) bool`
- `Overlaps(other Range[T]) bool`
- `Empty() bool`
- `String() string`

`float32` and `float64` are covered by `cmp.Ordered`, but NaN endpoints must be
rejected by constructors because ordinary ordered comparisons with NaN always
return false and would make membership behavior misleading.

The zero value of `Range[T]` should be safe to call and behave as an empty
open-open range. Public docs should direct callers to the constructors for any
non-empty range.

Empty ranges contain no values, never overlap, and `ContainsRange` returns
false when either the receiver or argument is empty.

### Bounded Stack

`collections.NewBoundedStack[T](capacity int) (*BoundedStack[T], error)` creates
a stack that keeps only the most recent `capacity` values. Pushing beyond
capacity drops the oldest bottom element. Index 0 is the top.

Required methods:

- `Capacity() int`
- `Len() int`
- `Empty() bool`
- `Push(value T)`
- `PushAll(values ...T)`
- `Pop() (T, bool)`
- `Peek() (T, bool)`
- `At(index int) (T, bool)`
- `Values() []T` returning top-to-bottom snapshot
- `Clear()`

The stack is not goroutine-safe. This avoids a locking contract that callers do
not yet need. If future consumers need shared-state containers, add separate
concurrency-safe variants with stress and race evidence.

### Ring Buffer

`collections.NewRingBuffer[T](capacity int) (*RingBuffer[T], error)` creates a
FIFO-oriented fixed-capacity ring. Adding beyond capacity overwrites the oldest
element. Index 0 is the oldest retained element.

Required methods:

- `Capacity() int`
- `Len() int`
- `Empty() bool`
- `Add(value T)`
- `AddAll(values ...T)`
- `At(index int) (T, bool)`
- `Values() []T` returning oldest-to-newest snapshot
- `Drop(n int) error`
- `Clear()`

The ring buffer is not goroutine-safe and does not claim FIFO fairness or
blocking queue behavior.

### Page

Use `Page[T]`, not `PaginatedList`, because Go callers usually prefer concise
value names. Fields stay unexported so `PageOf` can preserve the snapshot
contract.

```go
type Page[T any] struct {
    items []T
    page int
    size int
    total int64
}
```

`PageOf(items []T, page, size int, total int64) (Page[T], error)` validates
`page >= 0`, `size > 0`, `total >= 0`, and offset arithmetic that fits in
`int64`. It snapshots `items` so callers cannot mutate stored page contents
accidentally. Methods:

- `Items() []T` returning a fresh snapshot while preserving nil versus empty
  input shape
- `PageNumber() int`
- `PageSize() int`
- `TotalItems() int64`
- `TotalPages() int64`
- `Offset() int64`
- `HasNext() bool`
- `HasPrevious() bool`

### Permutations

`collections.Permutations[T](values []T) iter.Seq[[]T]` returns lazy
permutations. Each yielded permutation must be a fresh slice snapshot. The
source input must be copied once when `Permutations` is called so later caller
mutations do not affect the sequence, even when the caller mutates the source
before ranging over the returned iterator.

Docs must call out factorial result growth and recommend early-stop iteration
for large inputs.

This deliberately excludes the Kotlin lazy `Permutation` list/stream hierarchy,
infinite numeric sequences, Java stream adapters, sorting/filtering wrappers,
and broad sequence DSL operations.

## Error and Edge Contracts

- Constructor validation errors are plain errors for now, matching current
  `core` and `collections` helper style. Do not add broad validation sentinels
  unless a caller matching need appears.
- Nil input to `Permutations` should behave like an empty input and yield one
  empty permutation, matching common combinatorics convention and making the
  iterator total.
- `PageOf(nil, ...)` should make `Items() == nil`; empty non-nil input should
  return an empty non-nil snapshot from `Items()`.
- `TotalPages()` and `Offset()` must avoid intermediate arithmetic overflow.
- Stack/ring constructors reject non-positive capacity.
- `Pop`, `Peek`, and `At` return `(zero, false)` instead of panicking.
- `Drop(n)` rejects negative `n`, treats `n == 0` as a no-op, and clears the
  ring when `n >= Len()`.

## Documentation and Examples

Update:

- `core/README.md`
- `core/README.ko.md`
- `collections/README.md`
- `collections/README.ko.md`
- package examples under `core/*_example_test.go` and
  `collections/*_example_test.go`

Docs must state:

- range boundary notation and invalid range behavior;
- stack order is top-to-bottom;
- ring order is oldest-to-newest;
- page numbering is 0-based;
- collection containers are not goroutine-safe;
- collection and page snapshots are shallow slice snapshots, not deep copies of
  pointed-to values;
- permutation result count grows factorially;
- broad Kotlin/JVM collection parity is excluded.

## Test Strategy

Use table-driven tests and examples:

- range constructor validity, `Contains`, `ContainsRange`, `Overlaps`, `Empty`,
  string formatting, boundary equality, zero-value behavior, and invalid range
  errors;
- stack capacity validation, overflow behavior, pop/peek empty behavior,
  snapshot order, `At`, clear, nil/empty value handling;
- ring capacity validation, overwrite behavior, `Drop`, `At`, snapshot order,
  clear, and empty behavior;
- page validation, total page calculation, offset, next/previous flags, and
  input snapshot immutability;
- permutations for empty, single, three-element, duplicate input, caller stop
  after first yield, and snapshot immutability.

Required validation commands:

```bash
go test -count=1 ./core ./collections
go test -race -count=1 ./core ./collections
go test ./...
git diff --check
make ci
```

Race validation is required as a repository gate, but no shared-state stress
test is required because this design explicitly avoids goroutine-safety claims
for the new mutable containers.

## Risks and Mitigations

| Risk | Mitigation |
|---|---|
| Public API grows into a Kotlin-shaped utility bag. | Keep only the listed symbols; record exclusions in README and review. |
| Mutable containers are mistaken as goroutine-safe. | Go doc comments, README behavior notes, and review checklist must state they are not goroutine-safe. |
| Permutation helper allocates too much or ignores early stop. | Use lazy `iter.Seq`, copy input once, yield per-result snapshots, and test early stop. |
| Permutation helper is misused for large inputs. | Document factorial growth and keep no materializing all-permutations helper. |
| Pagination semantics conflict with future repository APIs. | Use 0-based page numbering from source issue, document it, keep the type value-only, and reject offset overflow. |
| Float range NaN behavior surprises callers. | Reject NaN endpoints in constructors and add float edge tests. |

## Non-Goals

- Kotlin operator overloads and DSL constructors.
- Java Stream, Eclipse Collections, primitive array, or JVM adapter parity.
- Thread-safe stack/ring implementations.
- Blocking queue behavior, fairness, or goroutine coordination.
- Infinite permutation/list abstractions.
- New dependencies.
- New `data`, `graph`, or `rule-engine` packages.

## Acceptance Criteria

- API design records the excluded Kotlin/JVM concepts.
- New primitives have table-driven tests, examples, and README coverage.
- Shared-state variants avoid goroutine-safety claims; race gate still passes.
- `go test -count=1 ./core ./collections`, `go test -race -count=1 ./core ./collections`,
  `go test ./...`, `git diff --check`, and `make ci` pass or any blocker is
  recorded with exact command evidence.

## Step DoD

| Step | Action | Expected DoD |
|---|---|---|
| Step 2 | Write this spec from current issue, docs, source, and standard-library evidence. | Spec exists in the feature worktree and self-review finds no placeholders or contradiction. |
| Step 2-R | Run 7-tier spec review. | Six perspective lanes plus main integration, `P0=0 P1=0`. |
| Step 3 | Write implementation plan. | Plan maps each API to RED/GREEN tasks, docs, examples, and validation commands. |
| Step 3-R | Run 7-tier plan review. | `P0=0 P1=0`; plan order and coverage are verified. |
| Step 4 | Implement via TDD. | Tests prove behavior before/with production code; docs and examples compile. |
| Step 4-T | Run tests. | Targeted tests, race gate, full tests, and `make ci` evidence recorded. |
| Step 5 | Verify against spec and plan. | Verifier artifact says implementation satisfies acceptance criteria. |
| Step 6-R | Run 7-tier code review. | `P0=0 P1=0`; review artifact saved. |
| Step 7+ | Lessons, PR, PR review, CI, final DoD. | Lessons commit before PR, PR body ends with `## DoD Status`, Step 7-R and CI pass. |
