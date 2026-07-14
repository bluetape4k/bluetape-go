# Issue #532 PostgreSQL Durable Batch Checkpoint Design

## 배경

`batch.Step`의 기존 checkpoint 경로는 `Writer.Write` 성공 후
`CheckpointStore.Save`를 별도로 호출한다. 이 계약은 process-local store와 외부
store를 같은 좁은 interface로 사용할 수 있다는 장점이 있지만, business write가
commit된 뒤 checkpoint 저장 전에 process가 종료되면 다음 실행이 같은 chunk를 다시
처리한다. 반대로 writer failure를 skip하면서 checkpoint를 전진시키면 데이터가 누락될
수 있어 현재 구현은 `ErrUnsafeWriterSkipCheckpoint`로 이를 막는다.

Issue #532는 기존 경로의 호환성을 유지하면서 PostgreSQL을 사용하는 caller가 business
write와 checkpoint CAS를 하나의 transaction으로 commit할 수 있는 opt-in 경로를
추가한다. 이 기능은 durable workflow engine, scheduler, job repository가 아니다.

## 목표

- 기존 `Writer + CheckpointStore` API와 동작을 깨지 않는다.
- `batch.Step`에 opt-in atomic writer/checkpoint 계약을 추가한다.
- `batch/sqlcheckpoint`에서 caller의 business SQL callback과 checkpoint update를 같은
  PostgreSQL transaction으로 실행한다.
- revision CAS로 같은 checkpoint key의 stale writer를 fencing한다.
- cancellation, rollback, conflict, commit-unknown을 서로 다른 복구 경계로 노출한다.
- pool, schema migration, runtime role, retry 정책을 caller가 소유하게 한다.
- 영어/한국어 README와 compile-checked example, sequence diagram으로 운영 계약을
  설명한다.

## 비목표

- Redis, S3 또는 다른 SQL dialect provider를 추가하지 않는다.
- Kafka, Redis, S3, 외부 API write와 PostgreSQL checkpoint 사이의 원자성을 주장하지
  않는다.
- scheduler, DAG, partition coordinator, durable job/step metadata, execution history,
  distributed lock을 추가하지 않는다.
- schema를 자동 생성·검증·수정하거나 migration tool을 제공하지 않는다.
- transaction 또는 commit-unknown을 자동 retry하지 않는다.
- benchmark, throughput 목표 또는 capacity 순위를 만들지 않는다. 이 증거는 issue #560의
  provider benchmark matrix 범위다.
- 새 third-party dependency를 추가하지 않는다.

## 현재 근거

- `batch.CheckpointStore`는 `Load`와 `Save`를 제공하며
  `MemoryCheckpointStore`가 tests/local job 용도로 구현한다.
- `batch.Step.flush`는 legacy writer 성공 후 checkpoint를 저장하므로 두 동작 사이에
  crash window가 있다.
- filter 또는 processor skip 경로는 reader checkpoint를 즉시 저장한다. Writer chunk
  skip은 checkpoint가 활성화된 경우 unsafe error로 중단한다.
- `sqlkit`은 `Session`, `Beginner`, `WithTx`를 제공하지만 `WithTx`는 commit failure의
  known/unknown 결과를 분류하지 않는다. 이 provider는 더 강한 오류 계약이 필요하므로
  `WithTx`를 그대로 사용하지 않는다.
- `leader/sql`과 `ratelimit/sql`은 caller-owned `*sql.DB`, fixed `public` relation,
  exported `SchemaSQL`, caller-owned migration, redacted operation error, writable-primary
  계약을 사용한다.
- Go 공식 transaction 문서는 한 `sql.Tx` 안의 변경이 `Commit` 성공 시 하나의 atomic
  change로 적용되고, commit failure 시 transaction 결과를 폐기해야 한다고 명시한다:
  <https://go.dev/doc/database/execute-transactions>.
