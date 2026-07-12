# Issue #528 PostgreSQL Leader Elector Design

Issue: [#528](https://github.com/bluetape4k/bluetape-go/issues/528)

Parent research: [#499](https://github.com/bluetape4k/bluetape-go/issues/499)

Date: 2026-07-12

Branch: `feat/issue-528-postgres-leader`

## Problem

`leader`는 backend-neutral `leader.Elector` contract와 Redis/MongoDB provider를
제공하지만, 이미 애플리케이션의 authoritative store인 PostgreSQL을 이용할 수 있는
provider는 없다. #499는 첫 SQL provider를 PostgreSQL row lease로 한정하고 advisory
lock, generic lock API, ORM integration은 별도 과제로 남겼다. #527에서 확정한 single
elector conformance contract를 변경하지 않고 같은 의미론을 제공해야 한다.

## Requirements and Scope

- `leader.Elector` single-leader contract를 구현한다.
- 공개 constructor는 caller-owned, concurrency-safe pool인 `*sql.DB`를 받는다.
- PostgreSQL의 atomic conditional upsert와 server clock을 correctness boundary로
  사용한다.
- migration, pool lifecycle, transaction, database role과 권한은 caller가 소유한다.
- 하나의 table과 database role을 공유하는 contender는 같은 trust domain에 속해야 한다.
  `KeyPrefix`는 collision namespace이지 authorization boundary가 아니다.
- 모든 mutation, reconciliation probe, observation은 동일한 writable-primary consistency
  domain으로 라우팅한다. Replica-routed reads는 지원하지 않는다.
- Redis/MongoDB와 같은 owner-token, renewal, cleanup-pending, commit-unknown 의미론을
  유지하고 `leader/leadertest` conformance suite를 그대로 통과한다.
- group/strategic election, fencing API, advisory lock, generic SQL dialect와 ORM adapter는
  이번 범위에서 제외한다.

## Current Evidence

| Source | Evidence |
|---|---|
| `leader/elector.go`, `leader/errors.go` | Public operation, sentinel, cleanup, and redacted `OperationError` contract. |
| `leader/redis/elector.go` | Owner token, local generation, renewal goroutine, and stale-cleanup protection. |
| `leader/mongo/elector.go` | Conditional takeover, token-bound renewal/resign, and bounded campaign retry. |
| `leader/leadertest` | The mandatory cross-provider single-elector conformance contract. |
| `sqlkit/session.go` | `sqlkit.Session` includes both `*sql.DB` and `*sql.Tx`, which is too broad for a long-lived elector. |
| `audit/sqloutbox` | Existing PostgreSQL identifier, schema, pgx stdlib, and Testcontainers patterns. |
| [PostgreSQL `INSERT`](https://www.postgresql.org/docs/current/sql-insert.html) | `ON CONFLICT DO UPDATE` is atomic; a false conflict `WHERE` predicate returns no row. |
| [PostgreSQL transaction isolation](https://www.postgresql.org/docs/current/transaction-iso.html) | `READ COMMITTED` re-evaluates the conflict update against the current row. |
| [PostgreSQL date/time functions](https://www.postgresql.org/docs/current/functions-datetime.html) | `clock_timestamp()` observes actual evaluation time instead of transaction-start time. |
| [Go `database/sql`](https://pkg.go.dev/database/sql) | `*sql.DB` is a long-lived, concurrency-safe pool; `*sql.Tx` is lifecycle-bound. |

CodeGraph returned no indexed nodes for this repository, so local structure evidence was
verified directly with `rg` and file inspection.

## Alternatives

| Approach | Decision | Reason |
|---|---|---|
| One-row lease with atomic `INSERT ... ON CONFLICT ... WHERE ... RETURNING` | Selected | One statement and one row-lock boundary handle both absent-row races and expired-row takeover. |
| `SELECT FOR UPDATE` followed by insert/update | Rejected | An absent row is not locked, extra round trips and transaction lifetime are required, and deadlock/retry handling expands. |
| PostgreSQL advisory lock | Rejected | Session locks require a dedicated pooled connection; transaction locks require a long transaction; neither provides TTL row-lease semantics. |
| Generic dialect or `sqlkit.Session` | Rejected | The SQL is PostgreSQL-specific and accepting `*sql.Tx` would allow leadership to outlive a caller transaction. |

## Selected Package and Public API

Add import path `leader/sql` with package name `sqlleader`:

```go
func New(db *sql.DB, opts leader.Options) (*Elector, error)

const SchemaSQL = `...`
```

`New` validates a non-nil pool and normalized `leader.Options`, creates a cryptographically
random owner token, and performs no I/O or DDL. The elector never closes `db`. The API has no
custom table-name or dialect option. A bounded exponential contention backoff starts at 25ms,
adds owner-token-derived jitter, and caps at `max(25ms, min(Lease/4, 1s))`. This avoids synchronized retries
while keeping expiry takeover responsive and the first API surface aligned with `leader/redis`;
deterministic failure hooks remain test-only.

After normalization, `New` also requires `RenewInterval < Lease`. This PostgreSQL-provider
safety validation prevents a configuration that deterministically reaches expiry before its
first scheduled renewal; broadening the shared `leader.Options` validation for existing
providers is a separate compatibility decision.

The PostgreSQL-specific implementation intentionally keeps the issue-mandated `leader/sql`
path. The package name, Go docs, and README call it `sqlleader` and state the PostgreSQL-only
contract; adding another SQL dialect would require a separate provider rather than silently
changing this package's SQL semantics.

`SchemaSQL` is provided for caller-controlled migrations. Exporting the SQL does not transfer
migration ownership to the package: production callers decide when, under which role, and in
which transaction it runs. `New` never executes it implicitly.

## Storage Model

The fixed, schema-qualified table name is `public.bluetape_leader_leases`. Every runtime
statement uses the qualified name, so pooled connections cannot resolve different lease tables
through different `search_path` values.

The v0.19.0 provider therefore supports databases where the caller's migration role can install
this fixed relation in `public`. Custom application schemas and configurable relation names are
explicit follow-up scope; callers that forbid all application objects in `public` cannot use
this first slice.

The migration role owns the relation. The runtime role receives only `SELECT`, `INSERT`,
`UPDATE`, and `DELETE` on this table and receives no schema `CREATE`, table ownership, DDL, or
trigger privileges. Deployments revoke `CREATE ON SCHEMA public FROM PUBLIC` or provide an
equivalently protected schema policy before installing the fixed relation. Separate trust
domains use separate databases or caller-managed role/RLS boundaries; this provider does not
build tenant authorization from `KeyPrefix`.

| Column | Type | Meaning |
|---|---|---|
| `leader_key` | `text primary key` | Normalized `<keyPrefix>:<group>` identity and unique contention point. |
| `group_name` | `text not null` | Human-readable group for operations. |
| `member_id` | `text not null` | Human-readable member for operations. |
| `owner_token` | `text not null` | Per-elector opaque ownership identity returned by `Leader`. |
| `lease_until` | `timestamptz not null` | Server-clock lease expiry. |
| `created_at` | `timestamptz not null` | First row creation time. |
| `updated_at` | `timestamptz not null` | Last successful acquire or renewal time. |

The primary key is the only correctness index. Expired rows are reused by takeover and need
not be deleted by a cleanup job. The schema contains no fencing generation because this issue
does not expose a fencing contract to protected resources.

## SQL Linearization Boundaries

### Acquire and takeover

Campaign executes one parameterized statement equivalent to:

```sql
INSERT INTO public.bluetape_leader_leases (
    leader_key, group_name, member_id, owner_token,
    lease_until, created_at, updated_at
)
VALUES (
    $1, $2, $3, $4,
    pg_catalog.clock_timestamp() + $5::bigint * interval '1 microsecond',
    pg_catalog.clock_timestamp(), pg_catalog.clock_timestamp()
)
ON CONFLICT (leader_key) DO UPDATE
SET group_name = EXCLUDED.group_name,
    member_id = EXCLUDED.member_id,
    owner_token = EXCLUDED.owner_token,
    lease_until = pg_catalog.clock_timestamp() + $5::bigint * interval '1 microsecond',
    updated_at = pg_catalog.clock_timestamp()
WHERE public.bluetape_leader_leases.lease_until <= pg_catalog.clock_timestamp()
   OR public.bluetape_leader_leases.owner_token = EXCLUDED.owner_token
RETURNING owner_token, lease_until;
```

Durations are bound as positive integer microseconds, rounding a positive sub-microsecond
duration up rather than truncating it to zero. `pg_catalog.clock_timestamp()` is repeated in
the conflict update so lock waiting cannot reuse a stale insert-side timestamp or an unsafe
function resolution path.

- A returned matching token means acquisition succeeded.
- `sql.ErrNoRows` means a live contender owns the row and is a normal retry condition.
- No explicit transaction or long-lived SQL lock is used.

Each SQL attempt has an internal deadline bounded by the caller context and by
`max(100ms, min(RenewInterval, 1s))`. This practical floor avoids turning small renewal settings
into immediate statement failures while bounding pool acquisition, row-lock waiting, and
half-open connection exposure for a long-lived `Campaign`. A timed-out dispatched mutation
still follows reconciliation; the timeout is never treated as proof that storage did not
change. If reconciliation observes another live owner or no current owner while the caller
context remains active, an internally generated attempt timeout returns to normal campaign
backoff instead of escaping as an operation error.

### Renewal

Renewal is one conditional `UPDATE ... RETURNING` keyed by `leader_key`, `owner_token`, and
`lease_until > pg_catalog.clock_timestamp()`. It computes the new expiry from the same
qualified server function.
Exactly one returned row means success; no row means the elector has lost leadership and an
expired owner cannot revive itself.

### Resign

Resign is `DELETE ... WHERE leader_key = $1 AND owner_token = $2`. Zero affected rows is
idempotent success. The owner-token predicate prevents a stale elector from deleting a
replacement owner that has the same `member_id`.

### Observation

`Leader` selects `owner_token` only where `leader_key` matches and
`lease_until > pg_catalog.clock_timestamp()`. An absent or expired row returns `"", nil`.
The token is collision-resistant ownership identity, not a secret, credential, authorization
grant, or fencing token. Callers should not log it unnecessarily.

## Local State and Concurrency

The elector uses the provider-common state shape: `owned`, `campaigning`, `cleanup`, a local
`generation`, renewal cancellation, and a completion channel protected by a mutex.

- `Campaign` rejects nil/canceled contexts and the shared duplicate, in-progress, and cleanup
  sentinel states before entering its retry loop.
- Confirmed acquisition starts exactly one background renewal loop.
- Each successful campaign increments the local generation. An older renewal goroutine may
  not clear a newer generation's state.
- A zero-row renewal is confirmed ownership loss: it makes `IsLeader` false and clears cleanup
  state. A dispatched or indeterminate renewal error makes `IsLeader` false, retains the token
  and generation as cleanup pending, stops further renewal traffic, and blocks a new campaign
  until bounded resign or TTL recovery.
- `Resign` first atomically sets `owned=false` and `cleanup=true`, then cancels and joins the
  exact renewal generation before the token-conditional delete. If its context ends while
  joining, it returns the context error but retains cleanup state and worker handles. A retry
  with a fresh context resumes the join and delete. No renewal may execute after the delete.
- The caller cancels campaign/renewer work and resigns before closing the shared DB pool.

The base `leader.Elector` contract intentionally adds no provider-specific callback or metrics
API. Operators observe synchronous campaign/resign errors, monitor `IsLeader` transitions in
the application lifecycle, and use `database/sql`/driver instrumentation for pool and statement
latency. An unexpected `true` to `false` transition is treated as a renew/loss alert; a following
`Campaign` returning `ErrCleanupPending` distinguishes unresolved cleanup from a clean loss.

## Error and Commit-Unknown Policy

Backend failures are wrapped with `leader.NewOperationError("postgres", operation, cause)`.
Rendered errors do not contain raw database text, while `errors.Is` and `errors.As` retain the
driver cause. A caller that explicitly unwraps and logs the driver cause bypasses this rendered
error redaction and owns that disclosure decision.

A context or transport error does not prove whether a PostgreSQL mutation committed. After an
acquire mutation error, the provider uses a fresh, internally bounded context to read the
current token from the same writable primary:

1. Its token is present and live: reconcile as successful acquisition.
2. Another token or no live token is observed: an internal attempt timeout resumes normal
   contention retry; another driver/caller mutation failure returns the sanitized original
   operation error.
3. The probe also fails: return the operation error joined with `leader.ErrCommitUnknown`, set
   cleanup pending, and make subsequent `Campaign` return `leader.ErrCleanupPending`.

For an indeterminate renewal, resign mutation error, or test-observed post-mutation failure, the
provider cannot prove the stored token state. It returns or records the sanitized operation
failure, retains cleanup pending, and stops claiming local leadership. A public resign failure
is joined with `leader.ErrCommitUnknown`. The caller retries bounded `Resign`; lease TTL is the
final safety fallback. A successful or already-absent resign clears cleanup pending.

Normal live-owner contention is not an error. Campaign waits with the bounded token-jittered
backoff and returns the caller context error when canceled. `New` enforces
`RenewInterval < Lease`; callers still leave enough remaining margin for expected database
latency and lock waits.

The test-only fault controller distinguishes `before`, `after`, and `reconcile` phases for each
mutation. `leadertest.Control.FailNext` maps to the post-mutation phase so its lost-response case
proves that storage changed before the response failed; provider-specific tests cover the other
phases.

Correctness assumes one writable primary and no lagging-replica routing. HA promotion must fence
the old writer before the new writer accepts leases. Deployments that cannot tolerate losing a
recent lease row during promotion require synchronous durability appropriate to their topology;
the provider cannot repair split-brain storage or asynchronous failover data loss.

## Test Strategy

Tests use the existing pgx stdlib driver and `testcontainers/postgres` fixture
(`postgres:16-alpine`) without adding dependencies. Container-backed packages run serially.

Required coverage:

- Run all mandatory `leader/leadertest.Run` cases through a PostgreSQL adapter.
- Prove concurrent initial upserts elect exactly one owner.
- Prove sustained contention obeys the bounded retry-rate policy instead of issuing a tight or
  synchronized UPSERT loop.
- Prove lease-expiry takeover, renewal, active-owner observation, and expired-row observation.
- Prove stale-token and same-member replacement resign safety and repeated resign idempotency.
- Prove cancellation while contending and while a statement is blocked.
- Prove the per-attempt deadline bounds pool acquisition/row-lock waits and an indeterminate
  timeout enters reconciliation.
- Prove renewal/backend failure clears local leadership and old generations cannot clear new
  state.
- Prove a post-mutation renewal failure leaves cleanup pending, stops further renewal traffic,
  and can be recovered by bounded resign or TTL takeover.
- Prove a timed-out resign waiting on a blocked renewal retains cleanup and worker handles, and
  a fresh-context retry joins the exact generation before deleting.
- Inject pre-mutation, post-mutation, and reconciliation-probe failures to verify redaction,
  `ErrCommitUnknown`, cleanup pending, and bounded recovery.
- Inject a PostgreSQL-shaped cause containing DSN/endpoint, relation/constraint, group/member,
  leader key, and owner-token markers; verify none occur in `err.Error()` while
  `errors.Is`/`errors.As` still reach the original cause.
- Verify `SchemaSQL` is idempotent and `New` neither creates schema nor closes the pool.
- Verify `New` rejects `RenewInterval >= Lease` after normalization.
- Verify all reconciliation/observation queries use the writable-primary pool supplied to
  `New`; document and exercise restart/takeover behavior without replica routing.
- Compile-check a public construction/lifecycle example, package documentation, and
  `var _ leader.Elector = (*Elector)(nil)`.
- Compile-check a lifecycle monitor that polls more frequently than `RenewInterval`, cancels
  protected work on an unexpected `IsLeader` transition, then performs bounded resign cleanup.

Verification gates:

```bash
go test -p 1 -count=1 ./leader ./leader/sql ./testcontainers/postgres
go test -p 1 -race -count=1 ./leader ./leader/sql
make ci
```

The pre-change baseline had one unrelated intermittent
`testing/TestCheckWaiterReleasedDiagnostics/wrong_error` failure. A targeted
`go test -count=20 ./testing -run '^TestCheckWaiterReleasedDiagnostics$'` rerun passed; any
recurrence must be reported separately rather than attributed to this provider.

## Documentation and Operations

- Add synchronized `leader/sql/README.md` and `leader/sql/README.ko.md` covering setup,
  caller-owned schema/pool lifecycle, migration/runtime role permissions, pool sizing,
  cancellation, shutdown order, renewal safety margin, and TTL fallback.
- Add the PostgreSQL provider to the English/Korean `leader` and root README backend lists.
- Add one paired SVG/PNG row-lease flow diagram and reference it from the provider README.
- State that this package is PostgreSQL-specific despite the `leader/sql` import path.
- State that database server time is authoritative and that the API does not provide fencing.
- State that owner tokens are not secrets or credentials, and `KeyPrefix` is not tenant
  authorization. Document mutually trusted contenders and separate role/RLS boundaries for
  separate trust domains.
- Document least-privilege runtime grants, protected `public` schema ownership, and the fact
  that logging an explicitly unwrapped driver cause bypasses rendered-error redaction.
- Document a writable-primary-only endpoint, old-primary fencing and durability assumptions,
  and a restart/failover exercise. Read/write splitting and replica-routed probes are unsupported.
- Recommend reserving enough pool capacity for active renewals plus application work. Alert when
  p99 pool-acquisition or statement latency consumes the `Lease-RenewInterval` margin.
- Define the runbook: alert on an unexpected `IsLeader` transition; stop protected work; attempt
  bounded `Resign`; retry as the DB recovers while `ErrCleanupPending` blocks new campaigns; if
  cleanup remains impossible, wait a full lease from the last possible mutation before restart
  or takeover.
- Publish exact schema `USAGE` and table DML grants plus a schema-shape verification query.
  `SchemaSQL` bootstrap idempotency does not validate a pre-existing incompatible relation.
- Explain that expiry is logical takeover, not PostgreSQL row deletion. Document cardinality and
  autovacuum monitoring plus an optional server-time, grace-window cleanup query for unused
  leader keys; cleanup must never delete a live row.
- Define shared-pool shutdown ordering: stop protected work, cancel/join all campaigns, bounded
  resign every elector, verify renewal traffic stopped, record unresolved full-lease waits, stop
  every remaining pool user, and only then close the caller-owned `*sql.DB`.
- State that callers must run `SchemaSQL` through their own migration process before `New` is
  used for operations.
- State that `SchemaSQL` is the v0.19 initial bootstrap contract, not a general migration
  engine. Future incompatible schema changes require versioned release migrations and CHANGELOG
  guidance; `CREATE TABLE IF NOT EXISTS` alone is not an upgrade mechanism.
- State the v0.19 fixed-`public` support constraint and track custom schema/table configuration
  separately rather than implying that hardened managed-database layouts already work.

## Acceptance Criteria

- `leader/sql` implements `leader.Elector` with `New(*sql.DB, leader.Options)`.
- All acquire, renew, resign, and lookup decisions use single parameterized PostgreSQL
  statements and server time.
- Ownership mutations are token-safe, contention-safe, context-aware, and compatible with the
  shared commit-unknown/cleanup contract.
- No DB pool, migration, caller transaction, or long-lived connection is owned by the provider.
- The full mandatory conformance suite and PostgreSQL-specific concurrency/failure tests pass.
- English/Korean public docs and the provider flow diagram are synchronized.
- Targeted tests, race verification, and `make ci` pass with fresh exit-code evidence.
