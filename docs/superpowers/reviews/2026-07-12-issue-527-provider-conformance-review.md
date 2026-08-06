# Issue #527 Step 6-R Provider Conformance Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

날짜: 2026-07-12
Milestone: 0.19.0
Reviewed implementation commit: `f2aea4924a6ed0b752f35830760e60daa93b054b`
Reviewed release-document commit: `1fa2f2271a807874672c30f14fc75e568075edfb`

## 판정

Final gate: P0=0 P1=0

The review found two release-document P1 issues: unsafe primary README recovery
snippets and a rollout contract available only in internal design material. The
snippets now use bounded same-owner cleanup and no-replay handling, and the
CHANGELOG plus every affected bilingual provider README links the public
v0.19.0 rollout runbook. Import/prose aliases were also aligned.

## 관점 요약

| Lane | P0 | P1 | P2 | Disposition |
|---|---:|---:|---:|---|
| Performance | 0 | 0 | 0 | Bounded cases, buffered exact-result channels, retry budget, and zero-allocation hot path pass. |
| Stability/reliability | 0 | 0 | 1 | A context-ignoring adapter worker cannot be forcibly killed; the runner returns within five seconds and the regression is subprocess-isolated. |
| Security | 0 | 0 | 0 | `lane timed out; main integration fallback performed`; targeted redaction, classifier, typed-error, owner, and token tests pass without forbidden marker exposure. |
| Operator/Ops | 0 | 0 | 1 | Public runbook now defines bounded labels, canary thresholds, two-TTL gates, and rollback completion; watchdog residual matches stability. |
| Developer/API | 0 | 0 | 0 | Neutral factory APIs, examples, `btredis` identifiers, and bilingual runbook links match the exported contracts. |
| User/caller | 0 | 0 | 0 | Bilingual snippets clean every non-nil lease, resign boundedly, and prohibit rate-limit replay after commit-unknown. |
| Main integration | 0 | 0 | 1 | All P1 fixes and the docs-only delta were rechecked; the watchdog limitation is accepted as non-blocking. |

## 검증 증거

The implementation commit passed:

```bash
go test -p 1 -count=1 ./leader/... ./lock/... ./ratelimit/...
go test -p 1 -race -count=1 ./leader/leadertest ./leader/redis ./leader/mongo \
  ./lock/locktest ./lock/redis ./ratelimit/ratelimittest ./ratelimit ./ratelimit/redis
make fmt-check
make tidy-check
make vet
make lint
TESTCONTAINERS_REUSE_ENABLE=false TESTCONTAINERS_RYUK_DISABLED=false make ci
```

After the docs-only review fixes, the focused provider/helper tests, helper race
tests, runnable examples, README heading/code-fence parity, link checks, and
`git diff --check` passed again.

The five-sample TokenBucket medians remained below the 10% investigation gate:
allowed 113.2 ns/op and rejected 75.33 ns/op, both at 0 B/op and 0 allocs/op.
The deterministic Redis retry budget remains below 12 attempts per second.

## Acceptance Traceability

| Acceptance | Evidence | Status |
|---|---|---|
| 1-2 Public runners, fixtures, named cases | `leadertest`, `locktest`, and `ratelimittest` exported harnesses and self-tests | PASS |
| 3-6 Existing provider adoption | Redis/Mongo single leader, Redis lock, and local/Redis limiter run the shared suites | PASS |
| 7 Cancellation, expiry, stale owner, concurrency | Reference/provider normal and race suites plus exact buffered result tests | PASS |
| 8 Compatibility and typed migration | Key/schema/token tests, typed wrappers, custom-token byte preservation, and caller audit | PASS |
| 9 GoDoc, examples, bilingual docs | Runnable helper examples and paired READMEs with equivalent recovery actions | PASS |
| 10 Repository verification | Serial tests, race gates, and full `make ci` pass | PASS |
| 11 Review convergence | Step 2-R, Step 3-R, and this Step 6-R are P0=0/P1=0; Step 7-R follows on the live PR | PASS |
| 12 PR metadata and merge approval | Branch targets #527/milestone 0.19.0/assignee `debop`; live PR verification remains | PENDING PR |
| 13 Renewal/resign fault evidence | Lost-owner, partial resign, retry budget, redaction, and takeover tests | PASS |
| 14 Release migration and rollback | Caller audit, CHANGELOG, and bilingual v0.19.0 rollout runbook | PASS |
| 15 Lost-response outcomes/no replay | Confirmed, bare-context, and typed commit-unknown tests at actual mutation boundaries | PASS |

## Accepted Residual P2

Go cannot terminate an arbitrary goroutine whose adapter ignores context. Each
named conformance case therefore fails within five seconds, while the hostile
blocking-adapter regression runs in a subprocess so CI cannot wedge. A future
helper revision may thread a case-owned context through every evaluator, but
this does not weaken the provider contracts or block #527.
