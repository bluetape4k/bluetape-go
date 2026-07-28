# PostgreSQL durable batch checkpoint 구현 계획

> 한국어 재작성 범위: 이 계획 문서는 한국어 운영 문서로 읽히도록 제목, 판단, 작업 설명, 위험, 검증, 롤백 문맥을 한국어로 정리한다. 명령, 경로, API 이름, 이슈/PR 번호, 브랜치명, 코드 블록, 테스트 출력 같은 증거 문자열은 정확성을 위해 원문 그대로 보존한다.


> **에이전트 작업자용:** 필수 하위 스킬: 사용 superpowers:subagent-driven-development (권장) 또는 superpowers:executing-plans to 이 계획을 작업 단위로 구현. 단계는 checkbox (`- [ ]`) 추적 문법을 사용.

**목표:** 추가 an opt-in batch path that commits PostgreSQL business writes 및 a revision-fenced durable checkpoint in one transaction 변경하지 않고 the 기존 `StepOptions`/`NewStep` API.

**아키텍처:** The root `batch` 패키지 adds provider-neutral atomic checkpoint types plus a separate `AtomicStepOptions`/`NewAtomicStep` constructor 및 an atomic run loop that chunks by consumed input. `batch/sqlcheckpoint` owns a 호출자-provided `*sql.DB`, typed codec, 및 tx-bound write callback; it uses a reserved savepoint ownership probe, split insert/update CAS, 및 distinct commit-unknown versus atomicity-unknown recovery barriers. Schema migration, pool lifetime, same-key serialization, callback resources, codec evolution, 및 recovery orchestration remain 호출자-owned.

**기술 스택:** Go 1.26, `context`, `database/sql`, `errors`, `crypto/sha256`, 기존 `sqlkit.Session`, pgx v5 `pgconn`, PostgreSQL Testcontainers, standard Go 테스트/race detector, CairoSVG-rendered SVG/PNG documentation.

---

## 파일 지도

| Area | 파일 | 책임 |
|---|---|---|
| Root atomic 계약 | `batch/errors.go`, `batch/atomic.go`, `batch/atomic_test.go`, `batch/testdata/compat/main.go`, `Makefile` | Provider-neutral sentinels, versioned checkpoint interface, additive atomic constructor/options, CI-enforced legacy source-compatibility fixture. |
| Atomic step runtime | `batch/step.go`, `batch/atomic_step.go`, `batch/atomic_step_test.go` | 보존 the legacy loop 및 add consumed-input atomic chunking, restore, status, counters, skip/filter, close, 및 없음-retry behavior. |
| SQL 공개 API/schema | `batch/sqlcheckpoint/{doc.go,options.go,options_test.go,schema.go,schema_test.go,writer.go}` | Caller-owned constructor, immutable limits/codec/callback, fixed DDL, key/checkpoint validation, 없음 implicit I/O. |
| SQL load/진단 | `batch/sqlcheckpoint/{load.go,load_test.go,errors.go,errors_test.go}` | Conditional payload projection, typed decode, redacted operation/codec 오류, correlation ID, zero/nil safety. |
| SQL atomic commit | `batch/sqlcheckpoint/{session.go,queries.go,commit.go,commit_test.go}` | Tx adapter, callback session, savepoint ownership proof, CAS, panic preservation, cancellation, rollback, 및 unknown classification. |
| PostgreSQL proof | `batch/sqlcheckpoint/{integration_test.go,security_test.go,stress_test.go}` | Sequential Testcontainers success, restart, conflicts, hostile lifecycle changes, raw transaction control, cancellation, pool drain, catalog/ACL, 및 race/stress proof. |
| Examples 및 패키지 docs | `batch/sqlcheckpoint/{example_test.go,readme_test.go,README.md,README.ko.md}`, `batch/{doc.go,README.md,README.ko.md}` | Compile-checked construction/recovery, callback safety, legacy non-atomic warning, bilingual parity. |
| Public docs | `README.md`, `README.ko.md`, `CHANGELOG.md`, `docs/release/v0.19.0-provider-conformance-runbook.md`, `docs/images/readme-diagrams/postgres-batch-checkpoint-atomic-sequence.{svg,png}` | Discoverability, migration/runbook gates, recovery drills, rollout/rollback, sequence diagram. |
| Workf낮음 evidence | `docs/superpowers/reviews/2026-07-14-issue-532-sql-checkpoint-{risk,plan-review,step-6r-code-review}.md`, `docs/lessons/2026-07-14-issue-532-sql-checkpoint.md` | Pre-implementation risks, 단계 3-R/6-R evidence, 및 reusable lessons. |

The core 계약 및 PostgreSQL provider remain one plan because neither is independently useful:
the provider must implement the new root interface, 및 the atomic step requires a provider to prove
its transaction 및 restart semantics. Each task still ends in a compiling, testable commit.

## 의존 순서 및 중지 조건

작업 0 freezes approved artifacts 및 risks. Tasks 1-2 establish the root 계약 및 atomic loop
전에 provider work. 작업 3 fixes the SQL API 및 schema; 작업 4 adds load 및 진단; 작업 5
adds the transaction/ownership state machine. 작업 6 supplies real PostgreSQL proof. Tasks 7-8
document 만 settled behavior. 작업 9 performs final verification 및 단계 6-R.

