# Issue #206 Range and Collection Primitives Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans or equivalent checklist execution. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Go-native range and collection primitives requested by #206 without porting Kotlin/JVM API shapes.

**Architecture:** Put ordered range values in `core`, because existing validation and numeric helpers already live there. Put bounded stack, ring buffer, page, and lazy permutations in `collections`, preserving current package conventions: generic helpers, plain errors, nil-vs-empty slice contracts, no new dependencies, and no goroutine-safety claims for mutable containers.

**Tech Stack:** Go 1.26.x, standard library `cmp`, `iter`, `math`, existing `go test`, `make ci`, `git diff --check`.

---

## API Decision

Use small first-party value/container APIs:

- `core.Range[T cmp.Ordered]` with unexported endpoints and boundary flags.
- `core.ClosedRange`, `core.ClosedOpenRange`, `core.OpenClosedRange`, and
  `core.OpenOpenRange` constructors.
- `collections.BoundedStack[T]`, `collections.RingBuffer[T]`,
  `collections.Page[T]`, and `collections.Permutations[T]`.

Rationale:

- Unexported fields preserve constructor validation and snapshot contracts.
- `iter.Seq` is the Go-native shape for lazy permutations.
- Pointer receivers fit mutable stack/ring state; value-returning constructors
  fit immutable range/page values.
- No sentinel errors are added yet because current `core` and `collections`
  validation helpers mostly use plain contextual errors.

## File Structure

- Create: `core/range.go`
- Create: `core/range_test.go`
- Create: `core/range_example_test.go`
- Modify: `core/README.md`
- Modify: `core/README.ko.md`
- Create: `collections/bounded_stack.go`
- Create: `collections/bounded_stack_test.go`
- Create: `collections/ring_buffer.go`
- Create: `collections/ring_buffer_test.go`
- Create: `collections/page.go`
- Create: `collections/page_test.go`
- Create: `collections/permutations.go`
- Create: `collections/permutations_test.go`
- Modify: `collections/collections_example_test.go`
- Modify: `collections/README.md`
- Modify: `collections/README.ko.md`

## Task 1: Range Value Type

**Files:**
- Create: `core/range_test.go`
- Create: `core/range.go`
- Create: `core/range_example_test.go`
- Modify: `core/README.md`, `core/README.ko.md`

- [ ] **Step 1: Write RED range tests**

Cover:

- four valid constructor boundary combinations;
- invalid reversed endpoints;
- equal endpoint validity for closed range and invalidity for open/half-open
  ranges;
- `Contains` boundary behavior;
- `ContainsRange` and `Overlaps` boundary equality behavior;
- empty ranges contain no values, never overlap, and make `ContainsRange`
  return false when either side is empty;
- `String()` notation;
- zero-value range is safe and empty;
- `float32` and `float64` NaN endpoints are rejected.

Run:

```bash
go test -count=1 ./core
```

Expected: FAIL because range symbols do not exist.

- [ ] **Step 2: Implement `core/range.go`**

Implementation notes:

- Use `cmp.Ordered`.
- Keep fields unexported.
- Add accessors: `Lower`, `Upper`, `LowerInclusive`, `UpperInclusive`.
- Keep `Empty` total and safe for zero values.
- Reject NaN endpoints with a small generic helper that type-switches on
  `float32` and `float64`.
- Implement `ContainsRange` and `Overlaps` using boundary-aware comparisons,
  not string notation.

- [ ] **Step 3: Add range examples and README docs**

Docs must state:

- boundary notation;
- invalid range behavior;
- NaN endpoint rejection;
- zero-value range is empty and constructors are preferred;
- Kotlin operator overloads and DSL constructors are excluded.

- [ ] **Step 4: Run GREEN range gate**

Run:

```bash
go test -count=1 ./core
```

Expected: PASS.

## Task 2: Bounded Stack

**Files:**
- Create: `collections/bounded_stack_test.go`
- Create: `collections/bounded_stack.go`

- [ ] **Step 1: Write RED stack tests**

Cover:

- constructor rejects non-positive capacity;
- `Capacity`, `Len`, and `Empty`;
- `Push` and `PushAll`;
- overflow drops the oldest bottom element;
- `Pop`, `Peek`, and `At` return `(zero, false)` on empty/out-of-range;
- `Values` returns top-to-bottom shallow snapshot;
- `Clear` resets state;
- nil/empty value handling for slice element types.

Run:

```bash
go test -count=1 ./collections
```

Expected: FAIL because stack symbols do not exist.

- [ ] **Step 2: Implement `BoundedStack`**

Implementation notes:

- Store values bottom-to-top internally for cheap append/pop.
- `Values` should allocate a new slice and reverse into top-to-bottom order.
- Trim overflow without panics when `PushAll` exceeds capacity.
- Add Go doc comments that the type is not goroutine-safe.

- [ ] **Step 3: Run GREEN stack gate**

Run:

```bash
go test -count=1 ./collections
```

Expected: PASS for stack tests.

## Task 3: Ring Buffer

**Files:**
- Create: `collections/ring_buffer_test.go`
- Create: `collections/ring_buffer.go`

- [ ] **Step 1: Write RED ring tests**

Cover:

