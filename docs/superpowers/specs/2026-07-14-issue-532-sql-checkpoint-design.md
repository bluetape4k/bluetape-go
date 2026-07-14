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
- PostgreSQL `pg_class.relpersistence`는 permanent/unlogged/temporary relation을 구분하고,
  `pg_rewrite`는 table/view rewrite rule을 저장한다. Durable provider preflight는 permanent
  table과 zero user rules를 요구한다:
  <https://www.postgresql.org/docs/current/catalog-pg-class.html>,
  <https://www.postgresql.org/docs/current/catalog-pg-rewrite.html>.
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

- callback은 반드시 전달받은 tx-bound `sqlkit.Session`만 사용해야 한다.
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
var ErrCheckpointVersionExhausted = errors.New("batch: checkpoint version exhausted")

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
증가한다. PostgreSQL `bigint` 범위까지만 허용한다. Maximum revision은 stale conflict와
구분되는 `ErrCheckpointVersionExhausted`로 중단하며 reload/retry로 복구되지 않는다.

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
5. 같은 `Step` instance의 concurrent `Run`은 지원하지 않는다. Caller는 `(namespace,
   key)`마다 Load부터 run 종료와 commit-unknown reconciliation까지 정확히 하나의 active
   run만 보장해야 한다. CAS는 accidental overlap을 막는 마지막 fencing guard이지
   scheduler/lock 대체재가 아니다.

### input-progress chunk commit

Atomic 경로는 output 개수만 세지 않고 마지막 successful commit 이후 소비한 input 개수를
`progressCount`로 센다. Kept output은 pending output chunk에 보관하고 filter/processor skip도
`progressCount`에는 포함한다. `progressCount > 0`이고 `progressCount == ChunkSize` 또는
EOF가 되면 pending output(비어 있을 수 있음)과 최신 reader checkpoint를 함께 commit한다.
Empty input과 exact-multiple boundary 뒤 EOF는 추가 commit/revision을 만들지 않는다. 이 규칙은 all-filter
workload의 transaction 수를 대략 `ceil(consumed inputs / ChunkSize)`로 제한하고,
checkpoint가 아직 쓰지 않은 pending output을 건너뛰는 것을 막는다.

1. Commit boundary에서 `CheckpointReader.Checkpoint`를 호출한다.
2. Checkpoint가 없거나 capture가 실패하면 business callback을 호출하지 않고 실패한다.
3. `AtomicWriter.Commit(ctx, key, expectedVersion, pending, checkpoint)`를 한 번 호출한다.
4. 성공하면 returned version을 다음 expected version으로 저장하고 `WriteCount`를 pending
   length만큼 증가시킨 뒤에만 pending chunk와 `progressCount`를 clear한다.
5. 알려진 callback/checkpoint/CAS failure이면 business rows와 checkpoint가 모두 rollback된
   상태로 실패한다.
6. `ErrCheckpointConflict`이면 stale run이므로 reload 없이 현재 run을 중단한다.
7. `ErrCheckpointVersionExhausted`이면 operator migration 전까지 permanent failure다.
8. `ErrCommitUnknown`이면 `WriteCount`를 증가시키거나 buffer를 clear하지 않고 현재 run을
   중단한다. Caller는 same-key actor를 계속 quiesce한 상태에서 fresh run으로 checkpoint를
   reload한다. Load는 authoritative resume position을 제공하지만 원래 ambiguous attempt를
   별도로 attribution하지는 않는다.

Atomic `Commit` error에는 `RetryPolicy` 또는 `SkipPolicy`를 적용하지 않는다. Callback failure,
conflict, context cancellation, transport failure를 자동 replay하면 duplicate business write
또는 stale checkpoint advance가 발생할 수 있기 때문이다. Existing processor retry와 legacy
writer retry/skip semantics는 변하지 않는다.

### filter와 processor skip