실행 every Testcontainers-backed command sequentially. 다음을 하지 않는다: run `make ci` concurrently 함께 a
PostgreSQL suite. 다음을 하지 않는다: add a dependency, modify the exported fields of `StepOptions`, change
`NewStep`, add automatic migration/retry, 또는 broaden the callback beyond `sqlkit.Session`. Any such
need stops execution 및 returns to design review. A failing ownership probe must never dispatch
checkpoint DML 또는 provider-owned `Commit`.
Throughput targets, capacity rankings, 및 new benchmarks are deferred to issue #560 및 must 아님
expand this issue's DoD.

### 작업 0: 고정 Approved Artifacts 및 예측 Risks

**복잡도:** Small documentation gate; blocks source edits.

**파일:**
- 검증: `docs/superpowers/specs/2026-07-14-issue-532-sql-checkpoint-design.md`
- 검증: `docs/superpowers/reviews/2026-07-14-issue-532-sql-checkpoint-spec-review.md`
- 검증: `docs/superpowers/plans/2026-07-14-issue-532-sql-checkpoint-plan.md`
- 생성: `docs/superpowers/reviews/2026-07-14-issue-532-sql-checkpoint-risk.md`

- [ ] **단계 1: 검증 the approved artifact-만 branch**

```bash
git status --short
git log --oneline origin/develop..HEAD
git diff --check origin/develop...HEAD
shasum -a 256 docs/superpowers/specs/2026-07-14-issue-532-sql-checkpoint-design.md
```

예상: clean worktree; approved spec hash
`3d0208d73a8bc62073a57e60a5607502d53e9c69d0642e2f88351286024ed21e`; 만 design,
review, 및 plan artifacts ahead of `origin/develop`; 없음 `batch/sqlcheckpoint` directory.

- [ ] **단계 2: 생성 the pre-implementation risk table**

생성 a Markdown table 함께 `Risk`, `Trigger`, `Signal`, `Prevention`, `Recovery`, 및 `Owner`.
Include concrete rows for buffered-output checkpoint loss, filter/skip transaction amplification,
same-key overlap, missing-row stale resurrection, revision exhaustion, codec drift, oversized
payload, callback captured DB/external side effect, raw COMMIT/ROLLBACK, COMMIT→BEGIN→failed SQL,
panic 후 raw COMMIT, ownership-probe timeout, provider-커밋 response loss, cancellation races,
pool starvation, unlogged/temp/hostile relation, RLS/trigger/rewrite drift, schema/table/column ACL
drift, PUBLIC object creation, WAL/dead-tuple/replication pressure, legacy cutover, rollback,
supervisor automatic panic replay, Testcontainers leakage, 및 bilingual/diagram drift.

- [ ] **단계 3: 기록 fresh baseline evidence**

```bash
go version
go list -m -f '{{.Path}} {{.Version}}' github.com/jackc/pgx/v5 github.com/testcontainers/testcontainers-go
go test -count=1 ./batch ./sqlkit
go test -count=1 ./...
```

예상: 모든 commands exit 0. 기록 the exact versions 및 exit codes in the risk artifact.

- [ ] **단계 4: 커밋 risk evidence 전에 source work**

```bash
git add docs/superpowers/reviews/2026-07-14-issue-532-sql-checkpoint-risk.md
git commit -m "docs: predict SQL checkpoint risks"
```

예상: the risk commit predates every source commit.

### 작업 1: 추가 the Provider-Neutral Atomic Contract

**복잡도:** Medium 공개 API change; legacy source compatibility is mandatory.

**Pattern skill:** `bluetape-go-patterns` API compatibility, 오류 wrapping, nil safety.

**파일:**
- 생성: `batch/atomic.go`
- 생성: `batch/atomic_test.go`
- 생성: `batch/testdata/compat/main.go`
- Modify: `batch/errors.go`
- Modify: `batch/step.go`
- Modify: `Makefile`

- [ ] **단계 1: Write RED 계약 및 compatibility 테스트**

추가 root sentinel/interface 테스트 및 this external-패키지 `testdata` compile fixture 함께 the
exact 기존 field order. Keeping it under `testdata` avoids normal `go vet` rejecting the
deliberately unkeyed literal:

```go
package compat

import "github.com/bluetape4k/bluetape-go/batch"

var _ = batch.StepOptions[int, int]{
    "legacy-unkeyed", 1, nil, nil, nil,
    batch.RetryPolicy{}, batch.SkipPolicy{}, nil, "",
}
```

Test `NewAtomicStep` for empty name, zero/default 및 negative chunk size, nil reader, nil processor,
nil atomic writer, non-`CheckpointReader`, negative retry/skip policies, default checkpoint key,
및 없음 reader/provider side effect on constructor failure. 검증 `AtomicStepOptions` exposes 없음
legacy writer/store field by compiling 만 the approved literal shape.
추가 `$(GO) test -vet=off ./batch/testdata/compat` to the `test` target 후 the normal serial suite,
so every `make test` 및 `make ci` run continuously enforces the deliberately unkeyed fixture.

- [ ] **단계 2: Observe RED**

```bash
go test -count=1 ./batch -run 'Atomic|Compatibility'
go test -vet=off ./batch/testdata/compat
```

