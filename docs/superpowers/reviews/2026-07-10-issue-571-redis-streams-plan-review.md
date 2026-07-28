# Issue #571 Redis Streams Primitive Plan Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

## 범위

- Spec: `docs/superpowers/specs/2026-07-10-issue-571-redis-streams-spec.md`
- Test specification: `docs/superpowers/specs/2026-07-10-issue-571-redis-streams-test-spec.md`
- Plan: `docs/superpowers/plans/2026-07-10-issue-571-redis-streams-plan.md`
- Existing code: `redis/errors.go`, `audit/sqloutbox/redisstreams/publisher.go`,
  `testing/concurrency/goroutine_stress.go`.
- Review mode: local six-perspective equivalent because no native subagent
  invocation surface is exposed in this session.

## Iteration Findings

| Priority | Area | Finding | Resolution |
|---|---|---|---|
| P1 | Diagnostics | A simple joined multi-stream key can make distinct ordered key sets share a redaction input. | Require a deterministic length-delimited ordered aggregation before redaction; no raw component is formatted. |
| P1 | Validation | `XAddArgs.Values` is an interface, so a typed-nil map/slice can bypass an `== nil` check and fail after dispatch. | Require typed-nil value rejection and a unit test before command dispatch. |

## 수렴된 관점

| Perspective | P0 | P1 | P2 | P3 | Result |
|---|---:|---:|---:|---:|---|
| Performance | 0 | 0 | 0 | 0 | One direct command per helper, no package loop, and a bounded concurrent append stress test; #560 owns performance measurements. |
| Stability | 0 | 0 | 0 | 0 | TDD precedes implementation; cancellation, pending/ack, auto-claim, explicit retention, and serial Testcontainers checks are ordered before repository CI. |
| Security | 0 | 0 | 0 | 0 | Typed causes survive through `OpError`; formatted errors redact single and multi-stream keys plus provider text. |
| Operator/Ops | 0 | 0 | 0 | 0 | Documentation and sequence visual expose pending, replay, consumer shutdown, and retention ownership; rollback is a code revert with no topology migration. |
| Developer/API | 0 | 0 | 0 | 0 | Narrow interfaces follow exact go-redis argument ordering and return its native values; #533 changes only its append dispatch. |
| User/Caller | 0 | 0 | 0 | 0 | Verbatim keys/payloads, explicit trim/delete, and at-least-once duplicate semantics remain caller-visible. |

## 통합 판정

Every spec invariant has a mapped unit/integration/provider/documentation task
and a fresh verification command. Tasks are ordered RED -> GREEN -> Redis
integration/race -> provider migration -> docs/diagram -> repository checks ->
implementation review/PR. The plan does not depend on a later artifact and
does not expand #571 into a broker or a consumer service.

P0=0 P1=0
