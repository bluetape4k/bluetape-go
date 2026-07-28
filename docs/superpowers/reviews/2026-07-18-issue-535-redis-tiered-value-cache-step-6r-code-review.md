# Issue #535 Redis Tiered Value Cache Step 6-R Code Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

이슈: #535

날짜: 2026-07-18

기준: `origin/develop` at `3684299c8e9cbcfd319e4dd73a556d8f9e2463ff`

Reviewed implementation SHA: `c2b8674f5c7192a5e751f65c469198259835d3ff`

게이트: six independent perspectives plus main-session integration.

## 수렴 이력

The first full implementation review examined `9e009a12209a8c0326aedba7c269c3b7943746a7`.
It found no P0, but identified missing hostile-concurrency and real-Redis
failure proofs, incomplete error-format and nested-clear diagnostics, and
operator/caller documentation gaps.

The review repairs converged through these exact commits:

| Commit | Decision |
|---|---|
| `1a7749176bc74f6857692c702d129b39357ac78c` | Added hostile concurrency, Redis cancellation/failure, redaction, nested progress, L1 provenance, and operations evidence. |
| `80511d8c5a03facb0b80a8f788c8fa2aad10f7e1` | Proved ticket admission, post-transition single invocation, and blocked publication. |
| `9dd9c1f419bb907c5f6ce2f246b7aa383f6b4906` | Made Redis/TLS baselines, timeout sizing, blocked alerts, and recovery executable in both READMEs. |
| `f329c92194680884af4261c620d6836c2e070d70` | Aligned public examples, field GoDoc, migration guidance, and the exact error surface. |
| `c2b8674f5c7192a5e751f65c469198259835d3ff` | Proved the example's separate clear-admin ACL identity. |

No independent lane timed out in the terminal exact-head review; main-session
fallback was not required.

## 최종 정확한 HEAD 결과

| 계층 | 관점 | 판정 | P0 | P1 | P2 | P3 |
|---|---|---:|---:|---:|---:|---:|
| 1 | Performance | PASS | 0 | 0 | 1 | 0 |
| 2 | Stability | PASS | 0 | 0 | 1 | 0 |
| 3 | Security | PASS | 0 | 0 | 0 | 0 |
| 4 | Operator/Ops | PASS | 0 | 0 | 1 | 0 |
| 5 | Developer/API | PASS | 0 | 0 | 0 | 1 |
| 6 | User/Caller | PASS | 0 | 0 | 0 | 0 |
| Main | Integration | PASS | 0 | 0 | 3 | 1 |

Every terminal lane reviewed the same implementation SHA
`c2b8674f5c7192a5e751f65c469198259835d3ff`.

## Accepted Non-Blocking Findings

1. Performance P2: healthy parallel L1 hits briefly share the decorator-wide
   `localState` mutex for admission, classification, and release. The mutex is
   not held over L1, Redis, serializer, or loader work. Issue #560 owns the
   parallel scaling, throughput, allocation, and mutex-profile evidence.
2. Stability P2: the admission/publication test uses the package-private ticket
   seam and synthetic operation labels. Actual loader, `SET`, and `DEL` call
   sites were inspected and preserve the required order, but a future shared
   admitted-action helper or AST-level check could couple the proof more tightly.
3. Operator P2: the public example sets `DialTimeout`, `ReadTimeout`, and
   `WriteTimeout` but omits `PoolTimeout`. Both package READMEs require all four;
   this is copy/paste hardening, not an implementation or operations-contract
   blocker.
4. Developer P3: zero-value tests cover tiered read, mutation, clear, and local
   management methods, while `GetOrLoad` and `GetOrLoadDefault` are covered by
   the same pre-use validation path rather than direct zero-value calls.

The pre-PR review accepted a conditional second `EXISTS` round trip after an
empty `GETRANGE`. Step 7-R later proved that the two commands did not form one
Redis snapshot and reopened this decision as a P1; see the repair record below.

## Step 7-R PR Review Repair

PR #610 was created at `53f71ca2e86ea2946ff0fd9badf1eb13db346124`.
The first post-PR performance and security lanes passed with no blockers, but
the stability lane found one P1: another client could create a non-empty value
between an empty `GETRANGE` and `EXISTS`, causing `ValueCache.Get` to fabricate
an empty hit and allowing `TieredCache` to publish it into L1.

The defect was reproduced against two real Redis clients by pausing the reader
after the first `GETRANGE`. The original implementation failed with
`Get() = ""/<nil>` when the expected value was `"created"`. The repair keeps the
one-command non-empty path but re-runs bounded `GETRANGE` plus `EXISTS` inside
one `MULTI`/`EXEC` transaction for an ambiguous empty first result. Restricted
ACL evidence was extended to require and prove `MULTI`/`EXEC` along with the
existing read commands.

Repair evidence:

- cross-client create interleaving test: RED on `53f71ca`, then PASS;
- restricted ordinary-identity empty/miss test: RED without transaction ACL,
  then PASS with `MULTI`/`EXEC`;
- full `cache/redisvalue` normal and race suites: PASS;
- spec, plan, README locale pair, documentation parity, and lesson updated;
- affected Step 7-R perspectives and full CI require rerun on the repair head.

## 검증 증거

- Admission/documentation focused tests: `-count=20` — PASS.
- Admission/documentation focused race tests: `-race -count=5` — PASS.
- `go test -p 1 -count=1 ./cache/redisvalue` — PASS.
- `go test -race -p 1 -count=1 ./cache/redisvalue` — PASS.
- Redis 7.4 Testcontainers integration, including cancellation, ACL, provider
  failure, blocked repair, pointer isolation, clear, and versioning — PASS.
- `make fmt-check` — PASS.
- `make tidy-check` — PASS.
- `make vet` — PASS.
- `make lint` — PASS, `0 issues`.
- `make ci` on the reviewed implementation SHA — PASS, including repository
  normal tests, race tests, and Testcontainers-backed packages.
- `git diff --check origin/develop...c2b8674f5c7192a5e751f65c469198259835d3ff` — PASS.

## 메인 통합 판정

PASS.

- P0 = 0
- P1 = 0
- Accepted P2 = 3
- Accepted P3 = 1
- The implementation matches the approved L1-reference/L2-serialization
  boundary, default-plus-per-cache override model, `TieredCache` decorator,
  direct-primary Redis topology, and RESP3 #536 separation.
- Stop condition reached: implementation and review evidence are complete;
  push and PR creation remain outside the current authority.

## DoD

| 항목 | 상태 |
|---|---|
| Six independent perspectives covered | Done. |
| Same exact implementation SHA reviewed | Done: `c2b8674f5c7192a5e751f65c469198259835d3ff`. |
| Main integration review completed | Done. |
| P0/P1 normalized | Done: `P0=0 P1=0`. |
| Accepted P2/P3 recorded | Done. |
| Targeted, race, integration, static, and full CI evidence | Done. |
| Push or PR side effect | Not performed; authority gate preserved. |
