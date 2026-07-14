# batch/sqlcheckpoint

English | [한국어](README.ko.md)

`batch/sqlcheckpoint` is an opt-in PostgreSQL provider that commits a
`batch.NewAtomicStep` output chunk and reader checkpoint in one transaction.
The business callback and revision CAS must both succeed before commit, closing
the crash window between the two independent writes in the legacy `Writer +
CheckpointStore` path. This package is not a scheduler, distributed lock, or
migration engine.

![PostgreSQL atomic batch checkpoint sequence](../../docs/images/readme-diagrams/postgres-batch-checkpoint-atomic-sequence.png)

## Architecture And Selection

`NewAtomicStep` is an additive constructor; existing `NewStep` callers are
unchanged. At startup, `Step` calls `AtomicCheckpointWriter.Load` for the
checkpoint and fencing revision. At each consumed-input chunk boundary, it
passes output, the latest checkpoint, and expected revision to
`AtomicCheckpointWriter.Commit`. The provider executes the caller's business
callback, checkpoint CAS, and provider-owned `Commit` in the same `sql.Tx`.

- Use the atomic path when PostgreSQL business rows and the checkpoint must
  commit together.
- External side effects in another database, queue, file, or HTTP API are not
  part of this transaction. Such topologies require idempotency and separate
  reconciliation.
- The legacy `Writer + CheckpointStore` path remains supported for workloads
  that accept at-least-once replay or keep business writes and checkpoints in
  different stores.

## Install

```go
import (
    "github.com/bluetape4k/bluetape-go/batch"
    "github.com/bluetape4k/bluetape-go/batch/sqlcheckpoint"
)
```

## Pool Ownership And Schema Bootstrap

Migration and runtime pools are caller-owned. `New` validates configuration
only: it performs no database I/O, does not execute `SchemaSQL`, does not
reconfigure a pool, and never closes one. Complete this order before opening
runtime traffic:

1. Create a deployment-only deployer login, a non-login migration owner, and a
   separate runtime role. The migration role is the non-login owner. As a
   controlled one-time ownership transfer, the deployer executes
   `ALTER SCHEMA public OWNER TO sqlcheckpoint_migration_owner`. The role name alone does not
   establish this public schema ownership prerequisite.
2. Before `SET LOCAL ROLE`, fail closed unless the caller verifies that
   `public` is still owned by `sqlcheckpoint_migration_owner`. Only the deployer
   login may assume that owner in the controlled deployment path.
3. Open a bounded transaction on the caller-owned migration pool and apply
   `SET LOCAL lock_timeout`, `SET LOCAL statement_timeout`, and `SET LOCAL ROLE`.
   Revoke schema `CREATE` from `PUBLIC`, then execute `SchemaSQL` as the owner.
4. Run pre-grant catalog/ACL validation for the relation owner, permanent
   ordinary table, exact
   columns/types/nullability/order, fixed constraints and primary key, absence
   of RLS/policies/triggers/rewrite rules, and schema/table/column ACLs. It must
   prove zero runtime grants. This is the required preflight before runtime grants;
   `IF NOT EXISTS` is not a substitute for this gate.
5. Apply the exact runtime grants: schema `USAGE` and table `SELECT`, `INSERT`,
   `UPDATE` without grant option.
6. Run post-grant effective privilege validation. It must prove only those
   privileges, `LOGIN NOINHERIT`, and no role membership (zero role membership),
   zero inheritance, and no grant option before commit.

```go
migrationCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()
if err = verifyPublicSchemaOwnedByMigrationOwner(migrationCtx, migrationDB); err != nil {
    return err
}
migrationTx, err := migrationDB.BeginTx(migrationCtx, nil)
if err != nil {
    return err
}
defer migrationTx.Rollback()

for _, statement := range []string{
    "set local lock_timeout = '5s'",
    "set local statement_timeout = '10s'",
    "set local role sqlcheckpoint_migration_owner",
    "revoke create on schema public from public",
    sqlcheckpoint.SchemaSQL,
} {
    if _, err = migrationTx.ExecContext(migrationCtx, statement); err != nil {
        return err
    }
}
if err = validateCheckpointCatalogAndACLs(migrationCtx, migrationTx, false); err != nil {
    return err // Application-supplied pre-grant gate; expects zero runtime grants.
}
for _, grant := range []string{
    "grant usage on schema public to app_runtime",
    "grant select, insert, update on public.bluetape_batch_checkpoints to app_runtime",
} {
    if _, err = migrationTx.ExecContext(migrationCtx, grant); err != nil {
        return err
    }
}
if err = validateCheckpointCatalogAndACLs(migrationCtx, migrationTx, true); err != nil {
    return err // Application-supplied post-grant effective privilege gate.
}
return migrationTx.Commit()
```