- PostgreSQL은 `INSERT ... ON CONFLICT DO UPDATE`가 high concurrency에서도 atomic
  insert-or-update outcome을 제공하고, `DO UPDATE ... WHERE` 조건이 false이면 row를
  반환하지 않는다고 명시한다:
  <https://www.postgresql.org/docs/current/sql-insert.html>.
- PostgreSQL Read Committed에서 concurrent `ON CONFLICT DO UPDATE`는 conflict row의
  최신 version에 update 조건을 다시 적용한다. 따라서 `revision = expected` 조건은
  stale writer를 한 statement에서 거부할 수 있다:
  <https://www.postgresql.org/docs/current/transaction-iso.html>.
- CodeGraph는 이 worktree에서 `0 nodes / 0 files`를 반환했다. 구조 근거는 direct source,
  GNO issue/docs, live GitHub issue #532/#504로 보완했다.

## 검토한 접근

### 접근 1: checkpoint만 SQL에 저장 (제외)

기존 `CheckpointStore`를 PostgreSQL로 구현하고 `Writer.Write` 뒤에 `Save`를 호출한다.

장점:

- core API 변경이 거의 없다.
- 다른 writer와 자유롭게 조합할 수 있다.

단점:

- business commit과 checkpoint commit 사이 crash window를 해결하지 못한다.
- issue가 요구하는 restart/retry 안정성을 durable store로 오해하게 만든다.

### 접근 2: opt-in atomic writer/checkpoint 계약 (채택)

`batch`가 database-agnostic `AtomicCheckpointWriter[T]` 계약을 제공하고,
`batch/sqlcheckpoint`가 caller callback과 checkpoint CAS를 같은 `sql.Tx`에서 실행한다.

장점:

- business rows와 checkpoint가 함께 commit 또는 rollback된다.
- 기존 API를 유지하면서 안전한 경로를 명시적으로 선택할 수 있다.
- core는 PostgreSQL과 codec을 알지 않는다.

단점:

- callback은 반드시 전달받은 `*sql.Tx`만 사용해야 한다.
- commit 결과가 불명확한 경우 caller가 reload/reconcile해야 한다.
- 기존 non-SQL writer와는 원자성을 제공하지 않는다.

### 접근 3: `Step`이 generic transaction/session을 직접 소유 (제외)

`batch.Step`이 `sqlkit.Session` 또는 public `DBTX`를 받아 writer와 checkpoint SQL을 직접
조립한다.

장점:

- 하나의 core API에서 여러 SQL 구현을 받을 수 있다.

단점:

- database concern이 core batch package로 침투한다.
- dialect, migration, callback ownership, commit classification이 넓은 public abstraction에
  섞인다.
- Go-native narrow package보다 JVM-style transaction template에 가까워진다.

## Core 공개 API

```go
package batch

var ErrCheckpointConflict = errors.New("batch: checkpoint revision conflict")
var ErrCommitUnknown = errors.New("batch: commit outcome unknown")

type VersionedCheckpoint struct {
    Value   any
    Version uint64
}

type AtomicCheckpointWriter[T any] interface {
    Load(context.Context, string) (VersionedCheckpoint, bool, error)
    Commit(
        context.Context,
        string,
        uint64,
        []T,
        any,
    ) (uint64, error)
}

type StepOptions[I any, O any] struct {
    // Existing fields remain unchanged.
    AtomicWriter AtomicCheckpointWriter[O]
}
```

`VersionedCheckpoint.Version`은 storage fencing version이며 business offset, row count,
timestamp 또는 reader progress 자체가 아니다. Missing checkpoint의 version은 zero다.
첫 commit은 expected version zero에서 version 1을 만든다. 이후 성공마다 정확히 1씩
증가한다. PostgreSQL `bigint` 범위까지만 허용하며 overflow 직전 상태는 conflict로
중단한다.

`AtomicCheckpointWriter`는 `Open`/`Close`를 제공하지 않는다. DB/pool, callback이 참조하는
repository/resource, migration lifecycle은 caller가 소유한다. `Step`은 기존 `Writer`
lifecycle만 계속 소유하고 atomic provider를 닫지 않는다.

