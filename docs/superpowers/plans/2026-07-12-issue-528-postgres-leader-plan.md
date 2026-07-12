# PostgreSQL Leader Elector Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a PostgreSQL row-lease implementation of `leader.Elector` at `leader/sql` that passes the shared provider conformance suite and ships with caller-owned schema guidance.

**Architecture:** `sqlleader.Elector` holds a caller-owned `*sql.DB`, an opaque owner token, and the same mutex-protected local lifecycle used by the Redis and Mongo providers. Acquire, renew, resign, and observation each use one schema-qualified PostgreSQL statement with `pg_catalog.clock_timestamp()`; mutation ambiguity is reconciled through a fresh bounded primary read and cleanup-pending recovery.

**Tech Stack:** Go 1.26, `database/sql`, pgx v5 stdlib, PostgreSQL 16 Testcontainers, `leader/leadertest`, CairoSVG and bluetape diagram audits.

---

## File Map

| Area | Files | Responsibility |
|---|---|---|
| Package/API | `leader/sql/doc.go`, `leader/sql/elector.go`, `leader/sql/backoff.go` | Public package contract, constructor validation, owner token, campaign loop, local state, bounded attempt/backoff policy. |
| PostgreSQL boundary | `leader/sql/schema.go`, `leader/sql/queries.go` | Exported bootstrap schema and all qualified parameterized SQL statements/mutation probes. |
| Lifecycle | `leader/sql/lifecycle.go` | Renewal generation, ownership loss, cleanup-pending resign and retry-safe join semantics. |
| Tests | `leader/sql/{elector_test.go,queries_test.go,lifecycle_test.go,conformance_test.go,security_test.go,readme_test.go,example_test.go}` | Constructor/schema, real PostgreSQL linearization, lifecycle/fault injection, mandatory conformance, least-privilege deployment, docs contract, compile-checked usage. |
| Provider docs | `leader/sql/README.md`, `leader/sql/README.ko.md` | Setup, schema/grants, primary-only routing, safety margins, lifecycle/runbook, failure modes. |
| Diagram | `docs/images/readme-diagrams/postgres-leader-row-lease-sequence.{svg,png}` | Acquire/renew/contention/resign and cleanup/TTL sequence. |
| Public index/release | `leader/README.md`, `leader/README.ko.md`, `README.md`, `README.ko.md`, `CHANGELOG.md` | Discoverability and v0.19.0 behavior/caveat summary. |
| Workflow evidence | `docs/superpowers/reviews/2026-07-12-issue-528-postgres-leader-risk.md`, later Step 6-R/7-R review artifacts | Pre-implementation prediction and final review evidence. |

## Dependency Order

Task 0 must precede every source edit. Task 1 establishes the API and schema contract. Task 2
implements the SQL linearization boundary consumed by Tasks 3-5. Task 3 owns lifecycle state;
Task 4 adds ambiguous-result recovery only after the happy-path lifecycle is green. Task 5 adds
the mandatory provider adapter. Tasks 6-8 consume the settled API and may proceed only after
Task 5 passes. Task 9 is the final local gate.

Do not run Testcontainers-backed tasks in parallel. Do not change `leader.Options`,
`leader.Elector`, or `leader/leadertest`; a required shared-contract change stops execution and
returns to design review.

### Task 0: Freeze Artifacts and Predict Implementation Risks

**Files:**
- Verify: `docs/superpowers/specs/2026-07-12-issue-528-postgres-leader-design.md`
- Verify: `docs/superpowers/plans/2026-07-12-issue-528-postgres-leader-plan.md`
- Create: `docs/superpowers/reviews/2026-07-12-issue-528-postgres-leader-risk.md`

- [ ] **Step 1: Verify the approved artifact-only state**

Run:

```bash
git status --short
git log --oneline origin/develop..HEAD
git diff --check origin/develop...HEAD
```

Expected: only the approved spec, plan, and their review amendments are ahead of `develop`; no
`leader/sql` source exists.

- [ ] **Step 2: Record the pre-implementation risk table**

Create the risk artifact with columns `Risk`, `Trigger`, `Signal`, `Prevention`, `Recovery`, and
`Owner`. Include at least: absent-row upsert race, lock/pool starvation, retry herd, stale
insert-side timestamp, renewal-after-resign, generation ABA, indeterminate acquire/renew/resign,
replica-routed probe, async failover loss, public-schema hijack, token/error leakage, unsafe lease
margin, Testcontainers leak, expired-row growth, and diagram/source drift.

- [ ] **Step 3: Capture baseline package and environment evidence**

Run:

```bash
go version
go list -m github.com/jackc/pgx/v5 github.com/testcontainers/testcontainers-go
go test -count=20 ./testing -run '^TestCheckWaiterReleasedDiagnostics$'
```

Expected: versions are recorded; the known baseline waiter test passes 20/20. Add the exact
outputs and the earlier one-off baseline failure note to the risk artifact.

- [ ] **Step 4: Commit risk evidence before source work**

```bash
git add docs/superpowers/reviews/2026-07-12-issue-528-postgres-leader-risk.md
git commit -m "docs: predict PostgreSQL leader risks"
```

Expected: the risk commit predates every `leader/sql` source commit.

### Task 1: Define the Public API and Caller-Owned Schema

**Files:**
- Create: `leader/sql/doc.go`
- Create: `leader/sql/elector.go`
- Create: `leader/sql/schema.go`
- Create: `leader/sql/elector_test.go`
- Create: `leader/sql/queries_test.go`

- [ ] **Step 1: Write RED constructor and schema tests**

Add tests with these exact assertions:

