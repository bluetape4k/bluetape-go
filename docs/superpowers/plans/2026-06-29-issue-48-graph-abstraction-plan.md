# Issue #48 Graph Abstraction Implementation Plan

> 한국어 운영 요약: 이 계획 문서는 사용자 협업용 실행 계획이다. 아래 원문에 포함된 명령, 경로, API 이름, issue/PR 번호, branch 이름, code block, test output은 추적성과 재현성을 위해 그대로 보존한다. 작업 순서, 위험, 검증, 롤백 판단은 한국어 독자가 바로 실행 경계를 이해할 수 있도록 이 메모를 우선 적용한다.
> 추가 한국어 요약: 이 문서의 실행 판단은 기존 순서를 따르며, 변경자는 작업 표와 검증 목록을 먼저 확인한 뒤 관련 테스트를 실행한다. 영어로 남은 항목은 코드 식별자 또는 재현 증거다.\n

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first `graph` package with safe Go-shaped values for graph I/O and examples without introducing backend repository/session abstractions.

**Architecture:** The package is model-only. Public structs keep internal fields unexported, constructors validate invariants, accessors return defensive copies, and JSON support validates decoded values. Follow-up packages own graph I/O, backend evaluation, and domain examples.

**Tech Stack:** Go 1.26.3, standard library only, `go test`, `go test -race`, serial repo `make` gates.

---

## File Structure

- Create `graph/doc.go`: package overview, explicit non-goals, and follow-up ownership.
- Create `graph/errors.go`: sentinel errors and redacted `ValidationError`.
- Create `graph/types.go`: `ElementID`, `Label`, `Properties`, `Vertex`, `EdgeEndpoints`, `RawEdgeEndpoints`, and `Edge`.
- Create `graph/path.go`: `PathStep`, `Path`, path constructors, accessors, and validating JSON helpers.
- Create `graph/types_test.go`: ID, label, properties, vertex, edge, JSON, and redaction tests.
- Create `graph/path_test.go`: path-step, empty path, weighted path, zero-value, defensive-copy, and JSON tests.
- Create `graph/example_test.go`: compile-only examples with the real import path.
- Create `graph/README.md` and `graph/README.ko.md`: user guidance and unsupported capability routing.
- Modify `README.md` and `README.ko.md`: active package table and package docs list.
- Modify `CHANGELOG.md` and `WIP.md`: release bookkeeping for the new public package.
- Add `docs/lessons/2026-06-29-graph-model-api-boundaries.md` before PR creation.

## Task 1: Element IDs, Labels, And Errors

**complexity:** medium

**Files:**
- Create: `graph/errors.go`
- Create: `graph/types.go`
- Test: `graph/types_test.go`

- [ ] **Step 1: Write failing tests for ID, label, and redacted validation errors**

```go
func TestElementIDAndLabelValidation(t *testing.T) {
	id, err := graph.NewElementID(" node-1 ")
	if err != nil {
		t.Fatalf("NewElementID error = %v", err)
	}
	if id.String() != "node-1" {
		t.Fatalf("ElementID = %q, want node-1", id.String())
	}

	if _, err := graph.NewElementID(" "); !errors.Is(err, graph.ErrInvalidElementID) {
		t.Fatalf("blank id error = %v, want ErrInvalidElementID", err)
	}
	if _, err := graph.ElementIDFromInt(-1); !errors.Is(err, graph.ErrInvalidElementID) {
		t.Fatalf("negative id error = %v, want ErrInvalidElementID", err)
	}

	label, err := graph.NewLabel(" Person ")
	if err != nil {
		t.Fatalf("NewLabel error = %v", err)
	}
	if label.String() != "Person" {
		t.Fatalf("Label = %q, want Person", label.String())
	}
	if _, err := graph.NewLabel(""); !errors.Is(err, graph.ErrInvalidLabel) {
		t.Fatalf("blank label error = %v, want ErrInvalidLabel", err)
	}
}

func TestValidationErrorRedactsValues(t *testing.T) {
	const secret = "token-secret-value"
	var vertex graph.Vertex
	err := json.Unmarshal(
		[]byte(`{"id":" ","label":"Person","properties":{"api_key":"`+secret+`"}}`),
		&vertex,
	)
	if !errors.Is(err, graph.ErrInvalidElementID) {
		t.Fatalf("error = %v, want ErrInvalidElementID", err)
	}
	var validation graph.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("error = %T, want ValidationError", err)
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(fmt.Sprintf("%+v", validation), secret) {
		t.Fatalf("validation error leaked raw value: %v %+v", err, validation)
	}
}
```