Filter 또는 processor skip으로 output이 없더라도 reader progress는 bounded interval로
durable해야 한다. Atomic 경로는 해당 item을 progress chunk에 포함하되 즉시 checkpoint를
전진시키지 않는다. Boundary에서 이전 kept output이 있으면 그 output과 최신 checkpoint를
같이 commit하고, pending output이 정말 비어 있을 때만 checkpoint-only transaction을
실행한다. SQL provider는 empty items에서 callback을 호출하지 않는다. `WriteCount`는 실제
committed output만 반영하고 기존 `FilterCount`/`SkipCount`는 유지한다. Crash 시 마지막
successful boundary 뒤의 filter/skip/processor work는 최대 `ChunkSize-1` inputs까지 replay될
수 있으며 이것이 atomic 경로의 명시적 at-least-once processing 경계다.

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

var ErrCallbackContractViolation = errors.New("sql checkpoint: callback contract violation")

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
nil `db`, nil write callback, nil encode/decode function을 거절한다. `db`를 ping하거나
`SchemaSQL`을 실행하지 않고 pool을 닫지 않는다. Writer zero value와 nil receiver는 panic
없이 initialization error를 반환한다. Direct `Load(nil, ...)`와 `Commit(nil, ...)`는 legacy
batch convention처럼 `context.Background()`로 normalize한다. Pre-canceled context는 callback
또는 DB dispatch 전에 원래 context error로 반환한다. `expectedVersion > math.MaxInt64`는
invalid argument이고, `expectedVersion == math.MaxInt64`는
`batch.ErrCheckpointVersionExhausted`다. 둘 다 callback/transaction 전에 반환한다.

한 constructed writer는 immutable configuration만 가지므로 여러 step/key에서 공유할 수
있다. 이때 caller의 codec과 callback도 concurrent-safe해야 한다. 같은 `(namespace, key)`의
active run은 앞서 정의한 exclusive coordination 계약을 따라야 하며 provider가 내부 mutex로
serialize하지 않는다.

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
- Load는 한 SELECT에서 revision, payload length와 configured limit 이하일 때만 payload를
  projection한다. Limit을 넘은 row는 payload bytes를 Go process로 반환하거나 Decode하지
  않고 safe oversized error로 실패한다. 정상 payload는 owned byte slice로 복사한 뒤
  Decode한다.
- Encode/decode failure는 sanitized `CodecError` string을 반환하면서 `Unwrap`으로 causal
  inspection을 보존한다. Returned error string에는 codec cause 또는 payload text를 포함하지
  않는다.
- Provider는 gob, JSON, schema version 또는 compression을 선택하지 않는다. Caller가 stable
  codec과 payload migration을 소유한다. Stored payload는 trusted input으로 가정하지 않으며
  codec은 malformed/untrusted bytes를 fail closed해야 한다. Typed nil/zero `C`의 허용 여부는
  caller codec이 결정하지만 untyped wrong checkpoint type은 DB access 전에 거절한다.

## Schema

고정 relation은 `public.bluetape_batch_checkpoints`다.

```sql
create table if not exists public.bluetape_batch_checkpoints (
    namespace bytea not null
        constraint bluetape_batch_checkpoints_namespace_size_check
        check (pg_catalog.octet_length(namespace) between 1 and 128),
    checkpoint_key bytea not null
        constraint bluetape_batch_checkpoints_key_size_check
        check (pg_catalog.octet_length(checkpoint_key) between 1 and 1024),
    revision bigint not null
        constraint bluetape_batch_checkpoints_revision_check check (revision > 0),
    payload bytea not null
        constraint bluetape_batch_checkpoints_payload_size_check
        check (pg_catalog.octet_length(payload) <= 16777216),
    updated_at timestamptz not null,
    constraint bluetape_batch_checkpoints_pkey primary key (namespace, checkpoint_key)
)
```

별도 index, TTL 또는 cleanup column은 추가하지 않는다. Checkpoint retention은 caller의 job
lifecycle/migration 정책이다. `SchemaSQL`은 bootstrap contract이며 existing object가 올바른지
검증하는 migration engine이 아니다. Caller는 migration role로 schema를 적용하고 table
shape, PK order, check constraint, RLS/triggers, owner와 privilege를 검증해야 한다.