```go
func TestNewValidatesInputs(t *testing.T) {
    valid := leader.Options{Group: "billing", MemberID: "worker-1", Lease: time.Second, RenewInterval: 100 * time.Millisecond}
    if _, err := New(nil, valid); err == nil { t.Fatal("New(nil) succeeded") }

    db := &sql.DB{}
    if _, err := New(db, leader.Options{}); err == nil { t.Fatal("New accepted invalid identities") }
    for _, renew := range []time.Duration{time.Second, 2 * time.Second} {
        opts := valid
        opts.RenewInterval = renew
        if _, err := New(db, opts); err == nil { t.Fatalf("New accepted renew=%s lease=%s", renew, opts.Lease) }
    }
}

func TestNewDoesNotTouchDatabase(t *testing.T) {
    db := &sql.DB{}
    e, err := New(db, leader.Options{Group: "billing", MemberID: "worker-1"})
    if err != nil || e == nil { t.Fatalf("New() elector=%v err=%v", e, err) }
}

func TestSchemaSQLHasExpectedShape(t *testing.T) {
    for _, required := range []string{
        "public.bluetape_leader_leases", "leader_key text primary key",
        "owner_token text not null", "lease_until timestamptz not null",
    } {
        if !strings.Contains(strings.ToLower(SchemaSQL), required) {
            t.Fatalf("SchemaSQL missing %q", required)
        }
    }
}
```

- [ ] **Step 2: Observe RED**

```bash
go test -count=1 ./leader/sql -run 'TestNew|TestSchema'
```

Expected: build FAIL because `leader/sql`, `New`, `Elector`, and `SchemaSQL` do not exist.

- [ ] **Step 3: Add the minimal package, constructor, and schema**

Use this public shape:

```go
type Elector struct {
    db    *sql.DB
    opts  leader.Options
    key   string
    token string

    mu          sync.RWMutex
    owned       bool
    campaigning bool
    cleanup     bool
    generation  uint64
    cancel      context.CancelFunc
    done        chan struct{}
    testHook    func(operation, phase string) error
}

func New(db *sql.DB, opts leader.Options) (*Elector, error) {
    if db == nil { return nil, errors.New("postgres leader database must not be nil") }
    normalized, err := opts.Normalize()
    if err != nil { return nil, err }
    if normalized.RenewInterval >= normalized.Lease {
        return nil, errors.New("postgres leader renew interval must be less than lease")
    }
    token, err := randomToken(normalized.MemberID)
    if err != nil { return nil, err }
    return &Elector{db: db, opts: normalized, key: normalized.KeyPrefix + ":" + normalized.Group, token: token}, nil
}
```

`randomToken` reads 16 bytes from `crypto/rand` and returns
`memberID + ":" + hex.EncodeToString(data[:])`. `SchemaSQL` uses
this exact bootstrap and has no implicit execution helper:

```go
const SchemaSQL = `create table if not exists public.bluetape_leader_leases (
    leader_key text primary key,
    group_name text not null,
    member_id text not null,
    owner_token text not null,
    lease_until timestamptz not null,
    created_at timestamptz not null,
    updated_at timestamptz not null
)`

func randomToken(memberID string) (string, error) {
    var data [16]byte
    if _, err := rand.Read(data[:]); err != nil { return "", err }
    return memberID + ":" + hex.EncodeToString(data[:]), nil
}
```

- [ ] **Step 4: Verify GREEN and commit**

```bash
gofmt -w leader/sql/doc.go leader/sql/elector.go leader/sql/schema.go leader/sql/elector_test.go leader/sql/queries_test.go
go test -count=1 ./leader/sql -run 'TestNew|TestSchema'
git add leader/sql/doc.go leader/sql/elector.go leader/sql/schema.go leader/sql/elector_test.go leader/sql/queries_test.go
git commit -m "feat: define PostgreSQL leader provider"
```

Expected: constructor/schema tests PASS without opening a database connection.

### Task 2: Implement Atomic PostgreSQL Lease Operations

**Files:**
- Create: `leader/sql/queries.go`
- Modify: `leader/sql/elector.go`
- Modify: `leader/sql/queries_test.go`

- [ ] **Step 1: Add RED real-PostgreSQL operation tests**

Create one serial Testcontainers fixture using `postgrestestcontainer.Start`, `sql.Open("pgx",
dsn)`, `db.PingContext`, and `db.ExecContext(ctx, SchemaSQL)`. Add subtests proving:

```go
func TestLeaseStatements(t *testing.T) {
    // subtests share one PostgreSQL 16 container and use unique Group values.
    t.Run("acquire-observe", testAcquireObserve)
    t.Run("concurrent-single-winner", testConcurrentSingleWinner)
    t.Run("active-contention", testActiveContention)
    t.Run("expiry-takeover", testExpiryTakeover)
    t.Run("stale-token-delete", testStaleTokenDelete)
    t.Run("schema-idempotent", testSchemaIdempotent)
    t.Run("hostile-schema-detected", testHostileSchemaDetected)
    t.Run("expired-cleanup-safety", testExpiredCleanupSafety)
}
```

For the exact-winner case, release 16 goroutines from one barrier, call private `tryAcquire` on
separate electors sharing the pool, and assert exactly one `true`. Replace the stored token with
a second owner before invoking the first owner's delete and assert the replacement survives.

Add `TestQueriesUseQualifiedServerClock` in the same package and assert acquire, renew, and lookup
contain `pg_catalog.clock_timestamp()` and `public.bluetape_leader_leases`, while no runtime query
contains an unqualified `clock_timestamp()` call.

In `hostile-schema-detected`, use transactional DDL to replace the relation with a compatible
seven-column shape that lacks the PK and has a user trigger/RLS, execute the Task 6 catalog gate,
assert it fails, and roll the transaction back. In `expired-cleanup-safety`, insert live,
recently-expired, and grace-expired rows, run the documented server-time cleanup, and prove it
removes only the grace-expired row while live takeover remains possible.