예상: build FAIL because the sentinels, atomic types, 및 constructor do 아님 exist.

- [ ] **단계 3: 추가 the minimal 공개 계약**

생성 the exact API 및 Go doc comments:

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

추가 these sentinels to `batch/errors.go`:

```go
ErrCheckpointConflict = errors.New("batch: checkpoint revision conflict")
ErrCommitUnknown = errors.New("batch: commit outcome unknown")
ErrAtomicityUnknown = errors.New("batch: atomicity outcome unknown")
ErrCheckpointVersionExhausted = errors.New("batch: checkpoint version exhausted")
```

추가 an unexported `atomic AtomicCheckpointWriter[O]` field to `Step`; do 아님 alter any exported
`StepOptions` field 또는 the `NewStep` signature.

- [ ] **단계 4: 구현 the additive constructor**

사용 the same validation order/defaults as `NewStep`, require `CheckpointReader`, normalize policies,
및 set 만 reader/processor/atomic/key fields:

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

- [ ] **단계 5: 검증 GREEN, full legacy regression, 및 commit**

```bash
gofmt -w batch/atomic.go batch/atomic_test.go batch/testdata/compat/main.go batch/errors.go batch/step.go
go test -count=1 ./batch
go test -vet=off ./batch/testdata/compat
git diff --check
git add Makefile batch/atomic.go batch/atomic_test.go batch/testdata/compat/main.go batch/errors.go batch/step.go
git commit -m "feat: add atomic batch checkpoint contract"
```

예상: 모든 legacy 테스트 및 the external unkeyed fixture PASS.

### 작업 2: 구현 Consumed-Input Atomic 단계 Execution

**복잡도:** High correctness path; use strict TDD 및 small commits if RED failures expose separate defects.

**파일:**
- 생성: `batch/atomic_step.go`
- 생성: `batch/atomic_step_test.go`
- Modify: `batch/step.go`

- [ ] **단계 1: Write RED restore, boundary, 및 counter 테스트**

생성 deterministic `checkpointReader` 및 `recordingAtomicWriter` fakes. Cover missing/기존
restore, one commit per consumed-input boundary, expected version advancement, empty input,
exact-multiple EOF, kept→filter, kept→processor-skip, 모든-filter, 모든-skip, mixed streams,
checkpoint-만 commits, capture missing/failure, conflict, exhaustion, commit unknown, atomicity
unknown, pre-cancel, 및 close failure. 검증 atomic 커밋 오류
never consult writer retry/skip policy 및 never increment `WriteCount` 또는 clear pending output.

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

- [ ] **단계 2: Observe RED**

```bash
go test -count=1 ./batch -run 'AtomicStep|AtomicBoundary|AtomicRestore|AtomicStatus'
```

예상: FAIL because `Step.Run` still enters the legacy writer path.

- [ ] **단계 3: Dispatch atomic steps 변경하지 않고 the legacy body**

At the top of `Run`, 후 context normalization 및 nil receiver handling, add:

```go
if s.atomic != nil {
    return s.runAtomic(ctx)
}
```

유지 the remaining legacy code 및 `NewStep` unchanged. 업데이트 `statusForError` so an 오류 matching
`ErrCommitUnknown` is `StatusFailed` even when it also wraps cancellation:

```go
func statusForError(err error) Status {
    if errors.Is(err, ErrCommitUnknown) { return StatusFailed }
    if isContextError(err) { return StatusCancelled }
    return StatusFailed
}
```

- [ ] **단계 4: 구현 the atomic loop 및 commit helper**

구현 `runAtomic` 함께 reader-만 lifecycle, Load/Restore 전에 reads, `progressCount` per
consumed input, pending kept output, 및 commit 만 at `progressCount == chunkSize` 또는 nonzero EOF.
The helper must capture the reader checkpoint 전에 calling the provider:

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

After a successful helper call, clear pending 및 progress; 후 an 오류, return immediately
without mutation. Processor retry/skip stays in `s.process`; filter 및 skip increment their legacy
counters 및 still count toward the boundary. 다음을 하지 않는다: call the atomic provider's Close method.

- [ ] **단계 5: 검증 counters, statuses, race behavior, 및 commit**

```bash
gofmt -w batch/step.go batch/atomic_step.go batch/atomic_step_test.go
go test -count=1 ./batch
go test -race -count=1 ./batch
git add batch/step.go batch/atomic_step.go batch/atomic_step_test.go
git commit -m "feat: execute atomic batch checkpoints"
```

예상: legacy 및 atomic 테스트 PASS; race detector exits 0.

### 작업 3: 정의 SQL Checkpoint Options, Constructor, 및 Schema

**복잡도:** Medium 공개 provider surface; 없음 database access in constructor.

**파일:**
- 생성: `batch/sqlcheckpoint/doc.go`
- 생성: `batch/sqlcheckpoint/options.go`
- 생성: `batch/sqlcheckpoint/options_test.go`
- 생성: `batch/sqlcheckpoint/schema.go`
- 생성: `batch/sqlcheckpoint/schema_test.go`
- 생성: `batch/sqlcheckpoint/writer.go`

- [ ] **단계 1: Write RED option, constructor, identity, 및 schema 테스트**

