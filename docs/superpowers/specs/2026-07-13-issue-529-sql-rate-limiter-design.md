# Issue #529 PostgreSQL Token Bucket Provider Design

## 배경

`ratelimit`는 process-local token bucket을, `ratelimit/redis`는 Redis Lua
script 기반 multi-process provider를 제공한다. Issue #527에서
`ratelimit/ratelimittest`가 provider 공통 계약을 고정했으며, issue #500은 SQL
provider를 database-only topology의 moderate-QPS 선택지로 제한했다. Issue #528의
`leader/sql`은 caller-owned `*sql.DB`, caller-owned migration, fixed PostgreSQL
schema와 server-time statement를 이미 검증했다.

Issue #529는 이 기반을 사용해 PostgreSQL-backed token bucket을 추가한다. 새
provider는 Redis의 hot-path 대체재나 ORM adapter가 아니며, root
`ratelimit.Limiter`와 공통 conformance contract를 그대로 구현해야 한다.

## 목표

- `ratelimit/sql` package에 PostgreSQL-backed `ratelimit.Limiter`를 제공한다.
- 모든 refill과 consume 판정을 PostgreSQL의 단일 atomic statement로 선형화한다.
- local/Redis provider와 동일한 burst, refill, rejection, cancellation,
  commit-unknown, key isolation 및 exact-admission 계약을 실행한다.
- pool, migration, cleanup scheduling을 caller가 소유하게 한다.
- runtime role의 권한을 고정 table에 대한 최소 DML로 제한한다.
- database-only topology의 moderate-QPS/fallback 용도와 운영 한계를 영어/한국어
  문서에 명시한다.

## 비목표

- Redis provider를 high-QPS 기본값에서 대체하지 않는다.
- ORM, `sqlkit.Session`, `DBTX`, `*sql.Tx` 또는 caller transaction을 공개 API로
  받지 않는다.
- schema migration이나 idle cleanup goroutine을 자동 실행하지 않는다.
- FIFO fairness, reservation, waiting queue, adaptive rate, hierarchical quota를
  추가하지 않는다.
- SQL provider benchmark나 production capacity 순위를 이번 issue에서 만들지 않는다.
- PostgreSQL 이외 dialect를 하나의 abstraction으로 숨기지 않는다.

## 현재 근거

- `ratelimit.Limiter`는
  `Allow(context.Context, string, int64) (ratelimit.Result, error)` 하나만 요구한다.
- `ratelimit/ratelimittest.Run`은 capability flag 없이 initial burst, rejection,
  refill, key isolation, cancellation 경계, lost response 및 exact concurrency를
  강제한다.
- `ratelimit/redis`는 micro-token 정수 단위, server time, exact caller key,
  `IdleTTL`, maximum key bytes, zero result on commit-unknown을 사용한다.
- `leader/sql`은 `New(*sql.DB, ...)`, exported `SchemaSQL`, caller-owned writable
  primary와 migration, fixed `public` relation, 최소 DML role을 사용한다.
- PostgreSQL `INSERT ... ON CONFLICT DO UPDATE`는 high concurrency에서도 atomic
  insert-or-update outcome을 제공하고 `RETURNING`으로 실제 변경 결과를 같은
  round trip에 돌려줄 수 있다.
- PostgreSQL `clock_timestamp()`는 실제 server time을 반환한다. 이 설계에서는
  VALUES tuple에서 한 번 평가해 `excluded.observed_at`으로 statement 전체에 같은
  시각을 사용한다.
- PostgreSQL `numeric`은 exact arithmetic을 제공한다. elapsed time과 rate의 곱은
  overflow 방지를 위해 `numeric`으로 계산하고 burst 이하로 clamp한 뒤 `bigint`로
  변환한다.

## 검토한 접근

### 접근 1: PostgreSQL 전용 package와 caller-owned `*sql.DB` (채택)

`ratelimit/sql.New(*sql.DB, Options)`가 concrete limiter를 만들고 root
`ratelimit.Limiter`를 직접 구현한다. SQL statement와 schema는 PostgreSQL 전용이며
pool, migration, cleanup schedule은 caller가 소유한다.

장점:

- `leader/sql`과 동일한 ownership/운영 계약이다.
- caller transaction과 quota debit이 결합되지 않는다.
- public API가 작고 conformance runner에 직접 연결된다.
- PostgreSQL atomic UPSERT/server time을 숨기지 않는다.

단점:

- 다른 SQL dialect는 별도 provider가 필요하다.
- caller가 migration과 cleanup job을 운영해야 한다.

### 접근 2: public `DBTX` 또는 `sqlkit.Session` provider (제외)

`*sql.DB`와 `*sql.Tx`를 함께 받는 interface를 공개하면 generated SQL/ORM과 결합하기
쉽다. 그러나 caller transaction rollback이 이미 반환된 rate-limit 판정을 취소하거나,
business transaction 재시도가 quota debit을 중복시킬 수 있다. context와
commit-unknown ownership도 불명확해져 root provider contract와 맞지 않는다.

### 접근 3: stored function 기반 provider (제외)

PL/pgSQL function은 복잡한 계산과 반환 shape를 단순화할 수 있다. 대신 runtime rollout에
function execute 권한, versioning, security-definer 검토가 추가되고 migration 경계가
무거워진다. 단일 DML UPSERT로 충분하므로 채택하지 않는다.

## 공개 API

Package name은 import alias 충돌을 피하도록 `sqlratelimit`로 선언한다.

```go
package sqlratelimit

type Options struct {
    Namespace     string
    RatePerSecond float64
    Burst         int64
    IdleTTL       time.Duration
    MaxKeyBytes   int
}

type Limiter struct { /* constructor-only */ }

func New(db *sql.DB, options Options) (*Limiter, error)
func (l *Limiter) Allow(ctx context.Context, key string, tokens int64) (ratelimit.Result, error)
func (l *Limiter) Cleanup(ctx context.Context, limit int) (int64, error)

const SchemaSQL = `...`
const MaxCleanupBatch = 1000
```

`Limiter`는 `ratelimit.Limiter`를 compile-time으로 구현한다. Zero value와 nil receiver는
panic하지 않고 initialization error를 반환한다. `New`는 `db`를 ping하거나 schema를
생성하지 않으며 `db`를 닫지 않는다.

### Options 계약

- `Namespace`: empty이면 `default`; non-empty 값은 trim한 결과를 사용하며 blank는
  거절한다. Normalized namespace는 최대 128 bytes다.
- `RatePerSecond`: finite positive 값이어야 하며 micro-token/sec로 반올림했을 때
  positive `int64`여야 한다.
- `Burst`: positive이며 micro-token 변환이 `int64` 범위를 넘지 않아야 한다.
- `IdleTTL`: negative이면 거절한다. Zero이면 두 full-refill window와 1분 중 큰 값이다.
  Explicit 값은 최소 한 full-refill window여야 한다. PostgreSQL microsecond precision에
  맞춰 올림하며 변환 overflow는 거절한다.
- `MaxKeyBytes`: zero이면 512, 허용 범위는 1..1024다. Provider hard ceiling 1024를
  넘는 설정은 거절한다.

Rate, burst, IdleTTL과 default key limit은 `ratelimit/redis`와 동일하다. PostgreSQL
primary-index와 row-cardinality 방어를 위해 namespace 128 bytes와 key 1024 bytes hard
ceiling을 추가한다. 공통 validation helper를 새 production package로 추출하지 않고
provider parity test로 공유 범위의 drift를 방지한다.

### Key 계약

- Blank key는 거절하지만 유효한 key의 bytes는 trim/normalize/hash하지 않는다.
- 길이는 UTF-8 rune 수가 아니라 raw byte 수로 `MaxKeyBytes`와 비교한다.
- bucket identity는 `(namespace bytes, bucket_key bytes)`다. 두 값은 PostgreSQL `bytea`로
  bind하여 NUL과 invalid UTF-8를 포함한 Go string bytes도 보존한다.
- namespace가 같고 options가 같은 limiter instance들은 같은 bucket을 공유한다.
- Namespace와 key는 plaintext naming data이며 authorization boundary나 secret이 아니다.
  Caller는 인증/인가 후 canonical identity를 선택하고 key cardinality와 신규 bucket
  생성률을 제한해야 한다. Cleanup은 active-cardinality 공격을 막지 못한다.

