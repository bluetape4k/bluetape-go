# Issue #173 Step 2-R Spec Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

이슈: #173
Spec: `docs/superpowers/specs/2026-06-12-issue-173-distributed-jwt-keychain-repositories-design.md`
날짜: 2026-06-12

## 검토 계약

Step 2-R used six independent native subagent lanes plus this main-session
integration review.

| Tier | Perspective | Final P0 | Final P1 | Notes |
| --- | --- | ---: | ---: | --- |
| 1 | Performance | 0 | 0 | P2: Step 3 should pin expected Redis command-count and benchmark budget. |
| 2 | Stability | 0 | 0 | P2: Step 3 should choose a concrete configured `KeyTTL` leeway rule. |
| 3 | Security | 0 | 0 | Redis signing authority, namespace validation, DTO validation, and algorithm mismatch covered. |
| 4 | Operator/Ops | 0 | 0 | Rollout, rollback, runbook, Redis ACL/TLS/persistence, and inspection guidance covered. |
| 5 | Developer/API | 0 | 0 | Named composition and narrowed cancellation contract cleared final affected rerun. |
| 6 | User/Caller | 0 | 0 | Token-continuity migration is scoped out for #173; `TryParse` signature issue resolved. |

## P1 Corrections Applied

| Area | Change |
| --- | --- |
| Constructor context | Distributed constructors now accept `ctx context.Context` and return `(*DistributedProvider, error)`. |
| Cold-start rotation | Construction uses repository `Rotate(ctx, create, now)` for atomic ensure-current behavior. |
| Context promise | Spec limits context cancellation to repository IO/store decisions and states HMAC entropy/RSA generation are not context-interruptible. |
| Provider surface | `DistributedProvider` now uses private named composition `provider *Provider`, not anonymous embedding. |
| Redis hot path | Redis layout uses by-`kid` hash lookup and bounded payload decode. |
| Redis rotate | Redis rotate uses two-phase Lua CAS with no `create` call on current-hit fast path. |
| TTL retention | `KeyTTL=0` means no Redis expiration; configured TTL must not expire non-expired retained key material. |
| Namespace/security | Namespace is explicit and validated; Redis is documented as JWT signing authority requiring trusted Redis boundaries. |
| DTO safety | DTO version, algorithm, key family, key material, and payload size validation are required. |
| Rollout/user docs | Fixed/local token-continuity migration is explicitly out of #173 unless a future import/export design exists. |

## 메인 통합 판정

`P0=0 P1=0`

Step 2-R is closed. P2 items are accepted into Step 3 planning:

- Pin Redis command-count and benchmark budget expectations.
- Choose the configured `KeyTTL` safety rule: fixed safety margin, repository
  option, or reject configured `KeyTTL` unless retention can be proven.
- Turn README runbook requirements into concrete documentation tasks with safe
  Redis inspection commands and expected failure/recovery checks.

No open P0/P1 findings remain after affected reruns.
