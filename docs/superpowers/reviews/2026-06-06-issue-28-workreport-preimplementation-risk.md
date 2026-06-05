# Issue 28 Workreport Pre-Implementation Risk

Issue: #28
Gate: Step 3-P / T0
Status: PASS

## Reviewed Evidence

- `docs/superpowers/specs/2026-06-06-issue-28-workreport-spec.md`
- `docs/superpowers/plans/2026-06-06-issue-28-workreport-plan.md`
- `testing/concurrency/*`
- `state/*`
- `CHANGELOG.md`
- `WIP.md`

## Risks

| Risk | Severity | Mitigation fed into implementation |
|---|---|---|
| Unknown failure policies could return an ambiguous report instead of a caller-checkable error. | P1 if missed | Export `ErrUnknownFailurePolicy`; make `Aggregate` return `(Report, error)`; test `errors.Is`. |
| Zero-value report could be mistaken for successful work. | P1 if missed | Keep zero status as unknown; predicates must return false for success/failure/terminal; document in `doc.go` and README. |
| Stop-on-failure aggregation could discard too much or too little child evidence. | P1 if missed | Preserve children through the stopping child; test truncation and child order. |
| Continue-on-failure aggregation could flatten child errors. | P1 if missed | Preserve all children and their `Err` fields; test `errors.Is` on child errors. |
| `workreport` could drift into runner-specific execution behavior before #27. | P2 | Keep package stateless and value-based; no goroutines, runner interfaces, retry, scheduler, or mutable context. |
| Stress/cancellation gate could be weak because the package is mostly value logic. | P2 | Use helper-based tests only where they add signal: immutable aggregation under `GoroutineStressTester`, cancellation report preservation under `AsyncJobTester`, plus race run. |

## Verdict

P0=0 P1=0. Step 4 implementation is unblocked.