Production topology는 별도 non-login migration owner와 runtime role을 사용한다. Migration
owner가 bounded lock/statement timeout 아래 schema를 소유·적용하고, `PUBLIC`의 schema
`CREATE`를 revoke한 뒤 runtime role에 `USAGE ON SCHEMA public`과 고정 table의 `SELECT`,
`INSERT`, `UPDATE`만 grant한다. Runtime은 schema/table owner 또는 migration-owner member가
아니며 `CREATE`, `ALTER`, `DROP`, `DELETE`, `TRUNCATE`, `REFERENCES`, `TRIGGER`, grant option,
inherited/PUBLIC write privilege를 갖지 않는다. Callback이 쓰는 business table 권한은 caller
책임이다.

Deployment preflight는 ordinary table relkind, permanent logged persistence
(`pg_class.relpersistence = 'p'`), expected non-login owner, exact column order/type/nullability,
fixed PK/check constraint names/order/validated definitions, RLS/forced-RLS off, zero policies,
zero user triggers, zero user `pg_rewrite` rules, schema owner/ACL과 runtime
direct/inherited/PUBLIC privilege를 catalog로 검증한다. `IF NOT EXISTS`는 이 검증을 대체하지
않는다. UNLOGGED/TEMP table과 INSERT/UPDATE rewrite rule은 durable CAS contract 밖이라
fail closed한다. Active checkpoint row를 삭제하거나 key를 재사용하는 retention은 해당 key의
모든 run과 unknown reconciliation을 quiesce/join한 maintenance window에서만 허용한다.

## SQL transaction과 CAS

`Commit`은 다음 순서만 허용한다.

1. context, writer initialization, key, expected version, checkpoint type을 검증한다.
   Expected version maximum은 DB state와 무관하게 더 증가시킬 수 없으므로
   `ErrCheckpointVersionExhausted`로 즉시 실패한다.
2. checkpoint를 encode하고 payload limit을 검증한다.
3. `db.BeginTx(ctx, nil)`로 transaction을 시작한다.
4. 즉시 rollback guard를 설치한다. Callback panic은 rollback을 시도한 뒤 그대로 propagate한다.
5. Items가 non-empty이면 caller input과 무관한 compile-time fixed reserved identifier로
   transaction savepoint guard를 만든 뒤 caller `WriteTxFunc(ctx, guardedSession, items)`를
   정확히 한 번 호출한다. Empty이면 callback과 savepoint guard를 모두 생략한다.
6. Callback 반환 뒤 checkpoint DML 전에 provider가 guard를 release한다. Guard release가
   실패하면 어떤 분류에서도 checkpoint DML이나 Commit을 실행하지 않는다. Raw transaction
   control의 positive evidence가 있으면 contract-violation/commit-unknown, cancellation 또는
   transport와 경합해 outcome을 증명할 수 없으면 commit-unknown으로 fail closed한다.
7. 같은 concrete `*sql.Tx`를 감싼 non-owning `sqlkit.Session`에서 checkpoint CAS DML 하나를
   실행한다.
8. CAS 성공 뒤 `ctx.Err()`를 commit dispatch 전에 다시 확인한다. 이미 취소됐다면 commit을
   호출하지 않고 rollback해 known cancellation로 반환한다.
9. `tx.Commit()`을 정확히 한 번 호출하고 성공 후에만 revision을 반환한다.

Create와 update는 expected version에 따라 서로 다른 single DML을 사용한다. Missing row는
expected zero일 때만 생성한다.

```sql
-- expectedVersion == 0
insert into public.bluetape_batch_checkpoints (
    namespace, checkpoint_key, revision, payload, updated_at
)
values ($1::bytea, $2::bytea, 1, $3::bytea, pg_catalog.clock_timestamp())
on conflict (namespace, checkpoint_key) do nothing
returning revision;

-- expectedVersion > 0
update public.bluetape_batch_checkpoints set
    revision = revision + 1,
    payload = $3::bytea,
    updated_at = pg_catalog.clock_timestamp()
where namespace = $1::bytea
  and checkpoint_key = $2::bytea
  and revision = $4::bigint
returning revision
```

