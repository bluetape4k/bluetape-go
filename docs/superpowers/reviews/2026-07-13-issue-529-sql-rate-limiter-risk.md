# Issue #529 PostgreSQL Rate Limiter Risk Prediction

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

## Baseline

- Branch: `feat/issue-529-sql-ratelimit`
- Base: `origin/develop@58a2e7a274408aaac964ad5eb46d837a292a9dfb`
- Go: `go1.26.5 darwin/arm64`
- pgx: `v5.9.2`
- Testcontainers: `v0.42.0`
- Pre-source full baseline: `go test -count=1 ./...` PASS
- Task 0 slice: `go test -count=1 ./ratelimit/... ./redis/...` PASS
- Artifact commits before source work: `e7c17b2`, `ae72179`

## 위험 표

| Risk | Trigger | Signal | Prevention | Recovery | Owner |
|---|---|---|---|---|---|
| First-insert race | Concurrent new key | More than burst admitted | One `INSERT ... ON CONFLICT` statement | Stop rollout; repair query; rerun exact stress | Provider |
| Fractional starvation | Frequent sub-microtoken polls | Refill never progresses | Persist `numeric(30,6)` carry | Repair arithmetic and replay precision tests | Provider |
| Arithmetic overflow | Extreme options/elapsed time | Negative or scan overflow | Checked int64 conversion, exact numeric clamp, duration saturation | Reject config or repair conversion | Provider |
| Stale lock observation | UPSERT waits for row lock | `updated_at` regresses, double refill | `greatest(stored, observed)` effective time | Stop rollout; restore monotonic query | Provider |
| Configuration mismatch pressure | Mixed options on one bucket | Locks/WAL/error spike | Fail closed without quota/config update; alert mismatch | Quiesce caller; migrate/delete/rotate namespace | Caller/Ops |
| Pool starvation | Hot Allow plus cleanup | WaitCount/latency/timeout growth | Bounded cleanup, small workers, caller timeouts | Pause cleanup; restore pool headroom | Caller/Ops |
| Lost response after debit | Transport/scan failure | Zero result with unknown outcome | Typed root commit-unknown; no retry | Absorb one debit or wait full-refill window | Caller |
| Cancellation boundary error | Cancel before/during/after SQL | Late debit or discarded confirmed result | Preflight, real lock cancel, post-scan confirmed result | Repair boundary; rerun race/count tests | Provider |
| Cleanup/Allow race | Expiry delete overlaps refresh | Refreshed row deleted incorrectly | One locked bounded DELETE with `SKIP LOCKED` | Pause cleanup; repair predicate/ordering | Provider |
| Cleanup backlog | Scheduler absent/slow | Oldest expiry/table size growth | Cadence below IdleTTL and bounded run budgets | Increase safe cadence after pressure check | Caller/Ops |
| Index/WAL/autovacuum pressure | Every Allow updates expiry index | WAL/dead tuples/index growth | Moderate-QPS positioning and pressure gates | Pause cleanup/canary; rollback provider | Ops |
| Public-schema hijack | Untrusted schema CREATE | Wrong owner/shape/object type | Verify owner, revoke PUBLIC CREATE before DDL, catalog preflight | Fail deployment; remove hostile object as migration owner | Security/Ops |
| Privilege inheritance | Runtime inherits elevated role | DDL unexpectedly succeeds | Effective direct/inherited/PUBLIC catalog tests | Revoke membership/grants; rotate runtime role | Security/Ops |
| RLS/trigger drift | Manual schema mutation | Hidden rows or altered writes | Require RLS off and zero user triggers | Fail readiness; restore verified schema | Security/Ops |
| Active-key cardinality abuse | Unbounded attacker identities | Persistent row growth | Caller authorization/cardinality/create-rate controls | Block source; rotate namespace if required | Caller/Security |
| Mixed-provider extra burst | Redis/local/SQL serve same quota | Multiple full bursts admitted | Independent canary namespace and single active provider | Quiesce old provider; wait refill window or approve budget | Caller/Ops |
| Replica routing | Pool reaches read replica | Read-only/stale failures | Writable-primary preflight on every endpoint | Remove endpoint; rollback canary | Ops |
| Failover replay/fencing | Old/new writers overlap | Duplicate debit or split brain | Fence old writer; prohibit replay; controlled exercise | Stop promotion; restore single writer | Ops |
| Async WAL loss | Promotion after non-durable debit | Over-admission after failover | Record synchronous_commit/RPO boundary | Rollback; account approved lost-debit budget | Ops |
| Testcontainers leakage | Pool/conn outlives fixture | Hangs, port/resource exhaustion | LIFO cleanup, bounded contexts, serial runs | Terminate fixture; rerun from scratch | Tests |
| Bilingual documentation drift | Public behavior changes | Conflicting caller guidance | Contract tests and paired commits | Repair both locale files before PR | Docs |

## 중단 조건

- Any P0/P1 review finding, exact-admission failure, worker leak, schema/privilege drift, or missing
  command exit code blocks the next task.
- A first failure that passes once on retry remains a stability investigation, not flaky evidence.
- No benchmark or capacity claim is permitted without a separate reproducible benchmark artifact.