- [ ] **Step 2: Observe RED**

```bash
go test -p 1 -count=1 ./leader/sql -run '^TestLeaseStatements$'
```

Expected: FAIL because the query constants and storage methods are absent.

- [ ] **Step 3: Implement the four one-statement boundaries**

Define qualified query constants. Acquire must contain this core:

```sql
insert into public.bluetape_leader_leases (
  leader_key, group_name, member_id, owner_token, lease_until, created_at, updated_at
) values (
  $1, $2, $3, $4,
  pg_catalog.clock_timestamp() + $5::bigint * interval '1 microsecond',
  pg_catalog.clock_timestamp(), pg_catalog.clock_timestamp()
)
on conflict (leader_key) do update set
  group_name = excluded.group_name,
  member_id = excluded.member_id,
  owner_token = excluded.owner_token,
  lease_until = pg_catalog.clock_timestamp() + $5::bigint * interval '1 microsecond',
  updated_at = pg_catalog.clock_timestamp()
where public.bluetape_leader_leases.lease_until <= pg_catalog.clock_timestamp()
   or public.bluetape_leader_leases.owner_token = excluded.owner_token
returning owner_token, lease_until
```

Implement private methods with `QueryRowContext(...).Scan(...)`:

```go
func (e *Elector) tryAcquire(ctx context.Context) (bool, error)
func (e *Elector) renew(ctx context.Context) (bool, error)
func (e *Elector) deleteOwner(ctx context.Context) error
func (e *Elector) lookupOwner(ctx context.Context) (string, error)
```

Map only `sql.ErrNoRows` to normal contention/no leader. Convert durations with a ceiling helper
that returns at least one microsecond. Renew predicates on key, token, and live expiry; delete
predicates on key and token; lookup predicates on key and live expiry. Every backend error is
wrapped with `leader.NewOperationError("postgres", operation, err)`.

Add `TestDurationMicrosCeilsPositive` with `1ns -> 1`, `1us -> 1`, `1001ns -> 2`, and a large
duration that does not overflow. README guidance—not a new public validation—explains that
sub-microsecond and near-zero lease settings are operationally unusable for network SQL.

Use these exact remaining statement shapes:

```sql
update public.bluetape_leader_leases
set lease_until = pg_catalog.clock_timestamp() + $3::bigint * interval '1 microsecond',
    updated_at = pg_catalog.clock_timestamp()
where leader_key = $1 and owner_token = $2
  and lease_until > pg_catalog.clock_timestamp()
returning lease_until;

delete from public.bluetape_leader_leases
where leader_key = $1 and owner_token = $2;

select owner_token from public.bluetape_leader_leases
where leader_key = $1 and lease_until > pg_catalog.clock_timestamp();
```

- [ ] **Step 4: Verify GREEN and commit**

```bash
gofmt -w leader/sql/queries.go leader/sql/elector.go leader/sql/queries_test.go
go test -p 1 -count=1 ./leader/sql -run '^TestLeaseStatements$'
git add leader/sql/queries.go leader/sql/elector.go leader/sql/queries_test.go
git commit -m "feat: add PostgreSQL leader lease statements"
```

Expected: all PostgreSQL statement subtests PASS; no new dependency appears in `go.mod`.

### Task 3: Add Campaign, Renewal, and Retry-Safe Resign Lifecycle

**Files:**
- Create: `leader/sql/backoff.go`
- Create: `leader/sql/lifecycle.go`
- Modify: `leader/sql/elector.go`
- Create: `leader/sql/lifecycle_test.go`

- [ ] **Step 1: Write RED lifecycle tests**

Cover these exact states with bounded contexts and `bttesting.Eventually`/`Consistently`:

```go
func testCampaignBlocksUntilContextOrTakeover(t *testing.T, db *sql.DB)
func testCampaignRejectsAlreadyOwnedAndInProgress(t *testing.T, db *sql.DB)
func testRenewalExtendsLease(t *testing.T, db *sql.DB)
func testZeroRowRenewalClearsLeadership(t *testing.T, db *sql.DB)
func testResignIsIdempotentAndTokenSafe(t *testing.T, db *sql.DB)
func testResignTimeoutRetainsCleanupForRetry(t *testing.T, db *sql.DB)
func testOldGenerationCannotClearNewOwnership(t *testing.T, db *sql.DB)
func testContentionBackoffIsBoundedAndNotTightLoop(t *testing.T, db *sql.DB)
func testLeaderRejectsNilAndCanceledContext(t *testing.T, db *sql.DB)
func testLeaderReturnsEmptyForMissingOrExpiredLease(t *testing.T, db *sql.DB)
func testConcurrentResignIsIdempotent(t *testing.T, db *sql.DB)
func testCanceledCampaignThenResignLeavesNoWorker(t *testing.T, db *sql.DB)
func testConstrainedPoolTimesOutWithoutLeaseOverstay(t *testing.T, db *sql.DB)
func testSharedPoolMultiElectorShutdown(t *testing.T, db *sql.DB)
```

Implement the listed cases as helper-backed subtests under one `TestPostgresLifecycle` so Task 3
starts one PostgreSQL container, not one per case. Do not call `t.Parallel` in any container-backed
parent or child test.

Use a test hook gate to block a renewal, start `Resign` with an expiring context, assert
`IsLeader()==false`, `cleanup==true`, and saved `done` remains; release the renewal, retry with a
fresh context, and assert the row is deleted and cleanup clears.

