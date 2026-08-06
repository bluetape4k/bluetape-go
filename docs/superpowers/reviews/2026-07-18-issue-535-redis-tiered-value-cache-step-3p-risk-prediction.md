# Issue #535 Step 3-P Risk Prediction

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

## 범위

This checkpoint predicts implementation and verification risks for the approved
Redis L2 `ValueCache[V]` and process-local `TieredCache[V]` decorator. The
approved specification and plan remain authoritative; this document records the
signals, mitigations, rollback boundaries, and rerun points required by the
Type A workflow before implementation.

## 예측 위험

| Risk | Early signal | Required mitigation | Rollback boundary | Rerun point |
| --- | --- | --- | --- | --- |
| Oversized Redis values amplify reads or decoder work | `GET` allocates the full payload, corrupt/large payload tests exceed configured bounds | Use bounded `GETRANGE`, distinguish exact-bound values with conditional `EXISTS`, reject before deserialize | Revert the provider task without touching the public tiered API | Task 2 focused unit tests and Task 9 Redis integration |
| Same-key ABA or retained coordination state leaks memory | waiter totals drift, map entries remain after completion, repeated tests flake | Generation-tagged coordinator entries, exact owner deletion, waiter release tests | Revert coordinator internals while preserving provider tasks | Task 4 repeated and race tests |
| Late L2 reads resurrect invalid local values | a read admitted before mutation populates L1 after delete/clear/block | Admission tickets, generation/epoch validation, local barrier, fail-closed repair state | Disable tiered population while preserving L2 correctness | Tasks 5, 6, 8 adversarial ordering tests |
| Redis-first mutation has an ambiguous commit outcome | context/network error occurs after Redis may have applied the command | Mandatory package-owned local cleanup, blocked state when cleanup cannot be proven, explicit recovery API | Keep Redis provider usable and block local tiered serving | Task 8 fault-injection and recovery tests |
| Caller cancellation prevents required cleanup | local entries or coordinator state survive a timed-out caller | Separate caller wait budget from bounded background cleanup context | Fall back to blocked local state and require recovery | Tasks 7, 8 cancellation and no-late-side-effect tests |
| Namespace clear misreports progress or loops on SCAN | cursor repeats, deleted count diverges, docs imply percentage completion | Sequential bounded `SCAN MATCH` plus `UNLINK`; document `ScannedKeys` as matching keys returned so far only | Keep per-key operations and withhold `Clear` | Tasks 3, 9, 11 clear tests and documentation review |
| Serializer concurrency is hidden behind a package lock | race test passes only after global serialization; throughput collapses | Require immutable caller-owned concurrent-safe serializer; no package-global serializer lock | Reject unsafe serializer use at the caller boundary | Tasks 2, 10 race and allocation evidence |
| Callers assume coherent multi-process near-cache invalidation | README or examples describe RESP3 tracking/coherence | State process-local L1 and stale-read boundary; defer RESP3 tracking to #536 | Remove tiered marketing and expose only explicit composition | Tasks 11 and 12 public-contract review |

## Implementation Stop Conditions

- Stop the current task on any unexplained RED-to-GREEN transition, retry-only
  pass, leaked coordinator state, late L1 resurrection, unbounded Redis read, or
  serializer lock introduced by the package.
- Preserve completed earlier tasks and repair only the failing task boundary.
- Keep PR creation blocked until Step 6-R reports P0=0 and P1=0 and every
  applicable verification command has fresh evidence.