Cover nil DB/callback/encode/decode, namespace default/exact bytes/129-byte rejection, key defaults
및 1..1024 hard bounds, payload defaults 및 1..16MiB hard bounds, constructor 없음 ping/schema I/O,
nil/zero writer safety, 및 compile-time interface implementation. 검증 모든 named constraints,
fixed column order, `bytea` identity, positive revision, payload ceiling, 및 primary key order.

- [ ] **단계 2: Observe RED**

```bash
go test -count=1 ./batch/sqlcheckpoint -run 'Options|New|Schema|Interface'
```

예상: 패키지 build FAIL because the provider does 아님 exist.

- [ ] **단계 3: 추가 exact 공개 types 및 normalization**

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

Normalize empty namespace to exact bytes `default`; preserve every non-empty byte including NUL 및
invalid UTF-8; reject namespace over 128 bytes. Normalize zero limits to defaults 및 reject values
outside hard bounds.

- [ ] **단계 4: 추가 the fixed schema 및 immutable writer**

사용 exactly the approved table 및 named constraints:

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

`New` stores the 호출자 DB, normalized options, codec, callback, 및 default transaction factory;
it does 아님 ping, migrate, close, 또는 mutate the pool. 추가
`var _ batch.AtomicCheckpointWriter[any] = (*Writer[any, any])(nil)`.

- [ ] **단계 5: 검증 GREEN 및 commit**

```bash
gofmt -w batch/sqlcheckpoint/doc.go batch/sqlcheckpoint/options.go batch/sqlcheckpoint/options_test.go batch/sqlcheckpoint/schema.go batch/sqlcheckpoint/schema_test.go batch/sqlcheckpoint/writer.go
go test -count=1 ./batch/sqlcheckpoint -run 'Options|New|Schema|Interface'
git add batch/sqlcheckpoint
git commit -m "feat: define SQL checkpoint provider"
```

예상: constructor 및 schema 테스트 PASS without opening a DB connection.

### 작업 4: 구현 Typed Load 및 Redacted Diagnostics

**복잡도:** Medium data/오류 boundary.

**파일:**
- 생성: `batch/sqlcheckpoint/load.go`
- 생성: `batch/sqlcheckpoint/load_test.go`
- 생성: `batch/sqlcheckpoint/errors.go`
- 생성: `batch/sqlcheckpoint/errors_test.go`
- Modify: `batch/sqlcheckpoint/writer.go`

- [ ] **단계 1: Write RED validation, decode, 및 redaction 테스트**

Cover nil context normalization, pre-cancel 함께 zero DB dispatch, empty/oversized/exact-byte key,
missing row, valid row, revision zero/negative, payload at limit, oversized stored payload 함께 nil
projection, malformed decode, owned payload copy, nil/zero receiver, nested `errors.Is/As`, 및 hostile
namespace/key/payload/DSN/panic/원인 markers absent from `Error`, `%v`, 및 `%+v`.

- [ ] **단계 2: Observe RED**

```bash
go test -count=1 ./batch/sqlcheckpoint -run 'Load|CodecError|OpError|KeyID|Redact'
```

예상: FAIL because Load 및 typed 오류 do 아님 exist.

- [ ] **단계 3: 추가 the one-query conditional projection**

```go
const loadSQL = `select revision,
       pg_catalog.octet_length(payload),
       case when pg_catalog.octet_length(payload) <= $3 then payload end
from public.bluetape_batch_checkpoints
where namespace = $1::bytea and checkpoint_key = $2::bytea`
```

`Load` validates 전에 dispatch, scans revision/length/nullable payload, maps `sql.ErrNoRows` to
`exists=false`, rejects length over configured/hard limits without Decode, copies payload bytes,
decodes `C`, 및 returns `batch.VersionedCheckpoint{Value: value, Version: uint64(revision)}`.
사용 an unexported `rowScanner`/`queryRow` function initialized from `db.QueryRowContext`; 패키지
테스트 replace 만 that function 함께 deterministic scanners, so 없음 mocking dependency is added.

- [ ] **단계 4: 추가 redacted `OpError`, `CodecError`, 및 KeyID**

Both 오류 expose fixed operation/family strings 및 causal `Unwrap` without rendering the 원인.
Compute KeyID by hashing an 8-byte big-endian namespace length fol낮음ed by exact namespace/key bytes,
then hex-encoding the first 10 digest bytes. 사용 the prefix `sql-checkpoint-key:` 및 document that
it is a sensitive pseudonymous diagnostic, 아님 a metric label 또는 authorization identifier.

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

- [ ] **단계 5: 검증 GREEN 및 commit**

```bash
gofmt -w batch/sqlcheckpoint/load.go batch/sqlcheckpoint/load_test.go batch/sqlcheckpoint/errors.go batch/sqlcheckpoint/errors_test.go batch/sqlcheckpoint/writer.go
go test -count=1 ./batch/sqlcheckpoint -run 'Load|CodecError|OpError|KeyID|Redact'
git add batch/sqlcheckpoint
git commit -m "feat: load typed SQL checkpoints"
```

예상: 모든 load/오류 테스트 PASS 및 hostile markers remain absent from formatted 오류.

### 작업 5: 구현 Atomic 커밋 및 Transaction Ownership Proof

