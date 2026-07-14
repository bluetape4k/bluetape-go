# PostgreSQL Durable Batch Checkpoint Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an opt-in batch path that commits PostgreSQL business writes and a revision-fenced durable checkpoint in one transaction without changing the existing `StepOptions`/`NewStep` API.

**Architecture:** The root `batch` package adds provider-neutral atomic checkpoint types plus a separate `AtomicStepOptions`/`NewAtomicStep` constructor and an atomic run loop that chunks by consumed input. `batch/sqlcheckpoint` owns a caller-provided `*sql.DB`, typed codec, and tx-bound write callback; it uses a reserved savepoint ownership probe, split insert/update CAS, and distinct commit-unknown versus atomicity-unknown recovery barriers. Schema migration, pool lifetime, same-key serialization, callback resources, codec evolution, and recovery orchestration remain caller-owned.

**Tech Stack:** Go 1.26, `context`, `database/sql`, `errors`, `crypto/sha256`, existing `sqlkit.Session`, pgx v5 `pgconn`, PostgreSQL Testcontainers, standard Go tests/race detector, CairoSVG-rendered SVG/PNG documentation.

---

## File Map

| Area | Files | Responsibility |
|---|---|---|
| Root atomic contract | `batch/errors.go`, `batch/atomic.go`, `batch/atomic_test.go`, `batch/compat_external_test.go` | Provider-neutral sentinels, versioned checkpoint interface, additive atomic constructor/options, legacy source-compatibility fixture. |
| Atomic step runtime | `batch/step.go`, `batch/atomic_step.go`, `batch/atomic_step_test.go` | Preserve the legacy loop and add consumed-input atomic chunking, restore, status, counters, skip/filter, close, and no-retry behavior. |
| SQL public API/schema | `batch/sqlcheckpoint/{doc.go,options.go,options_test.go,schema.go,schema_test.go,writer.go}` | Caller-owned constructor, immutable limits/codec/callback, fixed DDL, key/checkpoint validation, no implicit I/O. |
| SQL load/diagnostics | `batch/sqlcheckpoint/{load.go,load_test.go,errors.go,errors_test.go}` | Conditional payload projection, typed decode, redacted operation/codec errors, correlation ID, zero/nil safety. |
| SQL atomic commit | `batch/sqlcheckpoint/{session.go,queries.go,commit.go,commit_test.go}` | Tx adapter, callback session, savepoint ownership proof, CAS, panic preservation, cancellation, rollback, and unknown classification. |
| PostgreSQL proof | `batch/sqlcheckpoint/{integration_test.go,security_test.go,stress_test.go}` | Sequential Testcontainers success, restart, conflicts, hostile lifecycle changes, raw transaction control, cancellation, pool drain, catalog/ACL, and race/stress proof. |
| Examples and package docs | `batch/sqlcheckpoint/{example_test.go,readme_test.go,README.md,README.ko.md}`, `batch/{doc.go,README.md,README.ko.md}` | Compile-checked construction/recovery, callback safety, legacy non-atomic warning, bilingual parity. |
| Public docs | `README.md`, `README.ko.md`, `CHANGELOG.md`, `docs/release/v0.19.0-provider-conformance-runbook.md`, `docs/images/readme-diagrams/postgres-batch-checkpoint-atomic-sequence.{svg,png}` | Discoverability, migration/runbook gates, recovery drills, rollout/rollback, sequence diagram. |
| Workflow evidence | `docs/superpowers/reviews/2026-07-14-issue-532-sql-checkpoint-{risk,plan-review,step-6r-code-review}.md`, `docs/lessons/2026-07-14-issue-532-sql-checkpoint.md` | Pre-implementation risks, Step 3-R/6-R evidence, and reusable lessons. |

The core contract and PostgreSQL provider remain one plan because neither is independently useful:
the provider must implement the new root interface, and the atomic step requires a provider to prove
its transaction and restart semantics. Each task still ends in a compiling, testable commit.

## Dependency Order and Stop Conditions

Task 0 freezes approved artifacts and risks. Tasks 1-2 establish the root contract and atomic loop
before provider work. Task 3 fixes the SQL API and schema; Task 4 adds load and diagnostics; Task 5
adds the transaction/ownership state machine. Task 6 supplies real PostgreSQL proof. Tasks 7-8
document only settled behavior. Task 9 performs final verification and Step 6-R.

Run every Testcontainers-backed command sequentially. Do not run `make ci` concurrently with a
PostgreSQL suite. Do not add a dependency, modify the exported fields of `StepOptions`, change
`NewStep`, add automatic migration/retry, or broaden the callback beyond `sqlkit.Session`. Any such
need stops execution and returns to design review. A failing ownership probe must never dispatch
checkpoint DML or provider-owned `Commit`.

### Task 0: Freeze Approved Artifacts and Predict Risks

**Complexity:** Small documentation gate; blocks source edits.

**Files:**
- Verify: `docs/superpowers/specs/2026-07-14-issue-532-sql-checkpoint-design.md`
- Verify: `docs/superpowers/reviews/2026-07-14-issue-532-sql-checkpoint-spec-review.md`
- Verify: `docs/superpowers/plans/2026-07-14-issue-532-sql-checkpoint-plan.md`
- Create: `docs/superpowers/reviews/2026-07-14-issue-532-sql-checkpoint-risk.md`

