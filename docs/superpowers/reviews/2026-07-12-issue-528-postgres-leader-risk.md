# Issue #528 PostgreSQL Leader Pre-Implementation Risk Record

Date: 2026-07-12
Milestone: 0.19.0
Spec commit: `f2ee638`
Plan commit: `693baf3`
Recorded before source changes: yes

## Step 3-R Gate

The approved implementation plan passed performance, stability, security,
operator/Ops, developer/API, user/caller, and main integration review with
P0=0 and P1=0. The performance lane did not return within the bounded review
window, so `lane timed out; main integration fallback performed`. Main
integration verified single-statement point operations, bounded retry and SQL
attempts, primary-only reconciliation, cleanup-pending recovery, schema/ACL
verification, least-privilege runtime testing, provider API consistency,
bilingual recovery guidance, diagram gates, and conservative rollback order.

## Environment Baseline

```text
go version go1.26.5 darwin/arm64
github.com/jackc/pgx/v5 v5.9.2
github.com/testcontainers/testcontainers-go v0.42.0
```

The pre-change repository test had previously produced one unrelated
`testing/TestCheckWaiterReleasedDiagnostics/wrong_error` mismatch. The fresh
baseline command below passed 20/20 in 0.662 seconds:

```bash
go test -count=20 ./testing -run '^TestCheckWaiterReleasedDiagnostics$'
```

## Predicted Risks

| Risk | Trigger | Signal | Prevention | Recovery | Owner |
|---|---|---|---|---|---|
| Absent-row upsert race | Multiple contenders campaign for a missing key | More than one successful owner or duplicate-key errors | One `INSERT ... ON CONFLICT ... WHERE ... RETURNING` statement and exact-winner test | Stop rollout, bounded resign winners, wait the maximum lease, revert SQL-boundary commit | Provider implementer |
| Pool or row-lock starvation | Exhausted pool, stalled transaction, or half-open connection | `DBStats.WaitCount/WaitDuration` grows; attempt or renew reaches its child deadline | Per-attempt contexts, renewal deadline, constrained-pool and blocked-row tests | Stop protected work, cancel/join renewal, bounded resign after DB recovery | Application operator |
| Retry herd | Many contenders observe one active lease or recover together | UPSERT operation count spikes or lock wait p99 rises | Token-jittered exponential backoff capped by lease and retry-count test | Halt canary, extend caller budget, revert backoff/lifecycle commit | Provider implementer |
| Stale insert-side timestamp | UPSERT waits on the unique row lock | New lease is shorter than configured or immediately expires | Recompute expiry with qualified `pg_catalog.clock_timestamp()` in the conflict update | Resign, wait lease expiry, revert query commit | Provider implementer |
| Renewal after resign | Resign overlaps an already-dispatched renewal | Renew count grows after delete or takeover is delayed | Atomically clear ownership, cancel/join exact generation, then token-delete | Retry same-elector resign; TTL/server-time expiry is final fallback | Provider implementer |
| Generation ABA | An old renewal loop completes after a new campaign | Old loop clears new `owned`/cleanup state | Generation and `done` identity checks plus race tests | Stop process, wait maximum lease, revert lifecycle commit | Provider implementer |
| Indeterminate acquire | Statement commits but response/context is lost | Own token exists while Campaign returns an error | Fresh bounded primary probe; own token confirms success; failed probe sets commit unknown | Preserve same elector, bounded resign, then full-lease wait | Caller |
| Indeterminate renewal | UPDATE commits but response is lost | Local ownership drops while stored lease may be extended | Renewal errors always clear `owned`, retain cleanup, and stop traffic | Same-elector bounded resign or conservative full-lease wait | Caller/operator |
| Indeterminate resign | DELETE commits but response is lost | `ErrCommitUnknown` after row may already be absent | Token-conditional delete and retry-safe already-absent success | Retry same elector with fresh bounded contexts | Caller |
| Replica-routed reconciliation | DSN or proxy sends reads to a lagging replica | Probe/Leader disagrees with mutations; false cleanup or split leadership | One writable-primary pool for every operation; HA preflight/canary | Fence non-authoritative writers, restore one primary, wait maximum lease | Database operator |
| Async failover lease loss | Promotion loses a recently committed row | New primary has no lease while old process still reports leader | Old-writer fencing and deployment-specific synchronous durability/canary | Stop all protected work, establish authoritative primary, conservative lease wait | Database operator |
| Public-schema hijack or drift | Pre-created relation, wrong owner/ACL, trigger, RLS, or missing PK | Catalog gate differs; least-privilege runtime test fails | Fixed qualified relation, exact catalog/ACL gate, protected schema, hostile-shape test | Block deployment; migration owner repairs schema before provider starts | Migration owner |
| Token or error leakage | Driver/hook cause contains DSN, identity, constraint, or token | Forbidden marker appears in rendered error/log | `leader.OperationError` redaction and PostgreSQL marker tests; no secrets in identities | Rotate exposed credentials, stop logging unwrapped cause, patch before rollout | Security owner |
| Unsafe lease margin or precision | `RenewInterval >= Lease` or impractically tiny durations | Constructor rejection or repeated immediate loss | Post-normalization ordering validation, microsecond ceiling tests, documented latency margin | Increase lease/renew settings and restart only after cleanup | Caller |
| Testcontainers leak or false PASS | Container/DB cleanup is skipped or test handle is lost | Docker resources remain; command lacks a final exit code | Serial fixtures, immediate cleanup registration, fresh reruns | Terminate leaked resources and rerun from scratch | Test owner |
| Expired-row growth | Dynamic leader keys accumulate indefinitely | Table cardinality, dead tuples, or autovacuum lag grows | Optional server-time grace cleanup and cleanup safety test | Run reviewed grace cleanup without touching live rows | Database operator |
| Diagram/source drift | Public sequence differs from implemented mutation phases | README art contradicts code or audit/PNG inspection | Source-backed one-asset loop and full DIA evidence ledger | Fix SVG, rerender/inspect PNG, do not ship stale art | Documentation owner |

## Stop Conditions

- Any unresolved P0/P1 review finding.
- More than one concurrent winner for a leader key.
- A renewal or resign race that leaves local leadership true after its safety
  budget or deletes a replacement owner.
- Replica-routed or multi-writer ambiguity without old-writer fencing.
- Schema/ACL gate mismatch, raw diagnostic marker leakage, race detector
  finding, missing Testcontainers cleanup, or any nonzero final verification
  exit code.
