# leader/sql

[English](README.md) | [한국어](README.ko.md)

`leader/sql` is a PostgreSQL-only `leader.Elector` backed by one server-time row lease per key. The caller owns migrations, the `*sql.DB`, roles, failover, and shutdown. Version 0.19.0 fixes the relation at `public.bluetape_leader_leases`; custom schemas and replica-routed reads are unsupported.

## Preflight

| Check | Required result |
|---|---|
| Migration role | Can create and own the fixed protected `public` relation. |
| Runtime role | Cannot create in `public`; has only schema usage and table DML. |
| Endpoint | Reaches the intended writable primary for every mutation, lookup, and reconciliation probe. |
| Timing | `Lease` and `RenewInterval` are both zero (normalized to 10s/3s), or `0 < RenewInterval < Lease`. Short custom leases set both explicitly. |
| Unsupported topology | Stop before migration if a custom schema or replica-routed endpoint is required. |

Do not continue when any preflight result differs.

## Diagram

![PostgreSQL row-lease sequence](../../docs/images/readme-diagrams/postgres-leader-row-lease-sequence.png)

## Import

```go
import sqlleader "github.com/bluetape4k/bluetape-go/leader/sql"
```

Blank-import `github.com/jackc/pgx/v5/stdlib` when using the `pgx` `database/sql` driver.

## Schema

Run `SchemaSQL` through the caller's migration process before starting an elector. Direct execution is suitable only for development/bootstrap. It is the v0.19.0 bootstrap contract, not an upgrade engine; future incompatible changes require versioned migrations.

```go
_, err := migrationDB.ExecContext(ctx, sqlleader.SchemaSQL)
```

Fail deployment unless this query returns the documented seven ordered columns: `leader_key`, `group_name`, `member_id`, `owner_token`, `lease_until`, `created_at`, and `updated_at`.

```sql
select column_name, data_type, is_nullable
from information_schema.columns
where table_schema = 'public' and table_name = 'bluetape_leader_leases'
order by ordinal_position;
```

Verify the protected object, primary key, triggers, and runtime privileges:

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

Expected values are relation kind `r`, the configured migration owner, RLS false, primary key `{leader_key}`, zero user triggers, and runtime `schema_usage=true`, `schema_create=false`, `table_dml=true`, `table_ddl=false`. Also inspect direct/inherited role memberships and `PUBLIC` grants.

## Least Privilege Grants

Application migration grant block:

```sql
grant usage on schema public to app_runtime;
grant select, insert, update, delete
on table public.bluetape_leader_leases to app_runtime;
```

Database-administrator hardening block; review database-wide impact before running it:

```sql
revoke create on schema public from public;
```

The migration role owns the table. The runtime role must not own it and must not have `CREATE`, `ALTER`, `TRUNCATE`, `REFERENCES`, or `TRIGGER` authority.

## Usage

```go
db, err := sql.Open("pgx", dsn) // caller-owned writable-primary pool
if err != nil { return err }
defer db.Close()

opts := leader.Options{
    Group: "billing-workers", MemberID: "worker-1",
    Lease: 30 * time.Second, RenewInterval: 10 * time.Second,
}
elector, err := sqlleader.New(db, opts)
if err != nil { return err }
campaignCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
defer cancel()
if err := elector.Campaign(campaignCtx); err != nil { return err }
// Poll IsLeader more frequently than RenewInterval and stop protected work on loss.
```

`New` never migrates or closes the pool. Call `Resign` with a fresh bounded context before closing it.

## Lease Semantics

Acquire/takeover is one atomic `INSERT ... ON CONFLICT ... WHERE ... RETURNING`. Renewal and resign require the same opaque owner token. PostgreSQL `pg_catalog.clock_timestamp()` is authoritative; process clocks do not decide ownership. Expiry permits logical takeover and does not delete the row.