**복잡도:** Critical transaction state machine; 없음 automatic retries.

**파일:**
- 생성: `batch/sqlcheckpoint/session.go`
- 생성: `batch/sqlcheckpoint/queries.go`
- 생성: `batch/sqlcheckpoint/commit.go`
- 생성: `batch/sqlcheckpoint/commit_test.go`
- Modify: `batch/sqlcheckpoint/errors.go`
- Modify: `batch/sqlcheckpoint/writer.go`

- [ ] **단계 1: Write RED deterministic transaction 테스트**

사용 an injected `beginTx` returning a fake `transaction` that records Savepoint, callback SQL,
Release/롤백-To, checkpoint DML, 커밋, 및 롤백. Cover validation/encode 전에 Begin,
empty callback suppression, exact single callback, insert/update CAS, conflict rollback, callback
오류, swal낮음ed `25P02`, panic, context 전에 커밋, 커밋 server rejection, transport loss,
rollback 오류 preservation, revision maximum, 및 없음 library retry. 검증 every 오류 returns
revision zero.
The writer receives an unexported `beginTx func(context.Context) (transaction, error)` initialized
from its 호출자 DB; 패키지 테스트 replace 만 this function. The harness asserts exactly one Load
query, one SAVEPOINT/RELEASE pair 및 one CAS for non-empty 커밋, zero guard/callback for empty
커밋, 및 zero CAS/커밋 후 any ownership-probe failure.

- [ ] **단계 2: Observe RED**

```bash
go test -count=1 ./batch/sqlcheckpoint -run 'Commit|Ownership|AtomicityPanic|Conflict|Exhausted'
```

예상: FAIL because 커밋 및 the transaction adapter do 아님 exist.

- [ ] **단계 3: 추가 a testable transaction adapter 및 guarded session**

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

Initialize the production factory when the transaction code lands:

```go
w.beginTx = func(ctx context.Context) (transaction, error) {
    tx, err := w.db.BeginTx(ctx, nil)
    if err != nil { return nil, err }
    return &sqlTransaction{tx: tx}, nil
}
```

The unexported guarded session forwards 만 `ExecContext`, `QueryContext`, 및 `QueryRowContext`;
it never exposes concrete `*sql.Tx`, 커밋, 롤백, 또는 the reserved guard identifier.

- [ ] **단계 4: 추가 fixed CAS statements 및 preflight**

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

Reject `expectedVersion > math.MaxInt64` as invalid 및 `== math.MaxInt64` as
`ErrCheckpointVersionExhausted` 전에 codec/callback/DB. 검증 checkpoint type `C`, Encode, 및
payload limits 전에 Begin. Map `sql.ErrNoRows` from either CAS statement to
`ErrCheckpointConflict` 및 rollback.

- [ ] **단계 5: 구현 the ownership probe 및 panic boundary**

사용 the fixed identifier `bluetape_sqlcheckpoint_guard`. For non-empty items, dispatch SAVEPOINT,
call the callback through a recovery wrapper, then probe 함께
`context.WithTimeout(context.WithoutCancel(ctx), time.Second)`:

```go
const savepointSQL = `savepoint bluetape_sqlcheckpoint_guard`
const releaseSavepointSQL = `release savepoint bluetape_sqlcheckpoint_guard`
const rollbackToSavepointSQL = `rollback to savepoint bluetape_sqlcheckpoint_guard`
```

Release success proves ownership. On PostgreSQL `25P02`, require 롤백-To success to prove the
original guard; then full rollback 및 return the callback/probe 오류 또는 re-panic the original
value. `25P01`, `3B001`, active-context `sql.ErrTxDone`, timeout, canceled-context `sql.ErrTxDone`,
bad connection, transport, 및 unclassified probe failures must return an 오류 matching both
`ErrAtomicityUnknown` 및 `ErrCommitUnknown`; add `ErrCallbackContractViolation` 만 for positive
lifecycle evidence. No unproven path may execute CAS 또는 커밋.

생성 `AtomicityPanic` 함께 a fixed sanitized `Error`, an `Unwrap` matching both sentinels, 및 a
`PanicValue` accessor that alone returns the original sensitive value. Ownership-proven panic
cleanup re-panics the original value; ownership-unknown cleanup panics `*AtomicityPanic`.
정의 exported `ErrCallbackContractViolation = errors.New("sql checkpoint: callback contract violation")`
in `errors.go` 및 join it 만 on positive lifecycle evidence.

- [ ] **단계 6: Complete CAS, cancellation, 및 커밋 classification**

After callback/probe success, run one CAS statement. Recheck `ctx.Err()` 전에 커밋 및 rollback
known cancellation. Call 커밋 once. A pgx `*pgconn.PgError` is a known server rejection; a
transport/bad-connection/in-flight cancellation/non-server result is a sanitized `OpError` joined
함께 `ErrCommitUnknown`, never `ErrAtomicityUnknown`. 보존 rollback causes inside the typed
오류 without rendering raw provider text.

- [ ] **단계 7: 검증 GREEN, focused race, 및 commit**