The constrained-pool case sets `MaxOpenConns(1)`, holds the only connection, and asserts attempt
and renewal deadlines increment `DBStats.WaitCount`/`WaitDuration`, do not tight-loop, and clear
local leadership rather than overstaying. The shared-pool shutdown case runs at least three
unique-group electors, cancels protected work/campaigns, bounded-resigns all of them, proves renew
operation counts stop and unresolved cleanup inventory is empty, then closes the pool only after
all users finish.

- [ ] **Step 2: Observe RED**

```bash
go test -p 1 -count=1 ./leader/sql -run '^TestPostgresLifecycle$'
```

Expected: FAIL because public lifecycle methods and retry state do not exist.

- [ ] **Step 3: Implement campaign and bounded backoff**

Implement the public loop in this order:

```go
func (e *Elector) Campaign(ctx context.Context) error {
    if ctx == nil { return leader.ErrInvalidContext }
    if err := ctx.Err(); err != nil { return err }
    if err := e.beginCampaign(); err != nil { return err }
    defer e.endCampaign()

    backoff := newBackoff(e.token, e.opts.Lease)
    for {
        acquired, err := e.acquireAttempt(ctx)
        if err != nil { return err }
        if acquired { e.startRenewal(); return nil }
        if err := backoff.wait(ctx); err != nil { return err }
    }
}
```

The attempt budget is bounded by the caller and
`max(100ms, min(RenewInterval, 1s))`. Backoff begins at 25ms, doubles, applies stable
owner-token jitter, and caps at `max(25ms, min(Lease/4, 1s))` without allocating a timer after
the context is done. Add `runTestHook(operation, phase string) error` as a default-nil internal
seam. Invoke the `renew/after` phase after a successful renewal statement and before the loop
publishes the result; Task 3 uses it only as a blocking lifecycle gate and Task 4 adds returned
failure semantics for every mutation.

- [ ] **Step 4: Implement generation-safe renewal and resign**

`startRenewal` increments `generation`, sets `owned=true`, clears cleanup, and starts one loop.
The loop uses `time.NewTicker(RenewInterval)`. Zero-row renewal calls
`clearOwnershipAfterLoss(generation, done, false)`; any indeterminate error passes `true`.
Every renewal derives and cancels a child context bounded by `RenewInterval`; pool wait, row lock,
or half-open connection may not retain `owned=true` beyond that budget. A renewal timeout is
indeterminate, sets cleanup pending, and stops further renewal traffic. Cover it with a real
row-lock deadline test as well as the blocking hook test.

`Resign` must atomically transition to `owned=false, cleanup=true`, cancel, join the saved exact
generation, and then call token-conditional delete. A join timeout returns `ctx.Err()` without
clearing `cancel`, `done`, generation, or cleanup. A later retry resumes the same join/delete.
Zero-row/already-absent delete is success and clears cleanup only when the generation still
matches.

Add the remaining public methods and compile-time assertion:

```go
var _ leader.Elector = (*Elector)(nil)

func (e *Elector) IsLeader() bool {
    e.mu.RLock()
    defer e.mu.RUnlock()
    return e.owned
}

func (e *Elector) Leader(ctx context.Context) (string, error) {
    if ctx == nil { return "", leader.ErrInvalidContext }
    if err := ctx.Err(); err != nil { return "", err }
    return e.lookupOwner(ctx)
}
```

`acquireAttempt` derives the internal context, calls `tryAcquire`, records whether only the
internal deadline fired (`attemptCtx.Err()!=nil && ctx.Err()==nil`), and passes that fact to the
Task 4 reconciliation path. Until Task 4, successful/no-row results work and non-contention
errors remain typed provider errors.

- [ ] **Step 5: Verify race safety and commit**

```bash
gofmt -w leader/sql/backoff.go leader/sql/lifecycle.go leader/sql/elector.go leader/sql/lifecycle_test.go
go test -p 1 -count=1 ./leader/sql -run '^TestPostgresLifecycle$'
go test -p 1 -race -count=1 ./leader/sql -run '^TestPostgresLifecycle$'
git add leader/sql/backoff.go leader/sql/lifecycle.go leader/sql/elector.go leader/sql/lifecycle_test.go
git commit -m "feat: manage PostgreSQL leader lifecycle"
```

Expected: PASS with zero races and no renewal traffic after a completed resign.

### Task 4: Reconcile Indeterminate Mutations and Preserve Cleanup

**Files:**
- Modify: `leader/sql/elector.go`
- Modify: `leader/sql/queries.go`
- Modify: `leader/sql/lifecycle.go`
- Modify: `leader/sql/lifecycle_test.go`

- [ ] **Step 1: Add RED phase-specific fault tests**

Define test-only phases `before`, `after`, and `reconcile` and add:

```go
func testAcquireLostResponseReconcilesOwnToken(t *testing.T, db *sql.DB)
func testAcquireProbeFailureReturnsCommitUnknown(t *testing.T, db *sql.DB)
func testInternalAttemptTimeoutWithOtherOwnerRetries(t *testing.T, db *sql.DB)
func testRenewLostResponseClearsOwnedAndKeepsCleanup(t *testing.T, db *sql.DB)
func testResignLostResponseIsCommitUnknownThenRetryable(t *testing.T, db *sql.DB)
func testPostgresOperationErrorRedactsMarkers(t *testing.T, db *sql.DB)
func testMutationFaultMatrix(t *testing.T, db *sql.DB)
func testBackendTerminationRecoveryAndTakeover(t *testing.T, db *sql.DB)
```

Implement these as subtests/helpers under one serial `TestPostgresFaultRecovery` fixture so Task 4
starts one PostgreSQL container and resets rows/fault state by unique group between cases.

