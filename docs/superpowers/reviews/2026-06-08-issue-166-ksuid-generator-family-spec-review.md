# Issue #166 KSUID Generator Family Spec Review

## Scope

- Spec:
  `docs/superpowers/specs/2026-06-08-issue-166-ksuid-generator-family-spec.md`
- Issue: #166 `Port KSUID generator family`
- Review gate: Step 2-R, 7-Tier spec review
- Worktree: `.worktrees/issue-166-ksuid`

## Initial Findings

| Tier | Reviewer | P0 | P1 | P2 | P3 | Summary |
|---|---:|---:|---:|---:|---:|---|
| Structural/API | architect subagent | 0 | 1 | 0 | 0 | Seconds and millis APIs were both bare 27-character Base62 strings, so parse/time APIs could silently reinterpret the wrong family. |
| Dependency/source parity | dependency-expert subagent | 0 | 1 | 0 | 0 | Spec called millis `bluetape4k-compatible` while using Segment/repo Base62, but Kotlin uses a different custom `BytesBase62` alphabet/bit-stream encoder. |
| Test/concurrency | test-engineer subagent | 0 | 0 | 3 | 0 | Deterministic decoded-byte assertions, custom clock concurrency docs, and validation acceptance needed to be explicit. |
| Docs/release/evidence | local verifier | 0 | 0 | 0 | 0 | Issue metadata, docs update scope, validation commands, and PR metadata requirements were present. |

## Repair

The spec was updated to keep issue #166 seconds-only and defer millisecond
KSUID to #171.

Repair details:

- Standard seconds KSUID remains in #166.
- Millis APIs are explicitly excluded from #166.
- #171 owns exact Kotlin compatibility vs distinguishable representation vs
  documented non-compatibility.
- Test requirements now include exact decoded-byte assertions.
- Concurrency contract now covers custom entropy readers and custom clock funcs.
- Acceptance criteria now require test requirements and validation commands to
  pass before PR.

## Affected Rerun

| Tier | Reviewer | P0 | P1 | P2 | P3 | Verdict |
|---|---:|---:|---:|---:|---:|---|
| Structural/API | architect subagent | 0 | 0 | 0 | 0 | PASS |
| Dependency/source parity | dependency-expert subagent | 0 | 0 | 0 | 0 | PASS |
| Test/concurrency | test-engineer subagent | 0 | 0 | 0 | 0 | PASS |
| Docs/release/evidence | local verifier | 0 | 0 | 0 | 0 | PASS |

## Evidence

- `gh issue view 166`: assignee `debop`, milestone `0.6.0`, labels
  `type: task`, `priority: p1`, `area: utilities`.
- `gh issue view 171`: assignee `debop`, milestone `0.6.1`, labels
  `type: task`, `priority: p1`, `area: utilities`.
- `github.com/segmentio/ksuid@v1.0.4/ksuid.go`: standard KSUID is
  4-byte timestamp, 16-byte payload, 20 bytes total, 27-character Base62.
- `bluetape4k-projects/utils/idgenerators/.../Ksuid.kt`: Kotlin
  `Ksuid.Millis` uses 8-byte millis timestamp and 12-byte payload.
- `bluetape4k-projects/utils/idgenerators/.../BytesBase62.kt`: Kotlin millis
  uses a custom Base62 encoder and alphabet that is not Segment Base62.
- `git diff --check`: PASS.

## Gate Verdict

PASS.

P0=0 P1=0. Step 3 planning is unblocked.