### `StepOptions` validation

- `Writer`와 `AtomicWriter` 중 정확히 하나만 설정해야 한다.
- `AtomicWriter`가 설정되면 `CheckpointStore`를 함께 설정할 수 없다.
- Atomic 경로에서는 `CheckpointKey`가 empty이면 기존과 같이 step name을 사용한다.
- Atomic 경로의 `Reader`는 `CheckpointReader`를 구현해야 한다. `NewStep`이 이를 검증해
  reader open 또는 database access 전에 configuration error를 반환한다.
- Legacy `Writer + CheckpointStore` validation과 runtime behavior는 호환성을 위해 유지한다.
- Atomic provider의 nil/zero value는 implementation error를 반환해야 하며 panic하면 안 된다.

## `Step` atomic data flow

### 시작과 restore

1. `Step.Run`은 기존처럼 nil context를 `context.Background()`로 normalize한다.
2. Pre-canceled context는 reader/database side effect 없이 반환한다.
3. Reader를 연 뒤 atomic writer의 `Load(ctx, key)`를 호출한다.
4. Checkpoint가 있으면 `Reader.Restore`에 value를 전달하고 returned version을 현재 run의
   expected version으로 보관한다. 없으면 version zero로 시작한다.
5. 같은 `Step` instance의 concurrent `Run`은 지원하지 않는다. 같은 key의 여러 process/run
   역시 권장하지 않으며 CAS는 마지막 fencing guard다.

### chunk commit

1. Chunk를 쓰기 전에 `CheckpointReader.Checkpoint`를 호출한다.
2. Checkpoint가 없거나 capture가 실패하면 business callback을 호출하지 않고 실패한다.
3. `AtomicWriter.Commit(ctx, key, expectedVersion, chunk, checkpoint)`를 한 번 호출한다.
4. 성공하면 returned version을 다음 expected version으로 저장하고 `WriteCount`를 chunk
   length만큼 증가시킨다.
5. 알려진 callback/checkpoint/CAS failure이면 business rows와 checkpoint가 모두 rollback된
   상태로 실패한다.
6. `ErrCheckpointConflict`이면 stale run이므로 reload 없이 현재 run을 중단한다.
7. `ErrCommitUnknown`이면 `WriteCount`를 증가시키지 않고 현재 run을 중단한다. Caller는 새
   run에서 checkpoint를 reload한다. Commit됐다면 다음 checkpoint부터 재개하고,
   rollback됐다면 같은 chunk를 replay한다.

Atomic `Commit` error에는 `RetryPolicy` 또는 `SkipPolicy`를 적용하지 않는다. Callback failure,
conflict, context cancellation, transport failure를 자동 replay하면 duplicate business write
또는 stale checkpoint advance가 발생할 수 있기 때문이다. Existing processor retry와 legacy
writer retry/skip semantics는 변하지 않는다.

### filter와 processor skip

Filter 또는 processor skip으로 output이 없더라도 reader progress는 durable해야 한다.
Atomic 경로는 empty `[]O`와 새 checkpoint를 같은 `Commit`에 전달한다. SQL provider는
business callback을 empty chunk에도 호출하며, callback은 이를 no-op으로 처리할 수 있어야
한다. Checkpoint CAS는 정상적으로 version을 증가시킨다. `WriteCount`는 증가하지 않고 기존
`FilterCount`/`SkipCount`만 증가한다.

## PostgreSQL provider 공개 API

Package path는 `batch/sqlcheckpoint`, package name은 `sqlcheckpoint`다.

