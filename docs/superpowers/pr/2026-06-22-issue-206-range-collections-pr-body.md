## Summary

- Add `core.Range` with closed/open boundary constructors, containment,
  overlap checks, NaN-safe membership, and zero-value empty behavior.
- Add `collections.BoundedStack`, `RingBuffer`, `Page`, and lazy
  `Permutations` using Go-native generic and `iter.Seq` APIs.
- Update English/Korean README files, compile-tested examples, verifier/review
  artifacts, and lessons.

Closes #206.

## Review

- Step 2-R spec review: `P0=0 P1=0`
- Step 3-R plan review: `P0=0 P1=0`
- Step 5 verifier: PASS
- Step 6-R code review: `P0=0 P1=0`

## DoD Status

| Gate | Status | Evidence |
|---|---|---|
| Worktree | PASS | `issue-206-range-collections` based on `origin/develop`. |
| TDD | PASS | RED undefined-symbol tests preceded implementation; targeted GREEN passed after implementation. |
| Targeted tests | PASS | `go test -count=1 ./core ./collections` |
| Race gate | PASS | `go test -race -count=1 ./core ./collections` |
| Full tests | PASS | `go test ./...` |
| Whitespace | PASS | `git diff --check` |
| CI | PASS | `make ci` |
| Lessons | PASS | `docs/lessons/2026-06-22-issue-206-range-collections.md` committed before PR. |
