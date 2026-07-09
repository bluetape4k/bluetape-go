# Issue #431 MongoDB Leader Research Review

Issue: [#431](https://github.com/bluetape4k/bluetape-go/issues/431)  
Branch: `feat/issue-431-leader-mongo-research`  
Baseline: `466745b`  
Date: 2026-07-09

## Scope

- `docs/research/2026-07-09-issue-431-leader-mongodb-storage.md`
- `docs/lessons/2026-07-09-issue-431-leader-mongo-research.md`
- `leader/README.md` and `leader/README.ko.md`
- `docs/research/README.md`
- `bluetape4k-wiki/research/2026-07-09-bluetape-go-leader-mongodb-storage.md`

## Evidence

- Issue #431 requires a research-first decision for MongoDB leader election
  storage before adding implementation work.
- Existing `leader` contracts separate single, group, and strategic elector
  responsibilities.
- Redis storage already uses different patterns for single, group, and
  strategic electors, so MongoDB should not bundle all variants into one first
  slice.
- MongoDB official docs support single-document atomic conditional writes, but
  TTL index behavior is cleanup-only and cannot be used as immediate lease
  validity.
- Follow-up issue [#485](https://github.com/bluetape4k/bluetape-go/issues/485)
  was created for the single-elector `leader/mongo` implementation only.

## 7-Tier Lanes

| Lane | Verdict | Notes |
|---|---|---|
| Performance | PASS | P0=0 P1=0. First slice avoids group/strategic fan-out and keeps one document per single leader key. |
| Stability | PASS | P0=0 P1=0. Renewal-loss and expired-document semantics are explicit implementation gates. |
| Security | PASS | P0=0 P1=0. Caller-owned collection and production write concern guidance avoid hidden credentials or clients. |
| Operator/Ops | PASS | P0=0 P1=0. TTL is documented as cleanup only; `majority` write concern is named as production guidance. |
| Developer/API | PASS | P0=0 P1=0. Proposed API remains idiomatic Go and reuses existing `leader.Options` contracts. |
| User/Caller | PASS | P0=0 P1=0. README points callers to the research boundary without claiming an implemented MongoDB backend. |
| Integration | PASS | P0=0 P1=0. Main-session review accepts a single-elector follow-up and defers group/strategic shapes. |

## Validation

| Command | Status | Evidence |
|---|---|---|
| `go test -count=1 ./leader` | PASS | Leader package tests passed. |
| `go test -race -count=1 ./leader` | PASS | Leader package race gate passed. |
| `go test -count=1 ./testcontainers/mongodb` | PASS | MongoDB fixture smoke test passed. |
| `git diff --check` | PASS | No whitespace errors. |
| `gno update` / `gno embed --collection bluetape4k-wiki` / representative `gno search` | PASS | New wiki note indexed and found by `leader MongoDB findOneAndUpdate lease_until` search; embed command exited 0 with a nonfatal Metal compile warning. |

## Findings

P0=0 P1=0

- P2 bounded: A MongoDB backend can use atomic single-document conditional
  writes for single-elector ownership, but TTL monitor cleanup must not be part
  of correctness.
- P2 bounded: Group and strategic variants are separate design problems and
  should not be bundled with the first MongoDB package issue.

## Residual Risk

The research does not run MongoDB contention benchmarks or implementation
tests. The future implementation PR still needs to prove server-time handling,
duplicate-key upsert races, failed-renewal behavior, and race-test stability.