```go
package sqlcheckpoint

const (
    DefaultMaxKeyBytes     = 512
    MaxKeyBytes            = 1024
    DefaultMaxPayloadBytes = 1 << 20
    MaxPayloadBytes        = 16 << 20
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

type WriteTxFunc[T any] func(context.Context, *sql.Tx, []T) error

type Writer[T any, C any] struct { /* constructor-only */ }

func New[T any, C any](
    db *sql.DB,
    options Options,
    codec Codec[C],
    write WriteTxFunc[T],
) (*Writer[T, C], error)

func (w *Writer[T, C]) Load(
    ctx context.Context,
    key string,
) (batch.VersionedCheckpoint, bool, error)

func (w *Writer[T, C]) Commit(
    ctx context.Context,
    key string,
    expectedVersion uint64,
    items []T,
    checkpoint any,
) (uint64, error)

const SchemaSQL = `...`
```

`Writer[T, C]`는 compile-time으로 `batch.AtomicCheckpointWriter[T]`를 구현한다. `New`는
`db`를 ping하거나 `SchemaSQL`을 실행하지 않고 pool을 닫지 않는다. Writer zero value와 nil
receiver는 panic 없이 initialization error를 반환한다.

### Options와 identity

- Empty namespace는 `default` bytes가 된다. Non-empty namespace와 key는 trim, Unicode
  normalize, case-fold 또는 hash하지 않는다.
- Namespace는 최대 128 raw bytes다. Key는 non-empty이며 `MaxKeyBytes`와 provider hard
  ceiling 1024 raw bytes 이하이어야 한다.
- `MaxKeyBytes` zero는 512다. 1..1024 밖의 값은 constructor error다.
- `MaxPayloadBytes` zero는 1 MiB다. 1..16 MiB 밖의 값은 constructor error다.
- Identity는 exact `(namespace bytes, checkpoint_key bytes)`이며 PostgreSQL `bytea` bind를
  사용한다. NUL과 invalid UTF-8를 포함한 Go string bytes도 보존한다.
- Namespace/key는 authorization boundary가 아니다. Caller가 인증·인가 후 canonical key와
  cardinality를 통제한다.

### Typed codec

- `Codec[C]`의 두 function은 모두 필수다.
- `Commit`은 checkpoint가 `C`가 아니면 transaction 시작 전에 type error를 반환한다.
- Encode는 transaction 시작 전에 수행한다. Encode failure나 payload limit 초과 시 business
  callback/database access가 없어야 한다.
- Load는 payload를 owned byte slice로 scan하고 Decode한다. Decode failure는 checkpoint를
  반환하지 않으며 causal error를 `%w`로 보존한다.
- Provider는 gob, JSON, schema version 또는 compression을 선택하지 않는다. Caller가 stable
  codec과 payload migration을 소유한다.

## Schema

고정 relation은 `public.bluetape_batch_checkpoints`다.

```sql
create table if not exists public.bluetape_batch_checkpoints (
    namespace bytea not null,
    checkpoint_key bytea not null,
    revision bigint not null check (revision > 0),
    payload bytea not null,
    updated_at timestamptz not null,
    primary key (namespace, checkpoint_key)
)
```

별도 index, TTL 또는 cleanup column은 추가하지 않는다. Checkpoint retention은 caller의 job
lifecycle/migration 정책이다. `SchemaSQL`은 bootstrap contract이며 existing object가 올바른지
검증하는 migration engine이 아니다. Caller는 migration role로 schema를 적용하고 table
shape, PK order, check constraint, RLS/triggers, owner와 privilege를 검증해야 한다.

Runtime role에는 writable primary의 고정 table에 대한 `SELECT`, `INSERT`, `UPDATE`만
필요하다. `CREATE`, `ALTER`, `DELETE`, schema `CREATE`, function execute 권한은 필요하지
않는다. Callback이 쓰는 business table 권한은 caller 책임이다.

## SQL transaction과 CAS

`Commit`은 다음 순서만 허용한다.

1. context, writer initialization, key, expected version, checkpoint type을 검증한다.
2. checkpoint를 encode하고 payload limit을 검증한다.
3. `db.BeginTx(ctx, nil)`로 transaction을 시작한다.
4. caller `WriteTxFunc(ctx, tx, items)`를 정확히 한 번 호출한다.
5. 같은 `tx`에서 checkpoint UPSERT를 실행한다.
6. UPSERT가 new revision을 반환하면 `tx.Commit()`을 정확히 한 번 호출한다.
7. Commit 성공 후에만 revision을 반환한다.

