# Issue #206 Range and Collection Primitives Lessons

## What Changed

- Added Go-native `core.Range` with open/closed boundary constructors,
  containment, overlap checks, zero-value empty behavior, and NaN-safe
  membership.
- Added `collections.BoundedStack`, `RingBuffer`, `Page`, and lazy
  `Permutations` using `iter.Seq`.
- Updated English and Korean READMEs plus compile-tested examples.

## Lessons

- `cmp.Ordered` includes floats, so constructor NaN rejection is not enough.
  Membership checks must also reject NaN values because ordinary comparisons
  with NaN are all false.
- Pagination helpers should avoid `page + 1` and `total + size - 1` style
  arithmetic. Compare against `totalPages - 1` after guarding zero pages, and
  compute total pages with division plus remainder.
- `iter.Seq` APIs should copy caller input at function-call time when the
  contract says later caller mutation cannot affect iteration. Copying inside
  the returned closure is too late.
- Fixed-capacity containers that discard values should clear discarded backing
  slots before reslicing to avoid retaining references longer than needed.
- Native subagent review can fail at the manager lifecycle layer. When that
  happens, record the unavailability and perform the same 7-tier review shape in
  the main session instead of weakening the gate.

## Verification

- `go test -count=1 ./core ./collections`
- `go test -race -count=1 ./core ./collections`
- `go test ./...`
- `git diff --check`
- `make ci`
