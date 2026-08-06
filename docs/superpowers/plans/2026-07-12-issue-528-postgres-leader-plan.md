# PostgreSQL leader elector 구현 계획

> 한국어 재작성 범위: 이 계획 문서는 한국어 운영 문서로 읽히도록 제목, 판단, 작업 설명, 위험, 검증, 롤백 문맥을 한국어로 정리한다. 명령, 경로, API 이름, 이슈/PR 번호, 브랜치명, 코드 블록, 테스트 출력 같은 증거 문자열은 정확성을 위해 원문 그대로 보존한다.


> **에이전트 작업자용:** 필수 하위 스킬: 사용 superpowers:subagent-driven-development (권장) 또는 superpowers:executing-plans to 이 계획을 작업 단위로 구현. 단계는 checkbox (`- [ ]`) 추적 문법을 사용.

**목표:** 추가 a PostgreSQL row-lease implementation of `leader.Elector` at `leader/sql` that passes the 공유 provider conformance suite 및 ships 함께 호출자-owned schema guidance.

**아키텍처:** `sqlleader.Elector` holds a 호출자-owned `*sql.DB`, an opaque owner token, 및 the same mutex-protected local lifecycle used by the Redis 및 Mongo providers. Acquire, renew, resign, 및 observation each use one schema-qualified PostgreSQL statement 함께 `pg_catalog.clock_timestamp()`; mutation ambiguity is reconciled through a fresh bounded primary read 및 cleanup-pending recovery.

**기술 스택:** Go 1.26, `database/sql`, pgx v5 stdlib, PostgreSQL 16 Testcontainers, `leader/leadertest`, CairoSVG 및 bluetape diagram audits.

---

## 파일 지도

| Area | 파일 | 책임 |
|---|---|---|
| Package/API | `leader/sql/doc.go`, `leader/sql/elector.go`, `leader/sql/backoff.go` | Public 패키지 계약, constructor validation, owner token, campaign loop, local state, bounded attempt/backoff policy. |
| PostgreSQL boundary | `leader/sql/schema.go`, `leader/sql/queries.go` | Exported bootstrap schema 및 모든 qualified parameterized SQL statements/mutation probes. |
| Lifecycle | `leader/sql/lifecycle.go` | Renewal generation, ownership loss, cleanup-pending resign 및 retry-safe join semantics. |
| Tests | `leader/sql/{elector_test.go,queries_test.go,lifecycle_test.go,conformance_test.go,security_test.go,readme_test.go,example_test.go}` | Constructor/schema, real PostgreSQL linearization, lifecycle/fault injection, mandatory conformance, least-privilege deployment, docs 계약, compile-checked usage. |
| Provider docs | `leader/sql/README.md`, `leader/sql/README.ko.md` | Setup, schema/grants, primary-만 routing, safety margins, lifecycle/runbook, failure modes. |
| Diagram | `docs/images/readme-diagrams/postgres-leader-row-lease-sequence.{svg,png}` | Acquire/renew/contention/resign 및 cleanup/TTL sequence. |
| Public index/release | `leader/README.md`, `leader/README.ko.md`, `README.md`, `README.ko.md`, `CHANGELOG.md` | Discoverability 및 v0.19.0 behavior/caveat summary. |
| Workf낮음 evidence | `docs/superpowers/reviews/2026-07-12-issue-528-postgres-leader-risk.md`, later 단계 6-R/7-R review artifacts | Pre-implementation prediction 및 final review evidence. |

## 의존 순서

작업 0 must precede every source edit. 작업 1 establishes the API 및 schema 계약. 작업 2
implements the SQL linearization boundary consumed by Tasks 3-5. 작업 3 owns lifecycle state;
작업 4 adds ambiguous-result recovery 만 후 the happy-path lifecycle is green. 작업 5 adds
the mandatory provider adapter. Tasks 6-8 consume the settled API 및 may proceed 만 후
작업 5 passes. 작업 9 is the final local gate.

다음을 하지 않는다: run Testcontainers-backed tasks in parallel. 다음을 하지 않는다: change `leader.Options`,
`leader.Elector`, 또는 `leader/leadertest`; a required 공유-계약 change stops execution 및
returns to design review.

### 작업 0: 고정 Artifacts 및 예측 Implementation Risks

**파일:**
- 검증: `docs/superpowers/specs/2026-07-12-issue-528-postgres-leader-design.md`
- 검증: `docs/superpowers/plans/2026-07-12-issue-528-postgres-leader-plan.md`
- 생성: `docs/superpowers/reviews/2026-07-12-issue-528-postgres-leader-risk.md`

