# Issue #207 Wildcard and Hash Utilities Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans or equivalent checklist execution. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Go-native wildcard matching and deterministic XXH64 byte/string helpers requested by #207.

**Architecture:** Keep the new public API in `core`, alongside the existing small shared helpers. Implement wildcard matching with parsed tokens and dynamic programming to avoid recursive backtracking risk. Use `github.com/cespare/xxhash/v2` only at raw bytes/string boundaries; do not add generic object hashing or JVM resource/system utility ports.

**Tech Stack:** Go 1.26.x, standard library slices/strings/unicode handling, `github.com/cespare/xxhash/v2`, existing `go test`, `make ci`, and `git diff --check`.

---

## API Decision

Add these public functions:

- `core.MatchWildcard(pattern, value string) (bool, error)`
- `core.FirstWildcardMatch(value string, patterns ...string) (int, error)`
- `core.MatchWildcardPath(pattern, path string) (bool, error)`
- `core.FirstWildcardPathMatch(path string, patterns ...string) (int, error)`
- `core.XXH64Bytes(value []byte) uint64`
- `core.XXH64String(value string) uint64`

Rationale:

- Pattern-first matching follows Go's `path.Match` shape.
- Returning `error` makes malformed trailing escapes explicit.
- `**` is path-only and special only when it is the complete segment.
- Raw bytes/string hashing avoids unstable generic object semantics.
- `XXH64` is named in the API so callers do not mistake it for cryptographic
  hashing or a future-swappable stable hash abstraction.

## File Structure

- Create: `core/wildcard.go`
- Create: `core/wildcard_test.go`
- Create: `core/hash.go`
- Create: `core/hash_test.go`
- Modify: `core/doc.go`
- Modify: `core/README.md`
- Modify: `core/README.ko.md`
- Modify: `go.mod`
- Modify: `go.sum` if `go mod tidy` changes it

## Task 1: Wildcard String Matching

**Files:**
- Create: `core/wildcard_test.go`
- Create: `core/wildcard.go`

- [ ] **Step 1: Write RED string wildcard tests**

Cover exact matches, `*`, `?`, consecutive stars, escaped wildcard literals,
escaped backslash, trailing escape errors, Unicode runes, case sensitivity, and
first-match index behavior.

Run:

```bash
go test -count=1 ./core
```

Expected: FAIL because `MatchWildcard` and `FirstWildcardMatch` do not exist.

- [ ] **Step 2: Implement string wildcard matching**

Implementation notes:

- Parse `pattern` into tokens: literal rune, any-rune token, and star token.
- Collapse consecutive star tokens.
- Return an error for a trailing `\`.
- Treat `\*`, `\?`, and `\\` as literal tokens; treat other escaped runes as
  literal runes.
- Convert `value` to `[]rune`.
- Use DP where `dp[i][j]` means the first `i` pattern tokens match the first
  `j` value runes.
- Implement `FirstWildcardMatch` by evaluating patterns in order and returning
  the first malformed pattern error.

- [ ] **Step 3: Run GREEN string wildcard gate**

Run:

```bash
go test -count=1 ./core
```

Expected: PASS for wildcard string tests.

## Task 2: Wildcard Path Matching

**Files:**
- Modify: `core/wildcard_test.go`
- Modify: `core/wildcard.go`

- [ ] **Step 1: Write RED path wildcard tests**

Cover `**` matching zero, one, and many path segments; `*` and `?` within
ordinary segments; `/` and `\` separators; case sensitivity; `**` only being
special as a full segment; malformed segment patterns; and first-match index
behavior.

Run:

```bash
go test -count=1 ./core
```

Expected: FAIL because `MatchWildcardPath` and `FirstWildcardPathMatch` do not
exist.

- [ ] **Step 2: Implement lexical path wildcard matching**

Implementation notes:

- Split both `pattern` and `path` on `/` and `\`.
- Drop empty segments from repeated, leading, or trailing separators.
- Reuse string wildcard matching for non-`**` segments.
- Treat a pattern segment exactly equal to `**` as zero-or-more segment match.
- Use DP across path pattern segments and path segments.

- [ ] **Step 3: Run GREEN path wildcard gate**

Run:

```bash
go test -count=1 ./core
```

Expected: PASS for wildcard path tests.

## Task 3: XXH64 Byte/String Helpers

**Files:**
- Create: `core/hash_test.go`
- Create: `core/hash.go`
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Write RED hash tests**

Cover empty bytes, empty string, ASCII, Unicode, repeated determinism, and
bytes/string equivalence for the same UTF-8 bytes. Use fixed XXH64 fixture
values generated from `github.com/cespare/xxhash/v2` for seed 0.

Run:

```bash
go test -count=1 ./core
```

Expected: FAIL because `XXH64Bytes` and `XXH64String` do not exist.

- [ ] **Step 2: Implement hash helpers**

Implementation notes:

- Import `github.com/cespare/xxhash/v2`.
- `XXH64Bytes` calls `xxhash.Sum64`.
- `XXH64String` calls `xxhash.Sum64String`.
- Add Go doc comments that these helpers are deterministic and
  non-cryptographic.
- Run `go mod tidy` so the dependency becomes direct if required.

- [ ] **Step 3: Run GREEN hash gate**

Run:

```bash
go test -count=1 ./core
```

Expected: PASS for hash tests.

## Task 4: Documentation

**Files:**
- Modify: `core/doc.go`
- Modify: `core/README.md`
- Modify: `core/README.ko.md`

- [ ] **Step 1: Update docs**

Document wildcard syntax, lexical path matching, case sensitivity, malformed
trailing escape errors, XXH64 examples, non-cryptographic warning, and excluded
JVM/resource/system/generic-object helpers.

- [ ] **Step 2: Run documentation checks**

Run:

```bash
go test -count=1 ./core
git diff --check
```

Expected: PASS.

## Task 5: Full Verification and Review Preparation

**Files:**
- All files changed by Tasks 1-4

- [ ] **Step 1: Run targeted package checks**

Run:

```bash
go test -count=1 ./core
go test -race -count=1 ./core
```

Expected: PASS.

- [ ] **Step 2: Run repo checks**

Run:

```bash
go test ./...
make fmt-check
make tidy-check
make vet
make lint
make ci
git diff --check
```

Expected: PASS, or capture exact unrelated environmental failure and rerun the
affected package sequentially when applicable.

- [ ] **Step 3: Prepare Step 6-R review evidence**

Create `docs/superpowers/reviews/2026-06-22-issue-207-wildcard-hash-utils-step-6r-code-review.md`
with 7-tier findings, fixed blockers, and final `P0=0 P1=0` verdict before
commit/PR.