- Missing row + expected zero는 revision 1을 insert한다.
- Existing row + exact expected revision은 revision을 1 증가시킨다.
- Missing row + nonzero expected, existing row + zero/stale expected는 no returned row이며
  `ErrCheckpointConflict`다.
- Expected version maximum은 update dispatch 전에 `ErrCheckpointVersionExhausted`다. 따라서
  no-row 결과는 conflict만 뜻하며 추가 classification SELECT가 필요 없다.
- Conflict에서는 callback이 앞서 쓴 business rows까지 transaction rollback한다.
- Runtime values는 positional bind만 사용한다. Relation/column/SQL identifier interpolation은
  금지한다.
- Callback은 trusted in-process code이며 전달받은 tx-bound `sqlkit.Session`만 사용해야 한다.
  Captured `*sql.DB`, 별도 transaction, network/external side effect, goroutine escape는 atomic
  contract 밖이며 문서와 example에서 금지한다. Session에는 Commit/Rollback이 노출되지 않는다.
- Callback은 checkpoint relation에 직접 접근하거나 role/search_path/session security state를
  변경하면 안 된다. Items slice는 callback duration 동안 read-only로 취급한다.
- Callback은 raw/procedure 기반 `BEGIN`, `COMMIT`, `ROLLBACK`, `SAVEPOINT`,
  `SET TRANSACTION` 또는 equivalent transaction-control을 실행하거나 session/items를 call
  이후 보관하면 안 된다. Guard name은 security secret이 아니라 package-reserved fixed
  identifier이고 concrete `*sql.Tx`는 callback에 노출하지 않는다. Trusted callback이 reserved
  guard에 직접 접근하는 adversarial behavior는 지원하지 않지만 accidental raw transaction
  control은 guard로 fail closed한다.
- Caller는 bounded run/commit context와 chunk size를 사용하고 role/database 수준의
  `lock_timeout`, `statement_timeout`, `idle_in_transaction_session_timeout`을 설정한다.
- Hot path budget은 `Load` 한 SELECT다. Non-empty `Commit`은 BeginTx + private guard
  SAVEPOINT/RELEASE + caller SQL + checkpoint DML 하나 + Commit이고, empty `Commit`은 guard와
  callback을 생략한다. Provider는 ping, catalog query, preflight SELECT 또는 retry를 hot
  path에 추가하지 않는다. Guard overhead는 transaction ownership을 fail closed하기 위해
  의도적으로 수용하며 benchmark/capacity 평가는 issue #560에서 수행한다.

## Error contract

### Provider-neutral sentinels

- `errors.Is(err, batch.ErrCheckpointConflict)`는 expected revision이 현재 row와 맞지 않아
  business/checkpoint transaction이 rollback됐음을 뜻한다.
- `errors.Is(err, batch.ErrCommitUnknown)`는 commit request가 PostgreSQL에 반영됐는지 caller가
  확정할 수 없음을 뜻한다. 성공 revision과 write count는 반환하지 않는다.
- `errors.Is(err, batch.ErrCheckpointVersionExhausted)`는 current row가 PostgreSQL bigint
  maximum에 도달한 permanent failure다. Stale conflict처럼 blind reload/retry하지 않는다.

### SQL provider operation error

`sqlcheckpoint.OpError`와 `CodecError`는 `errors.As`로 검사할 수 있고 causal error를
`Unwrap`한다. Error string은 operation과 redacted family만 포함하고 raw namespace/key,
payload, SQL, DSN, endpoint, codec/provider cause text를 포함하지 않는다. `KeyID`는
namespace byte length를 8-byte big-endian으로 prefix한 뒤 exact namespace/key bytes를
SHA-256하고 앞 10 bytes를 hex encode한다. 이는 pseudonymous correlation ID일 뿐 secret,
authorization identifier 또는 enumeration 방어가 아니다. 외부 trust boundary와 metric label에는
KeyID를 노출하지 않는다.

- Validation, checkpoint type/encode failure, pre-canceled context는 database operation
  error로 감싸지 않는다.
- Begin failure는 transaction이 시작되지 않았으므로 operation error이지만 commit-unknown이
  아니다.
