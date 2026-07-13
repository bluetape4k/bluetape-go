# PostgreSQL Rate Limiter Provider Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a PostgreSQL-backed `ratelimit.Limiter` at `ratelimit/sql` for database-only, moderate-QPS deployments, with exact atomic admission, provider-neutral indeterminate-outcome errors, caller-owned schema, and bounded cleanup.

**Architecture:** `sqlratelimit.Limiter` owns no connection lifecycle or goroutines; it stores a caller-owned `*sql.DB` and normalized immutable options. Each `Allow` call uses one schema-qualified parameterized PostgreSQL UPSERT whose single server timestamp drives exact numeric refill, debit, expiry, and returned durations. Cleanup is a separate bounded `DELETE ... FOR UPDATE SKIP LOCKED` operation, while shared root error contracts let callers inspect Redis and SQL failures uniformly.

**Tech Stack:** Go 1.26, `database/sql`, pgx v5 stdlib/`pgconn`, PostgreSQL Testcontainers, `ratelimit/ratelimittest`, standard-library hashing and errors.

---

## File Map

| Area | Files | Responsibility |
|---|---|---|
| Root contract | `ratelimit/errors.go`, `ratelimit/errors_test.go`, `ratelimit/redis/{limiter.go,operation_error_test.go,conformance_test.go}` | Provider-neutral `ErrCommitUnknown` and `OperationError`, plus backward-compatible Redis matching. |
| SQL API/options | `ratelimit/sql/{doc.go,options.go,options_test.go,limiter.go,limiter_test.go}` | Public constructor/options, caller-owned pool, validation, exact byte identity, nil/zero safety. |
| SQL schema/statements | `ratelimit/sql/{schema.go,queries.go,queries_test.go}` | Fixed bootstrap DDL, atomic UPSERT, result conversion, configuration mismatch, bounded cleanup. |
| SQL diagnostics | `ratelimit/sql/{errors.go,errors_test.go}` | Typed redacted operation failures and known-rollback versus indeterminate classification. |
| Provider proof | `ratelimit/sql/{conformance_test.go,security_test.go,stress_test.go,example_test.go,readme_test.go}` | Mandatory contract, deterministic fault injection, multi-pool exactness, least privilege/catalog checks, compile-checked usage, docs parity. |
| Package docs | `ratelimit/sql/README.md`, `ratelimit/sql/README.ko.md`, `ratelimit/README.md`, `ratelimit/README.ko.md` | API, migration, cleanup, topology, failure handling, and provider selection. |
| Public release docs | `README.md`, `README.ko.md`, `CHANGELOG.md`, `docs/release/v0.19.0-provider-conformance-runbook.md` | Discoverability, 0.19.0 caveat, rollout/rollback, HA/RPO, telemetry gates. |
| Workflow evidence | `docs/superpowers/reviews/2026-07-13-issue-529-sql-rate-limiter-{risk,plan-review}.md`, later Step 6-R/7-R artifacts | Pre-implementation risks and review convergence evidence. |

## Dependency Order and Write Scope

Task 0 freezes reviewed artifacts and risk evidence. Task 1 must land before SQL code because
Tasks 4 and 6 inspect provider-neutral error types. Task 2 defines the SQL package, normalized
values, and schema consumed by every later task. Task 3 establishes successful atomic admission;
Task 4 then adds failure classification and deterministic cancellation/lost-response proof. Task 5
adds cleanup after the bucket lifecycle is stable. Task 6 owns real-PostgreSQL conformance,
concurrency, precision, schema, and least-privilege proof. Tasks 7 and 8 document only the settled
API. Task 9 runs final verification and Step 6-R.

Tasks 1 and 2 have disjoint write scopes but execute sequentially to keep RED/GREEN evidence and
commits unambiguous. No Testcontainers-backed command may run concurrently with another real
service suite or delegated worker. Do not change `ratelimit.Limiter`, `ratelimittest.Harness`, or
the public Redis constructor. A required change to those contracts stops execution and returns to
Step 2 design review.

### Task 0: Freeze Artifacts and Predict Risks

**Complexity:** Small documentation gate; blocks all source edits.

**Files:**
- Verify: `docs/superpowers/specs/2026-07-13-issue-529-sql-rate-limiter-design.md`
- Verify: `docs/superpowers/reviews/2026-07-13-issue-529-sql-rate-limiter-spec-review.md`
- Verify: `docs/superpowers/plans/2026-07-13-issue-529-sql-rate-limiter-plan.md`
- Create: `docs/superpowers/reviews/2026-07-13-issue-529-sql-rate-limiter-risk.md`

- [ ] **Step 1: Verify the approved artifact-only branch**

```bash
git status --short
git log --oneline origin/develop..HEAD
git diff --check origin/develop...HEAD
```

Expected: the reviewed spec, spec review, approved plan, and plan review are the only changes
ahead of `origin/develop`; no `ratelimit/sql` directory exists.

- [ ] **Step 2: Record the pre-implementation risk table**

Create the risk artifact with columns `Risk`, `Trigger`, `Signal`, `Prevention`, `Recovery`, and
`Owner`. Include these concrete rows: first-insert race, fractional-refill starvation, arithmetic
or duration overflow, configuration mismatch lock/WAL pressure, connection-pool starvation,
response loss after debit, cancellation before/after scan, cleanup/Allow race, cleanup backlog,
HOT/index/autovacuum pressure, public-schema hijack, privilege inheritance, RLS/trigger drift,
active-key cardinality abuse, mixed-provider extra burst, replica routing, failover replay,
asynchronous WAL loss, Testcontainers leakage, and bilingual documentation drift.

- [ ] **Step 3: Capture environment and baseline evidence**

```bash
go version
go list -m -f '{{.Path}} {{.Version}}' github.com/jackc/pgx/v5 github.com/testcontainers/testcontainers-go
go test -count=1 ./ratelimit/... ./redis/...
```

Expected: Go and dependency versions are recorded and the existing rate-limit/Redis slice passes.
Record that the feature-branch baseline `go test -count=1 ./...` passed before source edits.

- [ ] **Step 4: Commit the risk artifact before source work**

```bash
git add docs/superpowers/reviews/2026-07-13-issue-529-sql-rate-limiter-risk.md
git commit -m "docs: predict PostgreSQL rate limiter risks"
```

Expected: the risk commit predates all source commits and supplies rollback/owner information for
every database, API, concurrency, security, and rollout hazard.

### Task 1: Add the Provider-Neutral Error Contract

**Complexity:** Medium shared-API change; preserve all Redis behavior and sentinels.

**Pattern skill:** `bluetape-go-patterns` error wrapping and backward compatibility.

**Files:**
- Create: `ratelimit/errors.go`
- Create: `ratelimit/errors_test.go`
- Modify: `ratelimit/redis/limiter.go`
- Modify: `ratelimit/redis/operation_error_test.go`
- Modify: `ratelimit/redis/conformance_test.go`

- [ ] **Step 1: Write RED root and Redis compatibility tests**

Add compile-time and nested wrapping tests with this contract:

```go
type operationErrorStub struct{}

func (operationErrorStub) Error() string     { return "rate limiter consume failed" }
func (operationErrorStub) Family() string    { return "rate limiter" }
func (operationErrorStub) Operation() string { return "consume" }
func (operationErrorStub) KeyID() string     { return "key:0123" }

func TestOperationErrorContract(t *testing.T) {
    var operationErr OperationError = operationErrorStub{}
    wrapped := fmt.Errorf("nested: %w", operationErr)
    var target OperationError
    if !errors.As(wrapped, &target) || target.Family() != "rate limiter" ||
        target.Operation() != "consume" || target.KeyID() != "key:0123" {
        t.Fatalf("OperationError inspection failed: %v", wrapped)
    }
}

func TestRedisCommitUnknownMatchesBothContracts(t *testing.T) {
    cause := errors.New("lost response")
    err := errors.Join(
        btredis.NewOpError(btredis.OpLabels{Family: "rate limiter", Operation: "consume"}, "raw-key", cause),
        ratelimit.ErrCommitUnknown,
        btredis.ErrCommitUnknown,
    )
    if !errors.Is(err, ratelimit.ErrCommitUnknown) || !errors.Is(err, btredis.ErrCommitUnknown) {
        t.Fatalf("commit unknown compatibility = %v", err)
    }
    var operationErr ratelimit.OperationError
    if !errors.As(err, &operationErr) { t.Fatalf("provider-neutral operation error missing: %v", err) }
}
```

`operationErrorStub` implements `error`, `Family`, `Operation`, and `KeyID`. Extend the existing
Redis lost-response/conformance tests to assert zero `Result`, both sentinels, the root interface,
and preservation through `fmt.Errorf("nested: %w", err)`.

- [ ] **Step 2: Observe RED**

```bash
go test -count=1 ./ratelimit ./ratelimit/redis -run 'OperationError|CommitUnknown'
```

Expected: build FAIL because the root sentinel/interface do not exist and Redis does not match the
root sentinel.

- [ ] **Step 3: Add the minimal root contract and Redis join**

Create:

```go
package ratelimit

import "errors"

// ErrCommitUnknown indicates that a dispatched debit may have committed.
var ErrCommitUnknown = errors.New("ratelimit: commit outcome unknown")

// OperationError exposes provider-neutral redacted failure diagnostics.
// KeyID is for sampled diagnostic correlation and must not be used as a metric label.
type OperationError interface {
    error
    Family() string
    Operation() string
    KeyID() string
}
```

In both Redis mutation-uncertainty branches, join the existing typed error with both sentinels
while preserving the current operation label: execution errors remain `consume` and result-scan
errors remain `parse-result`:

```go
// Eval/Slice execution failure keeps the existing "consume" label.
return ratelimit.Result{}, errors.Join(
    operationError(ctx, "consume", bucketKey, err),
    ratelimit.ErrCommitUnknown,
    btredis.ErrCommitUnknown,
)

// Result conversion failure keeps the existing "parse-result" label.
return ratelimit.Result{}, errors.Join(
    operationError(ctx, "parse-result", bucketKey, err),
    ratelimit.ErrCommitUnknown,
    btredis.ErrCommitUnknown,
)
```

Add `var _ ratelimit.OperationError = (*btredis.OpError)(nil)` in the Redis package tests and
assert both `consume` and `parse-result` survive nested root-interface inspection. Do not change
`btredis.ErrCommitUnknown`, existing operation labels, or any shared Redis error string.

- [ ] **Step 4: Verify GREEN, full Redis regression, and commit**

```bash
gofmt -w ratelimit/errors.go ratelimit/errors_test.go ratelimit/redis/limiter.go ratelimit/redis/operation_error_test.go ratelimit/redis/conformance_test.go
go test -count=1 ./ratelimit ./ratelimit/redis ./redis
git add ratelimit/errors.go ratelimit/errors_test.go ratelimit/redis/limiter.go ratelimit/redis/operation_error_test.go ratelimit/redis/conformance_test.go
git commit -m "feat: add provider-neutral rate limit errors"
```

Expected: all tests PASS; Redis lost responses match old and new sentinels without exposing keys or
causes. Roll back this commit if any existing Redis caller-facing test changes unexpectedly.

### Task 2: Define SQL Options, Constructor, and Schema

**Complexity:** Medium public API/schema contract.

**Pattern skill:** `bluetape-go-patterns` package design, Go docs, validation, caller ownership.

**Files:**
- Create: `ratelimit/sql/doc.go`
- Create: `ratelimit/sql/options.go`
- Create: `ratelimit/sql/options_test.go`
- Create: `ratelimit/sql/limiter.go`
- Create: `ratelimit/sql/limiter_test.go`
- Create: `ratelimit/sql/errors.go`
- Create: `ratelimit/sql/errors_test.go`
- Create: `ratelimit/sql/schema.go`
- Create: `ratelimit/sql/queries_test.go`

- [ ] **Step 1: Write RED constructor, option, key, nil, and schema tests**

Use table tests covering nil DB; NaN/Inf/zero/tiny/overflow rate; zero/negative/overflow burst;
negative, sub-full-refill, microsecond-ceiling, and overflow TTL; namespace default/trim/blank/129
bytes; key limit default and bounds 1..1024; blank/NUL/invalid-UTF8/exact-byte/oversized keys; and
constructor no-I/O. `Allow` nil-context and nil/zero-receiver behavior belongs to Task 3,
after the method exists; `Cleanup` nil/zero behavior belongs to Task 5. Assert the constructor shape:

```go
func TestNewDoesNotTouchDatabase(t *testing.T) {
    db := &sql.DB{}
    limiter, err := New(db, Options{RatePerSecond: 10, Burst: 20})
    if err != nil || limiter == nil { t.Fatalf("New() = %v, %v", limiter, err) }
}

func TestSchemaSQLHasFixedContract(t *testing.T) {
    normalized := strings.ToLower(SchemaSQL)
    for _, required := range []string{
        "public.bluetape_ratelimit_buckets", "namespace bytea not null",
        "bucket_key bytea not null", "tokens_micros numeric(30, 6) not null",
        "primary key (namespace, bucket_key)",
        "bluetape_ratelimit_buckets_expires_at_idx", "(expires_at)",
    } {
        if !strings.Contains(normalized, required) { t.Fatalf("SchemaSQL missing %q", required) }
    }
}
```

Use a private `normalizeKey` unit test to prove successful keys return the original bytes unchanged
and invalid keys produce no backend call. Assert `MaxCleanupBatch == 1000` and nested
`errors.Is(fmt.Errorf("nested: %w", ErrConfigurationMismatch), ErrConfigurationMismatch)` is true.

- [ ] **Step 2: Observe RED**

```bash
go test -count=1 ./ratelimit/sql -run 'TestNew|TestOptions|TestKey|TestSchema|ConfigurationMismatch'
```

Expected: build FAIL because `ratelimit/sql`, `Options`, `Limiter`, `SchemaSQL`, and
`MaxCleanupBatch` do not exist.