첫 version과 update를 한 statement로 처리한다.

```sql
insert into public.bluetape_batch_checkpoints as checkpoint (
    namespace, checkpoint_key, revision, payload, updated_at
)
values ($1::bytea, $2::bytea, 1, $3::bytea, pg_catalog.clock_timestamp())
on conflict (namespace, checkpoint_key) do update set
    revision = checkpoint.revision + 1,
    payload = excluded.payload,
    updated_at = excluded.updated_at
where checkpoint.revision = $4::bigint
  and checkpoint.revision < 9223372036854775807::bigint
returning revision
```

- Missing row + expected zero는 revision 1을 insert한다.
- Existing row + exact expected revision은 revision을 1 증가시킨다.
- Existing row + stale/zero expected 또는 revision exhaustion은 no returned row이며
  `ErrCheckpointConflict`다.
- Conflict에서는 callback이 앞서 쓴 business rows까지 transaction rollback한다.
- Runtime values는 positional bind만 사용한다. Relation/column/SQL identifier interpolation은
  금지한다.
- Callback은 전달받은 `*sql.Tx`만 사용해야 한다. Captured `*sql.DB` 또는 별도 transaction을
  사용한 write는 atomic contract 밖이며 문서와 example에서 금지한다.
- Callback은 provider가 소유한 transaction을 commit/rollback하거나 `*sql.Tx`를 보관하면
  안 된다. Items slice는 callback duration 동안 read-only로 취급한다.

## Error contract

### Provider-neutral sentinels

- `errors.Is(err, batch.ErrCheckpointConflict)`는 expected revision이 현재 row와 맞지 않아
  business/checkpoint transaction이 rollback됐음을 뜻한다.
- `errors.Is(err, batch.ErrCommitUnknown)`는 commit request가 PostgreSQL에 반영됐는지 caller가
  확정할 수 없음을 뜻한다. 성공 revision과 write count는 반환하지 않는다.

### SQL provider operation error

`sqlcheckpoint.OpError`는 `errors.As`로 검사할 수 있고 causal error를 `Unwrap`한다. Error
string은 operation과 redacted `KeyID` family만 포함하고 raw namespace/key, payload, SQL,
DSN, endpoint 또는 provider cause text를 포함하지 않는다.

- Validation, checkpoint type/encode failure, pre-canceled context는 database operation
  error로 감싸지 않는다.
- Begin failure는 transaction이 시작되지 않았으므로 operation error이지만 commit-unknown이
  아니다.
- Callback 또는 checkpoint statement failure 뒤 provider는 rollback한다. PostgreSQL
  server error와 성공한 rollback은 known rollback이며 commit-unknown이 아니다.
- CAS no-row는 `ErrCheckpointConflict`이며 rollback된다.
- Commit의 PostgreSQL server rejection은 known failure로 분류한다.
- Commit의 transport loss, in-flight cancellation/deadline, bad connection 또는 결과를
  확정할 수 없는 non-server error는 `OpError`, original/context cause,
  `batch.ErrCommitUnknown`을 `errors.Join`한다.
- Rollback error는 원래 known failure와 join한다. Provider는 rollback failure를 숨기지
  않지만 commit을 호출하지 않은 path를 commit-unknown으로 승격하지 않는다.
- 모든 `Commit` error는 revision zero를 반환한다. Provider와 `Step`은 자동 retry하지 않는다.

Commit-unknown 뒤 caller의 유일한 자동화 가능한 안전 행동은 fresh run에서 `Load`하는
것이다. 저장된 checkpoint가 전진했다면 business write도 같은 transaction으로 commit된
것이고, 전진하지 않았다면 callback write도 rollback된 것이다. 다른 actor가 같은 key를
계속 진행할 수 있으므로 old expected version으로 `Commit`을 재호출하면 안 된다.