- Callback 또는 checkpoint statement failure 뒤 provider는 rollback한다. PostgreSQL
  server error와 성공한 rollback은 known rollback이며 commit-unknown이 아니다.
- CAS no-row는 `ErrCheckpointConflict`이며 rollback된다.
- Guard release는 caller context가 이미 취소됐더라도 short bounded internal cleanup context로
  시도하되 business/checkpoint work를 진행시키는 용도로 사용하지 않는다. Release 성공 뒤
  callback error 또는 caller cancellation이 있으면 rollback하고 known failure/cancellation로
  반환한다.
- Guard release가 `SQLSTATE 25P02`처럼 active failed transaction을 확인하면 rollback한다.
  Rollback 성공은 known failure이고 rollback 실패는 기존 rollback-error 계약을 따른다.
- Active caller context의 `sql.ErrTxDone`, `SQLSTATE 25P01`(no active transaction) 또는
  `SQLSTATE 3B001`(invalid savepoint)처럼 lifecycle violation의 positive evidence가 있으면
  sanitized operation error, `sqlcheckpoint.ErrCallbackContractViolation`,
  `batch.ErrCommitUnknown`에 match한다.
- Guard release가 cancellation/deadline, canceled-context `sql.ErrTxDone`, transport loss,
  bad connection 또는 분류 불가능한 non-server error로 실패하면 raw Commit과 automatic
  rollback의 경합을 증명할 수 없으므로 `ErrCallbackContractViolation`을 추측하지 않고
  sanitized operation error와 `batch.ErrCommitUnknown`에만 match한다. 어떤 release failure도
  checkpoint DML 또는 provider-owned Commit으로 이어지지 않는다.
- Commit의 PostgreSQL server rejection은 known failure로 분류한다.
- Commit의 transport loss, in-flight cancellation/deadline, bad connection 또는 결과를
  확정할 수 없는 non-server error는 original/context cause를 `OpError.Unwrap` 안에만 보관하고
  sanitized `OpError`와 `batch.ErrCommitUnknown`만 `errors.Join`한다.
- Rollback error는 원래 known failure와 함께 sanitized `OpError.Unwrap` 내부에 보존한다.
  Provider는 rollback failure를 숨기지 않지만 raw cause를 outer error string에 join하거나
  commit을 호출하지 않은 path를 commit-unknown으로 승격하지 않는다.
- 모든 `Commit` error는 revision zero를 반환한다. Provider와 `Step`은 자동 retry하지 않는다.
- `Step.statusForError`는 `batch.ErrCommitUnknown`을 context cancellation보다 먼저 검사해
  `StatusFailed`로 분류한다. Known pre-dispatch/callback cancellation만 `StatusCancelled`다.

Commit-unknown 뒤 caller의 유일한 자동화 가능한 안전 행동은 same-key exclusivity를 유지한
fresh run에서 `Load`하는 것이다. 다른 actor가 개입하지 않았다면 checkpoint의 전진 여부와
business transaction outcome이 일치한다. Exclusivity를 위반해 다른 actor가 진행했다면 Load는
현재 authoritative resume position만 제공하며 원래 ambiguous attempt의 attribution 증거가
아니다. 어떤 경우에도 old expected version으로 `Commit`을 재호출하면 안 된다.

## Failure modes와 recovery

| Failure | Observable result | Durable state | Recovery |
|---|---|---|---|
| Encode/type validation failure | causal validation error | no DB access | fix codec/value |
| Begin failure | redacted operation error | no transaction | caller-controlled retry with fresh context |
| Business callback failure | causal operation error | business/checkpoint rollback | fix/retry whole run |
| Checkpoint statement/server failure | causal operation error | business/checkpoint rollback | repair DB/schema then rerun |
| Revision conflict/missing after prior Load | `ErrCheckpointConflict` | stale transaction rollback | stop stale run; quiesce and start fresh load |
| Revision exhausted | `ErrCheckpointVersionExhausted` | row unchanged | quiesce, reconcile business state, migrate to a new key/namespace |
| Context canceled before dispatch | original context error | no late write | caller decides whether to restart |
| Context canceled or connection lost during commit | `ErrCommitUnknown` plus cause | committed or rolled back | fresh load; never blind replay |
| Decode failure on restart | causal decode error | stored row unchanged | deploy compatible codec/migration |
| Callback uses captured DB | outside atomic guarantee | independent write may survive | programming error; tests/examples forbid |
| Same key concurrent runs | one CAS winner, losers conflict | exactly one checkpoint revision wins | mandatory external serialization; CAS is final guard |

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
- Joined cancellation cause가 있는 commit-unknown은 `StatusFailed`, known cancellation은
  `StatusCancelled`로 구분된다.