Here `migrationDB` is the caller-owned deployer-login migration pool and
`verifyPublicSchemaOwnedByMigrationOwner` performs the fail-closed owner check.
The one-time transfer is an explicit deployer prerequisite, never an automatic
library action. `validateCheckpointCatalogAndACLs` is application-supplied and
must implement both pre-grant and post-grant checks shown above.
The fixed relation is `public.bluetape_batch_checkpoints`. Custom schemas,
custom table names, and auto-migration are unsupported. The runtime role must
not own the schema/table or inherit the migration owner.

```sql
grant usage on schema public to app_runtime;
grant select, insert, update
on table public.bluetape_batch_checkpoints to app_runtime;
```

Do not grant schema `CREATE`, table `ALTER`, `DROP`, `DELETE`, `TRUNCATE`,
`REFERENCES`, `TRIGGER`, or grant options to the runtime role. Preflight must
also reject privileges inherited through another role or `PUBLIC`. The
application separately owns privileges for callback business tables.

## JSON Codec And Atomic Step Construction

The caller selects the checkpoint type and storage encoding. This example
combines a caller-owned runtime pool, JSON codec, tx-bound business insert, and
`NewAtomicStep`. Apply `SchemaSQL` through the migration pool before passing the
runtime pool to the provider.

```go
type checkpoint struct {
    Offset int64 `json:"offset"`
}

codec := sqlcheckpoint.Codec[checkpoint]{
    Encode: func(value checkpoint) ([]byte, error) {
        return json.Marshal(value)
    },
    Decode: func(payload []byte) (checkpoint, error) {
        var value checkpoint
        err := json.Unmarshal(payload, &value)
        return value, err
    },
}

atomicWriter, err := sqlcheckpoint.New(
    runtimeDB,
    sqlcheckpoint.Options{Namespace: "orders-v2"},
    codec,
    func(ctx context.Context, tx sqlkit.Session, orders []Order) error {
        for _, order := range orders {
            if _, err := tx.ExecContext(ctx,
                "insert into processed_orders (id, amount) values ($1, $2)",
                order.ID, order.Amount,
            ); err != nil {
                return err
            }
        }
        return nil
    },
)
if err != nil {
    return err
}

step, err := batch.NewAtomicStep(batch.AtomicStepOptions[Input, Order]{
    Name:          "persist-orders",
    ChunkSize:     100,
    Reader:        checkpointReader,
    Processor:     processor,
    AtomicWriter:  atomicWriter,
    CheckpointKey: "tenant:blue",
})
```

See [`example_test.go`](example_test.go) for compile-checked construction and
recovery examples.

## Callback Transaction Contract

For a commit with output items, the provider creates a fixed private
`SAVEPOINT`, calls the callback exactly once, and probes transaction ownership.
A checkpoint-only commit skips both the callback and `SAVEPOINT`.

- Use only the tx-bound `sqlkit.Session` passed to the callback.
- A captured `*sql.DB`, separate transaction, escaped session/items in a
  goroutine, or network/external side effect is outside the atomic guarantee.
- Do not access the checkpoint relation, change role/search-path/security
  state, or mutate the items slice during the callback.
- Do not issue raw `BEGIN`, `COMMIT`, `ROLLBACK`, `SAVEPOINT`, `SET TRANSACTION`,
  or equivalent procedure-based transaction control.
- Do not retain the callback session or items after return. Propagate context
  cancellation and do not retry inside the callback.

If the ownership probe cannot prove the original transaction, the provider
does not issue checkpoint CAS or its own commit and fails closed. Positive
lifecycle evidence also matches `ErrCallbackContractViolation`.