- [ ] **Step 1: Verify the approved artifact-only branch**

```bash
git status --short
git log --oneline origin/develop..HEAD
git diff --check origin/develop...HEAD
shasum -a 256 docs/superpowers/specs/2026-07-14-issue-532-sql-checkpoint-design.md
```

Expected: clean worktree; approved spec hash
`3d0208d73a8bc62073a57e60a5607502d53e9c69d0642e2f88351286024ed21e`; only design,
review, and plan artifacts ahead of `origin/develop`; no `batch/sqlcheckpoint` directory.

- [ ] **Step 2: Create the pre-implementation risk table**

Create a Markdown table with `Risk`, `Trigger`, `Signal`, `Prevention`, `Recovery`, and `Owner`.
Include concrete rows for buffered-output checkpoint loss, filter/skip transaction amplification,
same-key overlap, missing-row stale resurrection, revision exhaustion, codec drift, oversized
payload, callback captured DB/external side effect, raw COMMIT/ROLLBACK, COMMIT→BEGIN→failed SQL,
panic after raw COMMIT, ownership-probe timeout, provider-Commit response loss, cancellation races,
pool starvation, unlogged/temp/hostile relation, RLS/trigger/rewrite drift, schema/table/column ACL
drift, PUBLIC object creation, WAL/dead-tuple/replication pressure, legacy cutover, rollback,
supervisor automatic panic replay, Testcontainers leakage, and bilingual/diagram drift.

- [ ] **Step 3: Record fresh baseline evidence**

```bash
go version
go list -m -f '{{.Path}} {{.Version}}' github.com/jackc/pgx/v5 github.com/testcontainers/testcontainers-go
go test -count=1 ./batch ./sqlkit
go test -count=1 ./...
```

Expected: all commands exit 0. Record the exact versions and exit codes in the risk artifact.

- [ ] **Step 4: Commit risk evidence before source work**

```bash
git add docs/superpowers/reviews/2026-07-14-issue-532-sql-checkpoint-risk.md
git commit -m "docs: predict SQL checkpoint risks"
```

Expected: the risk commit predates every source commit.

### Task 1: Add the Provider-Neutral Atomic Contract

**Complexity:** Medium public API change; legacy source compatibility is mandatory.

**Pattern skill:** `bluetape-go-patterns` API compatibility, error wrapping, nil safety.

**Files:**
- Create: `batch/atomic.go`
- Create: `batch/atomic_test.go`
- Create: `batch/compat_external_test.go`
- Modify: `batch/errors.go`
- Modify: `batch/step.go`

- [ ] **Step 1: Write RED contract and compatibility tests**

Add root sentinel/interface tests and this external-package compile fixture with the exact existing
field order:

```go
package batch_test

import "github.com/bluetape4k/bluetape-go/batch"

var _ = batch.StepOptions[int, int]{
    "legacy-unkeyed", 1, nil, nil, nil,
    batch.RetryPolicy{}, batch.SkipPolicy{}, nil, "",
}
```

Test `NewAtomicStep` for empty name, zero/default and negative chunk size, nil reader, nil processor,
nil atomic writer, non-`CheckpointReader`, negative retry/skip policies, default checkpoint key,
and no reader/provider side effect on constructor failure. Assert `AtomicStepOptions` exposes no
legacy writer/store field by compiling only the approved literal shape.

- [ ] **Step 2: Observe RED**

```bash
go test -count=1 ./batch -run 'Atomic|LegacyUnkeyed|Compatibility'
```

Expected: build FAIL because the sentinels, atomic types, and constructor do not exist.

- [ ] **Step 3: Add the minimal public contract**

Create the exact API and Go doc comments:

```go
package batch

import "context"

type VersionedCheckpoint struct {
    Value   any
    Version uint64
}

type AtomicCheckpointWriter[T any] interface {
    Load(context.Context, string) (VersionedCheckpoint, bool, error)
    Commit(context.Context, string, uint64, []T, any) (uint64, error)
}

type AtomicStepOptions[I any, O any] struct {
    Name          string
    ChunkSize     int
    Reader        Reader[I]
    Processor     Processor[I, O]
    AtomicWriter  AtomicCheckpointWriter[O]
    RetryPolicy   RetryPolicy
    SkipPolicy    SkipPolicy
    CheckpointKey string
}
```

Add these sentinels to `batch/errors.go`:

```go
ErrCheckpointConflict = errors.New("batch: checkpoint revision conflict")
ErrCommitUnknown = errors.New("batch: commit outcome unknown")
ErrAtomicityUnknown = errors.New("batch: atomicity outcome unknown")
ErrCheckpointVersionExhausted = errors.New("batch: checkpoint version exhausted")
```

Add an unexported `atomic AtomicCheckpointWriter[O]` field to `Step`; do not alter any exported
`StepOptions` field or the `NewStep` signature.

- [ ] **Step 4: Implement the additive constructor**

Use the same validation order/defaults as `NewStep`, require `CheckpointReader`, normalize policies,
and set only reader/processor/atomic/key fields:

```go
func NewAtomicStep[I any, O any](options AtomicStepOptions[I, O]) (*Step[I, O], error) {
    if options.Name == "" { return nil, fmt.Errorf("step name must not be empty") }
    if options.ChunkSize == 0 { options.ChunkSize = DefaultChunkSize }
    if options.ChunkSize < 0 { return nil, fmt.Errorf("chunk size must be positive") }
    if options.Reader == nil { return nil, fmt.Errorf("reader must not be nil") }
    if options.Processor == nil { return nil, fmt.Errorf("processor must not be nil") }
    if options.AtomicWriter == nil { return nil, fmt.Errorf("atomic writer must not be nil") }
    if _, ok := options.Reader.(CheckpointReader); !ok {
        return nil, fmt.Errorf("reader does not support checkpoints")
    }
    retry, err := options.RetryPolicy.normalize()
    if err != nil { return nil, err }
    skip, err := options.SkipPolicy.normalize()
    if err != nil { return nil, err }
    if options.CheckpointKey == "" { options.CheckpointKey = options.Name }
    return &Step[I, O]{
        name: options.Name, chunkSize: options.ChunkSize, reader: options.Reader,
        processor: options.Processor, atomic: options.AtomicWriter,
        retry: retry, skip: skip, key: options.CheckpointKey,
    }, nil
}
```

- [ ] **Step 5: Verify GREEN, full legacy regression, and commit**

```bash
gofmt -w batch/atomic.go batch/atomic_test.go batch/compat_external_test.go batch/errors.go batch/step.go
go test -count=1 ./batch
git diff --check
git add batch/atomic.go batch/atomic_test.go batch/compat_external_test.go batch/errors.go batch/step.go
git commit -m "feat: add atomic batch checkpoint contract"
```

Expected: all legacy tests and the external unkeyed fixture PASS.

### Task 2: Implement Consumed-Input Atomic Step Execution

**Complexity:** High correctness path; use strict TDD and small commits if RED failures expose separate defects.

**Files:**
- Create: `batch/atomic_step.go`
- Create: `batch/atomic_step_test.go`
- Modify: `batch/step.go`

- [ ] **Step 1: Write RED restore, boundary, and counter tests**

Create deterministic `checkpointReader` and `recordingAtomicWriter` fakes. Cover missing/existing
restore, one commit per consumed-input boundary, expected version advancement, empty input,
exact-multiple EOF, kept→filter, kept→processor-skip, all-filter, all-skip, mixed streams,
checkpoint-only commits, capture missing/failure, conflict, exhaustion, commit unknown, atomicity
unknown, pre-cancel, and close failure. Assert atomic Commit errors
never consult writer retry/skip policy and never increment `WriteCount` or clear pending output.

The fake records immutable copies:

```go
type atomicCommit[T any] struct {
    key        string
    expected   uint64
    items      []T
    checkpoint any
}

func (w *recordingAtomicWriter[T]) Commit(_ context.Context, key string, expected uint64, items []T, checkpoint any) (uint64, error) {
    copied := append([]T(nil), items...)
    w.commits = append(w.commits, atomicCommit[T]{key, expected, copied, checkpoint})
    if w.err != nil { return 0, w.err }
    return expected + 1, nil
}
```

- [ ] **Step 2: Observe RED**

```bash
go test -count=1 ./batch -run 'AtomicStep|AtomicBoundary|AtomicRestore|AtomicStatus'
```

Expected: FAIL because `Step.Run` still enters the legacy writer path.

- [ ] **Step 3: Dispatch atomic steps without changing the legacy body**

At the top of `Run`, after context normalization and nil receiver handling, add:

```go
if s.atomic != nil {
    return s.runAtomic(ctx)
}
```

Keep the remaining legacy code and `NewStep` unchanged. Update `statusForError` so an error matching
`ErrCommitUnknown` is `StatusFailed` even when it also wraps cancellation:

```go
func statusForError(err error) Status {
    if errors.Is(err, ErrCommitUnknown) { return StatusFailed }
    if isContextError(err) { return StatusCancelled }
    return StatusFailed
}
```

- [ ] **Step 4: Implement the atomic loop and commit helper**

Implement `runAtomic` with reader-only lifecycle, Load/Restore before reads, `progressCount` per
consumed input, pending kept output, and commit only at `progressCount == chunkSize` or nonzero EOF.
The helper must capture the reader checkpoint before calling the provider:

```go
func (s *Step[I, O]) commitAtomic(ctx context.Context, report *Report, reader CheckpointReader, expected uint64, pending []O) (uint64, error) {
    checkpoint, exists, err := reader.Checkpoint(ctx)
    if err != nil { return 0, err }
    if !exists { return 0, fmt.Errorf("reader checkpoint is unavailable") }
    version, err := s.atomic.Commit(ctx, s.key, expected, pending, checkpoint)
    if err != nil { return 0, err }
    report.WriteCount += len(pending)
    return version, nil
}
```

After a successful helper call, clear pending and progress; after an error, return immediately
without mutation. Processor retry/skip stays in `s.process`; filter and skip increment their legacy
counters and still count toward the boundary. Do not call the atomic provider's Close method.

- [ ] **Step 5: Verify counters, statuses, race behavior, and commit**

```bash
gofmt -w batch/step.go batch/atomic_step.go batch/atomic_step_test.go
go test -count=1 ./batch
go test -race -count=1 ./batch
git add batch/step.go batch/atomic_step.go batch/atomic_step_test.go
git commit -m "feat: execute atomic batch checkpoints"
```

Expected: legacy and atomic tests PASS; race detector exits 0.

### Task 3: Define SQL Checkpoint Options, Constructor, and Schema

**Complexity:** Medium public provider surface; no database access in constructor.

