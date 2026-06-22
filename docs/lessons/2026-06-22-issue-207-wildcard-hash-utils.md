# Issue #207 Wildcard and Hash Utilities Lessons

## What Changed

- Added Go-native wildcard helpers in `core` for case-sensitive string matching,
  lexical path matching, escaped literals, and `**` path segments.
- Added deterministic XXH64 byte/string helpers with an explicit
  non-cryptographic boundary.
- Updated English and Korean `core` READMEs and preserved the `cespare/xxhash`
  dependency rationale in `bluetape4k-wiki`.

## Lessons

- Slash-separated wildcard path patterns need an explicit escape rule. Treat
  `/` and `\` as input separators, but support `\*`, `\?`, and `\\` escapes
  inside slash-separated pattern segments so callers can match literal wildcard
  characters without losing Windows-path portability.
- Generic object hashing is the wrong parity target for Go. Keep hash helpers
  at byte/string or caller-owned encoding boundaries instead of trying to mimic
  JVM `hashCode()` behavior.
- `make tidy-check` expects intentional `go.mod` or `go.sum` updates to be
  staged before the check, because it verifies that `go mod tidy` produces no
  additional unstaged diff.
- When editing a worktree, use absolute paths with `apply_patch`. A relative
  patch can accidentally write to the main checkout when the tool does not take
  a working directory.
- Native subagent cleanup can hang at the manager layer. If stale slots cannot
  be recovered, record the unavailability and perform the same 7-tier review
  shape in the main session instead of blocking indefinitely.

## Verification

- `go test -count=1 ./core`
- `go test -race -count=1 ./core`
- `go test ./...`
- `make fmt-check`
- `make tidy-check`
- `make vet`
- `make lint`
- `make ci`
- `git diff --check`
