# Issue #532 PostgreSQL Batch Checkpoint Step 6-R Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

## 범위와 판정

- Base: `873555f` (`origin/develop`)
- Branch: `feat/issue-532-sql-checkpoint`
- Reviewed implementation candidate: `e8130e8`
- Scope: additive atomic batch-step contracts, PostgreSQL checkpoint load and
  callback-plus-CAS commit, transaction-ownership proof, typed and redacted
  unknown outcomes, compatibility, security/catalog proof, bilingual guidance,
  sequence diagram, changelog, and the v0.19.0 rollout runbook.
- Final verdict: `P0=0`, `P1=0`, `P2=2` advisory.
- Pre-PR gate: PASS. No unresolved blocking finding remains.

## 6개 검토 관점

| Perspective | Final result | Evidence |
|---|---|---|
| Performance | PASS with P2 advisory | `Load` is one bounded query. A non-empty `Commit` uses one explicit transaction, one savepoint, the callback, one ownership probe, one checkpoint INSERT/UPDATE CAS, and one commit; it never retries. Empty input suppresses the transaction and callback. Advisory: `Load` defensively clones the scan-owned payload, temporarily retaining two bounded payload buffers before codec allocation. Measure before changing this safety boundary under issue #560. |
| Stability | PASS with P2 advisory | Consumed-input boundaries, exact CAS winner/loser behavior, explicit Read Committed isolation, cancellation checks, rollback-to-savepoint recovery from `25P02`, commit-unknown classification, panic identity, restart, and bounded stress are covered. Advisory: `Reader.Close(context.WithoutCancel(ctx))` can wait indefinitely on a caller implementation that never returns; it matches the legacy step contract, does not leak a detached goroutine, and has no universally safe library-selected timeout. |
| Security | PASS after fixes | All provider SQL and identifiers are fixed; values are bound and bounded; rendered errors redact keys, namespace, payload, DSN, and SQL. The callback receives a guarded session and transaction escape fails closed. Catalog validation covers the exact table, constraints, primary index, ACLs, RLS, policies, triggers, rewrite rules, role attributes, and both role-membership directions. Runtime receives only required DML and schema usage. |
| Operator/Ops | PASS via bounded main fallback | Migration ownership, pre/post-grant catalog gates, primary-only routing, transaction affinity, HA/RPO evidence, recovery drills, low-cardinality telemetry, `sql.DBStats`, WAL/autovacuum/replication signals, quiesce/reconciliation, canary, shutdown, rollback, and retention are executable contracts in both README locales and the release runbook. The helper lane timed out; main integration performed the read-only fallback. |
| Developer/API | PASS after fixes | The legacy `StepOptions` and `NewStep` surface remains source-compatible through an external unkeyed-literal fixture. New APIs are additive and documented. Public docs fix Read Committed behavior, ambient-isolation non-inheritance, codec/callback concurrency, same-key serialization, and stable operation constants including `OperationCommit`. No dependency was added. |
| User/caller | PASS after fixes | The primary example drives `Step.Run`, checks `Report.Err`, distinguishes commit unknown from atomicity unknown, keeps the key quiesced, and performs a bounded fresh `Atomic Writer.Load`. English/Korean guidance is synchronized. The final SVG/PNG routes both initial and recovery loads to the atomic writer and a negative contract test rejects the obsolete reader activation. |

## Material findings closed

- **P1 stability:** transaction isolation could inherit an ambient
  Repeatable Read default and turn a same-key race into a generic failure.
  Provider commits now explicitly request Read Committed, with a PostgreSQL
  regression that changes the database default and preserves the exact winner
  and conflict loser.
- **P1 security:** the initial role proof left inbound membership and privileged
  owner/runtime attributes insufficiently constrained. The catalog gate now
  accepts only the approved deployer-to-owner edge, rejects every runtime edge,
  and requires privileged role attributes to be false.
- **P1 user/API:** caller-facing recovery did not connect `Step.Run` through
  `Report.Err` to the commit-unknown and atomicity-unknown procedures. The
  compile-checked example and both README locales now show that complete path.
- **P2 stability:** a lint repair based on direct recovered-error equality could
  lose panic identity. The regression now proves original panic identity while
  preserving the sanitized unknown-atomicity boundary.
- **P2 user/caller:** two diagram defects sent commit-unknown recovery to the
  legacy reader and left that reader visibly active. Both were repaired, the
  PNG was rerendered and inspected at original resolution, and positive plus
  negative contract markers prevent recurrence.
- **P2 API:** the commit operation string was a public diagnostic convention
  without an exported constant. Stable operation constants now define the
  caller contract and production call sites use them.