The redaction cause contains distinct DSN, endpoint, relation, constraint, group, member, key,
and token markers. Assert none occur in `err.Error()` while `errors.Is` and `errors.As` reach the
original cause and `*leader.OperationError`.

`TestMutationFaultMatrix` is table-driven over campaign/renew/resign and before/after/reconcile
phases. Each row states whether SQL changed storage, the returned error/sentinel, `owned`,
`cleanup`, bounded resign behavior, and whether TTL takeover is required. A before failure never
matches `ErrCommitUnknown`; an after failure is ambiguous; reconcile applies only to acquisition.

For `TestInternalAttemptTimeoutWithOtherOwnerRetries`, begin a real transaction that updates and
holds the target row lock, confirm the lock holder and blocked UPSERT through a channel plus
`pg_stat_activity`, then assert operation counts show timeout, reconciliation, and backoff before
rolling back the transaction in cleanup. Sleeps alone are not synchronization evidence.

For `TestBackendTerminationRecoveryAndTakeover`, acquire a lease, hold its row from an admin
transaction so renewal blocks, find the blocked renewal PID in `pg_stat_activity`, and call
`pg_catalog.pg_terminate_backend(pid)` from a separate admin connection. Assert the bounded renew
attempt makes `IsLeader` false and cleanup pending; roll back the blocker, prove the pool
reconnects, perform bounded resign or wait server-time lease expiry, then prove a fresh elector
takes over.

- [ ] **Step 2: Observe RED**

```bash
go test -p 1 -count=1 ./leader/sql -run '^TestPostgresFaultRecovery$'
```

Expected: FAIL because mutation phases and fresh-context reconciliation are absent.

- [ ] **Step 3: Implement bounded primary reconciliation**

After an acquire statement error, create a fresh background context with the same attempt budget
and call the same-pool live-token lookup:

```go
owner, probeErr := e.lookupOwner(reconcileCtx)
switch {
case probeErr == nil && owner == e.token:
    return true, nil
case probeErr == nil && internalAttemptTimeout:
    return false, nil
case probeErr == nil:
    return false, operationErr
default:
    e.markCleanupPending()
    return false, errors.Join(operationErr, leader.ErrCommitUnknown)
}
```

The `after` hook runs only after storage success. Renewal post-mutation failure clears local
ownership, retains cleanup, and stops the loop. Resign mutation/post-mutation failure retains
cleanup and returns `errors.Join(OperationError, leader.ErrCommitUnknown)`; a subsequent
already-absent delete succeeds.

Keep the hook field/helper unexported and unreachable from `New` or any exported option. Every
hook error crosses the same redacted `leader.OperationError("postgres", operation, cause)` boundary;
the fault matrix rejects any direct raw hook error. Task 9 verifies the exported package surface
contains only the approved constructor, schema constant, elector type, and elector methods.

- [ ] **Step 4: Verify GREEN and commit**

```bash
gofmt -w leader/sql/elector.go leader/sql/queries.go leader/sql/lifecycle.go leader/sql/lifecycle_test.go
go test -p 1 -count=1 ./leader/sql -run '^TestPostgresFaultRecovery$'
go test -p 1 -race -count=1 ./leader/sql
git add leader/sql/elector.go leader/sql/queries.go leader/sql/lifecycle.go leader/sql/lifecycle_test.go
git commit -m "feat: reconcile PostgreSQL leader mutations"
```

Expected: typed/redacted errors preserve causes; ambiguous mutations never allow a fresh campaign
before cleanup.

### Task 5: Pass the Mandatory Provider Conformance Suite

**Files:**
- Create: `leader/sql/conformance_test.go`
- Create: `leader/sql/security_test.go`
- Modify: `leader/sql/lifecycle_test.go`

- [ ] **Step 1: Build the PostgreSQL conformance control**

Implement `leadertest.Control` in package `sqlleader` with mutex-protected failure maps and
operation counters keyed by normalized leader key. `ReplaceOwner` performs a server-time upsert,
`Owner` performs the qualified live-token read, and `FailNext` installs one post-mutation hook.
Use a unique group per conformance case and one serial PostgreSQL container.

```go
func TestPostgresElectorConformance(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
    defer cancel()
    db := openPostgresDB(ctx, t)
    if _, err := db.ExecContext(ctx, SchemaSQL); err != nil { t.Fatal(err) }
    control := newPostgresConformanceControl(db)
    leadertest.Run(t, leadertest.Harness{
        New: func(tb testing.TB, opts leader.Options) (leader.Elector, error) {
            elector, err := New(db, opts)
            if err == nil { elector.testHook = control.hook(opts) }
            return elector, err
        },
        Control: control,
    })
}
```

- [ ] **Step 2: Observe RED**

```bash
go test -p 1 -count=1 ./leader/sql -run '^TestPostgresElectorConformance$'
```

Expected: at least one named mandatory case fails until adapter phase/count semantics match the
real mutation boundaries.

- [ ] **Step 3: Make all 15 mandatory cases GREEN without changing the runner**

Run the suite repeatedly while fixing only `leader/sql` or its adapter:

```bash
go test -p 1 -count=10 ./leader/sql -run '^TestPostgresElectorConformance$'
```

Expected: 10/10 PASS for acquire-observe, owned-duplicate, campaign-in-progress,
contention-cancel, campaign-lost-response, renewal, renew-failure, owner-loss,
expiry-takeover, resign-idempotent, resign-retry, stale-resign, exact-contention, nil-context,
and redaction.

- [ ] **Step 4: Prove the documented least-privilege role**