**Files:**
- Create: `batch/sqlcheckpoint/doc.go`
- Create: `batch/sqlcheckpoint/options.go`
- Create: `batch/sqlcheckpoint/options_test.go`
- Create: `batch/sqlcheckpoint/schema.go`
- Create: `batch/sqlcheckpoint/schema_test.go`
- Create: `batch/sqlcheckpoint/writer.go`

- [ ] **Step 1: Write RED option, constructor, identity, and schema tests**

Cover nil DB/callback/encode/decode, namespace default/exact bytes/129-byte rejection, key defaults
and 1..1024 hard bounds, payload defaults and 1..16MiB hard bounds, constructor no ping/schema I/O,
nil/zero writer safety, and compile-time interface implementation. Assert all named constraints,
fixed column order, `bytea` identity, positive revision, payload ceiling, and primary key order.

- [ ] **Step 2: Observe RED**

```bash
go test -count=1 ./batch/sqlcheckpoint -run 'Options|New|Schema|Interface'
```

Expected: package build FAIL because the provider does not exist.

- [ ] **Step 3: Add exact public types and normalization**

```go
const (
    DefaultMaxKeyBytes = 512
    MaxKeyBytes = 1024
    DefaultMaxPayloadBytes = 1 << 20
    MaxPayloadBytes = 16 << 20
)

type Options struct {
    Namespace       string
    MaxKeyBytes     int
    MaxPayloadBytes int
}

type Codec[C any] struct {
    Encode func(C) ([]byte, error)
    Decode func([]byte) (C, error)
}

type WriteTxFunc[T any] func(context.Context, sqlkit.Session, []T) error
```

Normalize empty namespace to exact bytes `default`; preserve every non-empty byte including NUL and
invalid UTF-8; reject namespace over 128 bytes. Normalize zero limits to defaults and reject values
outside hard bounds.

- [ ] **Step 4: Add the fixed schema and immutable writer**

Use exactly the approved table and named constraints:

```sql
create table if not exists public.bluetape_batch_checkpoints (
    namespace bytea not null constraint bluetape_batch_checkpoints_namespace_size_check
        check (pg_catalog.octet_length(namespace) between 1 and 128),
    checkpoint_key bytea not null constraint bluetape_batch_checkpoints_key_size_check
        check (pg_catalog.octet_length(checkpoint_key) between 1 and 1024),
    revision bigint not null constraint bluetape_batch_checkpoints_revision_check
        check (revision > 0),
    payload bytea not null constraint bluetape_batch_checkpoints_payload_size_check
        check (pg_catalog.octet_length(payload) <= 16777216),
    updated_at timestamptz not null,
    constraint bluetape_batch_checkpoints_pkey primary key (namespace, checkpoint_key)
)
```

`New` stores the caller DB, normalized options, codec, callback, and default transaction factory;
it does not ping, migrate, close, or mutate the pool. Add
`var _ batch.AtomicCheckpointWriter[any] = (*Writer[any, any])(nil)`.

- [ ] **Step 5: Verify GREEN and commit**

```bash
gofmt -w batch/sqlcheckpoint/doc.go batch/sqlcheckpoint/options.go batch/sqlcheckpoint/options_test.go batch/sqlcheckpoint/schema.go batch/sqlcheckpoint/schema_test.go batch/sqlcheckpoint/writer.go
go test -count=1 ./batch/sqlcheckpoint -run 'Options|New|Schema|Interface'
git add batch/sqlcheckpoint
git commit -m "feat: define SQL checkpoint provider"
```

Expected: constructor and schema tests PASS without opening a DB connection.

### Task 4: Implement Typed Load and Redacted Diagnostics

**Complexity:** Medium data/error boundary.

**Files:**
- Create: `batch/sqlcheckpoint/load.go`
- Create: `batch/sqlcheckpoint/load_test.go`
- Create: `batch/sqlcheckpoint/errors.go`
- Create: `batch/sqlcheckpoint/errors_test.go`
- Modify: `batch/sqlcheckpoint/writer.go`

- [ ] **Step 1: Write RED validation, decode, and redaction tests**

Cover nil context normalization, pre-cancel with zero DB dispatch, empty/oversized/exact-byte key,
missing row, valid row, revision zero/negative, payload at limit, oversized stored payload with nil
projection, malformed decode, owned payload copy, nil/zero receiver, nested `errors.Is/As`, and hostile
namespace/key/payload/DSN/panic/cause markers absent from `Error`, `%v`, and `%+v`.

- [ ] **Step 2: Observe RED**

```bash
go test -count=1 ./batch/sqlcheckpoint -run 'Load|CodecError|OpError|KeyID|Redact'
```

Expected: FAIL because Load and typed errors do not exist.

- [ ] **Step 3: Add the one-query conditional projection**

```go
const loadSQL = `select revision,
       pg_catalog.octet_length(payload),
       case when pg_catalog.octet_length(payload) <= $3 then payload end
from public.bluetape_batch_checkpoints
where namespace = $1::bytea and checkpoint_key = $2::bytea`
```

`Load` validates before dispatch, scans revision/length/nullable payload, maps `sql.ErrNoRows` to
`exists=false`, rejects length over configured/hard limits without Decode, copies payload bytes,
decodes `C`, and returns `batch.VersionedCheckpoint{Value: value, Version: uint64(revision)}`.
Use an unexported `rowScanner`/`queryRow` function initialized from `db.QueryRowContext`; package
tests replace only that function with deterministic scanners, so no mocking dependency is added.