## Acceptance traceability

| Criterion | Result | Evidence |
|---|---|---|
| Atomic batch boundary advances by consumed input | PASS | `AtomicStep`, partial-output/filtered-item tests, empty and exact-multiple EOF tests, and pending-slice mutation guards. |
| Business output and checkpoint commit together | PASS | Guarded callback session, one transaction, insert/update CAS, rollback tests, restart integration, and the sequence diagram. |
| Exact concurrent conflict result | PASS | Barrier-controlled PostgreSQL race proves one success and one `ErrCheckpointConflict`, including an ambient Repeatable Read default. |
| Cancellation, callback failure, and panic boundaries | PASS | Context checks before CAS/commit dispatch, savepoint ownership probe, `25P02` recovery, bounded cancellation stress, original-panic and sanitized-panic tests. |
| Commit and atomicity unknown recovery | PASS | Typed errors and stable operation constants, lost-response integration, competing-actor attribution, `Report.Err` example, quiesce and bounded fresh-load guidance. |
| Caller-owned schema, pool, and least privilege | PASS | Constructor I/O-free tests, `SchemaSQL`, exact catalog/ACL/role validator, hostile-object cases, migration example, and runbook. |
| Compatibility and dependency discipline | PASS | External unkeyed `StepOptions` fixture is part of `make test`; `go.mod` and `go.sum` are unchanged. |
| English/Korean docs, diagram, changelog, runbook | PASS | Paired package/root indexes, README contract and parity tests, SVG/PNG source pair, `CHANGELOG.md`, and v0.19.0 deployment gates. |

## Verification evidence

- Contract, provider unit, PostgreSQL integration, hostile-catalog, concurrency,
  cancellation, ownership, restart, and unknown-outcome suites passed on the
  implementation candidate.
- `go test -count=1 ./batch ./batch/sqlcheckpoint`: PASS; final pre-review run
  completed in 0.303s and 11.330s.
- Same scope under `-race`: PASS; 1.427s and 15.201s.
- Twenty repetitions of concurrent conflict, cancellation, and ownership
  regressions: PASS in 63.880s.
- External compatibility fixture: PASS.
- `make vet` and `make lint`: PASS; lint reported zero issues after the panic
  identity repair.
- First complete post-repair `make ci`: PASS, exit 0, at `59d0e0c`; all later
  implementation-candidate changes were diagram assets and their contract test.
- Diagram RED/GREEN evidence: the ownership test first failed on the wrong
  recovery target, then failed on the stale reader activation. Final SVG XML
  validation and the 3000x3080 PNG render passed; SHA-256 is
  `d122af987ab0260453959a26da3af7ade21162304fc54b66c57442cf66819463`.
- `git diff --check origin/develop...e8130e8`: PASS.

## Conditional and delivery evidence

| Conditional gate | Result | Scope evidence |
|---|---|---|
| New runtime dependency | N/A | Standard `database/sql` and the existing PostgreSQL test driver are reused; module files are unchanged. |
| Module/CI registration | N/A | The package is inside the existing Go module and normal repository targets discover it. |
| Benchmark/chart | DEFERRED | No throughput claim was added; issue #560 owns benchmark and capacity work. |
| Sequence diagram | PASS | Source SVG and rendered PNG expose load ownership, callback/CAS transaction, rollback/conflict, and unknown-outcome recovery. |
| Live PR review, metadata, and GitHub CI | PENDING | The branch must be pushed, the PR must mirror issue #532, and required checks must finish before an explicit merge decision. |

## Retained P2 advisories

- `Load` makes a bounded defensive payload copy after `database/sql` scan. The
  worst supported payload can temporarily occupy two buffers before codec
  allocations. Issue #560 should measure whether removing that copy is useful
  and prove the scanner ownership assumption before changing it.
- `Reader.Close(context.WithoutCancel(ctx))` can block when a caller-supplied
  reader never returns. This preserves cleanup after cancellation and matches
  the legacy step behavior; callers must enforce an outer shutdown deadline.

## 배포 소유 잔여 위험

- Production capacity, hot-key latency, WAL, dead tuples, autovacuum, pool
  margin, and telemetry thresholds require a production-like canary.
- Every runtime and reconciliation endpoint must stay on one fenced writable
  primary with transaction affinity; replicas, multi-primary, and replaying
  proxies remain unsupported.
- Durability/RPO and old-writer fencing require a controlled HA exercise; a
  reconnect test is not promotion evidence.
- Callers own namespace/key authorization, same-key serialization, business
  idempotency, codec/encryption migration, quiesce hooks, reconciliation,
  retention, and the shutdown deadline around reader cleanup.