As the Testcontainers superuser, create a unique migration owner and login runtime role. Install
the table under the migration owner, grant only schema `USAGE` and table
`SELECT,INSERT,UPDATE,DELETE`, and open a second `*sql.DB` as the runtime role. Through that pool,
run acquire, renew, lookup, resign, and the conformance suite; assert `create table`, `alter table`,
`truncate`, and `create trigger` fail. Query `has_schema_privilege`, `has_table_privilege`, role
membership, table owner, PK, trigger count, and RLS state and assert the Task 6 deployment gate.
Drop roles/tables with bounded cleanup.

```bash
go test -p 1 -count=1 ./leader/sql -run '^TestRuntimeRoleLeastPrivilege$'
```

Expected: all elector operations PASS; every DDL/ownership escalation is denied.

- [ ] **Step 5: Commit conformance and privilege proof**

```bash
gofmt -w leader/sql/conformance_test.go leader/sql/security_test.go leader/sql/lifecycle_test.go
go test -p 1 -race -count=1 ./leader/sql -run 'TestPostgresElectorConformance|TestRuntimeRoleLeastPrivilege'
git add leader/sql/conformance_test.go leader/sql/security_test.go leader/sql/lifecycle_test.go
git commit -m "test: cover PostgreSQL leader conformance"
```

Expected: the unchanged shared runner passes against the real PostgreSQL provider.

### Task 6: Add Compile-Checked Usage and Bilingual Provider Documentation

**Files:**
- Create: `leader/sql/example_test.go`
- Create: `leader/sql/readme_test.go`
- Create: `leader/sql/README.md`
- Create: `leader/sql/README.ko.md`

- [ ] **Step 1: Add a complete lifecycle example**

The external-package example blank-imports `github.com/jackc/pgx/v5/stdlib`, opens a caller-owned
`*sql.DB` with `db, err := sql.Open("pgx", dsn)`, executes `SchemaSQL` as an explicitly marked
development/bootstrap step, constructs the elector, campaigns with a bounded context,
polls at less than `RenewInterval`, cancels protected work on an unexpected `IsLeader` loss, and
performs bounded resign before closing the pool. Keep it compile-checked without requiring a live
database by placing runtime calls in a helper not executed by the example output test.

Use this exact result ordering and same-elector cleanup shape in the helper:

```go
err := elector.Campaign(campaignCtx)
switch {
case err == nil:
    // A confirmed owner-token probe may return success even after campaignCtx expired.
    // From this point the caller owns cleanup and must not discard elector.
    defer boundedResign(elector, opts.Lease)
    if campaignCtx.Err() != nil { return campaignCtx.Err() }
case errors.Is(err, leader.ErrCommitUnknown), errors.Is(err, leader.ErrCleanupPending):
    stopProtectedWork()
    return boundedResign(elector, opts.Lease)
default:
    return err
}
```

`boundedResign` creates a fresh short context for each retry on the same elector. If cleanup does
not succeed, it waits one full `opts.Lease` measured from the last failed cleanup attempt before
allowing process restart/reuse; it never constructs a replacement elector to bypass cleanup.

- [ ] **Step 2: Write the synchronized README pair**

Both READMEs must include the same headings and commands: Import, Schema, Least Privilege Grants,
Usage, Lease Semantics, Primary/Failover Contract, Pool and Timing, Failure Recovery, Shutdown,
Expired Row Cleanup, Security Boundaries, Test. Include exact SQL grants:

```sql
grant usage on schema public to app_runtime;
grant select, insert, update, delete
on table public.bluetape_leader_leases to app_runtime;
```

Put `revoke create on schema public from public;` in a separate DB-administrator hardening block,
not in the application migration copy/paste block, because it affects every role in the database.

State that `public` is fixed in v0.19.0, custom schemas are unsupported, tokens are not secrets or
fencing credentials, `KeyPrefix` is not authorization, reads must not route to replicas, and
`SchemaSQL` is bootstrap rather than an upgrade engine.

Start with a preflight table that checks: migration role can create the fixed protected `public`
relation; runtime role cannot create there; the DSN reaches a writable primary; `Lease` and
`RenewInterval` are either both zero (10s/3s defaults after normalization) or satisfy
`0 < RenewInterval < Lease`; and short custom leases set both values explicitly. Unsupported
custom-schema or replica-routed environments stop before migration.

Also state that `Group`, `MemberID`, `KeyPrefix`, and owner tokens are stored/returned in plaintext
and must not contain credentials, secrets, or sensitive customer identifiers. RLS is not
configured by the provider; a caller-supplied policy must independently prove all four DML paths
and `ON CONFLICT DO UPDATE`, otherwise RLS is unsupported for that deployment.

Include this schema-shape check and fail deployment if the seven ordered rows differ from the
documented contract:

```sql
select column_name, data_type, is_nullable
from information_schema.columns
where table_schema = 'public' and table_name = 'bluetape_leader_leases'
order by ordinal_position;
```

Add catalog checks for the protected object itself:

```sql
select c.relkind, pg_catalog.pg_get_userbyid(c.relowner) as owner,
       c.relrowsecurity, c.relforcerowsecurity
from pg_catalog.pg_class c
join pg_catalog.pg_namespace n on n.oid = c.relnamespace
where n.nspname = 'public' and c.relname = 'bluetape_leader_leases';

select array_agg(a.attname order by key_column.ordinality) as primary_key_columns
from pg_catalog.pg_constraint con
cross join lateral unnest(con.conkey) with ordinality as key_column(attnum, ordinality)
join pg_catalog.pg_attribute a on a.attrelid = con.conrelid and a.attnum = key_column.attnum
where con.conrelid = 'public.bluetape_leader_leases'::regclass and con.contype = 'p';

select count(*) as user_trigger_count
from pg_catalog.pg_trigger
where tgrelid = 'public.bluetape_leader_leases'::regclass and not tgisinternal;

select has_schema_privilege(current_user, 'public', 'USAGE') as schema_usage,
       has_schema_privilege(current_user, 'public', 'CREATE') as schema_create,
       has_table_privilege(current_user, 'public.bluetape_leader_leases', 'SELECT,INSERT,UPDATE,DELETE') as table_dml,
       has_table_privilege(current_user, 'public.bluetape_leader_leases', 'TRUNCATE,REFERENCES,TRIGGER') as table_ddl;
```