This API does not provide fencing. Tokens identify one elector instance but are not secrets, credentials, or fencing values. A confirmed token probe may make `Campaign` succeed after its caller context expires; the caller then owns cleanup and should inspect `campaignCtx.Err()` before starting protected work.

## Primary/Failover Contract

Every mutation, `Leader` lookup, and reconciliation probe must reach the same writable primary. Never route reads to replicas. Before and after a controlled HA exercise, capture:

```sql
select pg_catalog.inet_server_addr() as server_addr,
       pg_catalog.inet_server_port() as server_port,
       pg_catalog.pg_is_in_recovery() as in_recovery,
       current_setting('transaction_read_only') as read_only,
       pg_catalog.pg_postmaster_start_time() as postmaster_started,
       pg_catalog.pg_current_wal_lsn() as wal_lsn;
```

Also record the HA controller's server identity and timeline before and after. Stop if `pg_is_in_recovery()` is true, `transaction_read_only` is `on`, identity is not the intended primary, or the old primary still accepts writes after promotion. Prove every elector/probe endpoint, restart or promote through the HA controller, fence the old writer before accepting leases on the new writer, then prove bounded cleanup or full-lease takeover. The local backend-termination test proves pool reconnection only; it is not promotion or fencing evidence. Choose durability appropriate to the topology.

## Pool and Timing

Reserve connections for active renewals plus application work. Alert when `DBStats.WaitCount` or `DBStats.WaitDuration` grows, `DBStats.InUse` approaches `DBStats.MaxOpenConnections`, or p99 pool/statement latency consumes the `Lease-RenewInterval` margin. `RenewInterval < Lease` is validated, but callers must leave a practical latency margin.

Each campaign attempt is internally bounded. Each renewal has a `RenewInterval` deadline, so pool or row-lock starvation clears local leadership instead of overstaying the safety budget.

## Failure Recovery

An unexpected `IsLeader` transition stops protected work immediately. Mutation failures are redacted `leader.OperationError` values; explicitly logging an unwrapped driver cause bypasses that redaction.

On `ErrCommitUnknown` or `ErrCleanupPending`, retain the same elector and retry bounded `Resign` with fresh contexts. Do not construct a replacement elector to bypass cleanup. If cleanup remains unresolved, wait a full lease from the last possible mutation or last failed cleanup attempt before restart/reuse. TTL/server-time expiry is the final safety fallback.

## Shutdown

Stop protected work, cancel/join campaigns, bounded-resign every elector, verify renewal traffic has stopped, inventory unresolved full lease waits, and stop every other pool user. Confirm `DBStats.InUse == 0` and only then close the caller-owned pool.

## Expired Row Cleanup

Expiry correctness never depends on deletion. Monitor table cardinality, `dead tuples`, and `autovacuum`. Optional storage hygiene may delete rows older than a grace interval larger than the maximum configured lease:

```sql
delete from public.bluetape_leader_leases
where lease_until < pg_catalog.clock_timestamp() - interval '1 day';
```

This is not the correctness TTL and must never delete a live row.

## Security Boundaries

`Group`, `MemberID`, `KeyPrefix`, and owner tokens are stored or returned in plaintext. Do not put credentials, secrets, or sensitive customer identifiers in them. `KeyPrefix` prevents naming collisions; it is not authorization. Separate trust domains use separate databases or caller-managed, independently verified authorization.

The provider does not configure RLS. A caller-supplied policy must independently prove SELECT, INSERT, UPDATE, DELETE, and `ON CONFLICT DO UPDATE`; otherwise RLS is unsupported for that deployment. Protect `public` against hostile object creation and verify owner, PK, trigger, RLS, membership, and `PUBLIC` grants before rollout.

## Test

```bash
go test -p 1 -count=1 ./leader/sql
go test -p 1 -race -count=1 ./leader/sql
go test -p 1 -count=10 ./leader/sql -run '^TestPostgresElectorConformance$'
```