- [ ] **단계 1: 검증 the approved artifact-만 state**

실행:

```bash
git status --short
git log --oneline origin/develop..HEAD
git diff --check origin/develop...HEAD
```

예상: 만 the approved spec, plan, 및 their review amendments are ahead of `develop`; 없음
`leader/sql` source exists.

- [ ] **단계 2: 기록 the pre-implementation risk table**

생성 the risk artifact 함께 columns `Risk`, `Trigger`, `Signal`, `Prevention`, `Recovery`, 및
`Owner`. Include at least: absent-row upsert race, lock/pool starvation, retry herd, stale
insert-side timestamp, renewal-후-resign, generation ABA, indeterminate acquire/renew/resign,
replica-routed probe, async failover loss, 공개-schema hijack, token/오류 leakage, unsafe lease
margin, Testcontainers leak, expired-row growth, 및 diagram/source drift.

- [ ] **단계 3: 캡처 baseline 패키지 및 environment evidence**

실행:

```bash
go version
go list -m github.com/jackc/pgx/v5 github.com/testcontainers/testcontainers-go
go test -count=20 ./testing -run '^TestCheckWaiterReleasedDiagnostics$'
```

예상: versions are recorded; the known baseline waiter 테스트 passes 20/20. 추가 the exact
outputs 및 the earlier one-off baseline failure note to the risk artifact.

- [ ] **단계 4: 커밋 risk evidence 전에 source work**

```bash
git add docs/superpowers/reviews/2026-07-12-issue-528-postgres-leader-risk.md
git commit -m "docs: predict PostgreSQL leader risks"
```

예상: the risk commit predates every `leader/sql` source commit.

### 작업 1: 정의 the Public API 및 Caller-Owned Schema

**파일:**
- 생성: `leader/sql/doc.go`
- 생성: `leader/sql/elector.go`
- 생성: `leader/sql/schema.go`
- 생성: `leader/sql/elector_test.go`
- 생성: `leader/sql/queries_test.go`

- [ ] **단계 1: Write RED constructor 및 schema 테스트**

추가 테스트 함께 these exact assertions:

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

- [ ] **단계 2: Observe RED**

```bash
go test -count=1 ./leader/sql -run 'TestNew|TestSchema'
```

예상: build FAIL because `leader/sql`, `New`, `Elector`, 및 `SchemaSQL` do 아님 exist.

- [ ] **단계 3: 추가 the minimal 패키지, constructor, 및 schema**

사용 this 공개 shape:

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

`randomToken` reads 16 bytes from `crypto/rand` 및 returns
`memberID + ":" + hex.EncodeToString(data[:])`. `SchemaSQL` uses
this exact bootstrap 및 has 없음 implicit execution helper:

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

- [ ] **단계 4: 검증 GREEN 및 commit**

```bash
gofmt -w leader/sql/doc.go leader/sql/elector.go leader/sql/schema.go leader/sql/elector_test.go leader/sql/queries_test.go
go test -count=1 ./leader/sql -run 'TestNew|TestSchema'
git add leader/sql/doc.go leader/sql/elector.go leader/sql/schema.go leader/sql/elector_test.go leader/sql/queries_test.go
git commit -m "feat: define PostgreSQL leader provider"
```

예상: constructor/schema 테스트 PASS without opening a database connection.

### 작업 2: 구현 Atomic PostgreSQL Lease Operations

**파일:**
- 생성: `leader/sql/queries.go`
- Modify: `leader/sql/elector.go`
- Modify: `leader/sql/queries_test.go`

- [ ] **단계 1: 추가 RED real-PostgreSQL operation 테스트**

생성 one serial Testcontainers fixture using `postgrestestcontainer.Start`, `sql.Open("pgx",
dsn)`, `db.PingContext`, and `db.ExecContext(ctx, SchemaSQL)`. 추가 subtests proving:

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
separate electors sharing the pool, 및 assert exactly one `true`. 교체 the stored token 함께
a second owner 전에 invoking the first owner's delete 및 assert the replacement survives.

추가 `TestQueriesUseQualifiedServerClock` in the same 패키지 및 assert acquire, renew, 및 lookup
contain `pg_catalog.clock_timestamp()` 및 `public.bluetape_leader_leases`, while 없음 runtime query
contains an unqualified `clock_timestamp()` call.

In `hostile-schema-detected`, use transactional DDL to replace the relation 함께 a compatible
seven-column shape that lacks the PK 및 has a 사용자 trigger/RLS, execute the 작업 6 catalog gate,
assert it fails, 및 roll the transaction back. In `expired-cleanup-safety`, insert live,
recently-expired, 및 grace-expired rows, run the documented server-time cleanup, 및 prove it
removes 만 the grace-expired row while live takeover remains possible.