Expected deployment values are relation kind `r`, the configured migration owner, RLS false,
primary key `{leader_key}`, zero user triggers, runtime `schema_usage=true`,
`schema_create=false`, `table_dml=true`, and `table_ddl=false`; also inspect direct/inherited role
memberships and `PUBLIC` grants.

Include this optional logical-expiry cleanup template and require a grace interval larger than
the maximum configured lease; explain that it is storage hygiene, never the correctness TTL:

```sql
delete from public.bluetape_leader_leases
where lease_until < pg_catalog.clock_timestamp() - interval '1 day';
```

Add a controlled HA canary checklist distinct from the local backend-termination test. Capture
before/after outputs for `pg_is_in_recovery()`, `transaction_read_only`, server identity/timeline,
and WAL position; prove every elector/probe endpoint reaches the writable primary; restart or
promote under the deployment's HA controller; fence the old writer before the new writer accepts
leases; then prove bounded cleanup or full-lease takeover. The local test proves pool reconnection
only and must not be reported as promotion/fencing evidence.

Use this exact before/after identity query and stop if `in_recovery=true`, `read_only=on`, the
endpoint is not the intended primary, or the old primary still accepts writes after promotion:

```sql
select pg_catalog.inet_server_addr() as server_addr,
       pg_catalog.inet_server_port() as server_port,
       pg_catalog.pg_is_in_recovery() as in_recovery,
       current_setting('transaction_read_only') as read_only,
       pg_catalog.pg_postmaster_start_time() as postmaster_started,
       pg_catalog.pg_current_wal_lsn() as wal_lsn;
```

Create `readme_test.go` with a table of stable required anchors that must occur in both README
files: `DBStats.WaitCount`, `DBStats.WaitDuration`, `DBStats.InUse`, `DBStats.MaxOpenConnections`,
`Lease-RenewInterval`, `ErrCommitUnknown`, `ErrCleanupPending`, `pg_is_in_recovery()`,
`transaction_read_only`, `full lease`, `dead tuples`, and `autovacuum`. This test prevents either
translation from dropping pool alerts, recovery branches, primary fencing, full-lease fallback,
row growth, or shutdown inventory.

- [ ] **Step 3: Verify docs/example and commit**