- [ ] **Step 4: Add redacted `OpError`, `CodecError`, and KeyID**

Both errors expose fixed operation/family strings and causal `Unwrap` without rendering the cause.
Compute KeyID by hashing an 8-byte big-endian namespace length followed by exact namespace/key bytes,
then hex-encoding the first 10 digest bytes. Use the prefix `sql-checkpoint-key:` and document that
it is a sensitive pseudonymous diagnostic, not a metric label or authorization identifier.

```go
func redactedKeyID(namespace, key []byte) string {
    hash := sha256.New()
    var size [8]byte
    binary.BigEndian.PutUint64(size[:], uint64(len(namespace)))
    _, _ = hash.Write(size[:])
    _, _ = hash.Write(namespace)
    _, _ = hash.Write(key)
    return "sql-checkpoint-key:" + hex.EncodeToString(hash.Sum(nil)[:10])
}
```

- [ ] **Step 5: Verify GREEN and commit**

```bash
gofmt -w batch/sqlcheckpoint/load.go batch/sqlcheckpoint/load_test.go batch/sqlcheckpoint/errors.go batch/sqlcheckpoint/errors_test.go batch/sqlcheckpoint/writer.go
go test -count=1 ./batch/sqlcheckpoint -run 'Load|CodecError|OpError|KeyID|Redact'
git add batch/sqlcheckpoint
git commit -m "feat: load typed SQL checkpoints"
```

Expected: all load/error tests PASS and hostile markers remain absent from formatted errors.

### Task 5: Implement Atomic Commit and Transaction Ownership Proof

**Complexity:** Critical transaction state machine; no automatic retries.

**Files:**
- Create: `batch/sqlcheckpoint/session.go`
- Create: `batch/sqlcheckpoint/queries.go`
- Create: `batch/sqlcheckpoint/commit.go`
- Create: `batch/sqlcheckpoint/commit_test.go`
- Modify: `batch/sqlcheckpoint/errors.go`
- Modify: `batch/sqlcheckpoint/writer.go`

- [ ] **Step 1: Write RED deterministic transaction tests**

Use an injected `beginTx` returning a fake `transaction` that records Savepoint, callback SQL,
Release/Rollback-To, checkpoint DML, Commit, and Rollback. Cover validation/encode before Begin,
empty callback suppression, exact single callback, insert/update CAS, conflict rollback, callback
error, swallowed `25P02`, panic, context before Commit, Commit server rejection, transport loss,
rollback error preservation, revision maximum, and no library retry. Assert every error returns
revision zero.

- [ ] **Step 2: Observe RED**

```bash
go test -count=1 ./batch/sqlcheckpoint -run 'Commit|Ownership|AtomicityPanic|Conflict|Exhausted'
```

Expected: FAIL because Commit and the transaction adapter do not exist.

- [ ] **Step 3: Add a testable transaction adapter and guarded session**

```go
type transaction interface {
    sqlkit.Session
    ScanRevision(context.Context, string, ...any) (int64, error)
    Commit() error
    Rollback() error
}

type sqlTransaction struct { tx *sql.Tx }

func (t *sqlTransaction) ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error) {
    return t.tx.ExecContext(ctx, q, args...)
}
func (t *sqlTransaction) QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error) {
    return t.tx.QueryContext(ctx, q, args...)
}
func (t *sqlTransaction) QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row {
    return t.tx.QueryRowContext(ctx, q, args...)
}
func (t *sqlTransaction) ScanRevision(ctx context.Context, q string, args ...any) (int64, error) {
    var revision int64
    err := t.tx.QueryRowContext(ctx, q, args...).Scan(&revision)
    return revision, err
}
func (t *sqlTransaction) Commit() error { return t.tx.Commit() }
func (t *sqlTransaction) Rollback() error { return t.tx.Rollback() }
```

The unexported guarded session forwards only `ExecContext`, `QueryContext`, and `QueryRowContext`;
it never exposes concrete `*sql.Tx`, Commit, Rollback, or the reserved guard identifier.

- [ ] **Step 4: Add fixed CAS statements and preflight**

```go
const insertCheckpointSQL = `insert into public.bluetape_batch_checkpoints
(namespace, checkpoint_key, revision, payload, updated_at)
values ($1::bytea, $2::bytea, 1, $3::bytea, pg_catalog.clock_timestamp())
on conflict (namespace, checkpoint_key) do nothing
returning revision`

const updateCheckpointSQL = `update public.bluetape_batch_checkpoints set
revision = revision + 1, payload = $3::bytea, updated_at = pg_catalog.clock_timestamp()
where namespace = $1::bytea and checkpoint_key = $2::bytea and revision = $4::bigint
returning revision`
```

Reject `expectedVersion > math.MaxInt64` as invalid and `== math.MaxInt64` as
`ErrCheckpointVersionExhausted` before codec/callback/DB. Assert checkpoint type `C`, Encode, and
payload limits before Begin. Map `sql.ErrNoRows` from either CAS statement to
`ErrCheckpointConflict` and rollback.

- [ ] **Step 5: Implement the ownership probe and panic boundary**