## Schema

고정 relation은 `public.bluetape_ratelimit_buckets`다.

```sql
create table if not exists public.bluetape_ratelimit_buckets (
    namespace bytea not null,
    bucket_key bytea not null,
    rate_micros_per_second bigint not null check (rate_micros_per_second > 0),
    burst_micros bigint not null check (burst_micros > 0),
    idle_ttl_micros bigint not null check (idle_ttl_micros > 0),
    tokens_micros numeric(30, 6) not null
        check (tokens_micros >= 0 and tokens_micros <= burst_micros),
    last_allowed boolean not null,
    updated_at timestamptz not null,
    expires_at timestamptz not null check (expires_at >= updated_at),
    primary key (namespace, bucket_key)
)
```

`last_allowed`는 각 UPSERT가 자신의 판정을 `RETURNING`으로 명확히 돌려주기 위한
linearized outcome이다. 동시 statement가 뒤이어 row를 바꾸더라도 PostgreSQL은 각
statement에 자신이 변경한 row version을 반환하므로 호출별 결과가 섞이지 않는다.
`tokens_micros`의 scale 6은 micro-token 아래 6자리 잔여를 보존한다. Frequent rejected
polling도 fractional refill을 잃지 않으며 full burst에서는 정확히 burst로 clamp한다.

별도 expiry index는 기본 schema에 포함한다.

```sql
create index if not exists bluetape_ratelimit_buckets_expires_at_idx
on public.bluetape_ratelimit_buckets (expires_at)
```

`SchemaSQL`은 bootstrap contract이지 upgrade/verification engine이 아니다. Callers는
provider 사용 전에 migration authority로 schema `PUBLIC CREATE`를 revoke하고 bounded
lock/statement timeout 아래 `SchemaSQL`을 실행한다. 그 뒤 ordinary table relkind,
migration owner, exact column order/type/nullability/check definitions, primary-key order,
expiry index validity/readiness, RLS/forced-RLS off, zero user triggers, runtime role의
non-owner/non-membership과 direct/inherited/PUBLIC privilege를 catalog query로 검증한다.
`IF NOT EXISTS`는 검증이 아니며 incompatible pre-existing object는 deployment failure다.
`New`와 `Allow`는 DDL이나 schema repair를 실행하지 않는다.

## Allow data flow

1. 기존 local/Redis 호환성을 위해 nil context는 `context.Background()`로 normalize한다.
   Pre-canceled context는 원래 context error로 backend traffic 없이 반환한다.
2. Limiter 초기화, key, requested token 범위를 local에서 검증한다.
3. requested/rate/burst/idle 값을 micro-token/microsecond 정수로 변환한다.
4. 한 `INSERT ... ON CONFLICT DO UPDATE ... RETURNING` statement를 실행한다.
5. INSERT path는 full burst에서 requested amount를 debit하고 허용한다.
6. UPDATE path는 기존 `updated_at`부터 server elapsed microseconds를 계산한다. Negative
   elapsed는 zero로 clamp한다. `elapsed_micros::numeric *
   rate_micros_per_second / 1_000_000`을 scale-6 `numeric` state에 더해 fractional
   micro-token을 보존하고 burst로 clamp한다. Result의 integer remaining만 명시적으로
   `floor(...)::bigint`로 변환한다.
7. Refilled tokens가 requested 이상이면 debit하고 `last_allowed=true`; 부족하면 debit하지
   않고 `last_allowed=false`다.
8. Allowed/rejected 모두 refill state, `updated_at`, `expires_at`을 갱신한다. 이는 Redis
   provider의 rejected-attempt TTL semantics와 같다.
9. Conflict update의 `WHERE`는 stored configuration이 요청 options와 모두 같을 때만
   update를 허용한다. `RETURNING` row가 없으면 debit/row version 없이
   `ErrConfigurationMismatch`를 반환한다.
10. 반환 micro-token으로 `Remaining`, `RetryAfter`, `ResetAfter`를 overflow-safe integer
    ceil 방식으로 계산한다. Duration 범위를 넘으면 `time.Duration(math.MaxInt64)`로
    saturate한다.