- Filter and processor skip coalesce by consumed-input chunk, commit pending kept output plus the
  latest checkpoint, and use checkpoint-only commit only when pending output is empty.
- Kept→filter and kept→processor-skip crash/restart cases never checkpoint past buffered output.
- All-filter, all-skip and mixed streams bound commit count by consumed-input chunk boundaries.
- Empty input과 exact-multiple input은 EOF에서 redundant Commit/revision을 만들지 않는다.
- Missing checkpoint capture prevents atomic callback.
- Public usage has compile-checked examples.

### `batch/sqlcheckpoint` unit tests

- Options, nil DB/callback/codec, nil/zero receiver, nil/pre-canceled context, exact key bytes,
  key/payload bounds, expected version range, checkpoint type, codec error and redacted error
  contracts.
- SQL statement shape has fixed identifiers, positional values, create/update revision guards and
  overflow guard. Missing+zero, missing+nonzero, existing+exact/stale and delete-after-Load cases
  prove no stale resurrection.
- Deterministic injected transaction harness proves callback/CAS-DML/commit order, single invocation,
  empty callback suppression, panic rollback, connection release, no automatic retry and
  commit-unknown classification.
- Expected maximum version은 DB/callback 없이 exhaustion error를 반환한다. Actual PostgreSQL
  raw COMMIT/ROLLBACK callback은 private guard release에서 checkpoint DML 전에 감지되어
  contract-violation + commit-unknown으로 fail closed한다. Raw COMMIT 직후 cancellation과
  release를 barrier로 경합시킨 경우는 contract violation을 추측하지 않더라도 commit-unknown이며
  checkpoint DML이 없어야 한다. 정상 callback cancellation은 release 성공 또는 active failed
  transaction + successful rollback 증거가 있을 때 known cancellation이다.
- `errors.Is`/`errors.As` survive wrapping and joined context causes.
- Hot-path harness proves one Load SELECT, non-empty Commit의 exactly one private
  SAVEPOINT/RELEASE pair와 one checkpoint DML, empty Commit의 zero guard/callback, 그리고 ping,
  catalog preflight/retry 부재를 검증한다.

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
- Barriered cancellation proves pre-Begin has no DB access, callback/checkpoint cancellation is known
  rollback with no late rows, and post-commit-dispatch cancellation is commit-unknown followed by
  fresh-load reconciliation.
- Namespace/key isolation including NUL and invalid UTF-8 bytes.
- All-filter/skip boundary with no pending output advances only checkpoint; mixed boundary commits
  pending output and checkpoint together.
- Provider never closes or reconfigures the caller pool.
- Catalog/security assertions reject oversized hostile rows, relation/column/constraint/PK drift,
  unlogged/temp persistence, rewrite rules, RLS/policies/user triggers, unsafe
  owner/member/PUBLIC ACL or excessive runtime privileges.
- Small-pool cancellation proves bounded transaction/connection release. Unknown+competing-actor
  harness proves Load cannot attribute the original attempt when exclusivity is violated.

Concurrency claims require bounded exact-outcome stress plus
`go test -race -count=1 ./batch ./batch/sqlcheckpoint`. Testcontainers suites are not run in
parallel with other Docker-backed suites.

## 문서와 diagram

- `batch/README.md`와 `batch/README.ko.md`는 legacy checkpoint path가 durable store를
  사용해도 business write와 원자적이지 않음을 명시한다.