Use the fixed identifier `bluetape_sqlcheckpoint_guard`. For non-empty items, dispatch SAVEPOINT,
call the callback through a recovery wrapper, then probe with
`context.WithTimeout(context.WithoutCancel(ctx), time.Second)`:

```go
const savepointSQL = `savepoint bluetape_sqlcheckpoint_guard`
const releaseSavepointSQL = `release savepoint bluetape_sqlcheckpoint_guard`
const rollbackToSavepointSQL = `rollback to savepoint bluetape_sqlcheckpoint_guard`
```

Release success proves ownership. On PostgreSQL `25P02`, require Rollback-To success to prove the
original guard; then full rollback and return the callback/probe error or re-panic the original
value. `25P01`, `3B001`, active-context `sql.ErrTxDone`, timeout, canceled-context `sql.ErrTxDone`,
bad connection, transport, and unclassified probe failures must return an error matching both
`ErrAtomicityUnknown` and `ErrCommitUnknown`; add `ErrCallbackContractViolation` only for positive
lifecycle evidence. No unproven path may execute CAS or Commit.

Create `AtomicityPanic` with a fixed sanitized `Error`, an `Unwrap` matching both sentinels, and a
`PanicValue` accessor that alone returns the original sensitive value. Ownership-proven panic
cleanup re-panics the original value; ownership-unknown cleanup panics `*AtomicityPanic`.
Define exported `ErrCallbackContractViolation = errors.New("sql checkpoint: callback contract violation")`
in `errors.go` and join it only on positive lifecycle evidence.

- [ ] **Step 6: Complete CAS, cancellation, and Commit classification**

After callback/probe success, run one CAS statement. Recheck `ctx.Err()` before Commit and rollback
known cancellation. Call Commit once. A pgx `*pgconn.PgError` is a known server rejection; a
transport/bad-connection/in-flight cancellation/non-server result is a sanitized `OpError` joined
with `ErrCommitUnknown`, never `ErrAtomicityUnknown`. Preserve rollback causes inside the typed
error without rendering raw provider text.

- [ ] **Step 7: Verify GREEN, focused race, and commit**

```bash
gofmt -w batch/sqlcheckpoint/session.go batch/sqlcheckpoint/queries.go batch/sqlcheckpoint/commit.go batch/sqlcheckpoint/commit_test.go batch/sqlcheckpoint/errors.go batch/sqlcheckpoint/writer.go
go test -count=1 ./batch/sqlcheckpoint -run 'Commit|Ownership|AtomicityPanic|Conflict|Exhausted'
go test -race -count=1 ./batch ./batch/sqlcheckpoint
git add batch/sqlcheckpoint batch/errors.go
git commit -m "feat: commit SQL checkpoints atomically"
```

Expected: deterministic tests and race detector PASS; operation logs prove no retry and no CAS after
an unproven ownership probe.

### Task 6: Prove PostgreSQL Restart, Concurrency, Security, and Failure Semantics

**Complexity:** High integration proof; run sequentially.

**Files:**
- Create: `batch/sqlcheckpoint/integration_test.go`
- Create: `batch/sqlcheckpoint/security_test.go`
- Create: `batch/sqlcheckpoint/stress_test.go`

- [ ] **Step 1: Build one sequential PostgreSQL fixture**

Use `testcontainers/postgres.Start`, blank-import pgx stdlib, open/ping the admin pool, apply
`SchemaSQL`, create a business table with an idempotency key, and register cleanup with `t.Cleanup`.
Use `t.Setenv` only for fixture configuration; do not use `t.Parallel` anywhere in these files.

- [ ] **Step 2: Prove success, restart, rollback, and exact CAS outcomes**

Test missing→revision 1, exact update→revision 2, business row plus checkpoint in one commit,
checkpoint statement failure rolling back business rows, callback failure rollback, success restart,
known rollback replay, NUL/invalid-UTF8 namespace/key isolation, checkpoint-only commit, and pool
ownership. With two pools loading the same version behind a barrier, assert exactly one business
row/checkpoint winner and one `ErrCheckpointConflict` loser.

- [ ] **Step 3: Prove raw transaction-control and panic failure shields**

Use real callback SQL for raw COMMIT, raw ROLLBACK, raw COMMIT followed by cancellation,
COMMIT→BEGIN→failing statement, raw COMMIT→panic, and normal panic. Assert raw paths execute no
checkpoint DML/provider Commit and return or panic with atomicity-unknown; normal panic preserves the
original value and leaves no business/checkpoint row. Verify `AtomicityPanic.Error`, `%v`, `%+v`, and
unwrap strings omit a secret-bearing panic/DSN while `PanicValue()` alone preserves it.

- [ ] **Step 4: Prove cancellation and small-pool release**

Barrier pre-Begin, callback, checkpoint DML, ownership probe, and provider Commit phases. Assert
pre-Begin has zero DB access; ownership-proven cancellation is known rollback; canceled-context
`sql.ErrTxDone` is atomicity-unknown; post-Commit-dispatch cancellation is commit-unknown followed by
fresh Load. With `MaxOpenConns(1)`, require connection reuse within one second plus a fixed 500ms
scheduler tolerance, then poll until `DBStats.InUse == 0`; assert no late callback, CAS, or Commit.

- [ ] **Step 5: Prove catalog, privileges, and hostile drift detection**