- [ ] **단계 2: Observe RED**

```bash
go test -p 1 -count=1 ./leader/sql -run '^TestLeaseStatements$'
```

예상: FAIL because the query constants 및 storage methods are absent.

- [ ] **단계 3: 구현 the four one-statement boundaries**

정의 qualified query constants. Acquire must contain this core:

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

구현 private methods 함께 `QueryRowContext(...).Scan(...)`:

```go
func (e *Elector) tryAcquire(ctx context.Context) (bool, error)
func (e *Elector) renew(ctx context.Context) (bool, error)
func (e *Elector) deleteOwner(ctx context.Context) error
func (e *Elector) lookupOwner(ctx context.Context) (string, error)
```

Map 만 `sql.ErrNoRows` to normal contention/없음 leader. Convert durations 함께 a ceiling helper
that returns at least one microsecond. Renew predicates on key, token, 및 live expiry; delete
predicates on key 및 token; lookup predicates on key 및 live expiry. Every backend 오류 is
wrapped 함께 `leader.NewOperationError("postgres", operation, err)`.

추가 `TestDurationMicrosCeilsPositive` 함께 `1ns -> 1`, `1us -> 1`, `1001ns -> 2`, 및 a large
duration that does 아님 overf낮음. README guidance—아님 a new 공개 validation—explains that
sub-microsecond 및 near-zero lease settings are operationally unusable for network SQL.

사용 these exact remaining statement shapes:

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

- [ ] **단계 4: 검증 GREEN 및 commit**

```bash
gofmt -w leader/sql/queries.go leader/sql/elector.go leader/sql/queries_test.go
go test -p 1 -count=1 ./leader/sql -run '^TestLeaseStatements$'
git add leader/sql/queries.go leader/sql/elector.go leader/sql/queries_test.go
git commit -m "feat: add PostgreSQL leader lease statements"
```

예상: 모든 PostgreSQL statement subtests PASS; 없음 new dependency appears in `go.mod`.

### 작업 3: 추가 Campaign, Renewal, 및 Retry-Safe Resign Lifecycle

**파일:**
- 생성: `leader/sql/backoff.go`
- 생성: `leader/sql/lifecycle.go`
- Modify: `leader/sql/elector.go`
- 생성: `leader/sql/lifecycle_test.go`

- [ ] **단계 1: Write RED lifecycle 테스트**

Cover these exact states 함께 bounded contexts 및 `bttesting.Eventually`/`Consistently`:

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

구현 the listed cases as helper-backed subtests under one `TestPostgresLifecycle` so 작업 3
starts one PostgreSQL container, 아님 one per case. 다음을 하지 않는다: call `t.Parallel` in any container-backed
parent 또는 child 테스트.

사용 a 테스트 hook gate to block a renewal, start `Resign` 함께 an expiring context, assert
`IsLeader()==false`, `cleanup==true`, 및 saved `done` remains; release the renewal, retry 함께 a
fresh context, 및 assert the row is deleted 및 cleanup clears.

The constrained-pool case sets `MaxOpenConns(1)`, holds the 만 connection, 및 asserts attempt
및 renewal deadlines increment `DBStats.WaitCount`/`WaitDuration`, do 아님 tight-loop, 및 clear
local leadership rather than overstaying. The 공유-pool shutdown case runs at least three
unique-group electors, cancels protected work/campaigns, bounded-resigns 모든 of them, proves renew
operation counts stop 및 unresolved cleanup inventory is empty, then closes the pool 만 후
모든 users finish.

- [ ] **단계 2: Observe RED**

```bash
go test -p 1 -count=1 ./leader/sql -run '^TestPostgresLifecycle$'
```

예상: FAIL because 공개 lifecycle methods 및 retry state do 아님 exist.

- [ ] **단계 3: 구현 campaign 및 bounded backoff**

구현 the 공개 loop in this order:

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

The attempt budget is bounded by the 호출자 및
`max(100ms, min(RenewInterval, 1s))`. Backoff begins at 25ms, doubles, applies stable
owner-token jitter, 및 caps at `max(25ms, min(Lease/4, 1s))` without allocating a timer 후
the context is done. 추가 `runTestHook(operation, phase string) error` as a default-nil internal
seam. Invoke the `renew/after` phase 후 a successful renewal statement 및 전에 the loop
publishes the result; 작업 3 uses it 만 as a blocking lifecycle gate 및 작업 4 adds returned
failure semantics for every mutation.

- [ ] **단계 4: 구현 generation-safe renewal 및 resign**

