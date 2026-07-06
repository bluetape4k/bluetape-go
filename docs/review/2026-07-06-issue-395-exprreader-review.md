# Issue 395 Expr Reader Review

## Scope

- Issue: #395
- Branch: `feat/issue-395-expr-rule-reader`
- Base: `origin/develop`
- Files reviewed:
  - `go.mod`
  - `go.sum`
  - `rules/README.md`
  - `rules/README.ko.md`
  - `rules/exprreader/reader.go`
  - `rules/exprreader/reader_test.go`

## Independent Lanes

### Code Review Lane

- P0: 0
- P1: 0
- Recommendation: comment
- Findings:
  - Medium: schema decoding accepted unknown YAML/JSON fields.
  - Low: engine integer parsing could overflow on large `uint64` values.
  - Low: tests needed direct coverage for builtin rejection, max-node rejection, and engine wrapping.

### Architecture Lane

- P0: 0
- P1: 0
- Recommendation: request changes before PR
- Findings:
  - Medium: permissive schema parsing was the main long-term API risk for a config-driven reader.
  - Low: `Load` checks context before and after `io.ReadAll`, but cannot interrupt a blocked bare `io.Reader`.

## Resolution

- Added strict YAML/JSON decoding with `yaml.Decoder.KnownFields(true)`.
- Rejected multiple rule documents.
- Added `int` range checks for `int64` and `uint64` engine values.
- Added tests for:
  - unknown top-level fields
  - unknown rule fields
  - unknown `set` payload fields
  - oversized engine integer values
  - builtin calls
  - max-node limits
  - engine `ErrRuleEvaluation` wrapping for generated predicates

The `io.Reader` cancellation limitation remains accepted because `Load` cannot safely preempt arbitrary blocking readers without changing the public input contract. The reader still checks context before and after the read, and generated rules check context before evaluation and execution.

## Final Verdict

- P0: 0
- P1: 0
- Status: approved after fixes

## Verification

- `go test -count=1 ./rules ./rules/...`
- `go test -race -count=1 ./rules ./rules/...`
- `go test -count=1 ./...`
- `git diff --check`
- `make fmt-check`
- `make vet`
- `make lint`
