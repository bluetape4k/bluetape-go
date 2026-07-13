# Issue #529 PostgreSQL Rate Limiter Step 6-R Review

## Scope and verdict

- Base: `58a2e7a` (`origin/develop`)
- Branch: `feat/issue-529-sql-ratelimit`
- Scope: provider-neutral rate-limit diagnostics, PostgreSQL token-bucket
  admission and cleanup, shared conformance, security/catalog proof, bilingual
  provider guidance, public indexes, changelog, and the v0.19.0 rollout runbook.
- Review execution: all six perspectives and integration were completed in the
  main session because the user explicitly requested no subagents.
- Final verdict: `P0=0`, `P1=0`, `P2=0`, `P3=0`.
- Pre-PR gate: PASS. No unresolved finding remains.

## Six review perspectives

| Perspective | Final result | Evidence |
|---|---|---|
| Performance | PASS after fix | `Allow` is one UPSERT/RETURNING statement and `Cleanup` is one bounded DELETE CTE. Cleanup now orders only by `expires_at`, and a 20,000-row `EXPLAIN (ANALYZE, BUFFERS)` regression requires the expiry-index scan with no Sort. Numeric carry, row-lock serialization, pool pressure, non-HOT expiry-index updates, WAL/autovacuum cost, and the absence of unsupported capacity claims were reviewed. |
| Stability | PASS after fixes | Dedicated `*sql.Conn` acquisition separates pre-dispatch pool cancellation from uncertain dispatched work. Real row-lock cancellation, post-scan confirmation, lost response, same-row Allow/Cleanup races, multi-pool exact admission, repeated conformance, race tests, and bounded worker completion pass. Shared no-refill cases no longer depend on a 10 ms scheduling window, and refill waits follow `RetryAfter`. |
| Security | PASS after P1 closure | SQL identifiers are fixed and qualified, all values are positional, keys are byte-preserving and bounded, and errors expose only bounded family/operation plus a sampled redacted key ID. The catalog validator now checks the exact nine columns, constraints and states, expiry-index target/shape, owner, ordinary-table kind, RLS/forced RLS, policies, and user triggers. Least-privilege runtime DML and hostile pre-existing objects are exercised before provider traffic. |
| Operator/Ops | PASS | Migration ownership, `PUBLIC`/inherited CREATE checks, bounded migration timeouts, catalog preflight, runtime grants, primary-only routing, server identity/timeline, cleanup pressure gates, telemetry, old-writer fencing, durability/RPO, no replay, cutover, rollback, and delayed destructive removal are documented in the bilingual provider README and release runbook. |
| Developer/API | PASS after fix | The additive `ratelimit/sql` package uses caller-owned `*sql.DB`, performs no constructor I/O, implements `ratelimit.Limiter`, keeps cleanup explicit, has safe nil/zero behavior and Go docs, and adds no dependency. Redis retains its existing sentinel while also supporting the root `OperationError` and `ErrCommitUnknown` contracts. The compile-checked example closes caller-owned pools safely. |
| User/caller | PASS after fix | Provider selection, result-on-error discard, commit-unknown no-replay, configuration migration/namespace rotation, mixed-provider extra-burst risk, cleanup count ambiguity, and caller-owned schema/pool/scheduler responsibilities are explicit in English and Korean. `Allow` now documents and demonstrates a caller-owned deadline rather than an unbounded copied example. |

## Material findings closed

- **P1 security:** the earlier catalog proof could accept hostile drift because
  it did not validate the complete relation and index contract. Exact catalog
  validation and normal/hostile integration cases now block traffic on drift.
- **P1 stability/correctness:** pool-wait cancellation was indistinguishable
  from a dispatched statement when `database/sql` acquired a connection inside
  `QueryRowContext`. Explicit connection acquisition now returns the original
  pre-dispatch context error and reserves commit-unknown for dispatched work.
- **P2 performance:** cleanup's `(expires_at, namespace, bucket_key)` ordering
  forced a backlog Sort despite the expiry index. Ordering only by expiry keeps
  selection bounded by the existing index; deterministic tie order was not a
  correctness requirement.
- **P2 stability:** conformance cases that expected no refill used a 100 token/s
  configuration, so adapter latency could refill the missing token before the
  assertion. Those cases use a refill interval longer than the case timeout,
  while the refill case performs bounded `RetryAfter`-driven checks.
- **P2 stability:** the Redis server-time refill regression failed once in an
  isolated 100-run reproduction. It now verifies the initial debit and waits on
  returned refill timing; the repaired test passed 100 consecutive runs.
- **P2 user/API:** the public example passed an unbounded background context to
  `Allow`. The example and paired READMEs now require a caller-owned deadline,
  and the README contract test preserves that guidance.

## Acceptance traceability

