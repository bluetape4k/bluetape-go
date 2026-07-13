# ratelimit/sql

[English](README.md) | [한국어](README.ko.md)

`ratelimit/sql` provides a PostgreSQL-backed token bucket for moderate-QPS,
database-only deployments. It is useful when application instances already
share one writable PostgreSQL primary and operating Redis only for rate limits
is not justified. It is not a Redis replacement for high-QPS or latency-critical
traffic.

Each `Allow` executes one `INSERT ... ON CONFLICT DO UPDATE ... RETURNING`
statement. PostgreSQL server time refills, checks, consumes, and returns the
bucket result while the primary-key row lock serializes callers for one key.
`Cleanup` uses one `DELETE` CTE with `FOR UPDATE SKIP LOCKED`; `limit` bounds
locked and deleted rows while the expiry index lets PostgreSQL stop at the
earliest available expired rows. Already locked rows can still be scanned and
skipped, so caller time and pressure budgets remain mandatory.

## Install

```go
import sqlratelimit "github.com/bluetape4k/bluetape-go/ratelimit/sql"
```

## Ownership And Schema

The database pool, migration, and cleanup scheduler are caller-owned. `New`
performs no I/O, does not run migrations, and never closes the pool. Apply the
fixed `SchemaSQL` relation as a migration owner before starting runtime traffic:

```go
if _, err := migrationDB.ExecContext(ctx, sqlratelimit.SchemaSQL); err != nil {
    return err
}
```

The fixed relation is `public.bluetape_ratelimit_buckets` with the fixed expiry
index `public.bluetape_ratelimit_buckets_expires_at_idx`. Custom schemas, table
names, ORM mappings, and caller transactions are unsupported. Before runtime
grants, catalog preflight must verify the column names/types/nullability/checks,
the `(namespace, bucket_key)` primary key, the expiry index, owner, and absence
of unexpected triggers, row-level security, or policies.

Use separate migration and runtime roles. A least-privilege runtime role needs
only schema usage and table DML:

```sql
grant usage on schema public to app_runtime;
grant select, insert, update, delete
on table public.bluetape_ratelimit_buckets to app_runtime;
```

Do not grant table ownership, schema `CREATE`, `TRUNCATE`, `REFERENCES`, or
`TRIGGER` to the runtime role. Also verify that role inheritance and `PUBLIC`
do not restore schema creation privileges.

## Usage

Pass a caller-owned deadline to `Allow`; the package does not add a timeout or
retry after dispatch.

```go
limiter, err := sqlratelimit.New(db, sqlratelimit.Options{
    Namespace:     "api-v1",
    RatePerSecond: 100,
    Burst:         200,
    IdleTTL:       10 * time.Minute,
    MaxKeyBytes:   512,
})
if err != nil {
    return err
}

result, err := limiter.Allow(ctx, "tenant:blue", 1)
if err != nil {
    // Ignore result. Never infer whether a commit-unknown debit happened.
    return err
}
if !result.Allowed {
    return fmt.Errorf("retry after %s", result.RetryAfter)
}
```

| Option | Contract |
|---|---|
| `Namespace` | Nonblank, at most 128 bytes; defaults to `default`. |
| `RatePerSecond` | Positive finite rate represented internally as microtokens. |
| `Burst` | Positive whole-token capacity. |
| `IdleTTL` | At least one full-refill window; the default is at least one minute and two refill windows. |
| `MaxKeyBytes` | `1..1024`; defaults to 512. |

Keys are nonblank arbitrary bytes carried through Go strings and stored as
`bytea`, including NUL and invalid UTF-8. Bound `MaxKeyBytes`, authenticate the
identity source, bound namespace creation, and monitor row cardinality. Never
put raw keys or namespaces in metrics.

## Results And Errors

Rejected requests are normal zero-error results. For every non-nil error,
discard the entire `Result`. Use provider-neutral inspection for diagnostics:

```go
result, err := limiter.Allow(ctx, key, 1)
if err != nil {
    var operation ratelimit.OperationError
    if errors.As(err, &operation) {
        recordBoundedCategory(operation.Family(), operation.Operation())
        // operation.KeyID() is for sampled diagnostics, never a metric label.
    }
    if errors.Is(err, ratelimit.ErrCommitUnknown) {
        // The debit may have committed once: no automatic replay.
    }
    return ratelimit.Result{}, err
}
```

| Condition | Meaning and caller action |
|---|---|
| `ErrConfigurationMismatch` | The existing row has different rate, burst, or idle TTL. Stop traffic for that namespace and perform configuration migration or namespace rotation. |
| `ErrCommitUnknown` | The dispatched statement may have committed. Discard the result and do not replay automatically. |
| `sqlratelimit.OpError` | A redacted typed failure; the original cause remains available to `errors.Is`/`errors.As`. |
| Context or database error without commit unknown | No successful result is available; follow the cause and service policy. |

Do not log raw errors, SQL, DSNs, endpoints, keys, or namespaces. `KeyID` is a
stable redacted correlation value for sampled diagnostics only, never a metric
label.

## Bounded Cleanup

Run `Cleanup` from a caller-owned scheduler with a fresh bounded context. The
cadence must be shorter than `IdleTTL`; use a limit in `1..1000`, bounded
database timeouts, jitter, a small worker count, and a per-run batch/time budget.

```go
cleanupCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
defer cancel()
deleted, err := limiter.Cleanup(cleanupCtx, 100)
if err != nil {
    // Cleanup returns count zero, although up to `limit` rows may have been
    // deleted. Retry current expired work; this is not an idempotent batch replay.
    return err
}
recordCleanupCount(deleted)
```

Pause cleanup when WAL growth, row-lock latency, pool waits, or autovacuum lag
cross predeclared pressure gates. Cleanup is maintenance, not a correctness
prerequisite for `Allow`.

## PostgreSQL And HA Boundary

- Route `Allow`, `Cleanup`, catalog checks, and reconciliation probes to one
  writable primary-only endpoint. Read replicas and transaction replay proxies
  are unsupported.
- Verify `pg_is_in_recovery() = false`, `transaction_read_only = off`, server
  identity, and HA timeline before promotion.
- Prove old-writer fencing, durability/RPO, and no statement replay during a
  controlled failover. A commit-unknown operation must not be replayed.
- Monitor bounded outcome/error categories, Allow latency, `sql.DBStats`,
  statement and row-lock latency, cleanup duration/count/error/backlog, oldest
  expiry, live/dead tuples, relation/index size, autovacuum lag, and WAL growth.

## Configuration And Provider Cutover

Rate, burst, and idle TTL are stored with each bucket. Changing them in place
causes `ErrConfigurationMismatch`; use a planned configuration migration after
quiescing traffic, or deploy with namespace rotation.

Local, Redis, and SQL quota state is not shared. Simultaneous mixed-provider
serving is prohibited because independent providers can grant multiple full bursts.
A canary must use an independent namespace and an independent cohort.
For cutover or rollback, quiesce the old provider and wait a full-refill window
before enabling exactly one new provider, or record an approved extra-burst budget
that covers the overlap.

Retain the SQL table, index, and grants during binary rollback. Remove them only
in a separate migration after a predeclared observation window proves zero SQL
provider binaries and zero SQL provider traffic.

## Unsupported Scope

This provider does not supply auto-migration, background cleanup, caller-owned
transaction participation, custom relations, ORM integration, reservations,
waiting/FIFO fairness, distributed multi-primary semantics, non-PostgreSQL
databases, or high-QPS performance claims.

## Tests

Tests require Docker and run PostgreSQL Testcontainers serially:

```bash
go test -count=1 ./ratelimit/sql
go test -race -p1 -count=1 ./ratelimit/sql
```

Production rollout must also follow the bilingual
[v0.19.0 provider runbook](../../docs/release/v0.19.0-provider-conformance-runbook.md).