## Failure modes와 recovery

| Failure | Observable result | Durable state | Recovery |
|---|---|---|---|
| Encode/type validation failure | causal validation error | no DB access | fix codec/value |
| Begin failure | redacted operation error | no transaction | caller-controlled retry with fresh context |
| Business callback failure | causal operation error | business/checkpoint rollback | fix/retry whole run |
| Checkpoint statement/server failure | causal operation error | business/checkpoint rollback | repair DB/schema then rerun |
| Revision conflict | `ErrCheckpointConflict` | stale transaction rollback | stop stale run; start fresh load |
| Context canceled before dispatch | original context error | no late write | caller decides whether to restart |
| Context canceled or connection lost during commit | `ErrCommitUnknown` plus cause | committed or rolled back | fresh load; never blind replay |
| Decode failure on restart | causal decode error | stored row unchanged | deploy compatible codec/migration |
| Callback uses captured DB | outside atomic guarantee | independent write may survive | programming error; tests/examples forbid |
| Same key concurrent runs | one CAS winner, losers conflict | exactly one checkpoint revision wins | serialize runs; CAS is final guard |

## Test design

### Core `batch` tests

- `NewStep` accepts exactly one of legacy writer or atomic writer.
- Atomic writer rejects legacy `CheckpointStore` combination and non-checkpoint reader.
- Existing legacy tests remain unchanged and prove compatibility.
- Atomic restore passes stored value and tracks version.
- Successful chunk commit captures checkpoint first, calls `Commit` once, advances version, then
  increments `WriteCount`.
- Conflict, callback error, cancellation, commit-unknown return failed/cancelled report without
  retry, skip or write-count increment.
- Filter and processor skip call empty atomic commit and advance checkpoint/version without
  incrementing `WriteCount`.
- Missing checkpoint capture prevents atomic callback.
- Public usage has compile-checked examples.

### `batch/sqlcheckpoint` unit tests

- Options, nil/zero receiver, exact key bytes, key/payload bounds, checkpoint type, codec error and
  redacted error contracts.
- SQL statement shape has fixed identifiers, positional values, revision guard and overflow guard.
- Deterministic injected transaction harness proves callback/UPSERT/commit order, single invocation,
  rollback, no automatic retry and commit-unknown classification.
- `errors.Is`/`errors.As` survive wrapping and joined context causes.

### PostgreSQL Testcontainers tests

Run sequentially and verify connection readiness before assertions.

- Schema bootstrap with caller-owned migration and runtime DML role.
- Successful business row + checkpoint atomic commit.
- Callback succeeds but checkpoint CAS/server operation fails: both rollback.
- Two writers load the same version: exactly one commits and the loser returns conflict with no
  business row.
- Restart after success resumes after the committed chunk.
- Restart after known rollback replays the chunk.
- Injected commit-unknown path performs no library retry; a fresh load reconciles state.
- Pre-cancellation and in-flight cancellation preserve typed context errors and no late confirmed
  side effect.
- Namespace/key isolation including NUL and invalid UTF-8 bytes.
- Filter/skip empty chunk advances only checkpoint.
- Provider never closes or reconfigures the caller pool.
- Catalog/security assertions reject hostile relation shape, RLS, user triggers, unsafe owner or
  excessive runtime privilege assumptions where applicable.

Concurrency claims require bounded exact-outcome stress plus
`go test -race -count=1 ./batch ./batch/sqlcheckpoint`. Testcontainers suites are not run in
parallel with other Docker-backed suites.

## 문서와 diagram

- `batch/README.md`와 `batch/README.ko.md`는 legacy checkpoint path가 durable store를
  사용해도 business write와 원자적이지 않음을 명시한다.
- `batch/sqlcheckpoint/README.md`와 `README.ko.md`는 architecture, API, schema/migration,
  runtime privileges, failure/recovery, callback rules, validation commands를 동등하게 제공한다.