Additional redaction tests must use actual secret-bearing invalid ID, label,
property, JSON, and `fmt.Stringer` inputs. Assert `err.Error()`, `%+v`, and
exported `ValidationError` fields never contain the raw secret. `ValidationError`
must not have a raw `Value any` field.

- [ ] **Step 2: Run red tests**

Run: `go test -count=1 ./graph`

Expected: FAIL because package `graph` does not exist.

- [ ] **Step 3: Implement `errors.go` and ID/label parts of `types.go`**

Create sentinel errors including reserved `ErrUnsupportedCapability`,
`ValidationError` with `Kind`, `Field`, redacted summary text, and `Cause`
only, plus `ElementID`, `Label`, `String`, `Validate`, `MarshalJSON`,
validating `UnmarshalJSON`, `New*`, `ElementIDFromInt`, and `MustElementID`.
Document `MustElementID` for constants/static fixtures only and test both its
success and panic paths. Add a test that no #48 constructor returns
`ErrUnsupportedCapability`.

- [ ] **Step 4: Run green tests**

Run: `go test -count=1 ./graph`

Expected: PASS for the new tests.

## Task 2: Properties, Vertex, Edge, And JSON Validation

**complexity:** high

**Files:**
- Modify: `graph/types.go`
- Test: `graph/types_test.go`

- [ ] **Step 1: Write failing tests for defensive copies, named endpoints, JSON, and unsupported raw IDs**

Tests must cover:

- `Properties(nil).Clone() == nil`; empty properties remain empty without
  unnecessary nil-to-empty allocation in accessors where nil is acceptable.
- `Properties.Clone` shallow-copies the map boundary.
- Nested mutable property values intentionally alias after clone; docs must state
  this is not a deep-copy, sanitization, or trust-boundary primitive.
- `NewVertex` and `NewEdge` reject zero `ElementID`/`Label`.
- `NewEdge` uses named-field `EdgeEndpoints{Start: ..., End: ...}` plus
  `EdgeEndpoints.Validate`; there is no exported `NewEdgeEndpoints(start, end)`
  helper.
- `ParseEdge` uses `RawEdgeEndpoints`.
- Returned `Properties()` maps are defensive copies.
- Direct JSON decode rejects blank scalar `ElementID`/`Label`, blank vertex/edge
  IDs or labels, and invalid endpoint fields.
- JSON shape is fixed: vertex uses `id`, `label`, `properties`; edge uses `id`,
  `label`, `start`, `end`, `properties`.
- Unsupported raw ID examples (`nil`, `bool`, `float64`, `struct`, `slice`, and
  secret-bearing `fmt.Stringer`) are not accepted by any broad `any` conversion.
  Do not add `ElementIDFromAny` or an equivalent helper.
- Zero `Vertex` and `Edge` accessors do not panic, `Validate` returns the typed
  sentinel, and returned properties are nil or empty.

- [ ] **Step 2: Run red tests**

Run: `go test -count=1 ./graph`

Expected: FAIL because vertex/edge APIs are not implemented.

- [ ] **Step 3: Implement properties, vertex, edge, and JSON**

Implement unexported fields, `Properties.Clone`, `NewVertex`, `ParseVertex`,
`EdgeEndpoints.Validate`, `NewEdge`, `ParseEdge`, accessors, `Validate`,
`MarshalJSON`, and validating `UnmarshalJSON`.

- [ ] **Step 4: Run green tests**

Run: `go test -count=1 ./graph`

Expected: PASS for Task 1 and Task 2 tests.

## Task 3: Path And Weighted Path Values

**complexity:** high

**Files:**
- Create: `graph/path.go`
- Test: `graph/path_test.go`

- [ ] **Step 1: Write failing path tests**

Tests must cover:

- Zero `Path` is empty and no method panics.
- Zero `PathStep` methods do not panic, `Validate` returns `ErrInvalidPath`, and
  accessors return `(zero, false)`.
- `VertexStep` and `EdgeStep` return `ErrInvalidPath` for invalid zero values.
- `NewPath` copies input steps and default weight equals edge count.
- `NewWeightedPath` rejects `NaN`, infinity, and negative weights.
- `Steps`, `Vertices`, and `Edges` return defensive slices.
- JSON shape is fixed: `PathStep` uses exactly one of `vertex` or `edge`; `Path`
  uses `steps` and `total_weight`.
- JSON decode rejects invalid path step shape and invalid weight.

- [ ] **Step 2: Run red tests**

Run: `go test -count=1 ./graph`

Expected: FAIL because path APIs are not implemented.

- [ ] **Step 3: Implement path values**

Implement unexported `PathStep` kind/value fields, validating `VertexStep` and
`EdgeStep`, `Path` constructors/accessors, finite-weight validation, and JSON
support for the fixed wire shape.

- [ ] **Step 4: Run green tests**