- `batch/sqlcheckpoint/README.md`와 `README.ko.md`는 architecture, API, schema/migration,
  runtime privileges, failure/recovery, callback rules, validation commands를 동등하게 제공한다.
- Public Go doc과 one compile-checked end-to-end example은 caller-owned schema bootstrap,
  `Codec`, `WriteTxFunc`의 tx-bound `Session.ExecContext`, `StepOptions.AtomicWriter`,
  checkpoint-only commit, `ErrCommitUnknown` 뒤 fresh-run Load를 포함한다. Captured DB write는
  금지 예제로 설명한다.
- 두 locale은 같은 English-label asset을 공유한다:
  `docs/images/readme-diagrams/postgres-batch-checkpoint-atomic-sequence.svg`와 `.png`.
- Diagram은 `Step`, `CheckpointReader`, `Atomic Writer`, `PostgreSQL` participant와 같은
  transaction frame 안의 business write + CAS, write failure rollback, revision conflict
  rollback, commit-unknown 뒤 새 Load를 보여주며 두 provider locale README에 embed한다.
- `$bluetape-diagram`으로 SVG/PNG를 생성하고 CairoSVG scale 2, XML/audit, paired asset,
  full-size PNG visual inspection을 통과한다. Mermaid/ASCII/Graphviz output은 public 최종
  asset으로 사용하지 않는다.

## Compatibility와 rollout

- Existing `CheckpointStore`, `MemoryCheckpointStore`, `Writer`, `StepOptions.Writer` behavior는
  유지한다. `AtomicWriter`를 사용하지 않는 call site에는 source/behavior change가 없어야
  한다.
- Legacy 경로는 deprecated하지 않는다. 서로 다른 storage에 write해야 하거나 at-least-once
  replay를 수용하는 caller에게 계속 유효하다.
- Existing job cutover는 단순 option toggle이 아니다. Caller는 old runs/intake를 quiesce하고,
  authoritative legacy checkpoint와 business state를 reconcile한 뒤 exact namespace/key와
  codec으로 SQL checkpoint를 seed하고 read-back 검증한다. Mixed old/new binaries를 차단하고
  canary cohort에는 isolated namespace/key와 business-data cohort를 사용한 뒤 정확히 하나의
  provider만 활성화한다. Legacy crash window 때문에 authoritative position을 증명할 수 없으면
  in-place seed를 금지하고 명시적으로 idempotent replay 또는 새 cohort restart를 선택한다.
- Rollback도 atomic runs를 quiesce하고 SQL checkpoint/business state를 legacy store로
  reconcile/export하거나 명시적으로 승인된 safe replay position을 선택해야 한다. Table,
  grants, old state는 rollback observation window가 끝날 때까지 보존한다.
- Provider는 PostgreSQL writable primary만 지원한다. Read replica, transaction-pooling으로
  transaction affinity를 깨는 proxy, callback이 별도 connection을 쓰는 topology는 지원하지
  않는다.
- 운영자는 primary identity/read-only/recovery 상태, durability/RPO 설정, no-replay canary
  결과를 promotion gate로 확인한다. Shutdown 순서는 intake 중지, run cancel/join,
  commit-unknown reconcile, transaction user drain 확인, 마지막으로 caller-owned pool close다.
- Library는 logger/metric registry를 소유하지 않는다. Caller runbook은 low-cardinality
  load/commit outcome, conflict, version exhaustion, commit-unknown, cancellation, latency,
  `sql.DBStats`, table/dead-tuple size, WAL/autovacuum 신호와 canary observation/rollback gate를
  정의한다. Raw key/KeyID는 metric label에 넣지 않는다.
- Root README locale package inventory와
  `docs/release/v0.19.0-provider-conformance-runbook.md`를 함께 갱신한다.
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
6. Filter/skip progress는 consumed-input boundary에서 pending output과 함께 전진하며,
   pending output이 없을 때만 checkpoint-only transaction을 사용한다.
7. Caller-owned pool/migration/codec/retry/retention과 최소 runtime privilege가 문서화된다.
8. English/Korean package/root README, provider runbook, Go doc/example, paired sequence diagram이
   source와 일치한다.
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