```bash
gofmt -w batch/sqlcheckpoint/session.go batch/sqlcheckpoint/queries.go batch/sqlcheckpoint/commit.go batch/sqlcheckpoint/commit_test.go batch/sqlcheckpoint/errors.go batch/sqlcheckpoint/writer.go
go test -count=1 ./batch/sqlcheckpoint -run 'Commit|Ownership|AtomicityPanic|Conflict|Exhausted'
go test -race -count=1 ./batch ./batch/sqlcheckpoint
git add batch/sqlcheckpoint batch/errors.go
git commit -m "feat: commit SQL checkpoints atomically"
```

예상: deterministic 테스트 및 race detector PASS; operation logs prove 없음 retry 및 없음 CAS 후
an unproven ownership probe.

### 작업 6: 증명 PostgreSQL Restart, Concurrency, Security, 및 Failure Semantics

**복잡도:** High integration proof; run sequentially.

**파일:**
- 생성: `batch/sqlcheckpoint/integration_test.go`
- 생성: `batch/sqlcheckpoint/security_test.go`
- 생성: `batch/sqlcheckpoint/stress_test.go`

- [ ] **단계 1: 구성 one sequential PostgreSQL fixture**

사용 `testcontainers/postgres.Start`, blank-import pgx stdlib, 및 register cleanup 함께 `t.Cleanup`.
The 보안 fixture executes the production order exactly: revoke PUBLIC CREATE, create/use the
non-login migration owner, apply `SchemaSQL`, pass catalog preflight, prove runtime access fails
전에 grants, grant 만 USAGE+SELECT/INSERT/UPDATE, then prove al낮음ed DML succeeds 및 forbidden
DDL/DML fails. A simpler admin-owned fixture may be used 만 for non-보안 transaction 테스트.
사용 `t.Setenv` 만 for fixture configuration; do 아님 use `t.Parallel` anywhere in these files.

- [ ] **단계 2: 증명 success, restart, rollback, 및 exact CAS outcomes**

Test missing→revision 1, exact update→revision 2, business row plus checkpoint in one commit,
checkpoint statement failure rolling back business rows, callback failure rollback, success restart,
known rollback replay, NUL/invalid-UTF8 namespace/key isolation, checkpoint-만 commit, 및 pool
ownership. With two pools loading the same version, write different business idempotency keys 및
hold both callbacks at a barrier immediately 전에 checkpoint CAS; then assert exactly one
business-row/checkpoint winner 및 one `ErrCheckpointConflict` loser.

- [ ] **단계 3: 증명 raw transaction-control 및 panic failure shields**

사용 real callback SQL for raw COMMIT, raw ROLLBACK, raw COMMIT fol낮음ed by cancellation,
COMMIT→BEGIN→failing statement, raw COMMIT→panic, 및 normal panic. 검증 raw paths execute 없음
checkpoint DML/provider 커밋 및 return 또는 panic 함께 atomicity-unknown; normal panic preserves the
original value 및 leaves 없음 business/checkpoint row. 검증 `AtomicityPanic.Error`, `%v`, `%+v`, 및
unwrap strings omit a secret-bearing panic/DSN while `PanicValue()` alone preserves it.

- [ ] **단계 4: 증명 cancellation 및 small-pool release**

Barrier pre-Begin, callback, checkpoint DML, ownership probe, 및 provider 커밋 phases. 검증
pre-Begin has zero DB access; ownership-proven cancellation is known rollback; canceled-context
`sql.ErrTxDone` is atomicity-unknown; post-커밋-dispatch cancellation is commit-unknown fol낮음ed by
fresh Load. With `MaxOpenConns(1)`, require connection reuse within one second plus a fixed 500ms
scheduler tolerance, then poll until `DBStats.In사용 == 0`; assert 없음 late callback, CAS, 또는 커밋.

- [ ] **단계 5: 증명 catalog, privileges, 및 hostile drift detection**

Query `pg_class`, `pg_attribute` including `attacl`, `pg_constraint`, `pg_index`, `pg_policy`,
`pg_trigger`, `pg_rewrite`, `pg_namespace`, 및 role membership. 검증 permanent ordinary table,
exact columns/types/nullability/order, fixed validated constraint names, PK order, 없음 RLS/policies,
사용자 triggers/rules, 및 없음 PUBLIC/inherited/column privilege. 생성 a non-login migration owner 및
runtime role; prove runtime has 만 schema USAGE plus SELECT/INSERT/UPDATE 및 cannot CREATE,
DELETE, TRUNCATE, REFERENCES, TRIGGER, ALTER, 또는 grant. Mutate one catalog property per subtest 및
assert the 테스트 preflight rejects it.

- [ ] **단계 6: 실행 bounded stress 및 모든 provider 테스트 sequentially**

```bash
go test -count=1 ./batch/sqlcheckpoint -run 'TestPostgres'
go test -count=20 ./batch/sqlcheckpoint -run 'Concurrent|Conflict|Cancellation|Ownership'
go test -race -count=1 ./batch ./batch/sqlcheckpoint
```

예상: every command exits 0; exact winner/loser 및 operation counts remain stable.

- [ ] **단계 7: 커밋 PostgreSQL proof**

```bash
git add batch/sqlcheckpoint/integration_test.go batch/sqlcheckpoint/security_test.go batch/sqlcheckpoint/stress_test.go
git commit -m "test: prove SQL checkpoint recovery"
```