## Progress, Revisions, And Conflicts

Identity is the exact, unnormalized raw-byte `(namespace, key)` pair. An empty
namespace defaults to `default`; a namespace is at most 128 bytes and a key
must be nonempty. The default key limit is 512 bytes and the hard ceiling is
1024 bytes. NUL and invalid UTF-8 remain byte-for-byte intact.

A missing checkpoint has expected revision zero. The first commit creates
revision 1 and each later success increments it by exactly one. A stale revision
returns `ErrCheckpointConflict`; the PostgreSQL bigint maximum returns
`ErrCheckpointVersionExhausted`. Conflict and exhaustion commit neither
business rows nor checkpoint progress. Never blindly retry an old expected
revision.

Filter and processor skips still count as consumed-input progress. A boundary
commits pending output with the checkpoint, or executes a checkpoint-only
transaction only when no output is pending. Empty input and exact-multiple EOF
do not add a redundant revision.

The caller must serialize the same `(namespace, key)` from `Load` through run
completion and any unknown-outcome reconciliation. Same-key execution must be
serialized externally; CAS is a final fencing guard against accidental overlap,
not a distributed lock or scheduler.

## Payload, Codec, And Encryption

The provider does not choose a codec, schema version, compression, or payload
migration. The default encoded payload limit is 1 MiB and the hard ceiling is
16 MiB. Encoding happens before the transaction. An oversized stored payload
is rejected without returning its bytes to the Go process or decoding it.
`CodecError` preserves its cause for `errors.Is`/`errors.As`, but its error
string omits payloads and raw codec causes.

Treat stored payload as untrusted and use a stable codec that fails closed on
malformed input. For sensitive checkpoints, decide whether TLS/database
encryption is sufficient; when necessary, use caller-owned authenticated codec
encryption plus key rotation and migration. The provider supplies no
application-level encryption or key management.

## Policy Boundary

For an atomic step, `RetryPolicy` and `SkipPolicy` apply to **processor failures
only**. They never apply to `AtomicCheckpointWriter.Commit`, the business
callback, checkpoint CAS, context/transport errors, `ErrCommitUnknown`,
`ErrAtomicityUnknown`, or another unknown-outcome error. Automatic replay could
duplicate a business write or advance a stale checkpoint.

## Errors And Recovery

| Condition | Durable state and caller action |
|---|---|
| Callback/checkpoint server failure with proven ownership and rollback | Business rows and checkpoint are rolled back together. Repair the cause, then start a fresh run. |
| `ErrCheckpointConflict` | The stale transaction is fully rolled back. Quiesce actors and restart from a fresh `Load`. |
| `ErrCheckpointVersionExhausted` | The row is unchanged. Quiesce and reconcile, then migrate to a new key or namespace. |
| Only `ErrCommitUnknown` matches | The provider-owned commit may have succeeded. Keep same-key exclusivity and use fresh `Load` for the authoritative resume position. Never replay the old expected revision. |
| `ErrAtomicityUnknown` also matches | Business/checkpoint attribution cannot be proven. Prohibit automatic fresh-run replay; call `quiesceCheckpointKey` and route to `reconcileCheckpoint` for manual reconciliation. |
| `AtomicityPanic` | This is an atomicity-unknown panic for a top-level supervisor. Inspect the sentinel before any generic restart. |

Only a provider-owned commit-unknown, and only while no other same-key actor can
intervene, permits a fresh `Load` to establish the resume position. This branch
requires `ErrCommitUnknown`, excludes `ErrAtomicityUnknown`, and requires
`errors.As` to yield `*sqlcheckpoint.OpError` with `Operation() == "commit"`.
A bare joined sentinel is not evidence of a provider-owned commit dispatch.

```go
var operationErr *sqlcheckpoint.OpError
if errors.Is(commitErr, batch.ErrCommitUnknown) &&
    !errors.Is(commitErr, batch.ErrAtomicityUnknown) &&
    errors.As(commitErr, &operationErr) &&
    operationErr.Operation() == "commit" {
    quiesceCheckpointKey(checkpointKey)
    checkpoint, exists, err := atomicWriter.Load(freshCtx, checkpointKey)
    // Reconcile exists/checkpoint while same-key intake remains quiesced.
    _, _, _ = checkpoint, exists, err
}
```