`startRenewal` increments `generation`, sets `owned=true`, clears cleanup, 및 starts one loop.
The loop uses `time.NewTicker(RenewInterval)`. Zero-row renewal calls
`clearOwnershipAfterLoss(generation, done, false)`; any indeterminate 오류 passes `true`.
Every renewal derives 및 cancels a child context bounded by `RenewInterval`; pool wait, row lock,
또는 half-open connection may 아님 retain `owned=true` beyond that budget. A renewal timeout is
indeterminate, sets cleanup pending, 및 stops further renewal traffic. Cover it 함께 a real
row-lock deadline 테스트 as well as the blocking hook 테스트.

`Resign` must atomically transition to `owned=false, cleanup=true`, cancel, join the saved exact
generation, 및 then call token-conditional delete. A join timeout returns `ctx.Err()` without
clearing `cancel`, `done`, generation, 또는 cleanup. A later retry resumes the same join/delete.
Zero-row/already-absent delete is success 및 clears cleanup 만 when the generation still
matches.

추가 the remaining 공개 methods 및 compile-time assertion:

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

`acquireAttempt` derives the internal context, calls `tryAcquire`, records whether 만 the
internal deadline fired (`attemptCtx.Err()!=nil && ctx.Err()==nil`), 및 passes that fact to the
작업 4 reconciliation path. Until 작업 4, successful/없음-row results work 및 non-contention
오류 remain typed provider 오류.

- [ ] **단계 5: 검증 race safety 및 commit**

```bash
gofmt -w leader/sql/backoff.go leader/sql/lifecycle.go leader/sql/elector.go leader/sql/lifecycle_test.go
go test -p 1 -count=1 ./leader/sql -run '^TestPostgresLifecycle$'
go test -p 1 -race -count=1 ./leader/sql -run '^TestPostgresLifecycle$'
git add leader/sql/backoff.go leader/sql/lifecycle.go leader/sql/elector.go leader/sql/lifecycle_test.go
git commit -m "feat: manage PostgreSQL leader lifecycle"
```

예상: PASS 함께 zero races 및 없음 renewal traffic 후 a completed resign.

### 작업 4: Reconcile Indeterminate Mutations 및 보존 Cleanup

**파일:**
- Modify: `leader/sql/elector.go`
- Modify: `leader/sql/queries.go`
- Modify: `leader/sql/lifecycle.go`
- Modify: `leader/sql/lifecycle_test.go`

- [ ] **단계 1: 추가 RED phase-specific fault 테스트**

정의 테스트-만 phases `before`, `after`, 및 `reconcile` 및 add:

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

구현 these as subtests/helpers under one serial `TestPostgresFaultRecovery` fixture so 작업 4
starts one PostgreSQL container 및 resets rows/fault state by unique group between cases.

The redaction 원인 contains distinct DSN, endpoint, relation, constraint, group, member, key,
및 token markers. 검증 none occur in `err.Error()` while `errors.Is` 및 `errors.As` reach the
original 원인 및 `*leader.OperationError`.

`TestMutationFaultMatrix` is table-driven over campaign/renew/resign 및 전에/후/reconcile
phases. Each row states whether SQL changed storage, the returned 오류/sentinel, `owned`,
`cleanup`, bounded resign behavior, 및 whether TTL takeover is required. A 전에 failure never
matches `ErrCommitUnknown`; an 후 failure is ambiguous; reconcile applies 만 to acquisition.

For `TestInternalAttemptTimeoutWithOtherOwnerRetries`, begin a real transaction that updates 및
holds the target row lock, confirm the lock holder 및 blocked UPSERT through a channel plus
`pg_stat_activity`, then assert operation counts show timeout, reconciliation, 및 backoff 전에
rolling back the transaction in cleanup. Sleeps alone are 아님 synchronization evidence.

For `TestBackendTerminationRecoveryAndTakeover`, acquire a lease, hold its row from an admin
transaction so renewal blocks, find the blocked renewal PID in `pg_stat_activity`, 및 call
`pg_catalog.pg_terminate_backend(pid)` from a separate admin connection. 검증 the bounded renew
attempt makes `IsLeader` false 및 cleanup pending; roll back the blocker, prove the pool
reconnects, perform bounded resign 또는 wait server-time lease expiry, then prove a fresh elector
takes over.

- [ ] **단계 2: Observe RED**

```bash
go test -p 1 -count=1 ./leader/sql -run '^TestPostgresFaultRecovery$'
```

예상: FAIL because mutation phases 및 fresh-context reconciliation are absent.

- [ ] **단계 3: 구현 bounded primary reconciliation**