### 작업 7: 추가 Compile-Checked Examples 및 Bilingual Package Documentation

**복잡도:** Medium 호출자 계약 및 recovery documentation.

**Writing skill:** `bluetape-writer` for 영문/한국어 parity 및 validation.

**파일:**
- 생성: `batch/sqlcheckpoint/example_test.go`
- 생성: `batch/sqlcheckpoint/readme_test.go`
- 생성: `batch/sqlcheckpoint/README.md`
- 생성: `batch/sqlcheckpoint/README.ko.md`
- Modify: `batch/doc.go`
- Modify: `batch/README.md`
- Modify: `batch/README.ko.md`

- [ ] **단계 1: Write RED example 및 README 계약 테스트**

Require both locale files to contain `NewAtomicStep`, `SchemaSQL`, `ErrCommitUnknown`,
`ErrAtomicityUnknown`, `AtomicityPanic`, `PanicValue`, `SAVEPOINT`, authenticated codec/encryption,
KeyID non-authorization/non-metric guidance, callback restrictions, same-key serialization,
migration/privilege, recovery, 및 validation commands. Require Go doc 및 both locale files to say
that `RetryPolicy` 및 `SkipPolicy` apply 만 to processor failures 및 never to
`AtomicCheckpointWriter.Commit`, callback, CAS, 또는 unknown-outcome 오류. Require both locale files to
embed `postgres-batch-checkpoint-atomic-sequence.png` 및 have equal fenced-code-block counts.

- [ ] **단계 2: 추가 compile-checked construction 및 recovery example**

생성 example for 호출자-owned migration/runtime pools, JSON codec, tx-bound business insert,
`NewAtomicStep`, provider-owned commit-unknown fresh Load, 및 top-level panic recovery. The recovery
branch must inspect `errors.Is(recoveredErr, batch.ErrAtomicityUnknown)` 전에 generic restart,
quiesce same-key intake, 및 route to a named `reconcileCheckpoint` helper. Never log
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

- [ ] **단계 3: Write 영문 및 한국어 패키지 docs in parity**

문서화 architecture, additive constructor, schema bootstrap order, callback transaction rules,
payload/codec/encryption, key identity, conflict/exhaustion, commit unknown versus atomicity unknown,
panic supervisor handling, rollout/rollback, retention, pool ownership, unsupported topology, 및
exact 테스트 commands. 업데이트 root batch docs to state legacy Writer+CheckpointStore is durable but
아님 atomic 함께 business writes.

- [ ] **단계 4: 검증 example 및 parity, then commit**

```bash
go test -count=1 ./batch ./batch/sqlcheckpoint -run 'Example|README|Readme'
git diff --check
git add batch/doc.go batch/README.md batch/README.ko.md batch/sqlcheckpoint/example_test.go batch/sqlcheckpoint/readme_test.go batch/sqlcheckpoint/README.md batch/sqlcheckpoint/README.ko.md
git commit -m "docs: explain atomic SQL checkpoints"
```

예상: example compile 및 both locale 계약 테스트 PASS.

### 작업 8: 생성 the Required Sequence Diagram 및 Release Runbook

**복잡도:** Medium 공개 documentation asset 및 운영자 gate.

**Diagram skill:** `$bluetape-diagram`; 없음 Mermaid, ASCII, 또는 Graphviz 공개 artifact.

**파일:**
- 생성: `docs/images/readme-diagrams/postgres-batch-checkpoint-atomic-sequence.svg`
- 생성: `docs/images/readme-diagrams/postgres-batch-checkpoint-atomic-sequence.png`
- Modify: `README.md`
- Modify: `README.ko.md`
- Modify: `CHANGELOG.md`
- Modify: `docs/release/v0.19.0-provider-conformance-runbook.md`
- Modify: `batch/sqlcheckpoint/README.md`
- Modify: `batch/sqlcheckpoint/README.ko.md`

- [ ] **단계 1: 사용 `$bluetape-diagram` to create the SVG source**

Draw one 영문-label sequence 함께 participants `Step`, `CheckpointReader`, `Atomic Writer`, 및
`PostgreSQL`. Show Load/Restore, consumed-input boundary, SAVEPOINT, business callback, ownership
probe, checkpoint CAS, 커밋, known rollback, conflict rollback, provider-커밋 unknown→fresh
Load, 및 atomicity unknown→quiesce/manual reconciliation. 유지 labels readable at README width.

- [ ] **단계 2: Render 및 audit paired assets**

```bash
cairosvg docs/images/readme-diagrams/postgres-batch-checkpoint-atomic-sequence.svg -o docs/images/readme-diagrams/postgres-batch-checkpoint-atomic-sequence.png -s 2
xmllint --noout docs/images/readme-diagrams/postgres-batch-checkpoint-atomic-sequence.svg
file docs/images/readme-diagrams/postgres-batch-checkpoint-atomic-sequence.svg docs/images/readme-diagrams/postgres-batch-checkpoint-atomic-sequence.png
```

예상: valid SVG XML 및 a full-size PNG at 2x scale. Inspect the PNG at original resolution 및
fix clipping, overlap, illegible text, missing arrows, 또는 ambiguous failure branches.

- [ ] **단계 3: 업데이트 root inventory, changelog, 및 runbook**

