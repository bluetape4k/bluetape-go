# Issue #529 PostgreSQL Rate Limiter Spec Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

## 범위

- Spec: `docs/superpowers/specs/2026-07-13-issue-529-sql-rate-limiter-design.md`
- Reviewed SHA-256: `0eb3e1c9a56219db110ebe4aa8c062c09bf187b959752a62fd246f3877313ffa`
- Base: `origin/develop@58a2e7a274408aaac964ad5eb46d837a292a9dfb`
- Research basis: issues #500, #527, #528, #529 and PostgreSQL `INSERT`,
  `RETURNING`, server-time, and exact-numeric contracts linked by the spec
- Artifact kind: spec

## Initial findings and repairs

| Priority | Lens | Evidence | Required edit | Resolution |
|---|---|---|---|---|
| P1 | Performance | Integer micro-token refill could discard sub-quantum progress under frequent polling. | Preserve fractional refill state and floor only the observable remaining count. | `numeric(30,6)` state and a repeated sub-quantum test are required. |
| P1 | Stability | Cancellation after linearization and cleanup response loss did not have an explicit outcome boundary. | Separate pre-dispatch, in-flight/scan, and post-scan cancellation; define cleanup count on errors. | Pre-dispatch returns the original context error; uncertain completion returns zero plus commit-unknown; confirmed scan wins; cleanup errors return zero count. |
| P1 | Security | Runtime values, hostile pre-existing schema, and unbounded identities needed stronger controls. | Require positional binds, catalog validation, least privilege, and hard namespace/key/cleanup bounds. | Fixed SQL, `bytea` identity, catalog preflight, role restrictions, and hard limits are specified. |
| P1 | Operator/Ops | Provider cutover, HA fencing/RPO, telemetry, and cleanup pressure gates were incomplete. | Add measurable rollout/rollback and writable-primary operating requirements. | Independent canary namespaces, single-provider cutover, fencing/RPO exercise, baseline-relative gates, and bounded cleanup controls are specified. |
| P1 | Developer/API | Provider-neutral error inspection, nil-context compatibility, arbitrary key bytes, and zero-result invariants were incomplete. | Define a root error contract and preserve existing provider behavior. | Root `ErrCommitUnknown`/`OperationError`, Redis compatibility, `bytea` keys, nil-context normalization, and zero results on all `Allow` errors are specified. |
| P1 | User/Caller | Caller misuse around configuration changes, replay, DB ownership, and mixed providers needed explicit guidance. | Add migration, replay, ownership, and cutover guidance. | Configuration mismatch requires migration or namespace rotation; unknown outcomes are not replayed; DB lifetime stays caller-owned; mixed-provider quota serving is prohibited. |

All initial P2 findings were either incorporated or made explicit: expiry-index
updates are documented as a HOT/WAL/autovacuum cost, duration calculations
saturate safely, public symbols require Go docs/examples, and configuration
mismatch is described as a quota/configuration no-op that can still incur row
lock and WAL/write overhead.

## Rerun results

| Perspective | P0 | P1 | P2 | P3 | Result |
|---|---:|---:|---:|---:|---|
| Performance | 0 | 0 | 0 | 0 | Independent rerun cleared the refill precision finding; its final P2 about an overly strong no-WAL mismatch claim was repaired before this reviewed hash. |
| Stability | 0 | 0 | 0 | 0 | Independent rerun passed the cancellation, cleanup, arithmetic, and lifecycle boundaries. |
| Security | 0 | 0 | 0 | 0 | Lane unavailable/thread limit; main integration fallback performed. Fixed identifiers, positional binds, catalog/role validation, redaction, and hard input/cardinality guidance close the initial findings. |
| Operator/Ops | 0 | 0 | 0 | 0 | Lane unavailable/thread limit; main integration fallback performed. Migration ownership, cleanup pressure, telemetry, HA fencing/RPO, and cutover/rollback are testable requirements. |
| Developer/API | 0 | 0 | 0 | 0 | Lane unavailable/thread limit; main integration fallback performed. The API is additive, implements the root interface, preserves Redis inspection compatibility, and makes DB ownership explicit. |
| User/Caller | 0 | 0 | 0 | 0 | Lane timed out; main integration fallback performed. Misuse, unsupported topology, migration, replay, cleanup, and provider-switch guidance are explicit. |

## 메인 세션 통합 판정

The spec fixes the public package boundary, exact token arithmetic, one-statement
linearization, caller-owned schema and cleanup, failure classification, and
operational promotion contract without adding an ORM or runtime dependency.
Alternatives and non-goals are explicit, every acceptance criterion is testable,
and English/Korean documentation parity is part of the DoD. No open design
decision remains for implementation planning.

P0=0 P1=0 P2=0 P3=0
