# Issue #536 RESP3 CLIENT TRACKING Spike Step 3-R Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

날짜: 2026-07-18 KST
이슈: [#536](https://github.com/bluetape4k/bluetape-go/issues/536)
Reviewed plan: `docs/superpowers/plans/2026-07-18-issue-536-resp3-client-tracking-spike-plan.md`
Reviewed test spec: `docs/superpowers/specs/2026-07-18-issue-536-resp3-client-tracking-spike-test-spec.md`
Final reviewed commit: `3d7567b13ebc3a427771734e42aa9b980a7d8388`

## 통합 판정

`PASS — P0=0 P1=0 P2=0`

Six independent lanes reviewed the same final commit. The main session checked
the plan and test specification against the approved Type B boundary, current
repository APIs, go-redis/v9 v9.20.0, Testcontainers v0.42.0, and the Step 2-R
design. No production implementation, exported API, dependency, background
pump, or reconnect subsystem is authorized by this review.

## 최종 정확한 커밋 결과

| Lane | P0 | P1 | P2 | Verdict |
|---|---:|---:|---:|---|
| Performance | 0 | 0 | 0 | PASS |
| Stability/concurrency | 0 | 0 | 0 | PASS |
| Security | 0 | 0 | 0 | PASS |
| Operator/Ops | 0 | 0 | 0 | PASS |
| Developer/API | 0 | 0 | 0 | PASS |
| User/caller | 0 | 0 | 0 | PASS |
| Main-session integration | 0 | 0 | 0 | PASS |

## 검토 중 해결한 발견 사항

### Findings from `5a850e4`, resolved by `fcfe6fa`

- Replaced the raw in-flight `WaitGroup` with a mutex-protected callback gate;
  shutdown now closes admission before waiting and proves a held callback is
  the reason the wait remains blocked.
- Separated the 100 ms injected timeout from 1-second test watchdogs.
- Added bounded two-key success and three-key middle-failure cases. Partial
  failure stops later keys and attempts one full `ClearLocal` repair with an
  independent 250 ms context.
- Made the physical key example compile-complete by checking each public
  builder result and using `redis.Key.Value`.
- Replaced inconsistent fixture helper wording with direct use of the existing
  upstream Redis module and actual container image inspection.
- Corrected the performance statement: one tracking owner maintains one
  dedicated socket and each explicit drain adds one Redis command.

### Findings from `fcfe6fa`, resolved by `72dd0ee`

- Required gate-closed callbacks to emit exactly one redacted
  `reason=shutdown` observation before returning the low-cardinality sentinel.
- Bounded image mismatch diagnostics to the configured image field rather than
  dumping the complete container configuration.
- Added actionable container start errors through
  `testcleanup.FormatStartError`.
- Added an idempotent, mutex-protected fixture closer registry. Sticky
  connections close before all four clients, and LIFO cleanup closes those
  resources before container termination.
- Added the missing empty key-list rejection case.
- Aligned the administrative endpoint wording with the directly owned
  disposable container.

### Finding from `72dd0ee`, resolved by `3d7567b`

- Corrected the architecture diagram so null invalidation remains a handler
  path while transport failure is detected by a tracked command and repaired
  by harness-owned L1 blocking and `ClearLocal` before replacement tracking.

## Main-Session Integration Check

- Every issue acceptance criterion maps to a named test or result-ledger row.
- The handler validates the whole frame before cleanup, uses an exact physical
  key allowlist, and never reverse-maps by prefix trimming.
- Multi-key cleanup has one bounded callback budget and an independent
  full-clear repair budget; failures remain redacted and observable.
- Shutdown cannot race a late `WaitGroup.Add`, unregister is not misrepresented
  as a quiescence barrier, and all owned resources have deterministic cleanup.
- Redis image identity, protocol, server information, connection IDs, and
  tracking state are captured as prerequisites rather than assumptions.
- RESP3 command-coupled delivery is not presented as autonomous coherence,
  performance evidence, or provider-wide compatibility.
- `redisnear.NewPubSub` remains the production strategy unless execution
  evidence and a separately approved Type A design justify another component.

## 검증

```bash
git diff --check 38364100af92d6da616ff89101109fc768e639a4..3d7567b13ebc3a427771734e42aa9b980a7d8388
go doc github.com/testcontainers/testcontainers-go/modules/redis.Run
go doc github.com/testcontainers/testcontainers-go.Container.Inspect
go doc github.com/bluetape4k/bluetape-go/internal/testcleanup.Register
go doc github.com/bluetape4k/bluetape-go/internal/testcleanup.FormatStartError
```

결과: PASS.

Runtime Redis behavior was intentionally not executed during Step 3-R. The
reviewed plan owns RED/GREEN implementation, serial Testcontainers repetition,
race verification, regression checks, and the final evidence ledger.