Statement는 observed server time을 VALUES tuple에서 정확히 한 번 평가하고 conflict
path에서는 `excluded.updated_at`을 재사용한다. SQL에는 caller wall clock을 전달하지 않는다.
Relation/schema/table/column/cast/order SQL은 compile-time constant다. Namespace, key,
requested/rate/burst/TTL/cleanup limit 등 모든 runtime value는 `database/sql` positional bind
argument로만 전달하며 `fmt`, string concatenation 또는 identifier interpolation을 금지한다.

## Configuration mismatch

같은 `(namespace, key)`에 rate, burst 또는 idle TTL이 다른 limiter가 접근하면 기존
bucket을 새 정책으로 암묵 전환하지 않는다.

- UPSERT conflict path는 stored configuration이 모두 같을 때만 refill/debit/time을
  갱신한다. `DO UPDATE ... WHERE` mismatch는 quota/config state나 새 updated tuple version을
  만들지 않는다. 다만 conflict-row lock과 WAL/write overhead는 발생할 수 있으므로
  exceptional path로 계측하고 지속 발생을 alert한다.
- 불일치 시 `RETURNING` row가 없고 Go layer는 `errors.Is`로 검사하는
  `ErrConfigurationMismatch`와 `ratelimit.Result{}`를 반환한다.
- Error string에는 raw key, DSN, SQL text 또는 stored configuration을 노출하지 않는다.
- 정책 변경은 caller가 별도 migration/삭제/namespace rotation으로 수행한다.

이 규칙은 두 instance의 다른 설정이 하나의 bucket을 번갈아 재해석하는 것을 막는다.

## Error contract

Root `ratelimit`는 provider-neutral inspection contract를 제공한다.

```go
// ratelimit package
var ErrCommitUnknown = errors.New("ratelimit: commit outcome unknown")
type OperationError interface {
    error
    Family() string
    Operation() string
    KeyID() string
}

// sqlratelimit package
var ErrConfigurationMismatch = errors.New("sql rate limiter: configuration mismatch")
type OpError struct { /* redacted labels and cause */ }
```

- SQL `*OpError`와 Redis `*btredis.OpError`는 `ratelimit.OperationError`를 구현한다.
  `Error`, `Unwrap`, `Family`, `Operation`, redacted `KeyID`는 nil/zero receiver에서도
  panic하지 않고 safe fallback을 반환한다.
- Redis commit-unknown은 backward compatibility를 위해 `ratelimit.ErrCommitUnknown`과
  기존 `btredis.ErrCommitUnknown` 모두에 match한다.
- Validation과 pre-canceled context는 provider error로 감싸지 않는다. 모든 `Allow`
  error는 `ratelimit.Result{}`를 반환한다.
- PostgreSQL `PgError`처럼 statement failure/rollback이 확인된 error는 typed operation
  error지만 `ErrCommitUnknown`을 붙이지 않는다.
- Transport error, in-flight context cancellation/deadline, response loss 또는 committed
  row의 scan/parse failure는 typed operation error와 `ErrCommitUnknown`을 `errors.Join`한다.
- Commit-unknown은 항상 zero `ratelimit.Result`다. Caller는 자동 replay하지 않고 한 번의
  debit을 budget에 흡수하거나 full-refill window를 기다린다.
- `errors.Is`와 `errors.As`는 nested wrapping에서도 보존한다.
- Error string은 operation/family만 포함하고 raw key, DSN, endpoint, SQL 및 provider
  cause text를 노출하지 않는다.
- Redacted `KeyID`는 sampled diagnostic log correlation에만 사용하고 metric label에는
  사용하지 않는다.

`Cleanup` error는 항상 count 0을 반환한다. Commit-unknown이면 최대 `limit` rows가 이미
삭제됐을 수 있다. 재시도는 현재 expiry predicate에 대해 safety-preserving이지만 같은 batch나
count를 재현하는 idempotent operation은 아니다.

## Cleanup

PostgreSQL에는 Redis key TTL과 같은 자동 row expiry가 없으므로 cleanup은 명시적이다.