When callback panic cleanup proves ownership, the original panic value is
re-raised. Otherwise, `*sqlcheckpoint.AtomicityPanic` is raised and matches both
`ErrAtomicityUnknown` and `ErrCommitUnknown`. Only trusted top-level recovery may
inspect `PanicValue()` as a sensitive diagnostic. Never log, trace, measure, or
return `AtomicityPanic.PanicValue` through an external response.

```go
defer func() {
    recovered := recover()
    recoveredErr, ok := recovered.(error)
    var atomicityPanic *sqlcheckpoint.AtomicityPanic
    if ok && errors.As(recoveredErr, &atomicityPanic) &&
        errors.Is(recoveredErr, batch.ErrAtomicityUnknown) {
        quiesceCheckpointKey(checkpointKey)
        reconcileCheckpoint(checkpointKey)
        return
    }
    if recovered != nil {
        panic(recovered)
    }
}()
```

`quiesceCheckpointKey` and `reconcileCheckpoint` are application-supplied,
fail-closed hooks; a missing or failed hook must not restart the job. An ordinary
error panic that merely wraps `ErrAtomicityUnknown` is not an `AtomicityPanic`
and is re-raised unchanged.

`OpError.KeyID` is a pseudonymous correlation value for sampled internal
diagnostics. `KeyID` is not an authorization identifier, secret, enumeration
defense, external trust-boundary value, or metric label. Do not log raw keys,
namespaces, payloads, SQL, DSNs, endpoints, or provider causes either.

## Rollout, Rollback, And Retention

Legacy cutover is not an option toggle. Quiesce intake and old runs, reconcile
the legacy checkpoint with business state, seed the SQL row using the exact
namespace/key and codec, and read it back. Block mixed old/new binaries. Use an
isolated namespace/key and business cohort for the canary, with exactly one
provider active. If the authoritative position cannot be proven, choose an
approved idempotent replay or new-cohort restart instead of an in-place seed.

Rollback also requires quiescing atomic runs and exporting/reconciling SQL and
business state into the legacy store. Preserve the table, grants, and old state
through the observation window. Checkpoint retention and row deletion belong
to the caller's job lifecycle; delete only in a maintenance window after every
run and unknown reconciliation for that key has been quiesced and joined.

## Operations And Supported Topology

The [v0.19.0 provider conformance rollout runbook](../../docs/release/v0.19.0-provider-conformance-runbook.md#sql-batch-checkpoint-deployment-gates)
is the production gate for catalog ownership, least-privilege grants, recovery
drills, telemetry, canary promotion, rollback, and retention.

- Use one writable PostgreSQL primary with connection-level transaction
  affinity. Read replicas, multi-primary, statement/transaction replay proxies,
  and transaction-pooling proxies that break affinity are unsupported.
- The caller configures bounded run/commit contexts and chunk sizes plus
  `lock_timeout`, `statement_timeout`, and
  `idle_in_transaction_session_timeout`.
- Shutdown order is: stop intake, cancel/join runs, reconcile unknown outcomes,
  confirm transaction drain, then close the caller-owned pool.
- Monitor bounded outcome categories, conflicts/exhaustion/unknown counts,
  latency, `sql.DBStats`, relation/dead tuples, WAL/autovacuum, and replication
  lag. Never use raw keys or `KeyID` as metric labels.

## Validation

Run PostgreSQL Testcontainers serially, never concurrently with another Docker
suite.

```bash
go test -count=1 ./batch ./batch/sqlcheckpoint -run 'Example|README|Readme'
go test -count=1 ./batch/sqlcheckpoint -run 'TestPostgres'
go test -count=20 ./batch/sqlcheckpoint -run 'Concurrent|Conflict|Cancellation|Ownership'
go test -race -count=1 ./batch ./batch/sqlcheckpoint
make ci
```

Before production promotion, also pass the catalog/privilege preflight and
rehearse commit-unknown and atomicity-unknown recovery.