- [ ] **Step 3: Add the minimal public package and normalized options**

Define:

```go
package sqlratelimit

type Options struct {
    Namespace     string
    RatePerSecond float64
    Burst         int64
    IdleTTL       time.Duration
    MaxKeyBytes   int
}

type Limiter struct {
    db       *sql.DB
    opts     options
    testHook func(operation string, phase testPhase, key string) error
}

type testPhase string

const (
    phaseBeforeLinearize testPhase = "before-linearize"
    phaseAfterLinearize  testPhase = "after-linearize"
)

func New(db *sql.DB, opts Options) (*Limiter, error) {
    if db == nil { return nil, errors.New("postgres rate limiter database must not be nil") }
    normalized, err := opts.normalize()
    if err != nil { return nil, err }
    return &Limiter{db: db, opts: normalized}, nil
}

var ErrConfigurationMismatch = errors.New("sql rate limiter: configuration mismatch")
```

The private `options` stores normalized namespace bytes, rounded positive
`rateMicrosPerSecond`, checked `burstMicros`, microsecond-ceiled `idleTTLMicros`, and key bound.
Reuse the Redis formulas behaviorally, not by importing a provider package. Use checked integer
helpers:

```go
const tokenScale int64 = 1_000_000
const defaultMaxKeyBytes = 512
const maxMaxKeyBytes = 1024
const maxNamespaceBytes = 128

func durationMicrosCeil(value time.Duration) (int64, error) {
    if value <= 0 { return 0, errors.New("duration must be positive") }
    micros := value / time.Microsecond
    if value%time.Microsecond != 0 { micros++ }
    return int64(micros), nil
}
```

Default `IdleTTL` is `max(2*fullRefill, time.Minute)` with overflow-safe saturation before the
microsecond conversion. `normalizeKey` rejects trimmed blank and byte length over the configured
limit but returns the original string. Nil context normalizes to `context.Background()`.

- [ ] **Step 4: Add the exact bootstrap schema**

```go
const MaxCleanupBatch = 1000

const SchemaSQL = `create table if not exists public.bluetape_ratelimit_buckets (
    namespace bytea not null,
    bucket_key bytea not null,
    rate_micros_per_second bigint not null check (rate_micros_per_second > 0),
    burst_micros bigint not null check (burst_micros > 0),
    idle_ttl_micros bigint not null check (idle_ttl_micros > 0),
    tokens_micros numeric(30, 6) not null check (tokens_micros >= 0 and tokens_micros <= burst_micros),
    last_allowed boolean not null,
    updated_at timestamptz not null,
    expires_at timestamptz not null check (expires_at >= updated_at),
    primary key (namespace, bucket_key)
);
create index if not exists bluetape_ratelimit_buckets_expires_at_idx
on public.bluetape_ratelimit_buckets (expires_at)`
```

Go doc must say `SchemaSQL` is caller-executed bootstrap, not verification/upgrade logic, and
`New` never executes it or closes the pool.

- [ ] **Step 5: Verify GREEN and commit**

```bash
gofmt -w ratelimit/sql/doc.go ratelimit/sql/options.go ratelimit/sql/options_test.go ratelimit/sql/limiter.go ratelimit/sql/limiter_test.go ratelimit/sql/errors.go ratelimit/sql/errors_test.go ratelimit/sql/schema.go ratelimit/sql/queries_test.go
go test -count=1 ./ratelimit/sql -run 'TestNew|TestOptions|TestKey|TestSchema|ConfigurationMismatch'
git add ratelimit/sql
git commit -m "feat: define PostgreSQL rate limiter provider"
```

Expected: constructor/schema tests PASS without opening a database connection and invalid options
fail before any backend access. No `go.mod` or `go.sum` change occurs.

### Task 3: Implement Atomic Server-Time Admission

**Complexity:** High database hot path and exact-arithmetic boundary.

**Pattern skill:** `bluetape-go-patterns` database/concurrency tests and error wrapping.

**Files:**
- Create: `ratelimit/sql/queries.go`
- Modify: `ratelimit/sql/limiter.go`
- Modify: `ratelimit/sql/queries_test.go`
- Modify: `ratelimit/sql/limiter_test.go`

- [ ] **Step 1: Add RED real-PostgreSQL happy-path and precision tests**

Create one serial fixture with a 90-second context, `postgrestestcontainer.Start`,
`sql.Open("pgx", dsn)`, `PingContext`, caller execution of `SchemaSQL`, and
`t.Cleanup(func() { _ = db.Close() })`. Every additional pool gets its own `t.Cleanup` close. Use
unique namespace/key values per subtest.
Add: initial full-burst debit; exact rejection result; refill; key and namespace isolation; shared
bucket across two independent pools; rejected-attempt expiry extension; NUL/invalid-UTF8 byte
identity; configuration mismatch row/quota/version no-op; repeated sub-microtoken progress; and
duration ceil/saturation scan boundaries.

Add local tests for nil context normalization, pre-canceled context with zero backend traffic, and
nil/zero `*Limiter` receivers returning zero result plus initialization error without panic. Once
`Allow` exists, add `var _ ratelimit.Limiter = (*Limiter)(nil)` beside the type.

For mismatch, read the row before/after and assert `tokens_micros`, rate, burst, TTL, `updated_at`,
and `xmin` are unchanged. For fractional carry, use a low rate and repeated rejected polls whose
individual elapsed refills are below one microtoken, then assert their accumulated state eventually
admits exactly when the summed server elapsed time permits.

Add a deterministic stale-observation regression: begin an admin transaction, lock the bucket row,
advance its `updated_at` and token state inside that transaction, start `Allow` and confirm through
`pg_stat_activity` that it is waiting on the row lock, then release the transaction. Assert the
waiter's older observed time never regresses `updated_at`/`expires_at` and the next request cannot
obtain excess admission from double refill.

- [ ] **Step 2: Observe RED**

```bash
go test -p 1 -count=1 ./ratelimit/sql -run '^TestAllowPostgres$'
```

Expected: FAIL because `Allow`, the UPSERT, result scanning, and configuration-mismatch mapping are absent.

- [ ] **Step 3: Implement the single-statement UPSERT**

Define one compile-time constant using only `$1..$6` for namespace bytes, key bytes, requested
micros, burst micros, rate micros/second, and idle TTL micros. Its shape must be:

```sql
insert into public.bluetape_ratelimit_buckets as bucket (
  namespace,bucket_key,rate_micros_per_second,burst_micros,idle_ttl_micros,
  tokens_micros,last_allowed,updated_at,expires_at
)
select $1::bytea,$2::bytea,$5::bigint,$4::bigint,$6::bigint,
  ($4::numeric-$3::numeric),true,observed_at,
  observed_at + $6::bigint * interval '1 microsecond'
from (select pg_catalog.clock_timestamp() as observed_at) as clock
on conflict (namespace,bucket_key) do update set
  tokens_micros = case when
    least(bucket.burst_micros::numeric,
      bucket.tokens_micros + greatest(0::numeric,
        extract(epoch from (greatest(bucket.updated_at,excluded.updated_at)-bucket.updated_at))) *
        bucket.rate_micros_per_second::numeric) >= $3::numeric
    then least(bucket.burst_micros::numeric,
      bucket.tokens_micros + greatest(0::numeric,
        extract(epoch from (greatest(bucket.updated_at,excluded.updated_at)-bucket.updated_at))) *
        bucket.rate_micros_per_second::numeric) - $3::numeric
    else least(bucket.burst_micros::numeric,
      bucket.tokens_micros + greatest(0::numeric,
        extract(epoch from (greatest(bucket.updated_at,excluded.updated_at)-bucket.updated_at))) *
        bucket.rate_micros_per_second::numeric)
  end,
  last_allowed = least(bucket.burst_micros::numeric,
    bucket.tokens_micros + greatest(0::numeric,
      extract(epoch from (greatest(bucket.updated_at,excluded.updated_at)-bucket.updated_at))) *
      bucket.rate_micros_per_second::numeric) >= $3::numeric,
  updated_at = greatest(bucket.updated_at,excluded.updated_at),
  expires_at = greatest(bucket.updated_at,excluded.updated_at) +
    bucket.idle_ttl_micros * interval '1 microsecond'
where bucket.rate_micros_per_second=excluded.rate_micros_per_second
  and bucket.burst_micros=excluded.burst_micros
  and bucket.idle_ttl_micros=excluded.idle_ttl_micros
returning last_allowed,
  pg_catalog.floor(tokens_micros)::bigint,
  pg_catalog.ceil(greatest(0::numeric,$3::numeric-tokens_micros)*1000000 /
    rate_micros_per_second)::bigint,
  pg_catalog.ceil((burst_micros::numeric-tokens_micros)*1000000 /
    rate_micros_per_second)::bigint
```

The repeated refill expression is intentional because PostgreSQL `ON CONFLICT DO UPDATE` has no
`FROM` clause. `greatest(bucket.updated_at, excluded.updated_at)` prevents a statement that waited
for the conflict-row lock from moving time backwards; losing lock-wait time is conservative and
cannot over-admit. Validate this exact statement directly against the fixture before wrapping it
in Go; do not replace it with a read-then-write sequence or a caller transaction.

- [ ] **Step 4: Scan and convert the confirmed result**

`Allow` performs local preflight, calls `QueryRowContext`, and scans
`allowed bool`, `remaining int64`, `retryMicros int64`, `resetMicros int64`. Map `sql.ErrNoRows`
to zero result plus `ErrConfigurationMismatch`. Convert nonnegative microseconds with a checked
saturating helper:

```go
func microsDuration(value int64) time.Duration {
    if value <= 0 { return 0 }
    if value > math.MaxInt64/int64(time.Microsecond) { return time.Duration(math.MaxInt64) }
    return time.Duration(value) * time.Microsecond
}
```

Return `ratelimit.Result{Allowed: allowed, Requested: tokens, Remaining: remaining,
RetryAfter: microsDuration(retryMicros), ResetAfter: microsDuration(resetMicros)}`. Every local or
backend error returns `ratelimit.Result{}`.

- [ ] **Step 5: Verify GREEN, inspect the query, and commit**

```bash
gofmt -w ratelimit/sql/queries.go ratelimit/sql/limiter.go ratelimit/sql/queries_test.go ratelimit/sql/limiter_test.go
go test -p 1 -count=1 ./ratelimit/sql -run 'TestAllowPostgres|TestAllowValidation|TestDuration'
rg -n 'fmt\.Sprintf|fmt\.Appendf|\+.*(namespace|key)|clock_timestamp\(\)' ratelimit/sql/queries.go
git add ratelimit/sql
git commit -m "feat: add atomic PostgreSQL rate limiting"
```

Expected: targeted tests PASS; inspection shows fixed SQL, exactly one textual
`clock_timestamp()` call in the UPSERT, and no runtime SQL interpolation. If mismatch changes
`xmin` or quota/config values, do not commit; repair the statement and rerun the whole fixture.

### Task 4: Classify Failures and Prove Cancellation Boundaries

**Complexity:** High indeterminate-mutation and fault-injection behavior.

**Files:**
- Modify: `ratelimit/sql/errors.go`
- Modify: `ratelimit/sql/errors_test.go`
- Modify: `ratelimit/sql/limiter.go`
- Modify: `ratelimit/sql/limiter_test.go`
- Create: `ratelimit/sql/conformance_test.go`

- [ ] **Step 1: Write RED typed-error and deterministic boundary tests**

Add tests proving: zero/nil `OpError` methods do not panic; nested `errors.As` reaches both
`*OpError` and `ratelimit.OperationError`; error strings omit raw key, namespace, DSN, SQL, endpoint,
and cause text; `KeyID` is stable for identical raw `(namespace,key)` bytes and differs for a second
key; known `*pgconn.PgError` failure is typed but not commit-unknown; transport/scan/lost-response
failure is typed plus root commit-unknown; all errors return zero result.

Use the package-private `testPhase` and constants introduced with `Limiter` in Task 2 to build
deterministic controls around both phases.

The `operation` argument is `allow` or `cleanup`; cleanup passes an empty key and never derives a
key ID from it. The before hook blocks before SQL dispatch and returns the original canceled context error without
operation wrapping or traffic. The after hook runs only after a complete successful `Scan`; a
caller cancellation while blocked there must still return the confirmed result. An injected error
there simulates response loss and must return zero result plus `OpError` and
`ratelimit.ErrCommitUnknown`, with exactly one stored debit.

Add a real in-flight cancellation test rather than relying only on hooks: create a bucket, acquire
and `defer Rollback` an admin transaction that row-locks it, start `Allow` with a bounded cancelable
context, poll `pg_stat_activity` until that backend is waiting on a lock, then cancel. Assert prompt
return with zero result, `context.Canceled`, `*OpError`, and `ratelimit.ErrCommitUnknown`; assert no
internal retry and, after releasing the admin transaction, no unexpected debit. Close the dedicated
`*sql.Conn` on every exit path. Run this case repeatedly and under race.

- [ ] **Step 2: Observe RED**

```bash
go test -p 1 -count=1 ./ratelimit/sql -run 'TestOpError|TestAllowCancel|TestAllowInFlightCancellation|TestAllowLostResponse|TestAllowKnownRollback'
```

Expected: FAIL because SQL diagnostics and phase controls are absent.

- [ ] **Step 3: Implement redacted SQL diagnostics**

Extend the Task 2 sentinel file with:

```go
type OpError struct {
    operation string
    keyID     string
    err       error
}

func (e *OpError) Error() string     { return e.Family() + " " + e.Operation() + " failed" }
func (e *OpError) Unwrap() error     { if e == nil { return nil }; return e.err }
func (e *OpError) Family() string    { return "rate limiter" }
func (e *OpError) Operation() string { if e == nil || e.operation == "" { return "operation" }; return e.operation }
func (e *OpError) KeyID() string     { if e == nil || e.keyID == "" { return "sql-rate-key:<missing>" }; return e.keyID }

func newOperationError(operation, namespace, key string, err error) error {
    return &OpError{operation: operation, keyID: redactedKeyID(namespace, key), err: err}
}

func newCleanupOperationError(err error) error {
    return &OpError{operation: "cleanup", keyID: "sql-rate-key:<cleanup>", err: err}
}

func redactedKeyID(namespace, key string) string {
    hash := sha256.New()
    var size [8]byte
    binary.BigEndian.PutUint64(size[:], uint64(len(namespace)))
    _, _ = hash.Write(size[:])
    _, _ = hash.Write([]byte(namespace))
    _, _ = hash.Write([]byte(key))
    return "sql-rate-key:" + hex.EncodeToString(hash.Sum(nil)[:10])
}
```

Compute `KeyID` as `sql-rate-key:` plus the first 20 lowercase hex characters of SHA-256 over
`binary.BigEndian uint64(namespace byte length) || namespace bytes || key bytes`; the length prefix
prevents ambiguous boundaries even when either Go string contains NUL. `Error()` contains only the
safe family and operation, not the key ID or causal text. Assert
`var _ ratelimit.OperationError = (*OpError)(nil)`.

- [ ] **Step 4: Implement known-rollback versus uncertain handling**

`sql.ErrNoRows` remains configuration mismatch. A `*pgconn.PgError` proves statement failure and
returns only `*OpError`. Errors after dispatch that are not PostgreSQL server errors, including
context cancellation/deadline, driver transport errors, scan conversion, and the after-scan
failure hook, return:

```go
return ratelimit.Result{}, errors.Join(
    newOperationError("allow", namespace, key, errors.Join(err, ctx.Err())),
    ratelimit.ErrCommitUnknown,
)
```

Join `ctx.Err()` only when non-nil. Do not retry. Complete the `Scan`, build the result, run the
after-linearize test hook, then return the confirmed result without rechecking `ctx.Err()`.

- [ ] **Step 5: Verify GREEN, conformance fault cases, and commit**

```bash
gofmt -w ratelimit/sql/errors.go ratelimit/sql/errors_test.go ratelimit/sql/limiter.go ratelimit/sql/limiter_test.go ratelimit/sql/conformance_test.go
go test -p 1 -count=1 ./ratelimit/sql -run 'TestOpError|TestAllowCancel|TestAllowInFlightCancellation|TestAllowLostResponse|TestAllowKnownRollback'
go test -race -p 1 -count=1 ./ratelimit/sql -run 'TestAllowCancel|TestAllowInFlightCancellation|TestAllowLostResponse'
go test -p 1 -count=10 ./ratelimit/sql -run '^TestAllowInFlightCancellation$'
git add ratelimit/sql
git commit -m "feat: harden PostgreSQL rate limit failures"
```

Expected: all boundary tests PASS under race; pre-dispatch cancellation stores no row, post-scan
cancellation returns success, and lost response stores exactly one debit but returns zero plus the
root sentinel.

### Task 5: Add Caller-Owned Bounded Cleanup

**Complexity:** Medium maintenance operation with lock and response-loss risks.

**Files:**
- Modify: `ratelimit/sql/limiter.go`
- Modify: `ratelimit/sql/queries.go`
- Modify: `ratelimit/sql/queries_test.go`
- Modify: `ratelimit/sql/errors_test.go`

- [ ] **Step 1: Add RED cleanup validation, lifecycle, and concurrency tests**

Test `limit` values `-1,0,1001` return count 0/error/no SQL; nil/zero limiter is safe; cleanup
deletes at most the requested count in expiry-index order; live/refreshed rows survive; two workers
using separate pools do not double-count a row; concurrent `Allow` either refreshes the row or
inserts a fresh full bucket after cleanup; and missing table/read-only/permission/response-loss
errors return count 0. On uncertain completion, a direct admin count may show up to `limit` rows
deleted, and a retry operates on the next currently expired batch rather than reproducing the
first count.

- [ ] **Step 2: Observe RED**

```bash
go test -p 1 -count=1 ./ratelimit/sql -run '^TestCleanup'
```

Expected: build FAIL because `Cleanup` and its statement are absent.

- [ ] **Step 3: Implement one bounded cleanup statement**

Use only a positional limit bind and server time:

```sql
with observed as materialized (
  select pg_catalog.clock_timestamp() as observed_at
), candidates as (
  select bucket.namespace,bucket.bucket_key
  from public.bluetape_ratelimit_buckets as bucket cross join observed
  where bucket.expires_at <= observed.observed_at
  order by bucket.expires_at,bucket.namespace,bucket.bucket_key
  limit $1
  for update of bucket skip locked
), deleted as (
  delete from public.bluetape_ratelimit_buckets as bucket
  using candidates
  where bucket.namespace=candidates.namespace
    and bucket.bucket_key=candidates.bucket_key
  returning 1
)
select count(*)::bigint from deleted
```

`Cleanup` normalizes nil context, rejects pre-canceled context and invalid limits before traffic,
and scans a single `int64`. Server `*pgconn.PgError` returns typed `cleanup` error only; all other
post-dispatch errors join the root commit-unknown sentinel. Every error returns count 0. Do not add
a ticker, goroutine, retry, transaction API, or pool close. Invoke the same private test hook with
operation `cleanup` before dispatch and after the count scan so response-loss and cancellation
tests are deterministic; cleanup operation errors use the fixed safe KeyID
`sql-rate-key:<cleanup>`.

- [ ] **Step 4: Verify GREEN and commit**

```bash
gofmt -w ratelimit/sql/limiter.go ratelimit/sql/queries.go ratelimit/sql/queries_test.go ratelimit/sql/errors_test.go
go test -p 1 -count=1 ./ratelimit/sql -run '^TestCleanup'
go test -race -p 1 -count=1 ./ratelimit/sql -run 'TestCleanupConcurrent|TestCleanupAllowRace'
git add ratelimit/sql
git commit -m "feat: add bounded rate limit cleanup"
```

Expected: cleanup tests PASS, each call touches at most 1000 rows, no worker blocks indefinitely,
and refreshed rows survive. If contention causes starvation or timeouts, stop and repair statement
ordering/fixture synchronization rather than increasing test timeouts.

### Task 6: Run Mandatory Conformance, Stress, and Security Proof

**Complexity:** High backend capability and deployment-contract proof.

**Files:**
- Complete: `ratelimit/sql/conformance_test.go`
- Create: `ratelimit/sql/stress_test.go`
- Create: `ratelimit/sql/security_test.go`
- Modify: `ratelimit/sql/queries_test.go`

- [ ] **Step 1: Wire the mandatory conformance harness without capability flags**

The factory opens an independent pgx `*sql.DB` per limiter, immediately registers
`tb.Cleanup(func() { _ = db.Close() })`, applies no DDL, sets a unique `conformance` namespace,
and installs only the private test hook. Container startup is registered first so LIFO cleanup
closes every pool before terminating the container. Adapt `ratelimit.Result` field by field. The
classifier is provider-neutral:

```go
IsProviderError: func(err error) bool {
    var target ratelimit.OperationError
    return errors.As(err, &target) && target.Family() == "rate limiter"
},
```

Implement `GateNext`, `FailNext`, and `OperationCount` with a mutex and one-shot channels, matching
the existing Redis control semantics. Run `ratelimittest.Run(t, harness)` with no skipped cases.

- [ ] **Step 2: Add exact multi-pool and repeated precision stress**

Open at least four independent pools against one fixture. Release `Burst+32` workers from a
barrier, call one-token `Allow`, require every worker to finish under a bounded context, and assert
the sum of admitted requested tokens is exactly `Burst`. Repeat with unique namespaces for 20
iterations. Register `t.Cleanup(db.Close)` immediately for every pool, close every acquired
`*sql.Conn`, and roll back every transaction on all exits. Add a repeated sub-quantum
rejection/refill test and a concurrent cleanup/Allow stress that records maximum latency without
asserting an unmeasured capacity number.

Add a named `TestCleanupAllowPoolContention` using one deliberately constrained shared pool with
`SetMaxOpenConns(2)` and `SetMaxIdleConns(2)`. Seed disjoint expired rows plus one hot bucket, issue
fixed counts of bounded `Allow` and `Cleanup` calls under per-call and global contexts, and assert
every call completes before the global deadline, outcomes account for every issued call, cleanup
deletes at least one expired row, the hot-bucket admitted sum is exactly `Burst`, and every worker
exits. Capture `DBStats.WaitCount`, `WaitDuration`, and maximum operation latency as diagnostics
only; do not turn them into an unsupported capacity threshold.

- [ ] **Step 3: Add schema and least-privilege deployment proof**

Create `ratelimit_migration_owner` and `ratelimit_runtime`; first verify `public` is owned by the
approved migration authority, revoke `CREATE ON SCHEMA public FROM PUBLIC`, and prove effective
schema `CREATE` is false for runtime, PUBLIC, and every inherited membership. Only then execute
`SchemaSQL` as the migration owner under bounded `lock_timeout` and `statement_timeout`, run the
full catalog preflight, and grant runtime only schema `USAGE` and table
`SELECT,INSERT,UPDATE,DELETE`. Run constructor, conformance, and cleanup through the runtime pool.
Register bounded fixture context cancellation and pool/connection cleanup immediately after every
successful acquisition; pool cleanup must run before Testcontainers termination, and cleanup
asserts `DBStats.InUse == 0` before close.
Assert runtime denial for schema/table create, alter, truncate, references, trigger, and grant.

Query catalogs to assert ordinary table relkind, exact column order/types/nullability, primary key
`namespace,bucket_key`, valid/ready expiry index, expected check definitions, migration owner,
runtime non-ownership/non-membership, direct/inherited/PUBLIC effective privilege boundaries,
RLS/forced-RLS false, and zero user triggers. In a rollbacked transaction, create incompatible
pre-existing relations/indexes, enable RLS, and add a user trigger; prove the documented catalog
preflight rejects every variant. Also prove a read-only transaction fails `Allow` without marking
the known PostgreSQL rejection commit-unknown.

For hostile-object cases, create a separate attacker role while schema CREATE is intentionally
available, pre-seed each conflicting relation/index/trigger, then revoke PUBLIC/attacker CREATE
before invoking bootstrap and preflight. Assert deployment fails closed, ownership is never
adopted or repaired, and no provider `Allow`/`Cleanup` traffic is issued.

- [ ] **Step 4: Run the serial backend proof**

```bash
go test -p 1 -count=1 ./ratelimit/sql -run 'TestPostgresRateLimiterConformance|TestMultiPool|TestFractional|TestCleanupAllowPoolContention|TestRuntimeRole|TestSchemaCatalog'
go test -race -p 1 -count=1 ./ratelimit/sql
go test -p 1 -count=10 ./ratelimit/sql -run 'TestPostgresRateLimiterConformance|TestMultiPoolExactAdmission|TestCleanupAllowPoolContention'
```

Expected: mandatory conformance has no skips; exact admission equals burst in every iteration;
race is clean; security/catalog cases pass. A one-off failure must be reproduced and explained;
do not label it flaky merely because a rerun passes.

- [ ] **Step 5: Commit provider proof**

```bash
git add ratelimit/sql/conformance_test.go ratelimit/sql/stress_test.go ratelimit/sql/security_test.go ratelimit/sql/queries_test.go
git commit -m "test: prove PostgreSQL rate limiter contract"
```

Expected: the commit contains only tests and no relaxed timeout, skipped case, benchmark claim, or
production behavior change.

### Task 7: Document the SQL Provider and Compile-Checked Usage

**Complexity:** Medium public API and bilingual operations guidance.

**Skills:** `bluetape-go-patterns` for Go docs; `bluetape-writer` for natural Korean parity;
`bluetape-diagram` for the shared sequence asset and rendered visual validation.

**Files:**
- Create: `ratelimit/sql/README.md`
- Create: `ratelimit/sql/README.ko.md`
- Create: `ratelimit/sql/example_test.go`
- Create: `ratelimit/sql/readme_test.go`
- Create: `docs/images/readme-diagrams/postgres-ratelimit-token-bucket-sequence.svg`
- Create: `docs/images/readme-diagrams/postgres-ratelimit-token-bucket-sequence.png`
- Modify: `ratelimit/sql/doc.go`
- Modify: exported-symbol Go docs in `ratelimit/sql/*.go`
- Modify: `ratelimit/README.md`
- Modify: `ratelimit/README.ko.md`

- [ ] **Step 1: Add RED example and documentation contract tests**

`ExampleNew` must compile this ownership shape: caller opens/closes pgx `*sql.DB`; migration owner
executes `SchemaSQL`; runtime calls `New`; `Allow` ignores every result when error is non-nil;
commit-unknown is not replayed; caller invokes bounded `Cleanup` from its own scheduler and discards
the returned count on every cleanup error rather than replaying the same batch. The example must
not require a live database to produce output.

`readme_test.go` checks both README files contain: the shared
`postgres-ratelimit-token-bucket-sequence.png` asset, `SchemaSQL`, `New`, `Allow`, `Cleanup`,
`ErrConfigurationMismatch`, `ErrCommitUnknown`, caller-owned DB/migration/scheduler,
moderate-QPS/non-Redis-replacement guidance, fixed relation, least-privilege grants, primary-only
routing, configuration migration/namespace rotation, no automatic replay, and language switches.
The SQL README assertions also require: cleanup error returns count zero although up to `limit` rows
may already be deleted; retry advances current expired work and is not idempotent batch replay;
local/Redis/SQL quota state is not shared; simultaneous mixed-provider serving is prohibited because
each provider can grant a full burst; canaries use independent namespaces/cohorts; and cutover
quiesces the old provider and waits a full-refill window or records an approved extra-burst budget.

- [ ] **Step 2: Observe RED**

