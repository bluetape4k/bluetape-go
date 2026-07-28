# Issue #590 Redis Rate Limiter Substrate Spec Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

## 범위

- Spec: `docs/superpowers/specs/2026-07-10-issue-590-ratelimit-redis-substrate-spec.md`
- Test spec: `docs/superpowers/specs/2026-07-10-issue-590-ratelimit-redis-substrate-test-spec.md`
- Current implementation: `ratelimit/redis/{limiter.go,options.go,limiter_test.go}`
- Shared dependency: `redis/errors.go`
- Review mode: local six-perspective equivalent. Native review-lane spawning
  is not exposed in this session; the main session independently applied every
  required perspective and owns the integration verdict.

## 발견 사항

| Perspective | P0 | P1 | P2 | P3 | Result |
|---|---:|---:|---:|---:|---|
| Performance | 0 | 0 | 0 | 0 | The decision changes error construction only. It retains one `Eval` call, the existing Lua script, and existing atomic Redis operation. Benchmark is correctly N/A. |
| Stability | 0 | 0 | 0 | 0 | Preflight cancellation stays outside the provider wrapper; the test plan preserves provider and late-context causes without timing-dependent network tests. |
| Security | 0 | 0 | 0 | 0 | `OpError` redacts the exact computed bucket key and provider text. Marker-based tests cover namespace and logical-key leakage. |
| Operator/Ops | 0 | 0 | 0 | 0 | Key layout, Redis state, TTL derivation, and rollback are unchanged; no storage migration or operational data rewrite is required. |
| Developer/API | 0 | 0 | 0 | 0 | No exported surface changes. The explicit low-cardinality labels and typed error preserve idiomatic `errors.Is`/`errors.As` use. |
| User/Caller | 0 | 0 | 0 | 0 | The spec deliberately preserves nonblank caller-key bytes and existing result/sentinel behavior, avoiding a compatibility-narrowing helper adoption. |

## 통합 판정

The design correctly rejects `KeyBuilder`, generic TTL validation, and the
compare-delete/extend script helpers because each has an incompatible input or
behavior contract. The remaining change is one narrow `Eval` error boundary
with deterministic typed/redacted diagnostics. Every acceptance criterion maps
to an existing or explicit new regression test.

P0=0 P1=0