- `Cleanup(ctx, limit)`은 `1 <= limit <= MaxCleanupBatch(1000)`만 허용한다. 범위 밖 입력은
  SQL traffic 없이 거절한다.
- `expires_at <= server time` row를 expiry index 순서로 최대 `limit`개 선택한다.
- `FOR UPDATE SKIP LOCKED`와 같은 statement 안의 bounded DELETE를 사용해 여러 cleanup
  worker가 중복 대기하지 않게 한다.
- Allow와 경합하면 row lock 순서로 선형화된다. Cleanup이 먼저 삭제하면 Allow는 fresh
  full bucket을 insert하고, Allow가 먼저 갱신하면 Cleanup 조건에서 제외된다.
- Package는 background goroutine, ticker 또는 scheduler를 소유하지 않는다.
- 운영 문서는 cleanup cadence를 `IdleTTL`보다 짧게 권장하되 correctness가 cleanup
  실행 여부에 의존하지 않음을 명시한다.
- Caller scheduler는 fresh bounded context, DB lock/statement timeout, run별 max
  batches/time budget, jitter와 작은 worker concurrency를 사용한다. WAL/row-lock/pool latency
  또는 autovacuum pressure가 threshold를 넘으면 pause한다.
- `expires_at` index column을 모든 allowed/rejected Allow가 갱신하므로 PostgreSQL HOT update를
  기대하지 않는다. 이는 moderate-QPS 선택의 비용이며 WAL, dead tuples, index growth와
  autovacuum을 관측한다. Benchmark 없이 capacity 수치를 주장하지 않는다.

## Cancellation과 timeout

- Nil context는 기존 provider와 같이 `context.Background()`로 normalize한다.
- Pre-canceled/deadline context는 SQL dispatch 전에 원래 error로 반환한다.
- DB execution 중 또는 complete `RETURNING` scan 전에 cancellation이면 statement가 이미
  commit됐을 수 있으므로 zero result + `OpError` + `ratelimit.ErrCommitUnknown`이다.
- Complete scan으로 result가 확정된 뒤 test hook/caller cancellation이 관측되면 확정된
  successful result를 반환하고 caller context를 다시 검사하지 않는다. 이는 mandatory
  `cancel-after-linearize` contract와 같다.
- Package는 caller deadline을 축소하거나 자체 retry/backoff를 추가하지 않는다.
- `database/sql`이 statement와 connection cancellation을 소유한다. Limiter는 goroutine을
  만들지 않는다.

## Security와 운영 경계

- `db`는 모든 operation을 같은 writable PostgreSQL primary로 route해야 한다.
- Transaction-pooling proxy는 각 Allow가 독립 statement이므로 허용할 수 있지만 read
  replica 또는 statement replay/failover semantics는 caller가 검증한다.
- Runtime role은 schema `USAGE`, table `SELECT/INSERT/UPDATE/DELETE`만 필요하다.
- Runtime role에는 schema `CREATE`, table `TRUNCATE/REFERENCES/TRIGGER`, migration owner
  membership을 부여하지 않는다.
- RLS와 user trigger는 기본 contract에서 지원하지 않는다. Migration 검증과
  least-privilege test가 이를 고정한다.
- Schema ownership은 migration role에 남고 runtime role은 table owner가 아니다.
- Deployment preflight는 모든 pool/proxy endpoint의 server identity/timeline,
  `pg_is_in_recovery()=false`, `transaction_read_only=off`를 확인한다. Promotion 전에 old
  writer를 fence하고 proxy statement/transaction replay를 금지하거나 동등한 proof를
  제공한다. `synchronous_commit`/RPO가 debit WAL loss와 over-admission에 미치는 경계를
  문서화한다. Connection termination test만으로 HA promotion/fencing을 증명하지 않는다.

## Test design

### Unit

- option defaults와 invalid/overflow/precision boundaries
- exact `bytea` key preservation(NUL/invalid UTF-8/SQL fragment 포함), blank/oversized key,
  namespace와 hard-ceiling rejection 및 zero SQL traffic
- micro-token conversion, repeated sub-quantum refill carry, explicit floor 경계와 duration
  ceil/saturation/overflow