| Criterion | Result | Evidence |
|---|---|---|
| 1. Constructor-only `New(*sql.DB, Options)` and root interface | PASS | `limiter.go`, `TestNewValidatesDatabaseAndDoesNotTouchIt`, and compile-time `ratelimit.Limiter` assertion. |
| 2. Caller-owned `SchemaSQL` and bounded `Cleanup` | PASS | `schema.go`, `Cleanup`, `TestCleanupPostgres`, both provider READMEs, and the compile-checked example. |
| 3. One-statement server-time atomic refill/debit | PASS | `allowQuery`, burst/rejection/refill integration, conflict-lock monotonic-time test, fractional carry, and exact multi-pool admission. |
| 4. Full `ratelimittest.Run` without skips | PASS | PostgreSQL, Redis, and memory harness suites; PostgreSQL conformance passed ten race-enabled repetitions. |
| 5. Exact multi-pool bounded stress and race | PASS | `TestMultiPoolExactAdmission`, `TestCleanupAllowPoolContention`, targeted repeats, package race, and full repository race. |
| 6. Cancellation and response-loss boundary | PASS | pre-canceled validation, pool-acquire cancellation, real in-flight row-lock cancellation, post-scan confirmation, lost-response debit proof, and mandatory conformance gates. |
| 7. Configuration mismatch is a quota no-op | PASS | `TestAllowPostgres/configuration-mismatch-is-zero-result` compares options, tokens, timestamp, and `xmin` before/after. |
| 8. Least privilege and exact catalog contract | PASS | `TestRuntimeRoleLeastPrivilege`, exact-schema acceptance, fourteen hostile-drift cases, hostile `IF NOT EXISTS` relation, and deployment catalog queries. |
| 9. English/Korean docs, root index, changelog, runbook | PASS | paired provider/root READMEs, README contract tests, root indexes, `CHANGELOG.md`, and bilingual v0.19.0 runbook sections. |
| 10. Targeted, race, static, and `make ci` gates | PASS | targeted/race/repeat commands below and fresh authoritative `make ci` exit 0. |
| 11. Cutover/rollback, HA fencing/RPO, telemetry gate | PASS | `SQL Rate Limiter Deployment Gates` in the v0.19.0 runbook and paired provider HA/cutover sections. |

## Verification evidence

- Catalog-validator RED: build failed because `validateRateLimitCatalog` was
  absent; GREEN normal plus fourteen hostile cases passed in 2.762s.
- `go test -p 1 -count=1 ./ratelimit/sql -run 'TestRuntimeRoleLeastPrivilege|TestHostileExistingSchemaFailsClosed|TestRateLimitCatalogValidator'`: PASS in 4.510s.
- Conformance timing RED: all three latency-sensitive no-refill cases and the
  eventual-refill script failed before the runner repair; the race-enabled
  GREEN regression suite passed in 2.290s.
- `go test -race -p 1 -count=10 ./ratelimit/sql -run '^TestPostgresRateLimiterConformance$'`: PASS in 15.411s.
- `go test -p 1 -count=10 ./ratelimit/sql -run 'TestPostgresRateLimiterConformance|TestMultiPoolExactAdmission|TestCleanupAllowPoolContention'`: PASS in 66.396s.
- `TESTCONTAINERS_REUSE_ENABLE=false go test -race -p 1 -count=1 ./ratelimit/...`: PASS; SQL 27.052s and Redis 26.007s in that run.
- Redis refill baseline reproduction: one failure in 100 runs; repaired test
  `go test -p 1 -count=100 ./ratelimit/redis -run '^TestLimiterRefillsFromRedisServerTime$'`: PASS in 83.214s.
- `TESTCONTAINERS_REUSE_ENABLE=false make ci`: PASS, exit 0; lint reported
  `0 issues`, SQL normal/race passed in 39.139s/43.710s, Redis in
  4.650s/8.224s, and shared conformance in 5.560s/6.600s.
- Exact final rate-limit scope:
  `TESTCONTAINERS_REUSE_ENABLE=false go test -race -p 1 -count=1 ./ratelimit/... ./redis/...`:
  PASS; SQL 22.175s, Redis rate limiter 6.273s, shared conformance 6.686s,
  root Redis 5.714s, and Redis streams 4.883s.
- A post-review repository rerun reached an unchanged `leader/sql` owner-loss
  timing failure after every rate-limit package passed. Both affected owner-loss
  fixtures then passed ten isolated repetitions in 22.255s; no `leader` file is
  present in the final diff. The earlier complete `make ci` exit 0 and the exact
  final scope command above are retained as the non-flaky evidence set.
- Final `make fmt-check`, `make tidy-check`, `make vet`, and `make lint`: PASS;
  lint reported `0 issues`.
- `git diff --check`: PASS.

## Explicit N/A evidence

| Conditional gate | Result | Scope evidence |
|---|---|---|
| New runtime dependency | N/A | `go.mod` and `go.sum` are unchanged; `database/sql` and the existing pgx driver are reused. |
| Module/BOM/CI registration | N/A | This is an additive package inside the existing Go module; `go test ./...` and `make ci` discover it without module registration. |
| ORM/Spring/Exposed/coroutines/streaming/JDK preview | N/A | The implementation is direct Go `database/sql`; none of these stacks or execution models is present. |
| Benchmark/chart | N/A | No capacity number or benchmark claim was added; moderate-QPS positioning is qualitative and deployment-owned. |
| New diagram | N/A | The approved spec states that text and tables cover the operational contract; no new interaction requiring a diagram was introduced. |
| Live PR review, metadata, and GitHub CI | N/A for this handoff | Push/PR authorization was not given. The validated branch is preserved before those external gates. |

## Residual deployment-owned risks

- Service-specific capacity, hot-key latency, WAL/dead-tuple/index growth, pool
  margins, cleanup cadence, and telemetry thresholds require a production-like
  canary and cannot be inferred from conformance tests.
- Every endpoint must remain on one fenced writable primary. Multi-primary,
  read-replica, and statement-replay topologies remain unsupported.
- Durability/RPO and old-writer fencing require a controlled HA exercise; a
  pool reconnect test is not promotion evidence.
- Callers must authorize and bound key/namespace cardinality. Cleanup limits
  storage lifetime but is not an abuse-control boundary.
