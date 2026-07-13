# Issue #529 PostgreSQL Rate Limiter Plan Review

## Scope

- Plan: `docs/superpowers/plans/2026-07-13-issue-529-sql-rate-limiter-plan.md`
- Reviewed SHA-256: `76ca819a5cb1beb10040b34d283ae60a30a6aff3dd0751a365e970c4e6f1adab`
- Spec: `docs/superpowers/specs/2026-07-13-issue-529-sql-rate-limiter-design.md`
- Base: `origin/develop@58a2e7a274408aaac964ad5eb46d837a292a9dfb`
- Artifact kind: plan

## Findings and repairs

| Priority | Lens | Evidence | Required plan edit | Resolution |
|---|---|---|---|---|
| P1 | Performance | Concurrent Cleanup/Allow stress recorded latency but did not prove progress with a constrained pool. | Add bounded small-pool completion, cleanup progress, exact admission, outcome accounting, and worker-exit assertions. | `TestCleanupAllowPoolContention` uses two connections, fixed work, per-call/global contexts, exact burst accounting, and diagnostic-only `DBStats`. |
| P1 | Stability | A pre-lock server observation could become stale while waiting for a conflict row and move `updated_at` backwards. | Use a monotonic effective timestamp and add a real row-lock regression. | The UPSERT consistently uses `greatest(bucket.updated_at, excluded.updated_at)`; a forced lock waiter proves no timestamp regression or double refill. |
| P1 | Stability | Hook tests did not prove cancellation during an actual dispatched PostgreSQL operation. | Add a real lock-wait cancellation test with typed commit-unknown behavior. | The plan polls `pg_stat_activity`, cancels a blocked `Allow`, and verifies prompt zero-result classification, no retry, and no unexpected debit under race/repetition. |
| P1 | Stability | Pool, connection, transaction, and fixture cleanup was not assigned consistently. | Require bounded contexts and immediate LIFO cleanup for every resource. | Every pool closes before its container; dedicated connections close and transactions roll back on all paths; security cleanup checks `DBStats.InUse`. |
| P1 | Stability/API | `ErrConfigurationMismatch`, `testPhase`, and interface assertions were ordered after consumers or before methods existed. | Reorder private types/sentinel/tests and delay the root interface assertion until `Allow` exists. | Every TDD slice now compiles independently; `Cleanup` receiver tests remain in the cleanup task. |
| P1 | Security | Revoking `PUBLIC CREATE` after `SchemaSQL` left an object-name hijack window. | Verify schema ownership and effective privileges, revoke first, then run bounded DDL and catalog preflight. | The plan and runbook enforce the safe order and attacker-role hostile-object fail-closed tests without provider traffic. |
| P1 | Operator/Ops | Cleanup scheduling, destructive rollback/removal, and release proof were not executable enough. | Specify cadence/budgets/pause/unknown-count semantics, retain objects on rollback, gate separate removal on zero usage, and add a runbook contract test. | Task 8 now fixes the full scheduler and removal boundary and validates required markers plus migration order. |
| P1 | Developer/API | Root diagnostics called `KeyID` low-cardinality and the Redis edit could overwrite the existing `parse-result` label. | Mark KeyID diagnostic-only/non-metric and preserve both Redis operation labels. | Public Go doc and nested tests enforce the metric-label prohibition and `consume`/`parse-result` compatibility. |
| P1 | Developer/API | The in-flight cancellation test was absent from its RED command. | Select the named test before implementation. | RED, GREEN, race, and count-10 commands all include the real DB cancellation case. |
| P2 | User/Caller | Package README proof omitted cleanup unknown-count semantics and mixed-provider extra-burst hazards. | Require both README locale pairs and the example to demonstrate safe error, canary, and cutover behavior. | README contract tests cover zero count/up-to-limit deletion, non-idempotent retry, isolated quota state, independent canaries, and quiesce/wait-or-budget cutover. |

Main integration also replaced an invalid `ON CONFLICT ... FROM` draft with a valid repeated exact
refill expression, removed undefined helper snippets, and moved nil/zero method tests to the tasks
where those methods exist. The repeated expression is intentional because PostgreSQL
`ON CONFLICT DO UPDATE` has no `FROM` clause.

## Final rerun results

| Perspective | P0 | P1 | P2 | P3 | Result |
|---|---:|---:|---:|---:|---|
| Performance | 0 | 0 | 0 | 0 | Final reviewed hash preserves one UPSERT/round trip, monotonic time, exact arithmetic, constrained-pool progress, bounded cleanup, and no capacity claim. |
| Stability | 0 | 0 | 0 | 0 | Lock-wait time, real in-flight cancellation, response loss, cleanup races, resource lifecycle, race, and repeated Testcontainers proof are ordered and bounded. |
| Security | 0 | 0 | 0 | 0 | Fixed SQL/binds, byte identity, bounds, redaction, schema ownership, pre-DDL privilege revocation, hostile objects, RLS/triggers, and effective privileges are covered. |
| Operator/Ops | 0 | 0 | 0 | 0 | Migration, cleanup, telemetry, writable-primary/HA/RPO, canary, rollback retention, and delayed destructive migration are executable and contract-tested. |
| Developer/API | 0 | 0 | 0 | 0 | All tasks compile in order; public/root/Redis compatibility, private test controls, examples, docs, commands, and no-dependency drift are explicit. |
| User/Caller | 0 | 0 | 0 | 0 | Result/count-on-error, replay, configuration migration, DB/schema/scheduler ownership, mixed providers, canary/cutover, and unsupported behavior are misuse-resistant. |

## Main-session integration verdict

All eleven acceptance criteria and every DoD item map to an ordered task, named files, RED/GREEN
commands, rollback points, and final evidence. New dependency/module/BOM/CI registration, ORM,
Spring, Exposed, coroutine, streaming, JDK preview, benchmark/chart, and diagram work are N/A with
concrete scope evidence. No open implementation decision or unresolved finding remains.

P0=0 P1=0 P2=0 P3=0