- Public Go doc과 example은 `WriteTxFunc`가 반드시 받은 `*sql.Tx`를 사용하는 모습을
  compile-check한다.
- 두 locale은 같은 English-label asset을 공유한다:
  `docs/images/readme-diagrams/postgres-batch-checkpoint-atomic-sequence.svg`와 `.png`.
- Diagram은 `Step`, `CheckpointReader`, `Atomic Writer`, `PostgreSQL` participant와 happy
  path, write failure rollback, revision conflict rollback, commit-unknown/reload를 보여준다.
- `$bluetape-diagram`으로 SVG/PNG를 생성하고 CairoSVG scale 2, XML/audit, paired asset,
  full-size PNG visual inspection을 통과한다. Mermaid/ASCII/Graphviz output은 public 최종
  asset으로 사용하지 않는다.

## Compatibility와 rollout

- Existing `CheckpointStore`, `MemoryCheckpointStore`, `Writer`, `StepOptions.Writer` behavior는
  유지한다. `AtomicWriter`를 사용하지 않는 call site에는 source/behavior change가 없어야
  한다.
- Legacy 경로는 deprecated하지 않는다. 서로 다른 storage에 write해야 하거나 at-least-once
  replay를 수용하는 caller에게 계속 유효하다.
- Atomic 경로는 새 opt-in API이므로 migration은 caller가 writer callback, typed codec,
  schema migration을 준비한 뒤 step option을 전환하는 방식이다.
- Rollback은 `AtomicWriter` option 사용을 중단하고 legacy writer/store로 돌아가는 것이다.
  이미 생성된 checkpoint table과 rows는 caller migration/retention 정책으로 보존하거나
  제거한다.
- Provider는 PostgreSQL writable primary만 지원한다. Read replica, transaction-pooling으로
  transaction affinity를 깨는 proxy, callback이 별도 connection을 쓰는 topology는 지원하지
  않는다.
- Source parity 분류는 `adapt`다. Durable checkpoint 개념은 유지하지만 Spring Batch job
  repository/scheduler shape는 Go package로 이식하지 않는다. Redis/S3는 `defer`, workflow
  engine은 `non-goal`이다.

## Acceptance criteria

1. Existing legacy batch tests와 API가 통과한다.
2. Opt-in atomic path가 business write와 checkpoint를 같은 PostgreSQL transaction으로
   commit한다.
3. CAS conflict가 stale business write와 checkpoint를 함께 rollback한다.
4. Commit-unknown은 typed sentinel/cause로 노출되고 library가 자동 retry하지 않는다.
5. Fresh load가 success/rollback/unknown 뒤 restart position을 안전하게 결정한다.
6. Filter/skip은 empty output transaction으로 checkpoint만 안전하게 전진시킨다.
7. Caller-owned pool/migration/codec/retry/retention과 최소 runtime privilege가 문서화된다.
8. English/Korean README, Go doc/example, paired sequence diagram이 source와 일치한다.
9. Targeted tests, race/stress, sequential PostgreSQL integration, `git diff --check`,
   `make ci`가 fresh exit 0이다.
10. Step 2-R, Step 3-R, Step 6-R, Step 7-R의 six perspectives + main integration이 모두
    latest P0=0/P1=0으로 수렴한다.

## Definition of Done

- Required spec/plan/review/lesson artifacts가 feature branch에 commit돼 있다.
- Public API, schema, error/recovery contract와 tests/docs/diagram이 acceptance criteria에
  trace된다.
- No new dependency, workflow, benchmark, Redis/S3, scheduler/DAG change가 diff에 없다.
- Issue #532 metadata를 mirror한 English PR이 생성되고 body의 마지막 H2가
  `## DoD Status`다.
- Local/full CI와 live PR required checks가 성공하고 review thread가 해결돼 있다.
- 최종 상태는 merge 전 `PENDING - PR ready for explicit merge decision`이며 자동 merge하지
  않는다.