Query `pg_class`, `pg_attribute` including `attacl`, `pg_constraint`, `pg_index`, `pg_policy`,
`pg_trigger`, `pg_rewrite`, `pg_namespace`, and role membership. Assert permanent ordinary table,
exact columns/types/nullability/order, fixed validated constraint names, PK order, no RLS/policies,
user triggers/rules, and no PUBLIC/inherited/column privilege. Create a non-login migration owner and
runtime role; prove runtime has only schema USAGE plus SELECT/INSERT/UPDATE and cannot CREATE,
DELETE, TRUNCATE, REFERENCES, TRIGGER, ALTER, or grant. Mutate one catalog property per subtest and
assert the test preflight rejects it.

- [ ] **Step 6: Run bounded stress and all provider tests sequentially**

```bash
go test -count=1 ./batch/sqlcheckpoint -run 'TestPostgres'
go test -count=20 ./batch/sqlcheckpoint -run 'Concurrent|Conflict|Cancellation|Ownership'
go test -race -count=1 ./batch ./batch/sqlcheckpoint
```

Expected: every command exits 0; exact winner/loser and operation counts remain stable.

- [ ] **Step 7: Commit PostgreSQL proof**

```bash
git add batch/sqlcheckpoint/integration_test.go batch/sqlcheckpoint/security_test.go batch/sqlcheckpoint/stress_test.go
git commit -m "test: prove SQL checkpoint recovery"
```

### Task 7: Add Compile-Checked Examples and Bilingual Package Documentation

**Complexity:** Medium caller contract and recovery documentation.

**Writing skill:** `bluetape-writer` for English/Korean parity and validation.

**Files:**
- Create: `batch/sqlcheckpoint/example_test.go`
- Create: `batch/sqlcheckpoint/readme_test.go`
- Create: `batch/sqlcheckpoint/README.md`
- Create: `batch/sqlcheckpoint/README.ko.md`
- Modify: `batch/doc.go`
- Modify: `batch/README.md`
- Modify: `batch/README.ko.md`

- [ ] **Step 1: Write RED example and README contract tests**

Require both locale files to contain `NewAtomicStep`, `SchemaSQL`, `ErrCommitUnknown`,
`ErrAtomicityUnknown`, `AtomicityPanic`, `PanicValue`, `SAVEPOINT`, callback restrictions,
same-key serialization, migration/privilege, recovery, and validation commands. Require both to
embed `postgres-batch-checkpoint-atomic-sequence.png` and have equal fenced-code-block counts.

- [ ] **Step 2: Add compile-checked construction and recovery examples**

Create examples for caller-owned migration/runtime pools, JSON codec, tx-bound business insert,
`NewAtomicStep`, provider-owned commit-unknown fresh Load, and top-level panic recovery. The recovery
branch must inspect `errors.Is(recoveredErr, batch.ErrAtomicityUnknown)` before generic restart,
quiesce same-key intake, and route to a named `reconcileCheckpoint` helper. Never log
`AtomicityPanic.PanicValue()`.

```go
defer func() {
    recovered := recover()
    recoveredErr, ok := recovered.(error)
    if ok && errors.Is(recoveredErr, batch.ErrAtomicityUnknown) {
        quiesceCheckpointKey(checkpointKey)
        reconcileCheckpoint(checkpointKey)
        return
    }
    if recovered != nil { panic(recovered) }
}()
```

- [ ] **Step 3: Write English and Korean package docs in parity**

Document architecture, additive constructor, schema bootstrap order, callback transaction rules,
payload/codec/encryption, key identity, conflict/exhaustion, commit unknown versus atomicity unknown,
panic supervisor handling, rollout/rollback, retention, pool ownership, unsupported topology, and
exact test commands. Update root batch docs to state legacy Writer+CheckpointStore is durable but
not atomic with business writes.

- [ ] **Step 4: Verify examples and parity, then commit**

```bash
go test -count=1 ./batch ./batch/sqlcheckpoint -run 'Example|README|Readme'
git diff --check
git add batch/doc.go batch/README.md batch/README.ko.md batch/sqlcheckpoint/example_test.go batch/sqlcheckpoint/readme_test.go batch/sqlcheckpoint/README.md batch/sqlcheckpoint/README.ko.md
git commit -m "docs: explain atomic SQL checkpoints"
```

Expected: examples compile and both locale contract tests PASS.

### Task 8: Create the Required Sequence Diagram and Release Runbook

**Complexity:** Medium public documentation asset and operator gate.

**Diagram skill:** `$bluetape-diagram`; no Mermaid, ASCII, or Graphviz public artifact.

**Files:**
- Create: `docs/images/readme-diagrams/postgres-batch-checkpoint-atomic-sequence.svg`
- Create: `docs/images/readme-diagrams/postgres-batch-checkpoint-atomic-sequence.png`
- Modify: `README.md`
- Modify: `README.ko.md`
- Modify: `CHANGELOG.md`
- Modify: `docs/release/v0.19.0-provider-conformance-runbook.md`
- Modify: `batch/sqlcheckpoint/README.md`
- Modify: `batch/sqlcheckpoint/README.ko.md`

- [ ] **Step 1: Use `$bluetape-diagram` to create the SVG source**

Draw one English-label sequence with participants `Step`, `CheckpointReader`, `Atomic Writer`, and
`PostgreSQL`. Show Load/Restore, consumed-input boundary, SAVEPOINT, business callback, ownership
probe, checkpoint CAS, Commit, known rollback, conflict rollback, provider-Commit unknown→fresh
Load, and atomicity unknown→quiesce/manual reconciliation. Keep labels readable at README width.