Run: `go test -count=1 ./graph`

Expected: PASS for all `graph` tests.

## Task 4: Examples And Package Documentation

**complexity:** medium

**Files:**
- Create: `graph/doc.go`
- Create: `graph/example_test.go`
- Create: `graph/README.md`
- Create: `graph/README.ko.md`

- [ ] **Step 1: Write compile-only examples**

Examples must show:

- Constructing vertex, edge, and path values.
- `errors.Is` and `errors.As` with validation errors without exposing raw
  sensitive values.
- Raw local backend-like structs adapted through
  `github.com/bluetape4k/bluetape-go/graph`, with no backend dependency.
- `NewElementID` or `ParseVertex` for caller/runtime input. `MustElementID`
  examples are limited to constants/static fixtures.

- [ ] **Step 2: Write package README pair**

Both `graph/README.md` and `graph/README.ko.md` must include Import, minimal
vertex/edge/path usage, `errors.Is`/`errors.As` validation handling, raw backend
record adaptation, unsupported capability routing, property ownership, and Test
sections. Document that `Properties` is shallow and not a sanitization or
trust-boundary primitive. Document that `ErrUnsupportedCapability` is reserved
for #49/#50/#51 capability boundaries and no #48 API returns it.

Every exported graph type, function, method, variable, and sentinel error must
have an English Go doc comment suitable for `pkg.go.dev`, including zero-value
and shallow ownership behavior where relevant.

- [ ] **Step 3: Verify docs compile**

Run: `go test -count=1 ./graph`

Expected: PASS including examples.

## Task 5: Root Docs And Release Bookkeeping

**complexity:** low

**Files:**
- Modify: `README.md`
- Modify: `README.ko.md`
- Modify: `CHANGELOG.md`
- Modify: `WIP.md`

- [ ] **Step 1: Update root README pair**

Add `graph` to the active package table and package documentation list. Remove
graph from the planned-family sentence without implying backend support.

- [ ] **Step 2: Update release bookkeeping**

Record the graph model package in `CHANGELOG.md` Unreleased and `WIP.md` for
the current milestone context. Add a release-support note: the package has no
service/runtime dependency; before a tag, rollback is removing `graph` plus docs
and bookkeeping; after a tag, changes must preserve Go API compatibility or be
deferred to a breaking release.

- [ ] **Step 3: Verify docs references**

Run: `rg -n "graph|Graph|#49|#50|#51|ErrUnsupportedCapability|Import|Usage|Test" README.md README.ko.md graph/README.md graph/README.ko.md CHANGELOG.md WIP.md`

Expected: graph package is active, unsupported capabilities route to follow-up
issues, and Korean/English README pairs are content-aligned by manual Step 6-R
review, not just grep-aligned.

## Task 6: Verification, Review, And Lessons

**complexity:** medium

**Files:**
- Create: `docs/lessons/2026-06-29-graph-model-api-boundaries.md`
- Create: `docs/superpowers/reviews/2026-06-29-issue-48-graph-abstraction-step-6r-code-review.md`

- [ ] **Step 1: Run targeted verification**

Run:

```bash
go test -count=1 ./graph
go test -race -count=1 ./graph
```

Expected: PASS.

- [ ] **Step 2: Run repo verification**

Run:

```bash
make test
make race
make fmt-check
make tidy-check
make vet
make lint
make ci
```

Expected: PASS, or record exact unavailable command/blocker output.

Stress tests are N/A for #48 because the model package has no goroutines, shared
mutable state, caches, workers, streaming, or batched importers. Keep
`go test -race -count=1 ./graph`; add bounded stress tests in #49 or any future
concurrent/backend adapter work.

- [ ] **Step 3: Run Step 6-R 7-tier code review**

Review the changed diff across performance, stability, security, operator,
developer/API, user/caller, and main integration. P0/P1 must be 0 before PR.
Verify `go doc ./graph` output, exported doc comments, fixed JSON shape,
reserved `ErrUnsupportedCapability`, absence of repository/session/schema/query/
transaction/capability interfaces, and README/README.ko parity.

- [ ] **Step 4: Add lesson**

Record why graph package APIs use named edge endpoint structs, validating path
step constructors, unexported fields, and shallow property ownership.

- [ ] **Step 5: Commit spec, plan, implementation, review, docs, and lesson**

Use a Lore-format commit message with Tested/Not-tested trailers based on the
fresh verification output.

- [ ] **Step 6: Create or update PR evidence**

Open or update the PR linked to #48. The PR body must include milestone `0.10.0`
context, a Step DoD table, exact verification outputs or blockers, Step 6-R
P0/P1=0 evidence, and `gh pr checks`/CI status evidence. Verify the final PR body
with `gh pr view --json body`.