```bash
gofmt -w leader/sql/example_test.go leader/sql/readme_test.go
go test -p 1 -count=1 ./leader/sql -run 'Example|TestREADMEContract'
rg -n 'public\.bluetape_leader_leases|ErrCommitUnknown|ErrCleanupPending|primary|replica|RenewInterval|KeyPrefix' leader/sql/README.md leader/sql/README.ko.md
paste <(rg '^## ' leader/sql/README.md) <(rg '^## ' leader/sql/README.ko.md)
rg -n '^```|SchemaSQL|grant |revoke |go test' leader/sql/README.md leader/sql/README.ko.md
git diff --check -- leader/sql
git add leader/sql/example_test.go leader/sql/readme_test.go leader/sql/README.md leader/sql/README.ko.md
git commit -m "docs: explain PostgreSQL leader operations"
```

Expected: example compiles; pasted heading rows have the same count and semantic order; each
setup/grant/test command has a corresponding block in both languages; every operational boundary
appears in both files.

### Task 7: Create and Verify the Row-Lease Sequence Diagram

**Files:**
- Create: `docs/images/readme-diagrams/postgres-leader-row-lease-sequence.svg`
- Create: `docs/images/readme-diagrams/postgres-leader-row-lease-sequence.png`
- Modify: `leader/sql/README.md`
- Modify: `leader/sql/README.ko.md`

- [ ] **Step 1: Pin source and visual references**

Read the implemented `leader/sql/{elector.go,queries.go,lifecycle.go}` and both provider READMEs.
Open these full-size references:

```text
/Users/debop/work/bluetape4k/bluetape4k-wiki/docs/diagrams/best-practices/assets/leader-core-sequence-03.png
docs/images/readme-diagrams/mongo-leader-election-sequence.png
```

Record the reader question: “How does one PostgreSQL row serialize acquire, renewal, contention,
commit-unknown cleanup, and safe resign?” The diagram kind is sequence; load only the already
selected common and sequence rules.

- [ ] **Step 2: Create one source-backed SVG**

Use participants Caller, `sqlleader.Elector`, caller-owned `*sql.DB`, and PostgreSQL primary.
Show numbered rows for Campaign, atomic UPSERT, live-owner retry, confirmed acquisition,
periodic token-bound UPDATE, Leader lookup, token-bound DELETE, and an `alt` frame for
commit-unknown probe/cleanup/lease expiry. Use explicit muted-color 16x16 sequence markers,
lifelines, activations, transparent branch frames, and readable row spacing.

- [ ] **Step 3: Parse, render, and audit the authoritative PNG**

```bash
xmllint --noout docs/images/readme-diagrams/postgres-leader-row-lease-sequence.svg
cairosvg docs/images/readme-diagrams/postgres-leader-row-lease-sequence.svg -o docs/images/readme-diagrams/postgres-leader-row-lease-sequence.png -s 2
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-connector-audit.py" docs/images/readme-diagrams/postgres-leader-row-lease-sequence.svg
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-geometry-audit.py" --fail-diagonal docs/images/readme-diagrams/postgres-leader-row-lease-sequence.svg
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-endpoint-audit.py" docs/images/readme-diagrams/postgres-leader-row-lease-sequence.svg
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-mixed-corner-audit.py" docs/images/readme-diagrams/postgres-leader-row-lease-sequence.svg
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-sequence-style-audit.py" docs/images/readme-diagrams/postgres-leader-row-lease-sequence.svg
```

Expected: XML/render succeed; meaningful participant/lifeline/message/label/marker/frame counts are
nonzero and all reported failures are zero. Open the PNG at full size after the final coordinate
change and record dimensions, label/line separation, arrowhead parity, branch transparency,
crossings, card intrusion, and whitespace in the Step 6-R evidence ledger.

- [ ] **Step 4: Embed and commit the paired asset**

Add `![PostgreSQL row-lease sequence](../../docs/images/readme-diagrams/postgres-leader-row-lease-sequence.png)`
to both provider READMEs, verify both relative targets, then run:

```bash
git diff --check -- docs/images/readme-diagrams/postgres-leader-row-lease-sequence.svg leader/sql/README.md leader/sql/README.ko.md
git add docs/images/readme-diagrams/postgres-leader-row-lease-sequence.svg docs/images/readme-diagrams/postgres-leader-row-lease-sequence.png leader/sql/README.md leader/sql/README.ko.md
git commit -m "docs: diagram PostgreSQL leader leases"
```

Expected: one canonical SVG/PNG pair is exposed from both language READMEs.

### Task 8: Update Public Indexes and Release Guidance

**Files:**
- Modify: `leader/README.md`
- Modify: `leader/README.ko.md`
- Modify: `README.md`
- Modify: `README.ko.md`
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Add synchronized backend discovery**

Add `leader/sql` to both leader backend sections, both root package tables, and both root
coordination link lists. Describe it as PostgreSQL-only, single-elector, caller-owned row leases;
do not imply group/strategic support.

- [ ] **Step 2: Add v0.19.0 CHANGELOG guidance**

Under the existing unreleased/v0.19.0 section, record the new provider, mandatory conformance,
fixed `public.bluetape_leader_leases` migration, primary-only routing, bounded resign/TTL recovery,
and absence of fencing/custom schema support.

- [ ] **Step 3: Verify parity and commit**

```bash
rg -n 'leader/sql|PostgreSQL|Postgres' README.md README.ko.md leader/README.md leader/README.ko.md CHANGELOG.md
git diff --check -- README.md README.ko.md leader/README.md leader/README.ko.md CHANGELOG.md
git add README.md README.ko.md leader/README.md leader/README.ko.md CHANGELOG.md
git commit -m "docs: publish PostgreSQL leader provider"
```

Expected: English/Korean discovery surfaces and release guidance agree on scope and caveats.

### Task 9: Run Final Local Gates and Prepare Review Evidence

**Files:**
- Create during workflow: `docs/superpowers/reviews/2026-07-12-issue-528-postgres-leader-step-6r-code-review.md`
- Modify only if review finds issues: files from Tasks 1-8

- [ ] **Step 1: Run targeted and stress verification from scratch**

```bash
go test -p 1 -count=10 ./leader/sql -run '^TestPostgresElectorConformance$'
go test -p 1 -count=1 ./leader ./leader/sql ./testcontainers/postgres
go test -p 1 -race -count=1 ./leader ./leader/sql
go doc github.com/bluetape4k/bluetape-go/leader/sql
```

Expected: all commands exit 0; Testcontainers tests are serial and race verification reports no
races; Go doc exposes no hook/fault configuration. Lost handles or missing exit codes are not
evidence—rerun from scratch.

- [ ] **Step 2: Run repository gates**

```bash
make fmt-check
make tidy-check
make vet
make lint
make ci
```

Expected: every command exits 0. If the known waiter diagnostic recurs, rerun its targeted 20x
proof before classifying it; do not hide a new provider failure behind the baseline note.

- [ ] **Step 3: Complete Step 6-R review and diagram ledger**

Run the six independent performance, stability, security, operator/Ops, developer/API, and
user/caller lanes. Main integration records P0/P1/P2 findings, every fix/rerun, and the complete
DIA-01..08 plus DIA-COM/SEQ evidence. Unresolved P0/P1 blocks PR creation.

- [ ] **Step 4: Commit only review-driven changes and evidence**

```bash
git status --short
git diff --check
git log --oneline origin/develop..HEAD
git add docs/superpowers/reviews/2026-07-12-issue-528-postgres-leader-step-6r-code-review.md
git commit -m "docs: review PostgreSQL leader provider"
```

Expected: worktree is clean, review verdict is P0=0/P1=0, and commits remain scoped to #528.

## Rollback Boundaries

- Revert documentation/index/diagram commits without changing the storage implementation.
- Revert in exact reverse dependency order: public indexes, diagram, provider docs/example,
  conformance adapter, reconciliation, lifecycle, SQL statements, then schema/API. Revert the
  risk artifact only after source rollback evidence is preserved.
- Never drop or mutate `public.bluetape_leader_leases` automatically during rollback; callers own
  migrations and may leave the unused compatible table in place.
- If runtime rollout produces commit-unknown or primary-routing ambiguity, stop protected work,
  first fence every non-authoritative writer and restore one authoritative writable primary.
  Record endpoint identity, recovery/read-only state, and database timeline evidence. Then cancel
  and join or terminate every elector, record every unresolved elector and the maximum configured
  lease, and start one wait of that maximum lease after the final join/process stop. Verify
  server-time expiry on the authoritative primary before enabling takeover, disabling the provider,
  or rolling application binaries back.