- [ ] **Step 2: Render and audit paired assets**

```bash
cairosvg docs/images/readme-diagrams/postgres-batch-checkpoint-atomic-sequence.svg -o docs/images/readme-diagrams/postgres-batch-checkpoint-atomic-sequence.png -s 2
xmllint --noout docs/images/readme-diagrams/postgres-batch-checkpoint-atomic-sequence.svg
file docs/images/readme-diagrams/postgres-batch-checkpoint-atomic-sequence.svg docs/images/readme-diagrams/postgres-batch-checkpoint-atomic-sequence.png
```

Expected: valid SVG XML and a full-size PNG at 2x scale. Inspect the PNG at original resolution and
fix clipping, overlap, illegible text, missing arrows, or ambiguous failure branches.

- [ ] **Step 3: Update root inventory, changelog, and runbook**

Add `batch/sqlcheckpoint` to both root package tables. Add the v0.19.0 feature entry. Extend the
runbook with PUBLIC CREATE revoke→owner migration→catalog preflight→runtime grant, column ACL,
writable-primary/RPO, representative callback reconciliation drills, safe replay/repair approval,
quiesce release, autovacuum/replication/dead-tuple thresholds, shutdown order, canary, rollback, and
retention. Keep English and Korean runbook sections structurally aligned.

- [ ] **Step 4: Verify public documentation and commit**

```bash
rg -n 'batch/sqlcheckpoint|ErrAtomicityUnknown|postgres-batch-checkpoint-atomic-sequence' README.md README.ko.md CHANGELOG.md batch/sqlcheckpoint/README.md batch/sqlcheckpoint/README.ko.md docs/release/v0.19.0-provider-conformance-runbook.md
go test -count=1 ./batch/sqlcheckpoint -run 'README|Readme'
git diff --check
git add README.md README.ko.md CHANGELOG.md docs/release/v0.19.0-provider-conformance-runbook.md docs/images/readme-diagrams/postgres-batch-checkpoint-atomic-sequence.svg docs/images/readme-diagrams/postgres-batch-checkpoint-atomic-sequence.png batch/sqlcheckpoint/README.md batch/sqlcheckpoint/README.ko.md
git commit -m "docs: publish SQL checkpoint operations"
```

### Task 9: Run Final Verification, Review, and Lesson Capture

**Complexity:** High release gate; no feature expansion.

**Verification skill:** `verification-before-completion`; Step 6-R uses six read-only perspectives plus main integration.

**Files:**
- Create: `docs/superpowers/reviews/2026-07-14-issue-532-sql-checkpoint-step-6r-code-review.md`
- Create: `docs/lessons/2026-07-14-issue-532-sql-checkpoint.md`
- Modify only if findings require: files already listed in Tasks 1-8

- [ ] **Step 1: Run focused formatting, unit, race, and PostgreSQL gates**

```bash
gofmt -w batch/*.go batch/sqlcheckpoint/*.go
go test -count=1 ./batch ./batch/sqlcheckpoint
go test -count=20 ./batch/sqlcheckpoint -run 'Concurrent|Conflict|Cancellation|Ownership'
go test -race -count=1 ./batch ./batch/sqlcheckpoint
git diff --check origin/develop...HEAD
```

Expected: every command exits 0 with a fresh observed exit code.

- [ ] **Step 2: Run the authoritative local gate from scratch**

```bash
make ci
```

Expected: fresh exit 0. If lint reports deleted-worktree cache paths, run
`golangci-lint cache clean && make lint`, then rerun `make ci` from scratch.

- [ ] **Step 3: Run Step 6-R in two bounded waves**

Wave 1: performance, stability, security. Wave 2: operator/Ops, developer/API, user/caller. Every
lane reviews the same exact HEAD read-only and reports P0/P1 with file/line evidence. The main
session integrates results, fixes findings, reruns affected and exact-final lanes, and records
`P0=0 P1=0`. No seventh integration subagent.

- [ ] **Step 4: Re-run verification after review repairs**

```bash
go test -count=1 ./batch ./batch/sqlcheckpoint
go test -race -count=1 ./batch ./batch/sqlcheckpoint
make ci
git status --short
git diff --check origin/develop...HEAD
```

Expected: all commands exit 0 and the worktree is clean after committing review repairs.

- [ ] **Step 5: Capture reusable lessons and commit evidence**

The lesson must record consumed-input checkpoint boundaries, insert/update CAS split, transaction
ownership proof, `25P02` Rollback-To requirement, commit unknown versus atomicity unknown, panic
supervisor handling, unkeyed option compatibility, small-pool cleanup, and catalog/ACL preflight.

```bash
git add docs/superpowers/reviews/2026-07-14-issue-532-sql-checkpoint-step-6r-code-review.md docs/lessons/2026-07-14-issue-532-sql-checkpoint.md
git commit -m "docs: record SQL checkpoint verification"
```

- [ ] **Step 6: Hand off to PR creation and live CI**

Prepare an English PR that mirrors issue #532, links the final spec/plan/reviews, includes exact
test evidence, and ends with `## DoD Status`. Push and create the PR, wait for all required checks,
address failures/review threads, then stop at `PENDING - PR ready for explicit merge decision`.
Do not merge without the user's explicit approval.