After an acquire statement 오류, create a fresh background context 함께 the same attempt budget
및 call the same-pool live-token lookup:

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

The `after` hook runs 만 후 storage success. Renewal post-mutation failure clears local
ownership, retains cleanup, 및 stops the loop. Resign mutation/post-mutation failure retains
cleanup 및 returns `errors.Join(OperationError, leader.ErrCommitUnknown)`; a subsequent
already-absent delete succeeds.

유지 the hook field/helper unexported 및 unreachable from `New` 또는 any exported option. Every
hook 오류 crosses the same redacted `leader.OperationError("postgres", operation, cause)` boundary;
the fault matrix rejects any direct raw hook 오류. 작업 9 verifies the exported 패키지 surface
contains 만 the approved constructor, schema constant, elector type, 및 elector methods.

- [ ] **단계 4: 검증 GREEN 및 commit**

```bash
gofmt -w leader/sql/elector.go leader/sql/queries.go leader/sql/lifecycle.go leader/sql/lifecycle_test.go
go test -p 1 -count=1 ./leader/sql -run '^TestPostgresFaultRecovery$'
go test -p 1 -race -count=1 ./leader/sql
git add leader/sql/elector.go leader/sql/queries.go leader/sql/lifecycle.go leader/sql/lifecycle_test.go
git commit -m "feat: reconcile PostgreSQL leader mutations"
```

예상: typed/redacted 오류 preserve causes; ambiguous mutations never al낮음 a fresh campaign
전에 cleanup.

### 작업 5: Pass the Mandatory Provider Conformance Suite

**파일:**
- 생성: `leader/sql/conformance_test.go`
- 생성: `leader/sql/security_test.go`
- Modify: `leader/sql/lifecycle_test.go`

- [ ] **단계 1: 구성 the PostgreSQL conformance control**

구현 `leadertest.Control` in 패키지 `sqlleader` 함께 mutex-protected failure maps 및
operation counters keyed by normalized leader key. `ReplaceOwner` performs a server-time upsert,
`Owner` performs the qualified live-token read, 및 `FailNext` installs one post-mutation hook.
사용 a unique group per conformance case 및 one serial PostgreSQL container.

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

- [ ] **단계 2: Observe RED**

```bash
go test -p 1 -count=1 ./leader/sql -run '^TestPostgresElectorConformance$'
```

예상: at least one named mandatory case fails until adapter phase/count semantics match the
real mutation boundaries.

- [ ] **단계 3: Make 모든 15 mandatory cases GREEN 변경하지 않고 the runner**

실행 the suite repeatedly while fixing 만 `leader/sql` 또는 its adapter:

```bash
go test -p 1 -count=10 ./leader/sql -run '^TestPostgresElectorConformance$'
```

예상: 10/10 PASS for acquire-observe, owned-duplicate, campaign-in-progress,
contention-cancel, campaign-lost-response, renewal, renew-failure, owner-loss,
expiry-takeover, resign-idempotent, resign-retry, stale-resign, exact-contention, nil-context,
및 redaction.

- [ ] **단계 4: 증명 the documented least-privilege role**

As the Testcontainers superuser, create a unique migration owner 및 login runtime role. Install
the table under the migration owner, grant 만 schema `USAGE` 및 table
`SELECT,INSERT,UPDATE,DELETE`, 및 open a second `*sql.DB` as the runtime role. Through that pool,
run acquire, renew, lookup, resign, 및 the conformance suite; assert `create table`, `alter table`,
`truncate`, 및 `create trigger` fail. Query `has_schema_privilege`, `has_table_privilege`, role
membership, table owner, PK, trigger count, 및 RLS state 및 assert the 작업 6 deployment gate.
Drop roles/tables 함께 bounded cleanup.

```bash
go test -p 1 -count=1 ./leader/sql -run '^TestRuntimeRoleLeastPrivilege$'
```

예상: 모든 elector operations PASS; every DDL/ownership escalation is denied.

- [ ] **단계 5: 커밋 conformance 및 privilege proof**

```bash
gofmt -w leader/sql/conformance_test.go leader/sql/security_test.go leader/sql/lifecycle_test.go
go test -p 1 -race -count=1 ./leader/sql -run 'TestPostgresElectorConformance|TestRuntimeRoleLeastPrivilege'
git add leader/sql/conformance_test.go leader/sql/security_test.go leader/sql/lifecycle_test.go
git commit -m "test: cover PostgreSQL leader conformance"
```

예상: the unchanged 공유 runner passes against the real PostgreSQL provider.

### 작업 6: 추가 Compile-Checked Usage 및 Bilingual Provider Documentation