- SQL result parsing, 모든 error의 zero return, typed/redacted errors,
  provider-neutral `errors.Is`/`errors.As`
- nil context parity, nil/zero limiter `Allow`/`Cleanup` safety와 no-backend validation

### PostgreSQL integration

- initial burst, rejection, server-time refill, namespace/key isolation
- same bucket shared by independent `*sql.DB` pools
- exact concurrent admissions across multiple pools/process-equivalent clients
- configuration mismatch is row-version/quota no-op
- rejected attempt extends expiry without debit
- explicit cleanup limit, response-loss count 0, concurrency, refreshed/reinserted-row safety,
  next-batch retry와 Allow/cleanup 동시 부하 latency/starvation
- table missing/schema mismatch/read-only connection/permission failure
- runtime role least privilege and catalog ownership/index/trigger/RLS assertions

### Conformance and fault injection

- `ratelimittest.Run`의 모든 mandatory case를 skip 없이 실행한다.
- Test-only hook/control은 before-linearize, after-linearize, operation count와 lost response를
  제공하며 production API에 노출하지 않는다.
- Pre-dispatch cancellation은 no debit/original context, execution 중 cancellation과 injected
  response loss는 zero result/commit-unknown/exact one debit, complete scan 후 cancellation은
  successful result를 증명한다.
- Provider classifier는 `ratelimit.OperationError`만 식별하고
  validation/context/raw cause는 거절한다.

### Race/stress

- `go test -race -count=1 ./ratelimit/sql`
- conformance 반복 실행
- bounded multi-pool stress에서 allowed 합이 정확히 burst이고 모든 worker가 종료됨을
  검증한다.
- Testcontainers-backed command는 다른 real-service suite와 병렬 실행하지 않는다.

## Documentation

- `ratelimit/sql/README.md`와 `README.ko.md`: API, schema, migration, cleanup,
  commit-unknown, configuration mismatch, topology, 최소 권한, moderate-QPS 경계
- `ratelimit/README.md`와 `README.ko.md`: local/Redis/PostgreSQL 선택 기준
- root `README.md`와 `README.ko.md`: package index
- `CHANGELOG.md`: 0.19.0 provider addition과 public caveat
- `docs/release/v0.19.0-provider-conformance-runbook.md`: rollout, role, schema 및
  conformance command
- 모든 exported symbol은 English Go doc을 갖고 `doc.go`, compile-checked `ExampleNew`,
  SchemaSQL/caller-owned DB, shared commit-unknown inspection과 bounded cleanup example을
  제공한다.
- Caller guide는 `ErrConfigurationMismatch`에서 policy migration/namespace rotation을
  선택하는 법, 모든 `Allow` error에서 result를 무시하는 법, commit-unknown을 자동 replay하지
  않는 법, `Limiter`가 `Close`를 제공하지 않고 DB lifetime을 소유하지 않는다는 점을
  명시한다.
- Runbook은 bounded-cardinality Allow latency/outcome/error categories, `DBStats` wait/in-use,
  statement/row-lock latency, cleanup count/duration/error/backlog/oldest expiry, live/dead
  tuples, table/index size, autovacuum lag와 WAL growth를 정의한다. Key/namespace/DSN/endpoint/raw
  error는 label/log에 넣지 않는다.
- Promotion/rollback threshold는 배포별 stable baseline 대비 값, 연속 breach 횟수와 최소
  canary observation window를 사전에 기록한다. Library가 universal threshold를 제공하지
  않는다.

이번 issue에서는 새 diagram/benchmark chart를 만들지 않는다. 기존 contract와 SQL flow는
text/table로 충분하며 시각 자산 추가는 acceptance criterion이 아니다.

## Failure modes와 대응

