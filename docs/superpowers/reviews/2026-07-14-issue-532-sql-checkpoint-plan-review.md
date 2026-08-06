# Issue #532 PostgreSQL Batch Checkpoint Plan Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

## 범위

- Plan: `docs/superpowers/plans/2026-07-14-issue-532-sql-checkpoint-plan.md`
- Reviewed commit: `4480cac0fe3e36ce0e232e41cc22c83fc96ab4a4`
- Reviewed SHA-256: `0d94a6c314c5500be42125308700de7cf639f4176da394223eb90b25e54c264e`
- Approved spec: `docs/superpowers/specs/2026-07-14-issue-532-sql-checkpoint-design.md`
- Base: `origin/develop@873555fdd34d66c8cb85c869898017ea0820f1c0`
- Artifact kind: implementation plan

## Initial findings and repairs

| Priority | Lens | Finding | Resolution |
|---|---|---|---|
| P2 | Performance | The plan did not explicitly defer benchmark/capacity work or assert exact hot-path operation counts. | Issue #560 owns throughput and capacity work. Load, non-empty Commit, empty Commit, and ownership-probe failure now have exact query/guard/CAS/Commit counts. |
| P2 | Stability | Two concurrent callbacks could collide on the same business idempotency key before reaching checkpoint CAS. | The PostgreSQL race test uses different business keys and a barrier immediately before CAS, preserving a deterministic checkpoint winner and conflict loser. |
| P2 | Security | The fixture could apply schema as admin instead of proving the production bootstrap order and least-privilege transition. | The security fixture executes PUBLIC CREATE revoke, migration-owner apply, catalog preflight, pre-grant denial, minimal grants, and post-grant allow/deny checks in order. |
| P2 | Security | Authenticated codec/encryption and KeyID diagnostic boundaries were not README contract markers. | Both locale contracts now require authenticated codec/encryption guidance and prohibit KeyID use for authorization or metric labels. |
| P2 | Operator/Ops | Observability signals and runbook regression markers were too implicit. | The plan enumerates low-cardinality outcomes, unknown classes, cancellation/latency, `sql.DBStats`, and raw-KeyID restrictions, then validates bootstrap, recovery, quiesce, shutdown, canary, rollback, and retention markers. |
| P2 | Developer/API | The deliberate external unkeyed-literal fixture was outside normal `go test ./...` and could be skipped after review repairs. | The Makefile test target runs the fixture with vet disabled, so `make test`, `make ci`, focused verification, and repair verification all enforce source compatibility. |
| P2 | Developer/API | Callers could interpret retry and skip policies as transaction retry controls. | Go doc and both locale READMEs must state that the policies apply only to processor failures and never to callback, CAS, Commit, or unknown-outcome errors. |

## 최종 재실행 결과

All completed lanes reviewed the exact plan commit and SHA-256 above. Three available review agents
were reused across two bounded waves. The security and user/caller agent repeatedly exceeded the
bounded response window; those attempts were closed and recorded as `lane timed out; main
integration fallback performed`. The main session completed those two perspectives read-only on the
same exact hash and owns the final integration verdict.

| Perspective | P0 | P1 | P2 | P3 | Result |
|---|---:|---:|---:|---:|---|
| Performance | 0 | 0 | 0 | 5 | PASS. Operation counts, payload bounds, pool drain, sequential stress, and issue #560 deferral are explicit. |
| Stability | 0 | 0 | 0 | 0 | PASS. Consumed-input chunking, CAS, ownership proof, panic/cancellation, restart, and compatibility repair gates converge. |
| Security | 0 | 0 | 0 | 0 | PASS via bounded main fallback. Fixed SQL, privilege bootstrap, catalog/ACL checks, codec limits, KeyID, and panic/error redaction remain fail closed. |
| Operator/Ops | 0 | 0 | 0 | 5 | PASS. Observable outcomes, recovery drills, quiesce, HA/RPO, pool/shutdown, retention, canary, and rollback are contract-checked. |
| Developer/API | 0 | 0 | 0 | 0 | PASS. The additive API, persistent unkeyed compatibility fixture, policy boundaries, typed errors, examples, and dependency discipline are explicit. |
| User/Caller | 0 | 0 | 0 | 0 | PASS via bounded main fallback. Setup, safe callback use, unknown-outcome recovery, bilingual docs, the mandatory diagram, live CI, and merge approval form one coherent path. |

The P3 notes are positive implementation evidence, not deferred defects. No actionable P2 remains.

## 메인 세션 통합 판정

The plan is executable in dependency order and preserves the approved stability-first design: each
business write and checkpoint CAS share one caller-configured PostgreSQL transaction, while any
unproven transaction ownership becomes an explicit quiesce-and-reconcile barrier. It keeps the
legacy `StepOptions` and `NewStep` surface source-compatible, makes the new API additive, and turns
the highest-risk transaction, recovery, privilege, pool, documentation, and diagram requirements
into deterministic tests or validation commands. The terminal workflow creates the PR, waits for
live CI, and stops for explicit merge approval.

P0=0 P1=0 P2=0 P3=10