**파일:**
- 생성: `leader/sql/example_test.go`
- 생성: `leader/sql/readme_test.go`
- 생성: `leader/sql/README.md`
- 생성: `leader/sql/README.ko.md`

- [ ] **단계 1: 추가 a complete lifecycle example**

The external-패키지 example blank-imports `github.com/jackc/pgx/v5/stdlib`, opens a 호출자-owned
`*sql.DB` 함께 `db, err := sql.Open("pgx", dsn)`, executes `SchemaSQL` as an explicitly marked
development/bootstrap step, constructs the elector, campaigns 함께 a bounded context,
polls at less than `RenewInterval`, cancels protected work on an unexpected `IsLeader` loss, 및
performs bounded resign 전에 closing the pool. 유지 it compile-checked without requiring a live
database by placing runtime calls in a helper 아님 executed by the example output 테스트.

사용 this exact result ordering 및 same-elector cleanup shape in the helper:

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
아님 succeed, it waits one full `opts.Lease` measured from the last failed cleanup attempt 전에
al낮음ing process restart/reuse; it never constructs a replacement elector to bypass cleanup.

- [ ] **단계 2: Write the synchronized README pair**

Both READMEs must include the same headings 및 commands: 가져오기, Schema, Least Privilege Grants,
Usage, Lease Semantics, Primary/Failover Contract, Pool 및 Timing, Failure Recovery, Shutdown,
Expired Row Cleanup, Security Boundaries, Test. Include exact SQL grants:

```sql
grant usage on schema public to app_runtime;
grant select, insert, update, delete
on table public.bluetape_leader_leases to app_runtime;
```

Put `revoke create on schema public from public;` in a separate DB-administrator hardening block,
아님 in the application migration copy/paste block, because it affects every role in the database.

State that `public` is fixed in v0.19.0, custom schemas are unsupported, tokens are 아님 secrets 또는
fencing credentials, `KeyPrefix` is 아님 authorization, reads must 아님 route to replicas, 및
`SchemaSQL` is bootstrap rather than an upgrade engine.

Start 함께 a preflight table that checks: migration role can create the fixed protected `public`
relation; runtime role cannot create there; the DSN reaches a writable primary; `Lease` 및
`RenewInterval` are either both zero (10s/3s defaults 후 normalization) 또는 satisfy
`0 < RenewInterval < Lease`; 및 short custom leases set both values explicitly. Unsupported
custom-schema 또는 replica-routed environments stop 전에 migration.

Also state that `Group`, `MemberID`, `KeyPrefix`, 및 owner tokens are stored/returned in plaintext
및 must 아님 contain credentials, secrets, 또는 sensitive customer identifiers. RLS is 아님
configured by the provider; a 호출자-supplied policy must independently prove 모든 four DML paths
및 `ON CONFLICT DO UPDATE`, otherwise RLS is unsupported for that deployment.

Include this schema-shape check 및 fail deployment if the seven ordered rows differ from the
documented 계약:

```sql
select column_name, data_type, is_nullable
from information_schema.columns
where table_schema = 'public' and table_name = 'bluetape_leader_leases'
order by ordinal_position;
```

추가 catalog checks for the protected object itself:

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
primary key `{leader_key}`, zero 사용자 triggers, runtime `schema_usage=true`,
`schema_create=false`, `table_dml=true`, 및 `table_ddl=false`; also inspect direct/inherited role
memberships 및 `PUBLIC` grants.

Include this optional logical-expiry cleanup template 및 require a grace interval larger than
the maximum configured lease; explain that it is storage hygiene, never the correctness TTL:

```sql
delete from public.bluetape_leader_leases
where lease_until < pg_catalog.clock_timestamp() - interval '1 day';
```

추가 a controlled HA canary checklist distinct from the local backend-termination 테스트. 캡처
전에/후 outputs for `pg_is_in_recovery()`, `transaction_read_only`, server identity/timeline,
및 WAL position; prove every elector/probe endpoint reaches the writable primary; restart 또는
promote under the deployment's HA controller; fence the old writer 전에 the new writer accepts
leases; then prove bounded cleanup 또는 full-lease takeover. The local 테스트 proves pool reconnection
만 및 must 아님 be reported as promotion/fencing evidence.

사용 this exact 전에/후 identity query 및 stop if `in_recovery=true`, `read_only=on`, the
endpoint is 아님 the intended primary, 또는 the old primary still accepts writes 후 promotion:

```sql
select pg_catalog.inet_server_addr() as server_addr,
       pg_catalog.inet_server_port() as server_port,
       pg_catalog.pg_is_in_recovery() as in_recovery,
       current_setting('transaction_read_only') as read_only,
       pg_catalog.pg_postmaster_start_time() as postmaster_started,
       pg_catalog.pg_current_wal_lsn() as wal_lsn;
```

