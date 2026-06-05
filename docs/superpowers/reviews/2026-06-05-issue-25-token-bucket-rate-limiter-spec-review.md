# Issue #25 Spec Review

Issue: #25
Milestone: 0.3.0
Date: 2026-06-05
Reviewed spec: `docs/superpowers/specs/2026-06-05-issue-25-token-bucket-rate-limiter-spec.md`
Research: `docs/research/2026-06-05-issue-25-token-bucket-rate-limiter.md`

## Step 2-R Scope

- Local token-bucket API, state model, and HTTP middleware.
- Redis-backed limiter API and Lua script boundary.
- Stress, cancellation, benchmark, and documentation requirements.
- Repository fit against `lock/redis`, `cache/rediscoord`, `resilience/http.go`,
  and `testing/concurrency`.

## Perspective Findings

| Perspective | P0 | P1 | P2 | P3 | Notes |
|---|---:|---:|---:|---:|---|
| Developer/API | 0 | 0 | 1 | 0 | Added unexported test clock requirement so refill/idle cleanup tests avoid sleeps. |
| Security | 0 | 0 | 1 | 0 | Added explicit proxy-header non-trust boundary for default remote-IP keying. |
| Ops/SRE | 0 | 0 | 1 | 0 | Added explicit microtoken overflow validation requirement. |
| User/caller | 0 | 0 | 0 | 0 | Rejection-vs-error semantics and custom `KeyFunc` are explicit. |

## 7-Tier Review

| Tier | P0 | P1 | P2 | P3 | Evidence |
|---|---:|---:|---:|---:|---|
| Tier 1 Security | 0 | 0 | 1 | 0 | Default remote-IP key now excludes proxy headers and tells users to provide trusted tenant keying. |
| Tier 2 Ops/SRE | 0 | 0 | 1 | 0 | Redis backend errors remain errors; idle TTL and overflow limits are required. |
| Tier 3 Structural impact | 0 | 0 | 0 | 0 | New `ratelimit` and `ratelimit/redis` packages avoid changing cache/resilience/lock contracts. |
| Tier 4 API quality | 0 | 0 | 0 | 0 | API uses Go `context.Context`, `net/http`, and existing Redis subpackage style. |
| Tier 5 Tests/types/silent failure | 0 | 0 | 1 | 0 | Test clock requirement added for deterministic refill and cleanup. |
| Tier 6 Performance/stability | 0 | 0 | 1 | 0 | Local benchmarks and hot-path rejection diagnostics required; Redis benchmarks kept opt-in/follow-up. |
| Tier 7 Docs/release/evidence | 0 | 0 | 0 | 0 | README pair, package READMEs, CHANGELOG, WIP, research index, and lessons are required. |

## Integrated Findings

| ID | Severity | Finding | Resolution |
|---|---|---|---|
| S2R-1 | P2 | Refill and idle cleanup tests need deterministic time control. | Added unexported test clock constructor requirement to spec and plan. |
| S2R-2 | P2 | Redis microtoken math needs explicit overflow validation. | Added safe positive `int64` microtoken validation requirement. |
| S2R-3 | P2 | Default remote-IP keying could imply proxy-header trust. | Added explicit non-trust boundary and custom `KeyFunc` guidance. |

## Convergence Verdict

- P0: 0
- P1: 0
- P2: 3 resolved in the spec/plan draft
- P3: 0

Step 2-R gate status: PASS. The spec is ready for Step 3 plan review.
