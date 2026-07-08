# Issue #434 UUID v7 hot-path review

Issue: #434
Date: 2026-07-08
Scope: benchmark evidence and measured rejection note for UUID v7 parallel
generation.

## Reviewed Artifacts

- `docs/research/2026-07-08-issue-434-uuid-v7-hotpath.md`
- `docs/research/outputs/issue-434/uuid-v7-baseline-count10.txt`
- `docs/research/outputs/issue-434/uuid-v7-atomic-count10.txt`
- `docs/research/outputs/issue-434/benchstat-atomic-candidate.txt`
- `docs/research/outputs/issue-434/uuid-v7-reuse-parallel-cpu-top.txt`
- `docs/research/outputs/issue-434/uuid-v7-reuse-parallel-mutex-top.txt`
- `docs/lessons/2026-07-08-issue-434-uuid-v7-hotpath.md`

## Findings

| Severity | Finding | Evidence |
|---|---|---|
| P0 | None | No production source diff remains for `id/uuid.go`; measured candidate did not justify code change. |
| P1 | None | `benchstat` shows `UUIDV7ReuseGeneratorParallel` baseline `192.6 ns/op +/- 1%` vs candidate `193.2 ns/op +/- 1%`; no hidden improvement is being claimed. |
| P2 | None | Follow-up hypotheses are scoped as future work and do not alter API contracts. |

## Lens Check

| Lens | Verdict | Evidence |
|---|---|---|
| Performance | Pass | Count=10 baseline and candidate outputs are preserved, plus `benchstat` comparison. |
| Stability | Pass | Existing implementation is retained; `go test -count=1 ./id` and `go test -race -count=1 ./id` passed during candidate validation. |
| Security | Pass | No entropy-grade weakening, no global generator state, no wire-format change. |
| Operator/Ops | Pass | Raw profile files and `pprof -top` summaries are stored for reproducibility. |
| Developer/API | Pass | No public API change; rejected atomic approach is documented to prevent repeated churn. |
| User/Caller | Pass | UUID v7 ordering, rollback, overflow, canonical format, and error contracts remain unchanged. |

Final verdict: PASS. P0=0 P1=0.