생성 `readme_test.go` 함께 a table of stable required anchors that must occur in both README
files: `DBStats.WaitCount`, `DBStats.WaitDuration`, `DBStats.InUse`, `DBStats.MaxOpenConnections`,
`Lease-RenewInterval`, `ErrCommitUnknown`, `ErrCleanupPending`, `pg_is_in_recovery()`,
`transaction_read_only`, `full lease`, `dead tuples`, 및 `autovacuum`. This 테스트 prevents either
translation from dropping pool alerts, recovery branches, primary fencing, full-lease fallback,
row growth, 또는 shutdown inventory.

- [ ] **단계 3: 검증 docs/example 및 commit**

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

예상: example compiles; pasted heading rows have the same count 및 semantic order; each
setup/grant/테스트 command has a corresponding block in both languages; every operational boundary
appears in both files.

### 작업 7: 생성 및 검증 the Row-Lease Sequence Diagram

**파일:**
- 생성: `docs/images/readme-diagrams/postgres-leader-row-lease-sequence.svg`
- 생성: `docs/images/readme-diagrams/postgres-leader-row-lease-sequence.png`
- Modify: `leader/sql/README.md`
- Modify: `leader/sql/README.ko.md`

- [ ] **단계 1: Pin source 및 visual references**

Read the implemented `leader/sql/{elector.go,queries.go,lifecycle.go}` 및 both provider READMEs.
Open these full-size references:

```text
/Users/debop/work/bluetape4k/bluetape4k-wiki/docs/diagrams/best-practices/assets/leader-core-sequence-03.png
docs/images/readme-diagrams/mongo-leader-election-sequence.png
```

기록 the reader question: “How does one PostgreSQL row serialize acquire, renewal, contention,
commit-unknown cleanup, 및 safe resign?” The diagram kind is sequence; load 만 the already
selected common 및 sequence rules.

- [ ] **단계 2: 생성 one source-backed SVG**

사용 participants Caller, `sqlleader.Elector`, 호출자-owned `*sql.DB`, 및 PostgreSQL primary.
Show numbered rows for Campaign, atomic UPSERT, live-owner retry, confirmed acquisition,
periodic token-bound UPDATE, Leader lookup, token-bound DELETE, 및 an `alt` frame for
commit-unknown probe/cleanup/lease expiry. 사용 explicit muted-color 16x16 sequence markers,
lifelines, activations, transparent branch frames, 및 readable row spacing.

- [ ] **단계 3: Parse, render, 및 audit the authoritative PNG**

```bash
xmllint --noout docs/images/readme-diagrams/postgres-leader-row-lease-sequence.svg
cairosvg docs/images/readme-diagrams/postgres-leader-row-lease-sequence.svg -o docs/images/readme-diagrams/postgres-leader-row-lease-sequence.png -s 2
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-connector-audit.py" docs/images/readme-diagrams/postgres-leader-row-lease-sequence.svg
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-geometry-audit.py" --fail-diagonal docs/images/readme-diagrams/postgres-leader-row-lease-sequence.svg
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-endpoint-audit.py" docs/images/readme-diagrams/postgres-leader-row-lease-sequence.svg
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-mixed-corner-audit.py" docs/images/readme-diagrams/postgres-leader-row-lease-sequence.svg
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-sequence-style-audit.py" docs/images/readme-diagrams/postgres-leader-row-lease-sequence.svg
```

예상: XML/render succeed; meaningful participant/lifeline/message/label/marker/frame counts are
nonzero 및 모든 reported failures are zero. Open the PNG at full size 후 the final coordinate
change 및 record dimensions, label/line separation, arrowhead parity, branch transparency,
crossings, card intrusion, 및 whitespace in the 단계 6-R evidence ledger.

- [ ] **단계 4: Embed 및 commit the paired asset**

추가 `![PostgreSQL row-lease sequence](../../docs/images/readme-diagrams/postgres-leader-row-lease-sequence.png)`
to both provider READMEs, verify both relative targets, then run:

```bash
git diff --check -- docs/images/readme-diagrams/postgres-leader-row-lease-sequence.svg leader/sql/README.md leader/sql/README.ko.md
git add docs/images/readme-diagrams/postgres-leader-row-lease-sequence.svg docs/images/readme-diagrams/postgres-leader-row-lease-sequence.png leader/sql/README.md leader/sql/README.ko.md
git commit -m "docs: diagram PostgreSQL leader leases"
```

예상: one canonical SVG/PNG pair is exposed from both language READMEs.

