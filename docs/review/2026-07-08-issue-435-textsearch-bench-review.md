# Issue #435 Textsearch benchmark review

Issue: #435
Date: 2026-07-08
Scope: benchmark suite and dependency-adoption evidence for `textsearch`.

## Reviewed Artifacts

- `textsearch/matcher_benchmark_test.go`
- `docs/research/2026-07-08-issue-435-textsearch-bench.md`
- `docs/research/outputs/issue-435/textsearch-bench.txt`
- `docs/research/outputs/issue-435/environment.md`
- `docs/lessons/2026-07-08-issue-435-textsearch-bench.md`

## Findings

| Severity | Finding | Evidence |
|---|---|---|
| P0 | None | Benchmark-only external imports do not change production `textsearch` APIs or matcher behavior. |
| P1 | None | Candidate comparison is scoped to raw matching and does not claim parity for Unicode normalization, offsets, boundaries, replacement, or masking. |
| P2 | None | Raw output and environment metadata are preserved for repeatability. |

## Lens Check

| Lens | Verdict | Evidence |
|---|---|---|
| Performance | Pass | `go test -run '^$' -bench . -benchmem ./textsearch` captured compile, contains, first, all matches, replacement, masking, and external raw candidates. |
| Stability | Pass | `go test -count=1 ./textsearch` passed after adding benchmark code. |
| Security | Pass | No masking or blockword semantics changed; benchmark docs explicitly avoid treating masking as a security boundary. |
| Operator/Ops | Pass | Output files include Go version, CPU, commit, module versions, licenses, archive state, stars, and pushed dates. |
| Developer/API | Pass | No public production API change; benchmark-only candidates are isolated to `_test.go`. |
| User/Caller | Pass | Existing first-party behavior remains intact; external candidates are not exposed to callers. |

Final verdict: PASS. P0=0 P1=0.
