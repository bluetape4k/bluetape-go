# Issue #166 KSUID Generator Family Plan Review

## Scope

- Plan:
  `docs/superpowers/plans/2026-06-08-issue-166-ksuid-generator-family-plan.md`
- Spec:
  `docs/superpowers/specs/2026-06-08-issue-166-ksuid-generator-family-spec.md`
- Issue: #166 `Port KSUID generator family`
- Review gate: Step 3-R, 7-Tier plan review

## Findings

| Tier | Reviewer | P0 | P1 | P2 | P3 | Summary |
|---|---:|---:|---:|---:|---:|---|
| Architect/implementer | architect subagent | 0 | 0 | 0 | 0 | Task order, API boundary, dependency leakage, and millis deferral are implementable. |
| Dependency/source parity | dependency-expert subagent | 0 | 0 | 0 | 0 | `segmentio/ksuid v1.0.4` with `FromParts` is suitable for seconds-only KSUID; `SetRand` is avoided. |
| Test/concurrency | test-engineer subagent | 0 | 0 | 2 | 0 | Add explicit out-of-range valid-Base62 invalid case and benchmark setup-cost guard. |
| Docs/release/evidence | local verifier | 0 | 0 | 0 | 0 | README locale set, CHANGELOG/WIP, validation commands, PR metadata, and Step 6-R are assigned. |

## Repair

The two P2 test-plan findings were applied:

- `TestKSUIDRejectsInvalidInput` now explicitly includes an out-of-range
  27-character Base62 case above the Segment max encoded KSUID.
- `BenchmarkKSUIDNextString` now requires one generator created outside the
  benchmark loop and repeated `NextString()` measurement, with error handling.

## Evidence

- `git diff --check`: PASS.
- Spec review artifact:
  `docs/superpowers/reviews/2026-06-08-issue-166-ksuid-generator-family-spec-review.md`.
- Follow-up #171 exists with assignee `debop`, milestone `0.6.1`, labels
  `type: task`, `priority: p1`, `area: utilities`.
- Plan references validation:
  `go test -count=1 ./id`, `go test -race -count=1 ./id`,
  targeted KSUID/stress tests, benchmarks, `go test -count=1 ./...`, and
  `make ci`.

## Gate Verdict

PASS.

P0=0 P1=0. Step 4 implementation is unblocked.
