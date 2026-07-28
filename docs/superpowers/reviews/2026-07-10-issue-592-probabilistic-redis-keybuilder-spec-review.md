# Issue #592 Probabilistic Redis Key Builder Spec Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

날짜: 2026-07-10 KST
게이트: Step 2-R
Spec: `docs/superpowers/specs/2026-07-10-issue-592-probabilistic-redis-keybuilder-spec.md`
Test spec: `docs/superpowers/specs/2026-07-10-issue-592-probabilistic-redis-keybuilder-test-spec.md`
Baseline: `9b8a0a1a80a041b0796bbe27ff9ee987db159c4b`

## 증거

- CodeGraph located `buildKeys` at `probabilistic/redis/keys.go:18`; direct
  source inspection completed the currently incomplete CodeGraph symbol index
  for the HLL and shared-builder symbols.
- GNO design evidence: Redis probabilistic filter design `#182` and Redis
  foundation spec `#569` preserve the same Cluster hash-tag and redaction
  concerns.
- `git diff --check` passed for the spec artifacts.
- Native reviewer spawning is not exposed in this session. This is a small,
  tightly coupled construction-only scope, so the main session performed the
  required six independent local-equivalent perspective passes and integration.

## 6개 관점 발견 사항

| Perspective | Reviewed scope | P0 | P1 | P2 | P3 | Result |
|---|---|---:|---:|---:|---:|---|
| Performance | `keys.go`, Redis command/key construction | 0 | 0 | 0 | 0 | No command, script, allocation-sensitive hot-path, or benchmark claim change is specified. |
| Stability | namespace validation, builder failure boundary, Testcontainers coverage | 0 | 0 | 0 | 0 | Local validation precedes shared construction; existing behavior/race coverage remains required. |
| Security | namespace/key/error redaction | 0 | 0 | 0 | 0 | Local sensitive-marker policy and 12-hex redacted ID are explicitly retained. |
| Operator/Ops | Cluster slot, rollback, migration, observability | 0 | 0 | 0 | 0 | Byte-identical key layout means no migration, key retirement, or runbook change. |
| Developer/API | package boundaries, Go error compatibility | 0 | 0 | 0 | 0 | Shared construction is adopted without exporting shared error types or changing `RedisError`. |
| User/Caller | valid/invalid inputs and compatibility | 0 | 0 | 0 | 0 | Caller namespace acceptance and all persisted key values remain stable; no README/API change is required. |

## 통합 판정

P0=0 P1=0 P2=0 P3=0

The spec is implementation-ready. The plan must require a private adapter that
maps any theoretically impossible fixed-prefix/structural-part builder failure
to a local opaque configuration error rather than wrapping a shared `redis`
validation error. This is an acceptance-criteria implementation detail, not a
new public behavior.

## 근거와 함께 거절함

| Rejected item | Rationale |
|---|---|
| Shared `Key.RedactedID` reuse | It is 24 hex characters and would change the public probabilistic diagnostic identifier. |
| Builder-owned namespace validation | It would bypass the provider's sensitive-input policy and alter caller-visible error behavior. |
| Benchmark run | No measurable performance contract changes; #560 owns comparison evidence. |