```bash
go test -count=1 ./ratelimit/sql -run 'ExampleNew|TestReadmeContract'
```

Expected: FAIL because the examples and README files do not exist.

- [ ] **Step 3: Write English and natural Korean package documentation**

Keep the two README files source-equivalent. Include architecture and one-statement behavior;
installation/import; schema bootstrap and full catalog preflight; minimum grants; API/config table;
arbitrary byte key and cardinality constraints; cleanup scheduling/pressure controls; result/error
handling; failure matrix; primary/proxy/HA/RPO requirements; observability without key/namespace
metric labels; configuration changes; provider cutover/rollback; and explicit unsupported ORM,
caller transaction, auto-migration, background cleanup, non-PostgreSQL, and high-QPS claims.

Update the parent `ratelimit` README pair with a local/Redis/PostgreSQL selection table and shared
`ratelimit.OperationError`/`ErrCommitUnknown` example. Require the pair to repeat that provider
quota state is not shared, mixed simultaneous serving can grant multiple full bursts, and safe
canary/cutover uses independent namespaces/cohorts plus quiesce-and-wait or an approved extra-burst
budget. Keep the benchmark chart N/A because issue #529 makes no measured capacity comparison. Add a
source-backed sequence diagram using the repository best-practices reference. It must answer where
dispatch begins, where same-key atomicity is established, which outcomes debit quota, and why a
commit-unknown result must not be replayed. Share one English-label PNG between both provider
READMEs and retain the matching SVG source.

- [ ] **Step 4: Verify docs parity and commit**

```bash
gofmt -w ratelimit/sql/doc.go ratelimit/sql/example_test.go ratelimit/sql/readme_test.go ratelimit/sql/*.go
go test -count=1 ./ratelimit/sql -run 'ExampleNew|TestReadmeContract'
xmllint --noout docs/images/readme-diagrams/postgres-ratelimit-token-bucket-sequence.svg
cairosvg docs/images/readme-diagrams/postgres-ratelimit-token-bucket-sequence.svg \
  -o docs/images/readme-diagrams/postgres-ratelimit-token-bucket-sequence.png -s 2
python3 ~/.codex/skills/bluetape-diagram/scripts/diagram-connector-audit.py \
  docs/images/readme-diagrams/postgres-ratelimit-token-bucket-sequence.svg
python3 ~/.codex/skills/bluetape-diagram/scripts/diagram-geometry-audit.py --fail-diagonal \
  docs/images/readme-diagrams/postgres-ratelimit-token-bucket-sequence.svg
python3 ~/.codex/skills/bluetape-diagram/scripts/diagram-endpoint-audit.py \
  docs/images/readme-diagrams/postgres-ratelimit-token-bucket-sequence.svg
python3 ~/.codex/skills/bluetape-diagram/scripts/diagram-mixed-corner-audit.py \
  docs/images/readme-diagrams/postgres-ratelimit-token-bucket-sequence.svg
python3 ~/.codex/skills/bluetape-diagram/scripts/diagram-sequence-style-audit.py \
  docs/images/readme-diagrams/postgres-ratelimit-token-bucket-sequence.svg
git diff --check
git add ratelimit/sql ratelimit/README.md ratelimit/README.ko.md docs/images/readme-diagrams
git commit -m "docs: document PostgreSQL rate limiting"
```

Expected: compile-checked examples and README contract tests PASS; exported symbols have English Go
docs; English/Korean documents cover the same operational decisions; the SVG parses, its PNG is
rendered at 2x, diagram audits report no geometry/endpoint/style failure, and the full-size PNG has
no clipped text, overlap, or unreadable label.

### Task 8: Update Public Index, Changelog, and Release Runbook

**Complexity:** Medium release-facing documentation.

**Skills:** `bluetape-writer` for Korean parity. The provider sequence asset is owned by Task 7.

**Files:**
- Modify: `README.md`
- Modify: `README.ko.md`
- Modify: `CHANGELOG.md`
- Modify: `docs/release/v0.19.0-provider-conformance-runbook.md`

- [ ] **Step 1: Add the provider to both root indexes and 0.19.0 changelog**

Add adjacent `ratelimit/sql` rows describing PostgreSQL atomic token buckets for moderate-QPS,
database-only deployments. Keep English and Korean package indexes aligned. Add an English
0.19.0 changelog bullet naming the caller-owned schema/cleanup boundary, shared root error
inspection, and non-Redis-replacement caveat.

- [ ] **Step 2: Extend the provider runbook with executable deployment gates**

Add exact commands/queries in this mandatory order: verify `public` schema ownership; revoke
`CREATE ON SCHEMA public FROM PUBLIC`; verify runtime/PUBLIC/inherited effective CREATE is false;
set bounded migration `lock_timeout`/`statement_timeout`; apply `SchemaSQL` as migration owner; run
the full catalog preflight; then grant minimum runtime DML. Follow with writable-primary endpoint checks
(`pg_is_in_recovery()=false`, `transaction_read_only=off`, server identity/timeline); serial
conformance/race commands; bounded cleanup; and rollback.

Define deployment-recorded gates relative to a stable baseline: bounded-cardinality Allow
latency/outcome/error categories, `DBStats` wait/in-use, statement/row-lock latency, cleanup
count/duration/error/backlog/oldest expiry, live/dead tuples, table/index size, autovacuum lag, and
WAL growth. Require a predeclared consecutive-breach count and minimum canary observation window,
without inventing universal numeric thresholds. Prohibit key, namespace, DSN, endpoint, and raw
errors in metric labels/logs; allow redacted `KeyID` only in sampled diagnostics.

Specify an executable caller scheduler contract: cadence shorter than configured `IdleTTL`; a
fresh bounded context per run; `Cleanup` limit in `1..1000`; database lock/statement timeouts;
predeclared maximum batches and elapsed budget per run; jitter; and small bounded worker
concurrency. Pause cleanup when predeclared WAL, row-lock, pool-wait, or autovacuum pressure gates
breach. On any cleanup error the returned count is zero although up to `limit` rows may already be
deleted; retry advances the currently expired work and must not claim to replay an idempotent batch.

Cutover uses an independent canary namespace/cohort, then quiesces the old provider and waits a
full-refill window or records an approved extra-burst budget before single-provider activation.
Rollback mirrors that boundary. Require controlled failover proof for old-writer fencing,
durability/RPO, no statement replay, and commit-unknown no-replay before production promotion.
Rollback retains the SQL relation, expiry index, and grants. Destructive table/grant removal is a
separate migration allowed only after a predeclared observation window shows zero SQL-provider
binary deployment and zero SQL-provider traffic; record a pre-removal rollback point and verify
the objects and privileges are absent only after that migration succeeds.

- [ ] **Step 3: Add and run a release-runbook contract test**

Extend `ratelimit/sql/readme_test.go` to read the release runbook and assert the presence and order
of these markers: public-schema ownership verification; `REVOKE CREATE ... FROM PUBLIC`; bounded
`lock_timeout`/`statement_timeout`; `SchemaSQL`; catalog preflight; runtime grants; writable-primary
checks; cleanup cadence/limit/run budget/pause/uncertain count; baseline-relative promotion window;
independent canary; old-writer fencing and RPO; rollback retention; zero-usage observation; and
separate destructive migration. Compare marker indexes so the migration steps cannot be reordered.