- constructor rejects non-positive capacity;
- `Capacity`, `Len`, and `Empty`;
- `Add` and `AddAll`;
- overwrite drops oldest retained value;
- `At` uses oldest-to-newest indexing and returns `(zero, false)` when invalid;
- `Values` returns oldest-to-newest shallow snapshot;
- `Drop(-1)` errors, `Drop(0)` no-ops, `Drop(n >= Len())` clears;
- `Clear` resets state.

Run:

```bash
go test -count=1 ./collections
```

Expected: FAIL because ring buffer symbols do not exist.

- [ ] **Step 2: Implement `RingBuffer`**

Implementation notes:

- Track `start` and `length` over a fixed-capacity slice.
- `Add` overwrites the oldest slot when full.
- `Values` should allocate a snapshot without exposing backing storage.
- Add Go doc comments that the type is not goroutine-safe and is not a
  blocking queue.

- [ ] **Step 3: Run GREEN ring gate**

Run:

```bash
go test -count=1 ./collections
```

Expected: PASS for ring tests.

## Task 4: Page Value

**Files:**
- Create: `collections/page_test.go`
- Create: `collections/page.go`

- [ ] **Step 1: Write RED page tests**

Cover:

- `PageOf` rejects negative page, non-positive size, negative total, and
  offset overflow;
- `Items` snapshots input and returns a fresh shallow snapshot each call;
- nil input keeps `Items() == nil`;
- empty non-nil input returns empty non-nil snapshot;
- `PageNumber`, `PageSize`, `TotalItems`, `TotalPages`, `Offset`,
  `HasNext`, and `HasPrevious`;
- `TotalPages` avoids intermediate overflow for `math.MaxInt64`.

Run:

```bash
go test -count=1 ./collections
```

Expected: FAIL because page symbols do not exist.

- [ ] **Step 2: Implement `Page`**

Implementation notes:

- Keep fields unexported.
- Validate `int64(page) * int64(size)` without overflow.
- Compute total pages with division/modulo instead of `total + size - 1`.
- `Items` should return nil for nil pages and a fresh shallow copy otherwise.

- [ ] **Step 3: Run GREEN page gate**

Run:

```bash
go test -count=1 ./collections
```

Expected: PASS for page tests.

## Task 5: Lazy Permutations

**Files:**
- Create: `collections/permutations_test.go`
- Create: `collections/permutations.go`

- [ ] **Step 1: Write RED permutation tests**

Cover:

- nil input yields one empty permutation;
- empty non-nil input yields one empty non-nil permutation;
- one element yields one permutation;
- three elements yield six permutations in deterministic order;
- duplicate values are treated as positions, not deduplicated values;
- caller early-stop prevents further yields;
- input mutation after iterator creation does not affect yielded values;
- each yield is a fresh snapshot.

Run:

```bash
go test -count=1 ./collections
```

Expected: FAIL because permutation symbols do not exist.

- [ ] **Step 2: Implement `Permutations`**

Implementation notes:

- Return `iter.Seq[[]T]`.
- Copy source input once when `Permutations` is called, before returning the
  sequence.
- Use in-place swap/backtracking over the private copy.
- Yield fresh snapshots and stop immediately when `yield` returns false.
- Do not add an all-permutations materializing helper.

- [ ] **Step 3: Run GREEN permutation gate**

Run:

```bash
go test -count=1 ./collections
```

Expected: PASS for permutation tests.

## Task 6: Examples and README Synchronization

**Files:**
- Modify: `core/range_example_test.go`
- Modify: `collections/collections_example_test.go`
- Modify: `core/README.md`, `core/README.ko.md`
- Modify: `collections/README.md`, `collections/README.ko.md`

- [ ] **Step 1: Add compile-tested examples**

Examples:

- range membership and boundary notation;
- stack overflow and top-to-bottom values;
- ring overwrite and oldest-to-newest values;
- page metadata;
- permutation early stop.

- [ ] **Step 2: Update English and Korean README files**

Both language files must document:

- range boundary notation and invalid ranges;
- stack/ring ordering;
- 0-based page numbering;
- not goroutine-safe containers;
- shallow snapshots;
- factorial permutation growth;
- excluded Kotlin/JVM parity shapes.

- [ ] **Step 3: Run doc/example gate**

Run:

```bash
go test -count=1 ./core ./collections
```

Expected: PASS.

## Task 7: Required Validation

- [ ] **Step 1: Targeted tests**

```bash
go test -count=1 ./core ./collections
```

- [ ] **Step 2: Race gate**

```bash
go test -race -count=1 ./core ./collections
```

- [ ] **Step 3: Full test suite**

```bash
go test ./...
```

- [ ] **Step 4: Whitespace gate**

```bash
git diff --check
```

- [ ] **Step 5: Full CI gate**

```bash
make ci
```

## Review Gates

- [ ] Step 3-R: Run 7-tier plan review. Required verdict: `P0=0 P1=0`.
- [ ] Step 4: Execute tasks with RED/GREEN evidence.
- [ ] Step 5: Verify implementation against spec and plan.
- [ ] Step 6-R: Run 7-tier code review. Required verdict: `P0=0 P1=0`.
- [ ] Step 7+: Commit lessons before PR, create PR with final `## DoD Status`
  section, verify PR body, run Step 7-R PR review, and wait for CI.