추가 `batch/sqlcheckpoint` to both root 패키지 tables. 추가 the v0.19.0 feature entry. Extend the
runbook 함께 PUBLIC CREATE revoke→owner migration→catalog preflight→runtime grant, column ACL,
writable-primary/RPO, representative callback reconciliation drills, safe replay/repair approval,
quiesce release, autovacuum/replication/dead-tuple thresholds, shutdown order, canary, rollback, 및
retention. 정의 낮음-cardinality load/commit outcomes, conflict/exhaustion,
commit-unknown/atomicity-unknown, cancellation/latency, 및 `sql.DBStats` signals; raw KeyID values
must never become metric labels. 유지 영문 및 한국어 runbook sections structurally aligned.

- [ ] **단계 4: 검증 공개 documentation 및 commit**

```bash
rg -n 'batch/sqlcheckpoint|ErrAtomicityUnknown|postgres-batch-checkpoint-atomic-sequence' README.md README.ko.md CHANGELOG.md batch/sqlcheckpoint/README.md batch/sqlcheckpoint/README.ko.md docs/release/v0.19.0-provider-conformance-runbook.md
rg -n 'PUBLIC CREATE|catalog preflight|recovery drill|quiesce|shutdown|canary|rollback|retention|DBStats|metric label' docs/release/v0.19.0-provider-conformance-runbook.md
go test -count=1 ./batch/sqlcheckpoint -run 'README|Readme'
git diff --check
git add README.md README.ko.md CHANGELOG.md docs/release/v0.19.0-provider-conformance-runbook.md docs/images/readme-diagrams/postgres-batch-checkpoint-atomic-sequence.svg docs/images/readme-diagrams/postgres-batch-checkpoint-atomic-sequence.png batch/sqlcheckpoint/README.md batch/sqlcheckpoint/README.ko.md
git commit -m "docs: publish SQL checkpoint operations"
```

### 작업 9: 실행 Final 검증, 리뷰, 및 Lesson 캡처

**복잡도:** High release gate; 없음 feature expansion.

**검증 skill:** `verification-before-completion`; 단계 6-R uses six read-만 perspectives plus main integration.

**파일:**
- 생성: `docs/superpowers/reviews/2026-07-14-issue-532-sql-checkpoint-step-6r-code-review.md`
- 생성: `docs/lessons/2026-07-14-issue-532-sql-checkpoint.md`
- Modify 만 if findings require: files already listed in Tasks 1-8

- [ ] **단계 1: 실행 focused formatting, unit, race, 및 PostgreSQL gates**

```bash
gofmt -w batch/*.go batch/sqlcheckpoint/*.go
go test -vet=off ./batch/testdata/compat
go test -count=1 ./batch ./batch/sqlcheckpoint
go test -count=20 ./batch/sqlcheckpoint -run 'Concurrent|Conflict|Cancellation|Ownership'
go test -race -count=1 ./batch ./batch/sqlcheckpoint
git diff --check origin/develop...HEAD
```

예상: every command exits 0 함께 a fresh observed exit code.

- [ ] **단계 2: 실행 the authoritative local gate from scratch**

```bash
make ci
```

예상: fresh exit 0. If lint reports deleted-worktree cache paths, run
`golangci-lint cache clean && make lint`, then rerun `make ci` from scratch.

- [ ] **단계 3: 실행 단계 6-R in two bounded waves**

Wave 1: 성능, 안정성, 보안. Wave 2: 운영자/Ops, 개발자/API, 사용자/호출자. Every
lane reviews the same exact HEAD read-만 및 reports P0/P1 함께 file/line evidence. The main
session integrates results, fixes findings, reruns affected 및 exact-final lanes, 및 records
`P0=0 P1=0`. No seventh integration subagent.

- [ ] **단계 4: Re-run verification 후 review repairs**

```bash
go test -vet=off ./batch/testdata/compat
go test -count=1 ./batch ./batch/sqlcheckpoint
go test -race -count=1 ./batch ./batch/sqlcheckpoint
make ci
git status --short
git diff --check origin/develop...HEAD
```

예상: 모든 commands exit 0 및 the worktree is clean 후 committing review repairs.

- [ ] **단계 5: 캡처 reusable lessons 및 commit evidence**

The lesson must record consumed-input checkpoint boundaries, insert/update CAS split, transaction
ownership proof, `25P02` 롤백-To requirement, commit unknown versus atomicity unknown, panic
supervisor handling, unkeyed option compatibility, small-pool cleanup, 및 catalog/ACL preflight.
It must also record that benchmark/capacity comparison remains deferred to issue #560.

```bash
git add docs/superpowers/reviews/2026-07-14-issue-532-sql-checkpoint-step-6r-code-review.md docs/lessons/2026-07-14-issue-532-sql-checkpoint.md
git commit -m "docs: record SQL checkpoint verification"
```

- [ ] **단계 6: Hand off to PR creation 및 live CI**

준비 an 영문 PR that mirrors issue #532, links the final spec/plan/reviews, includes exact
테스트 evidence, 및 ends 함께 `## DoD Status`. Push 및 create the PR, wait for 모든 required checks,
address failures/review threads, then stop at `PENDING - PR ready for explicit merge decision`.
다음을 하지 않는다: merge without the 사용자's explicit approval.
