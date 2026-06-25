# Issue #207 Wildcard and Hash Utilities Step 6-R Review

Scope: `core` wildcard matching, XXH64 helpers, README pair, dependency
metadata, spec/plan/review artifacts, and lessons.

Baseline: `origin/develop` at `0ea2bfc`.

## Gate Result

P0=0 P1=0

Final verdict: PASS.

Native subagent lanes were unavailable because stale-agent cleanup attempts
hung until user interruption. The six-lane 7-tier frame was completed in the
main session and recorded here as the review evidence.

## Lane Results

| Lane | P0 | P1 | P2 | P3 | Verdict | Evidence |
|---|---:|---:|---:|---:|---|---|
| Performance/runtime | 0 | 0 | 0 | 0 | PASS | Wildcard matching uses dynamic programming over runes and path segments, avoiding recursive backtracking. No goroutine, lock, IO, or service hot path was added. |
| Stability/correctness | 0 | 0 | 0 | 0 | PASS | String trailing escapes return `ErrMalformedWildcardPattern`; path matching is lexical and does not read or clean the filesystem. |
| Security | 0 | 0 | 0 | 0 | PASS | XXH64 helpers are documented as non-cryptographic. No auth, secret, SQL, command, path traversal, or deserialization boundary was introduced. |
| Operator/Ops | 0 | 0 | 0 | 0 | PASS | Runtime service behavior, config, migrations, logging, metrics, and shutdown paths are unchanged. The dependency promotion is narrow and validated by module checks. |
| Developer/API | 0 | 0 | 0 | 0 | PASS | Public APIs are small, explicit, and Go-shaped: string/path wildcard helpers are separate, error returns are explicit, and hash helpers only accept byte/string inputs. |
| User/caller docs | 0 | 0 | 0 | 0 | PASS | README and Korean README document syntax, lexical path behavior, escaped literals, deterministic XXH64, non-crypto limits, and excluded JVM/resource/system/generic helpers. |

## Findings

No P0/P1 findings.

Resolved P2: path pattern backslash semantics were initially ambiguous for
wildcard segments and escaped literal `*`/`?` characters. The implementation now
keeps `/` and `\` as input separators while documenting and testing escaped
literal wildcard characters inside slash-separated pattern segments.

## Validation

| Command / Review | Status | Evidence |
|---|---|---|
| `go test -count=1 ./core` | PASS | Targeted core tests passed after wildcard and hash implementation. |
| `go test -race -count=1 ./core` | PASS | Targeted race gate passed for the changed package. |
| `go test ./...` | PASS | Full repository tests passed after final path-escape adjustment. |
| `make fmt-check` | PASS | Formatting gate passed. |
| `make tidy-check` | PASS | Module metadata gate passed after intentional `go.mod` promotion was staged. |
| `make vet` | PASS | Vet gate passed. |
| `make lint` | PASS | Lint reported `0 issues.` |
| `make ci` | PASS | Full CI target exited 0 after lint, normal tests, race tests, and Testcontainers-backed packages. |
| `git diff --check` | PASS | Whitespace check clean. |
| Web research preservation | PASS | `bluetape4k-wiki` note committed and pushed as `5e5dfcf`; `gno update`, `gno embed --collection bluetape4k-wiki`, and `gno search "cespare xxhash Go dependency review" -c bluetape4k-wiki` passed. |

## Residual Risk

Path matching is lexical only and does not model OS-specific filesystem
normalization, symlinks, or case-folding. That is intentional for portable
library helpers.