Run:

```bash
go test -count=1 ./ratelimit/sql -run '^TestReleaseRunbookContract$'
```

Expected: PASS only when the executable migration, cleanup, promotion, rollback, and removal
contracts are present in the required order.

- [ ] **Step 4: Verify public-doc parity and commit**

```bash
rg -n 'ratelimit/sql|PostgreSQL|moderate-QPS|ErrCommitUnknown|Cleanup' README.md README.ko.md CHANGELOG.md docs/release/v0.19.0-provider-conformance-runbook.md
go test -count=1 ./ratelimit/sql -run '^TestReleaseRunbookContract$'
git diff --check
git add README.md README.ko.md CHANGELOG.md docs/release/v0.19.0-provider-conformance-runbook.md ratelimit/sql/readme_test.go
git commit -m "docs: add SQL rate limiter rollout guidance"
```

Expected: all four public surfaces discover the provider and its caveats; bilingual root entries
are aligned; the runbook contains measurable promotion, rollback, HA, and cleanup evidence.

### Task 9: Final Verification, Acceptance Mapping, and Pre-PR Review

**Complexity:** High integration gate; no feature edits unless a failure is traced and repaired.

**Skills:** `verification-before-completion`, `requesting-code-review`, and
`bluetape-full-feature` Step 6-R with all six review lenses plus main integration.

**Files:**
- Review: all files changed from `origin/develop...HEAD`
- Create/update: the issue #529 Step 6-R review artifact under `docs/superpowers/reviews/`
- Create if durable learning warrants it: `docs/lessons/2026-07-13-issue-529-sql-rate-limiter.md`

- [ ] **Step 1: Run cheap/static gates**

```bash
gofmt -w ratelimit/errors.go ratelimit/errors_test.go ratelimit/redis/*.go ratelimit/sql/*.go
make fmt-check
make tidy-check
make vet
make lint
git diff --check
```

Expected: all commands exit 0, no `go.mod`/`go.sum` drift, and no formatting changes remain.

- [ ] **Step 2: Run targeted, race, and repository gates serially**

```bash
go test -p 1 -count=1 ./ratelimit/... ./redis/...
go test -race -p 1 -count=1 ./ratelimit/sql ./ratelimit/redis
go test -p 1 -count=10 ./ratelimit/sql -run 'TestPostgresRateLimiterConformance|TestMultiPoolExactAdmission|TestCleanupAllowPoolContention'
make ci
```

Expected: every command exits 0. `make ci` is the authoritative final local gate. Lost process
handles or missing exit codes are not evidence; rerun such a command from scratch.

- [ ] **Step 3: Map every acceptance criterion to evidence**

Record a table in the Step 6-R artifact mapping spec criteria 1..11 to exact tests, docs, and
commands. Explicitly record N/A evidence: no new runtime dependency, module/BOM/CI registration,
ORM/Spring/Exposed/coroutine/streaming/JDK-preview work, and benchmark/chart. Record the sequence
diagram as PASS with the source/render paths, audit output, and full-size inspection evidence.

- [ ] **Step 4: Run the six review lenses and main integration**

Review the exact `origin/develop...HEAD` diff for performance, stability, security, operator/Ops,
developer/API, and user/caller. Fix every P0/P1, rerun the affected targeted proof and lens, and
record P2/P3 repair or explicit follow-up rationale. Main integration checks SQL correctness,
error evidence integrity, docs parity, release readiness, and absence of unsupported claims.

Expected exit verdict: `P0=0 P1=0`, no unresolved finding, no placeholder, and clean status except
the review/lesson artifact being committed.

- [ ] **Step 5: Commit final evidence and prepare the PR handoff**

```bash
git add docs/superpowers/reviews docs/lessons
git commit -m "docs: record PostgreSQL rate limiter verification"
git status --short --branch
git log --oneline origin/develop..HEAD
```

Expected: branch is clean and ahead of `origin/develop`; implementation, tests, docs, and review
evidence are committed. Stop before push/PR if authorization has not yet been given. When a PR is
authorized, copy issue #529 milestone `0.19.0`, assignee `debop`, and labels; end the PR body with
`## DoD Status`; wait for live CI/review and an explicit merge decision.

## Acceptance Traceability

| Spec acceptance criterion | Plan evidence |
|---|---|
| 1. Constructor-only `New(*sql.DB, Options)` and root interface | Task 2 constructor/interface tests and Go docs |
| 2. Caller-owned `SchemaSQL` and bounded `Cleanup` | Tasks 2, 5, 7, and 8 |
| 3. One-statement server-time atomic refill/debit | Task 3 query/integration proof |
| 4. Full `ratelimittest.Run` without skips | Tasks 4 and 6 |
| 5. Exact multi-pool stress and race | Task 6 and Task 9 repeated/race commands |
| 6. Cancellation/lost-response debit boundary | Task 4 deterministic gates and Task 6 conformance |
| 7. Configuration mismatch quota no-op | Task 3 row/config/`xmin` assertions |
| 8. Least privilege and schema/catalog contract | Task 6 security fixture and Task 8 runbook |
| 9. English/Korean docs, shared sequence diagram, root index, changelog, runbook | Tasks 7 and 8 |
| 10. Targeted/race/static/`make ci` verification | Task 9 |
| 11. Cutover/rollback, HA fencing/RPO, telemetry gate | Task 8 plus operator review in Task 9 |

## DoD and Conditional Review Coverage

- Spec and plan review artifacts must end at P0=0/P1=0 before Task 0/implementation.
- TDD RED and GREEN commands are named for every production task; commits follow each green slice.
- Performance covers one DB round trip, exact numeric cost, lock/pool pressure, expiry-index write
  amplification, bounded cleanup, and absence of unsupported benchmark claims.
- Stability covers pre/in/post dispatch cancellation, response loss, cleanup/Allow races, pool and
  container ownership, worker completion, no retry/goroutine leaks, and repeated conformance.
- Security covers fixed qualified SQL, positional values, arbitrary bytes, hard bounds, redaction,
  schema ownership, effective privileges, RLS/triggers, and hostile pre-existing objects.
- Operator/Ops covers migration, cleanup, telemetry, writable-primary routing, failover fencing/RPO,
  cutover/rollback, and removal only after zero SQL-binary usage.
- Developer/API covers additive package shape, root error compatibility, nil/zero safety, Go docs,
  examples, and no new dependency.
- User/caller covers result-on-error handling, no unknown replay, configuration migration,
  caller-owned DB/schema/scheduler, provider selection, and unsupported behavior.
- New module/BOM/CI registration, Spring, Exposed, coroutines, streaming, JDK preview, and benchmark
  charts are N/A with the concrete repository/scope evidence above; they are not silently skipped.
  The sequence diagram is required evidence for the distributed execution and retry boundary.