| Failure mode | Signal | Required behavior |
|---|---|---|
| 동시 first insert/consume | burst보다 많은 allow | atomic UPSERT와 exact-admission stress |
| client clock skew | refill drift | server time only |
| arithmetic overflow/precision loss | negative/oversized tokens, wrong retry | numeric intermediate, int64 validation, boundary tests |
| frequent sub-quantum polling | refill starvation | scale-6 numeric carry and repeated rejection test |
| response loss after debit | caller retries and double debit | zero result + typed commit-unknown, no internal retry |
| conflicting configuration | bucket policy oscillation | quota no-op + configuration mismatch |
| cleanup races Allow | refreshed bucket deletion | row lock/expiry predicate in one statement |
| migration drift | scan/permission/constraint failures | explicit SchemaSQL, catalog and least-privilege tests |
| replica/failover routing | stale or rejected writes | writable-primary operational requirement |
| unbounded stale rows | table growth | caller-scheduled bounded Cleanup and expiry index |
| active-cardinality abuse | persistent table growth | caller auth/cardinality/create-rate controls; cleanup is not a defense |

## Compatibility와 migration

- 기존 local/Redis API와 저장 형식은 변경하지 않는다.
- 새 package는 additive이며 기존 caller migration이 없다.
- SQL relation name과 columns는 0.19.0 public operational contract다.
- Future schema changes는 backward-compatible migration 또는 새 relation/version을
  요구한다. Provider가 runtime DDL로 자동 repair하지 않는다.
- Go module에 새 runtime dependency를 추가하지 않는다. 기존 `database/sql`, pgx driver와
  PostgreSQL Testcontainers fixture를 재사용한다.
- Redis/local/SQL은 quota state를 공유하지 않는다. 같은 logical quota를 mixed-provider로
  동시에 serve하면 각각 full burst를 제공하므로 금지한다. Canary는 독립 namespace와
  cohort를 사용한다.
- Cutover/rollback은 old provider traffic을 quiesce하고 최소 full-refill window를 기다리거나
  승인된 extra-burst budget을 기록한 뒤 한 provider로 전환한다. SQL table/grant 제거는 SQL
  binary 사용량이 0임을 관찰한 뒤 별도 migration에서 수행한다.
- Controlled HA exercise에서 old-writer fencing, durability/RPO, failover 중
  commit-unknown no-replay를 검증해야 production promotion할 수 있다.

## Acceptance criteria

1. `New(*sql.DB, Options)`가 constructor-only limiter를 만들고 root
   `ratelimit.Limiter`를 구현한다.
2. `SchemaSQL`과 `Cleanup`이 caller-owned migration/maintenance 경계를 제공한다.
3. Atomic server-time UPSERT가 refill/consume/rejection을 한 statement에서 선형화한다.
4. `ratelimittest.Run` 전체가 skip 없이 통과한다.
5. Multi-pool bounded stress와 race에서 exact burst admission을 증명한다.
6. Cancellation과 response-loss가 no-late-debit 또는 exact-one-debit/commit-unknown으로
   구분된다.
7. Configuration mismatch가 quota를 변경하지 않는다.
8. Least-privilege runtime role과 schema/catalog contract가 검증된다.
9. English/Korean README, root index, CHANGELOG와 release runbook이 동기화된다.
10. Targeted tests, race, formatter/lint/static checks와 `make ci`가 통과한다.
11. Provider cutover/rollback, HA fencing/RPO와 telemetry promotion gate가 runbook에
    검증 가능하게 기록된다.

## DoD

- Spec과 implementation plan의 7-Tier review가 P0=0/P1=0이다.
- TDD RED/GREEN evidence와 spec acceptance mapping이 남는다.
- Step 6-R pre-PR review와 live PR review가 P0=0/P1=0이다.
- PR metadata가 issue #529의 milestone, labels, assignee를 반영하고 body의 마지막 section이
  `## DoD Status`다.
- GitHub CI 성공 후 explicit merge decision을 기다린다.

## References

- GitHub issues #500, #527, #528, #529, #560
- `ratelimit`, `ratelimit/redis`, `ratelimit/ratelimittest`
- `leader/sql`
- PostgreSQL `INSERT`: https://www.postgresql.org/docs/current/sql-insert.html
- PostgreSQL `RETURNING`: https://www.postgresql.org/docs/current/dml-returning.html
- PostgreSQL date/time functions: https://www.postgresql.org/docs/current/functions-datetime.html
- PostgreSQL numeric types: https://www.postgresql.org/docs/current/datatype-numeric.html