### 작업 8: 업데이트 Public Indexes 및 Release Guidance

**파일:**
- Modify: `leader/README.md`
- Modify: `leader/README.ko.md`
- Modify: `README.md`
- Modify: `README.ko.md`
- Modify: `CHANGELOG.md`

- [ ] **단계 1: 추가 synchronized backend discovery**

추가 `leader/sql` to both leader backend sections, both root 패키지 tables, 및 both root
coordination link lists. Describe it as PostgreSQL-만, single-elector, 호출자-owned row leases;
do 아님 imply group/strategic support.

- [ ] **단계 2: 추가 v0.19.0 CHANGELOG guidance**

Under the 기존 unreleased/v0.19.0 section, record the new provider, mandatory conformance,
fixed `public.bluetape_leader_leases` migration, primary-만 routing, bounded resign/TTL recovery,
및 absence of fencing/custom schema support.

- [ ] **단계 3: 검증 parity 및 commit**

```bash
rg -n 'leader/sql|PostgreSQL|Postgres' README.md README.ko.md leader/README.md leader/README.ko.md CHANGELOG.md
git diff --check -- README.md README.ko.md leader/README.md leader/README.ko.md CHANGELOG.md
git add README.md README.ko.md leader/README.md leader/README.ko.md CHANGELOG.md
git commit -m "docs: publish PostgreSQL leader provider"
```

예상: 영문/한국어 discovery surfaces 및 release guidance agree on scope 및 caveats.

### 작업 9: 실행 Final Local Gates 및 준비 리뷰 증거

**파일:**
- 생성 during workf낮음: `docs/superpowers/reviews/2026-07-12-issue-528-postgres-leader-step-6r-code-review.md`
- Modify 만 if review finds issues: files from Tasks 1-8

- [ ] **단계 1: 실행 targeted 및 stress verification from scratch**

```bash
go test -p 1 -count=10 ./leader/sql -run '^TestPostgresElectorConformance$'
go test -p 1 -count=1 ./leader ./leader/sql ./testcontainers/postgres
go test -p 1 -race -count=1 ./leader ./leader/sql
go doc github.com/bluetape4k/bluetape-go/leader/sql
```

예상: 모든 commands exit 0; Testcontainers 테스트 are serial 및 race verification reports 없음
races; Go doc exposes 없음 hook/fault configuration. Lost handles 또는 missing exit codes are 아님
evidence—rerun from scratch.

- [ ] **단계 2: 실행 repository gates**

```bash
make fmt-check
make tidy-check
make vet
make lint
make ci
```

예상: every command exits 0. If the known waiter diagnostic recurs, rerun its targeted 20x
proof 전에 classifying it; do 아님 hide a new provider failure behind the baseline note.

- [ ] **단계 3: Complete 단계 6-R review 및 diagram ledger**

실행 the six independent 성능, 안정성, 보안, 운영자/Ops, 개발자/API, 및
사용자/호출자 lanes. Main integration records P0/P1/P2 findings, every fix/rerun, 및 the complete
DIA-01..08 plus DIA-COM/SEQ evidence. Unresolved P0/P1 blocks PR creation.

- [ ] **단계 4: 커밋 만 review-driven changes 및 evidence**

```bash
git status --short
git diff --check
git log --oneline origin/develop..HEAD
git add docs/superpowers/reviews/2026-07-12-issue-528-postgres-leader-step-6r-code-review.md
git commit -m "docs: review PostgreSQL leader provider"
```

예상: worktree is clean, review verdict is P0=0/P1=0, 및 commits remain scoped to #528.

## 롤백 Boundaries

- Revert documentation/index/diagram commits 변경하지 않고 the storage implementation.
- Revert in exact reverse dependency order: 공개 indexes, diagram, provider docs/example,
  conformance adapter, reconciliation, lifecycle, SQL statements, then schema/API. Revert the
  risk artifact 만 후 source rollback evidence is preserved.
- Never drop 또는 mutate `public.bluetape_leader_leases` automatically during rollback; callers own
  migrations 및 may leave the unused compatible table in place.
- If runtime rollout produces commit-unknown 또는 primary-routing ambiguity, stop protected work,
  first fence every non-authoritative writer 및 restore one authoritative writable primary.
  기록 endpoint identity, recovery/read-만 state, 및 database timeline evidence. Then cancel
  및 join 또는 terminate every elector, record every unresolved elector 및 the maximum configured
  lease, 및 start one wait of that maximum lease 후 the final join/process stop. 검증
  server-time expiry on the authoritative primary 전에 enabling takeover, disabling the provider,
  또는 rolling application binaries back.
