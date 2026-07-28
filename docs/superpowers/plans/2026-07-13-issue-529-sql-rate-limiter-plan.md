# PostgreSQL rate limiter provider 구현 계획

> 한국어 재작성 범위: 이 계획 문서는 한국어 운영 문서로 읽히도록 제목, 판단, 작업 설명, 위험, 검증, 롤백 문맥을 한국어로 정리한다. 명령, 경로, API 이름, 이슈/PR 번호, 브랜치명, 코드 블록, 테스트 출력 같은 증거 문자열은 정확성을 위해 원문 그대로 보존한다.


> **에이전트 작업자용:** 필수 하위 스킬: 사용 superpowers:subagent-driven-development (권장) 또는 superpowers:executing-plans to 이 계획을 작업 단위로 구현. 단계는 checkbox (`- [ ]`) 추적 문법을 사용.

**목표:** 추가 a PostgreSQL-backed `ratelimit.Limiter` at `ratelimit/sql` for database-만, moderate-QPS deployments, 함께 exact atomic admission, provider-neutral indeterminate-outcome 오류, 호출자-owned schema, 및 bounded cleanup.

**아키텍처:** `sqlratelimit.Limiter` owns 없음 connection lifecycle 또는 goroutines; it stores a 호출자-owned `*sql.DB` 및 normalized immutable options. Each `Al낮음` call uses one schema-qualified parameterized PostgreSQL UPSERT whose single server timestamp drives exact numeric refill, debit, expiry, 및 returned durations. Cleanup is a separate bounded `DELETE ... FOR UPDATE SKIP LOCKED` operation, while 공유 root 오류 contracts let callers inspect Redis 및 SQL failures uniformly.

**기술 스택:** Go 1.26, `database/sql`, pgx v5 stdlib/`pgconn`, PostgreSQL Testcontainers, `ratelimit/ratelimittest`, standard-library hashing 및 오류.

---

## 파일 지도

| Area | 파일 | 책임 |
|---|---|---|
| Root 계약 | `ratelimit/errors.go`, `ratelimit/errors_test.go`, `ratelimit/redis/{limiter.go,operation_error_test.go,conformance_test.go}` | Provider-neutral `ErrCommitUnknown` 및 `OperationError`, plus backward-compatible Redis matching. |
| SQL API/options | `ratelimit/sql/{doc.go,options.go,options_test.go,limiter.go,limiter_test.go}` | Public constructor/options, 호출자-owned pool, validation, exact byte identity, nil/zero safety. |
| SQL schema/statements | `ratelimit/sql/{schema.go,queries.go,queries_test.go}` | Fixed bootstrap DDL, atomic UPSERT, result conversion, configuration mismatch, bounded cleanup. |
| SQL 진단 | `ratelimit/sql/{errors.go,errors_test.go}` | Typed redacted operation failures 및 known-rollback versus indeterminate classification. |
| Provider proof | `ratelimit/sql/{conformance_test.go,security_test.go,stress_test.go,example_test.go,readme_test.go}` | Mandatory 계약, deterministic fault injection, multi-pool exactness, least privilege/catalog checks, compile-checked usage, docs parity. |
| Package docs | `ratelimit/sql/README.md`, `ratelimit/sql/README.ko.md`, `ratelimit/README.md`, `ratelimit/README.ko.md` | API, migration, cleanup, topology, failure handling, 및 provider selection. |
| Public release docs | `README.md`, `README.ko.md`, `CHANGELOG.md`, `docs/release/v0.19.0-provider-conformance-runbook.md` | Discoverability, 0.19.0 caveat, rollout/rollback, HA/RPO, telemetry gates. |
| Workf낮음 evidence | `docs/superpowers/reviews/2026-07-13-issue-529-sql-rate-limiter-{risk,plan-review}.md`, later 단계 6-R/7-R artifacts | Pre-implementation risks 및 review convergence evidence. |

## 의존 순서 및 쓰기 범위

작업 0 freezes reviewed artifacts 및 risk evidence. 작업 1 must land 전에 SQL code because
Tasks 4 및 6 inspect provider-neutral 오류 types. 작업 2 defines the SQL 패키지, normalized
values, 및 schema consumed by every later task. 작업 3 establishes successful atomic admission;
작업 4 then adds failure classification 및 deterministic cancellation/lost-response proof. 작업 5
adds cleanup 후 the bucket lifecycle is stable. 작업 6 owns real-PostgreSQL conformance,
concurrency, precision, schema, 및 least-privilege proof. Tasks 7 및 8 document 만 the settled
API. 작업 9 runs final verification 및 단계 6-R.

Tasks 1 및 2 have disjoint write scopes but execute sequentially to keep RED/GREEN evidence 및
commits unambiguous. No Testcontainers-backed command may run concurrently 함께 another real
service suite 또는 delegated worker. 다음을 하지 않는다: change `ratelimit.Limiter`, `ratelimittest.Harness`, 또는
the 공개 Redis constructor. A required change to those contracts stops execution 및 returns to
단계 2 design review.

### 작업 0: 고정 Artifacts 및 예측 Risks

**복잡도:** Small documentation gate; blocks 모든 source edits.

**파일:**
- 검증: `docs/superpowers/specs/2026-07-13-issue-529-sql-rate-limiter-design.md`
- 검증: `docs/superpowers/reviews/2026-07-13-issue-529-sql-rate-limiter-spec-review.md`
- 검증: `docs/superpowers/plans/2026-07-13-issue-529-sql-rate-limiter-plan.md`
- 생성: `docs/superpowers/reviews/2026-07-13-issue-529-sql-rate-limiter-risk.md`

- [ ] **단계 1: 검증 the approved artifact-만 branch**

```bash
git status --short
git log --oneline origin/develop..HEAD
git diff --check origin/develop...HEAD
```

예상: the reviewed spec, spec review, approved plan, 및 plan review are the 만 changes
ahead of `origin/develop`; 없음 `ratelimit/sql` directory exists.

- [ ] **단계 2: 기록 the pre-implementation risk table**

생성 the risk artifact 함께 columns `Risk`, `Trigger`, `Signal`, `Prevention`, `Recovery`, 및
`Owner`. Include these concrete rows: first-insert race, fractional-refill starvation, arithmetic
또는 duration overf낮음, configuration mismatch lock/WAL pressure, connection-pool starvation,
response loss 후 debit, cancellation 전에/후 scan, cleanup/Al낮음 race, cleanup backlog,
HOT/index/autovacuum pressure, 공개-schema hijack, privilege inheritance, RLS/trigger drift,
active-key cardinality abuse, mixed-provider extra burst, replica routing, failover replay,
asynchronous WAL loss, Testcontainers leakage, 및 bilingual documentation drift.

- [ ] **단계 3: 캡처 environment 및 baseline evidence**

```bash
go version
go list -m -f '{{.Path}} {{.Version}}' github.com/jackc/pgx/v5 github.com/testcontainers/testcontainers-go
go test -count=1 ./ratelimit/... ./redis/...
```

예상: Go 및 dependency versions are recorded 및 the 기존 rate-limit/Redis slice passes.
기록 that the feature-branch baseline `go test -count=1 ./...` passed 전에 source edits.

- [ ] **단계 4: 커밋 the risk artifact 전에 source work**

```bash
git add docs/superpowers/reviews/2026-07-13-issue-529-sql-rate-limiter-risk.md
git commit -m "docs: predict PostgreSQL rate limiter risks"
```

예상: the risk commit predates 모든 source commits 및 supplies rollback/owner information for
every database, API, concurrency, 보안, 및 rollout hazard.

### 작업 1: 추가 the Provider-Neutral Error Contract

**복잡도:** Medium 공유-API change; preserve 모든 Redis behavior 및 sentinels.

**Pattern skill:** `bluetape-go-patterns` 오류 wrapping 및 backward compatibility.

**파일:**
- 생성: `ratelimit/errors.go`
- 생성: `ratelimit/errors_test.go`
- Modify: `ratelimit/redis/limiter.go`
- Modify: `ratelimit/redis/operation_error_test.go`
- Modify: `ratelimit/redis/conformance_test.go`

- [ ] **단계 1: Write RED root 및 Redis compatibility 테스트**

추가 compile-time 및 nested wrapping 테스트 함께 this 계약:

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

`operationErrorStub` implements `error`, `Family`, `Operation`, 및 `KeyID`. Extend the 기존
Redis lost-response/conformance 테스트 to assert zero `Result`, both sentinels, the root interface,
및 preservation through `fmt.Errorf("nested: %w", err)`.

- [ ] **단계 2: Observe RED**

```bash
go test -count=1 ./ratelimit ./ratelimit/redis -run 'OperationError|CommitUnknown'
```

예상: build FAIL because the root sentinel/interface do 아님 exist 및 Redis does 아님 match the
root sentinel.

- [ ] **단계 3: 추가 the minimal root 계약 및 Redis join**

생성:

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

In both Redis mutation-uncertainty branches, join the 기존 typed 오류 함께 both sentinels
while preserving the current operation label: execution 오류 remain `consume` 및 result-scan
오류 remain `parse-result`:

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

추가 `var _ ratelimit.OperationError = (*btredis.OpError)(nil)` in the Redis 패키지 테스트 및
assert both `consume` 및 `parse-result` survive nested root-interface inspection. 다음을 하지 않는다: change
`btredis.ErrCommitUnknown`, 기존 operation labels, 또는 any 공유 Redis 오류 string.

- [ ] **단계 4: 검증 GREEN, full Redis regression, 및 commit**

```bash
gofmt -w ratelimit/errors.go ratelimit/errors_test.go ratelimit/redis/limiter.go ratelimit/redis/operation_error_test.go ratelimit/redis/conformance_test.go
go test -count=1 ./ratelimit ./ratelimit/redis ./redis
git add ratelimit/errors.go ratelimit/errors_test.go ratelimit/redis/limiter.go ratelimit/redis/operation_error_test.go ratelimit/redis/conformance_test.go
git commit -m "feat: add provider-neutral rate limit errors"
```

예상: 모든 테스트 PASS; Redis lost responses match old 및 new sentinels without exposing keys 또는
causes. Roll back this commit if any 기존 Redis 호출자-facing 테스트 changes unexpectedly.

### 작업 2: 정의 SQL Options, Constructor, 및 Schema

**복잡도:** Medium 공개 API/schema 계약.

**Pattern skill:** `bluetape-go-patterns` 패키지 design, Go docs, validation, 호출자 ownership.

**파일:**
- 생성: `ratelimit/sql/doc.go`
- 생성: `ratelimit/sql/options.go`
- 생성: `ratelimit/sql/options_test.go`
- 생성: `ratelimit/sql/limiter.go`
- 생성: `ratelimit/sql/limiter_test.go`
- 생성: `ratelimit/sql/errors.go`
- 생성: `ratelimit/sql/errors_test.go`
- 생성: `ratelimit/sql/schema.go`
- 생성: `ratelimit/sql/queries_test.go`

- [ ] **단계 1: Write RED constructor, option, key, nil, 및 schema 테스트**

사용 table 테스트 covering nil DB; NaN/Inf/zero/tiny/overf낮음 rate; zero/negative/overf낮음 burst;
negative, sub-full-refill, microsecond-ceiling, 및 overf낮음 TTL; namespace default/trim/blank/129
bytes; key limit default 및 bounds 1..1024; blank/NUL/invalid-UTF8/exact-byte/oversized keys; 및
constructor 없음-I/O. `Al낮음` nil-context 및 nil/zero-receiver behavior belongs to 작업 3,
후 the method exists; `Cleanup` nil/zero behavior belongs to 작업 5. 검증 the constructor shape:

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

사용 a private `normalizeKey` unit 테스트 to prove successful keys return the original bytes unchanged
및 invalid keys produce 없음 backend call. 검증 `MaxCleanupBatch == 1000` 및 nested
`errors.Is(fmt.Errorf("nested: %w", ErrConfigurationMismatch), ErrConfigurationMismatch)` is true.

- [ ] **단계 2: Observe RED**

```bash
go test -count=1 ./ratelimit/sql -run 'TestNew|TestOptions|TestKey|TestSchema|ConfigurationMismatch'
```

예상: build FAIL because `ratelimit/sql`, `Options`, `Limiter`, `SchemaSQL`, 및
`MaxCleanupBatch` do 아님 exist.

- [ ] **단계 3: 추가 the minimal 공개 패키지 및 normalized options**

정의:

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
`rateMicrosPerSecond`, checked `burstMicros`, microsecond-ceiled `idleTTLMicros`, 및 key bound.
Reuse the Redis formulas behaviorally, 아님 by importing a provider 패키지. 사용 checked integer
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

Default `IdleTTL` is `max(2*fullRefill, time.Minute)` 함께 overf낮음-safe saturation 전에 the
microsecond conversion. `normalizeKey` rejects trimmed blank 및 byte length over the configured
limit but returns the original string. Nil context normalizes to `context.Background()`.

- [ ] **단계 4: 추가 the exact bootstrap schema**

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

Go doc must say `SchemaSQL` is 호출자-executed bootstrap, 아님 verification/upgrade logic, 및
`New` never executes it 또는 closes the pool.

- [ ] **단계 5: 검증 GREEN 및 commit**

```bash
gofmt -w ratelimit/sql/doc.go ratelimit/sql/options.go ratelimit/sql/options_test.go ratelimit/sql/limiter.go ratelimit/sql/limiter_test.go ratelimit/sql/errors.go ratelimit/sql/errors_test.go ratelimit/sql/schema.go ratelimit/sql/queries_test.go
go test -count=1 ./ratelimit/sql -run 'TestNew|TestOptions|TestKey|TestSchema|ConfigurationMismatch'
git add ratelimit/sql
git commit -m "feat: define PostgreSQL rate limiter provider"
```

예상: constructor/schema 테스트 PASS without opening a database connection 및 invalid options
fail 전에 any backend access. No `go.mod` 또는 `go.sum` change occurs.

### 작업 3: 구현 Atomic Server-Time Admission

**복잡도:** High database hot path 및 exact-arithmetic boundary.

**Pattern skill:** `bluetape-go-patterns` database/concurrency 테스트 및 오류 wrapping.

**파일:**
- 생성: `ratelimit/sql/queries.go`
- Modify: `ratelimit/sql/limiter.go`
- Modify: `ratelimit/sql/queries_test.go`
- Modify: `ratelimit/sql/limiter_test.go`

- [ ] **단계 1: 추가 RED real-PostgreSQL happy-path 및 precision 테스트**

생성 one serial fixture 함께 a 90-second context, `postgrestestcontainer.Start`,
`sql.Open("pgx", dsn)`, `PingContext`, 호출자 execution of `SchemaSQL`, 및
`t.Cleanup(func() { _ = db.Close() })`. Every additional pool gets its own `t.Cleanup` close. 사용
unique namespace/key values per subtest.
추가: initial full-burst debit; exact rejection result; refill; key 및 namespace isolation; 공유
bucket across two independent pools; rejected-attempt expiry extension; NUL/invalid-UTF8 byte
identity; configuration mismatch row/quota/version 없음-op; repeated sub-microtoken progress; 및
duration ceil/saturation scan boundaries.

추가 local 테스트 for nil context normalization, pre-canceled context 함께 zero backend traffic, 및
nil/zero `*Limiter` receivers returning zero result plus initialization 오류 without panic. Once
`Al낮음` exists, add `var _ ratelimit.Limiter = (*Limiter)(nil)` beside the type.

For mismatch, read the row 전에/후 및 assert `tokens_micros`, rate, burst, TTL, `updated_at`,
및 `xmin` are unchanged. For fractional carry, use a 낮음 rate 및 repeated rejected polls whose
individual elapsed refills are be낮음 one microtoken, then assert their accumulated state eventually
admits exactly when the summed server elapsed time permits.

추가 a deterministic stale-observation regression: begin an admin transaction, lock the bucket row,
advance its `updated_at` 및 token state inside that transaction, start `Al낮음` 및 confirm through
`pg_stat_activity` that it is waiting on the row lock, then release the transaction. 검증 the
waiter's older observed time never regresses `updated_at`/`expires_at` 및 the next request cannot
obtain excess admission from double refill.

- [ ] **단계 2: Observe RED**

```bash
go test -p 1 -count=1 ./ratelimit/sql -run '^TestAllowPostgres$'
```

예상: FAIL because `Al낮음`, the UPSERT, result scanning, 및 configuration-mismatch mapping are absent.

- [ ] **단계 3: 구현 the single-statement UPSERT**

정의 one compile-time constant using 만 `$1..$6` for namespace bytes, key bytes, requested
micros, burst micros, rate micros/second, 및 idle TTL micros. Its shape must be:

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

The repeated refill expression is intentional because PostgreSQL `ON CONFLICT DO UPDATE` has 없음
`FROM` clause. `greatest(bucket.updated_at, excluded.updated_at)` prevents a statement that waited
for the conflict-row lock from moving time backwards; losing lock-wait time is conservative 및
cannot over-admit. Validate this exact statement directly against the fixture 전에 wrapping it
in Go; do 아님 replace it 함께 a read-then-write sequence 또는 a 호출자 transaction.

- [ ] **단계 4: Scan 및 convert the confirmed result**

`Al낮음` performs local preflight, calls `QueryRowContext`, 및 scans
`al낮음ed bool`, `remaining int64`, `retryMicros int64`, `resetMicros int64`. Map `sql.ErrNoRows`
to zero result plus `ErrConfigurationMismatch`. Convert nonnegative microseconds 함께 a checked
saturating helper:

```go
func microsDuration(value int64) time.Duration {
    if value <= 0 { return 0 }
    if value > math.MaxInt64/int64(time.Microsecond) { return time.Duration(math.MaxInt64) }
    return time.Duration(value) * time.Microsecond
}
```

Return `ratelimit.Result{Al낮음ed: al낮음ed, Requested: tokens, Remaining: remaining,
RetryAfter: microsDuration(retryMicros), ResetAfter: microsDuration(resetMicros)}`. Every local 또는
backend 오류 returns `ratelimit.Result{}`.

- [ ] **단계 5: 검증 GREEN, inspect the query, 및 commit**

```bash
gofmt -w ratelimit/sql/queries.go ratelimit/sql/limiter.go ratelimit/sql/queries_test.go ratelimit/sql/limiter_test.go
go test -p 1 -count=1 ./ratelimit/sql -run 'TestAllowPostgres|TestAllowValidation|TestDuration'
rg -n 'fmt\.Sprintf|fmt\.Appendf|\+.*(namespace|key)|clock_timestamp\(\)' ratelimit/sql/queries.go
git add ratelimit/sql
git commit -m "feat: add atomic PostgreSQL rate limiting"
```

예상: targeted 테스트 PASS; inspection shows fixed SQL, exactly one textual
`clock_timestamp()` call in the UPSERT, 및 없음 runtime SQL interpolation. If mismatch changes
`xmin` 또는 quota/config values, do 아님 commit; repair the statement 및 rerun the whole fixture.

### 작업 4: Classify Failures 및 증명 Cancellation Boundaries

**복잡도:** High indeterminate-mutation 및 fault-injection behavior.

**파일:**
- Modify: `ratelimit/sql/errors.go`
- Modify: `ratelimit/sql/errors_test.go`
- Modify: `ratelimit/sql/limiter.go`
- Modify: `ratelimit/sql/limiter_test.go`
- 생성: `ratelimit/sql/conformance_test.go`

- [ ] **단계 1: Write RED typed-오류 및 deterministic boundary 테스트**

추가 테스트 proving: zero/nil `OpError` methods do 아님 panic; nested `errors.As` reaches both
`*OpError` 및 `ratelimit.OperationError`; 오류 strings omit raw key, namespace, DSN, SQL, endpoint,
및 원인 text; `KeyID` is stable for identical raw `(namespace,key)` bytes 및 differs for a second
key; known `*pgconn.PgError` failure is typed but 아님 commit-unknown; transport/scan/lost-response
failure is typed plus root commit-unknown; 모든 오류 return zero result.

사용 the 패키지-private `testPhase` 및 constants introduced 함께 `Limiter` in 작업 2 to build
deterministic controls around both phases.

The `operation` argument is `al낮음` 또는 `cleanup`; cleanup passes an empty key 및 never derives a
key ID from it. The 전에 hook blocks 전에 SQL dispatch 및 returns the original canceled context 오류 without
operation wrapping 또는 traffic. The 후 hook runs 만 후 a complete successful `Scan`; a
호출자 cancellation while blocked there must still return the confirmed result. An injected 오류
there simulates response loss 및 must return zero result plus `OpError` 및
`ratelimit.ErrCommitUnknown`, 함께 exactly one stored debit.

추가 a real in-flight cancellation 테스트 rather than relying 만 on hooks: create a bucket, acquire
및 `defer 롤백` an admin transaction that row-locks it, start `Al낮음` 함께 a bounded cancelable
context, poll `pg_stat_activity` until that backend is waiting on a lock, then cancel. 검증 prompt
return 함께 zero result, `context.Canceled`, `*OpError`, 및 `ratelimit.ErrCommitUnknown`; assert 없음
internal retry 및, 후 releasing the admin transaction, 없음 unexpected debit. Close the dedicated
`*sql.Conn` on every exit path. 실행 this case repeatedly 및 under race.

- [ ] **단계 2: Observe RED**

```bash
go test -p 1 -count=1 ./ratelimit/sql -run 'TestOpError|TestAllowCancel|TestAllowInFlightCancellation|TestAllowLostResponse|TestAllowKnownRollback'
```

예상: FAIL because SQL 진단 및 phase controls are absent.

- [ ] **단계 3: 구현 redacted SQL 진단**

Extend the 작업 2 sentinel file 함께:

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

Compute `KeyID` as `sql-rate-key:` plus the first 20 낮음ercase hex characters of SHA-256 over
`binary.BigEndian uint64(namespace byte length) || namespace bytes || key bytes`; the length prefix
prevents ambiguous boundaries even when either Go string contains NUL. `Error()` contains 만 the
safe family 및 operation, 아님 the key ID 또는 causal text. 검증
`var _ ratelimit.OperationError = (*OpError)(nil)`.

- [ ] **단계 4: 구현 known-rollback versus uncertain handling**

`sql.ErrNoRows` remains configuration mismatch. A `*pgconn.PgError` proves statement failure 및
returns 만 `*OpError`. Errors 후 dispatch that are 아님 PostgreSQL server 오류, including
context cancellation/deadline, driver transport 오류, scan conversion, 및 the 후-scan
failure hook, return:

```go
return ratelimit.Result{}, errors.Join(
    newOperationError("allow", namespace, key, errors.Join(err, ctx.Err())),
    ratelimit.ErrCommitUnknown,
)
```

Join `ctx.Err()` 만 when non-nil. 다음을 하지 않는다: retry. Complete the `Scan`, build the result, run the
후-linearize 테스트 hook, then return the confirmed result without rechecking `ctx.Err()`.

- [ ] **단계 5: 검증 GREEN, conformance fault cases, 및 commit**

```bash
gofmt -w ratelimit/sql/errors.go ratelimit/sql/errors_test.go ratelimit/sql/limiter.go ratelimit/sql/limiter_test.go ratelimit/sql/conformance_test.go
go test -p 1 -count=1 ./ratelimit/sql -run 'TestOpError|TestAllowCancel|TestAllowInFlightCancellation|TestAllowLostResponse|TestAllowKnownRollback'
go test -race -p 1 -count=1 ./ratelimit/sql -run 'TestAllowCancel|TestAllowInFlightCancellation|TestAllowLostResponse'
go test -p 1 -count=10 ./ratelimit/sql -run '^TestAllowInFlightCancellation$'
git add ratelimit/sql
git commit -m "feat: harden PostgreSQL rate limit failures"
```

예상: 모든 boundary 테스트 PASS under race; pre-dispatch cancellation stores 없음 row, post-scan
cancellation returns success, 및 lost response stores exactly one debit but returns zero plus the
root sentinel.

### 작업 5: 추가 Caller-Owned Bounded Cleanup

**복잡도:** Medium maintenance operation 함께 lock 및 response-loss risks.

**파일:**
- Modify: `ratelimit/sql/limiter.go`
- Modify: `ratelimit/sql/queries.go`
- Modify: `ratelimit/sql/queries_test.go`
- Modify: `ratelimit/sql/errors_test.go`

- [ ] **단계 1: 추가 RED cleanup validation, lifecycle, 및 concurrency 테스트**

Test `limit` values `-1,0,1001` return count 0/오류/없음 SQL; nil/zero limiter is safe; cleanup
deletes at most the requested count in expiry-index order; live/refreshed rows survive; two workers
using separate pools do 아님 double-count a row; concurrent `Al낮음` either refreshes the row 또는
inserts a fresh full bucket 후 cleanup; 및 missing table/read-만/permission/response-loss
오류 return count 0. On uncertain completion, a direct admin count may show up to `limit` rows
deleted, 및 a retry operates on the next currently expired batch rather than reproducing the
first count.

- [ ] **단계 2: Observe RED**

```bash
go test -p 1 -count=1 ./ratelimit/sql -run '^TestCleanup'
```

예상: build FAIL because `Cleanup` 및 its statement are absent.

- [ ] **단계 3: 구현 one bounded cleanup statement**

사용 만 a positional limit bind 및 server time:

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

`Cleanup` normalizes nil context, rejects pre-canceled context 및 invalid limits 전에 traffic,
및 scans a single `int64`. Server `*pgconn.PgError` returns typed `cleanup` 오류 만; 모든 other
post-dispatch 오류 join the root commit-unknown sentinel. Every 오류 returns count 0. 다음을 하지 않는다: add
a ticker, goroutine, retry, transaction API, 또는 pool close. Invoke the same private 테스트 hook 함께
operation `cleanup` 전에 dispatch 및 후 the count scan so response-loss 및 cancellation
테스트 are deterministic; cleanup operation 오류 use the fixed safe KeyID
`sql-rate-key:<cleanup>`.

- [ ] **단계 4: 검증 GREEN 및 commit**

```bash
gofmt -w ratelimit/sql/limiter.go ratelimit/sql/queries.go ratelimit/sql/queries_test.go ratelimit/sql/errors_test.go
go test -p 1 -count=1 ./ratelimit/sql -run '^TestCleanup'
go test -race -p 1 -count=1 ./ratelimit/sql -run 'TestCleanupConcurrent|TestCleanupAllowRace'
git add ratelimit/sql
git commit -m "feat: add bounded rate limit cleanup"
```

예상: cleanup 테스트 PASS, each call touches at most 1000 rows, 없음 worker blocks indefinitely,
및 refreshed rows survive. If contention causes starvation 또는 timeouts, stop 및 repair statement
ordering/fixture synchronization rather than increasing 테스트 timeouts.

### 작업 6: 실행 Mandatory Conformance, Stress, 및 Security Proof

**복잡도:** High backend capability 및 deployment-계약 proof.

**파일:**
- Complete: `ratelimit/sql/conformance_test.go`
- 생성: `ratelimit/sql/stress_test.go`
- 생성: `ratelimit/sql/security_test.go`
- Modify: `ratelimit/sql/queries_test.go`

- [ ] **단계 1: Wire the mandatory conformance harness without capability flags**

The factory opens an independent pgx `*sql.DB` per limiter, immediately registers
`tb.Cleanup(func() { _ = db.Close() })`, applies 없음 DDL, sets a unique `conformance` namespace,
및 installs 만 the private 테스트 hook. Container startup is registered first so LIFO cleanup
closes every pool 전에 terminating the container. Adapt `ratelimit.Result` field by field. The
classifier is provider-neutral:

```go
IsProviderError: func(err error) bool {
    var target ratelimit.OperationError
    return errors.As(err, &target) && target.Family() == "rate limiter"
},
```

구현 `GateNext`, `FailNext`, 및 `OperationCount` 함께 a mutex 및 one-shot channels, matching
the 기존 Redis control semantics. 실행 `ratelimittest.Run(t, harness)` 함께 없음 skipped cases.

- [ ] **단계 2: 추가 exact multi-pool 및 repeated precision stress**

Open at least four independent pools against one fixture. Release `Burst+32` workers from a
barrier, call one-token `Al낮음`, require every worker to finish under a bounded context, 및 assert
the sum of admitted requested tokens is exactly `Burst`. Repeat 함께 unique namespaces for 20
iterations. Register `t.Cleanup(db.Close)` immediately for every pool, close every acquired
`*sql.Conn`, 및 roll back every transaction on 모든 exits. 추가 a repeated sub-quantum
rejection/refill 테스트 및 a concurrent cleanup/Al낮음 stress that records maximum latency without
asserting an unmeasured capacity number.

추가 a named `TestCleanupAl낮음PoolContention` using one deliberately constrained 공유 pool 함께
`SetMaxOpenConns(2)` 및 `SetMaxIdleConns(2)`. Seed disjoint expired rows plus one hot bucket, issue
fixed counts of bounded `Al낮음` 및 `Cleanup` calls under per-call 및 global contexts, 및 assert
every call completes 전에 the global deadline, outcomes account for every issued call, cleanup
deletes at least one expired row, the hot-bucket admitted sum is exactly `Burst`, 및 every worker
exits. 캡처 `DBStats.WaitCount`, `WaitDuration`, 및 maximum operation latency as 진단
만; do 아님 turn them into an unsupported capacity threshold.

- [ ] **단계 3: 추가 schema 및 least-privilege deployment proof**

생성 `ratelimit_migration_owner` 및 `ratelimit_runtime`; first verify `public` is owned by the
approved migration authority, revoke `CREATE ON SCHEMA public FROM PUBLIC`, 및 prove effective
schema `CREATE` is false for runtime, PUBLIC, 및 every inherited membership. Only then execute
`SchemaSQL` as the migration owner under bounded `lock_timeout` 및 `statement_timeout`, run the
full catalog preflight, 및 grant runtime 만 schema `USAGE` 및 table
`SELECT,INSERT,UPDATE,DELETE`. 실행 constructor, conformance, 및 cleanup through the runtime pool.
Register bounded fixture context cancellation 및 pool/connection cleanup immediately 후 every
successful acquisition; pool cleanup must run 전에 Testcontainers termination, 및 cleanup
asserts `DBStats.In사용 == 0` 전에 close.
검증 runtime denial for schema/table create, alter, truncate, references, trigger, 및 grant.

Query catalogs to assert ordinary table relkind, exact column order/types/nullability, primary key
`namespace,bucket_key`, valid/ready expiry index, expected check definitions, migration owner,
runtime non-ownership/non-membership, direct/inherited/PUBLIC effective privilege boundaries,
RLS/forced-RLS false, 및 zero 사용자 triggers. In a rollbacked transaction, create incompatible
pre-기존 relations/indexes, enable RLS, 및 add a 사용자 trigger; prove the documented catalog
preflight rejects every variant. Also prove a read-만 transaction fails `Al낮음` without marking
the known PostgreSQL rejection commit-unknown.

For hostile-object cases, create a separate attacker role while schema CREATE is intentionally
available, pre-seed each conflicting relation/index/trigger, then revoke PUBLIC/attacker CREATE
전에 invoking bootstrap 및 preflight. 검증 deployment fails closed, ownership is never
adopted 또는 repaired, 및 없음 provider `Al낮음`/`Cleanup` traffic is issued.

- [ ] **단계 4: 실행 the serial backend proof**

```bash
go test -p 1 -count=1 ./ratelimit/sql -run 'TestPostgresRateLimiterConformance|TestMultiPool|TestFractional|TestCleanupAllowPoolContention|TestRuntimeRole|TestSchemaCatalog'
go test -race -p 1 -count=1 ./ratelimit/sql
go test -p 1 -count=10 ./ratelimit/sql -run 'TestPostgresRateLimiterConformance|TestMultiPoolExactAdmission|TestCleanupAllowPoolContention'
```

예상: mandatory conformance has 없음 skips; exact admission equals burst in every iteration;
race is clean; 보안/catalog cases pass. A one-off failure must be reproduced 및 explained;
do 아님 label it flaky merely because a rerun passes.

- [ ] **단계 5: 커밋 provider proof**

```bash
git add ratelimit/sql/conformance_test.go ratelimit/sql/stress_test.go ratelimit/sql/security_test.go ratelimit/sql/queries_test.go
git commit -m "test: prove PostgreSQL rate limiter contract"
```

예상: the commit contains 만 테스트 및 없음 relaxed timeout, skipped case, benchmark claim, 또는
production behavior change.

### 작업 7: 문서화 the SQL Provider 및 Compile-Checked Usage

**복잡도:** Medium 공개 API 및 bilingual operations guidance.

**Skills:** `bluetape-go-patterns` for Go docs; `bluetape-writer` for natural 한국어 parity;
`bluetape-diagram` for the 공유 sequence asset 및 rendered visual validation.

**파일:**
- 생성: `ratelimit/sql/README.md`
- 생성: `ratelimit/sql/README.ko.md`
- 생성: `ratelimit/sql/example_test.go`
- 생성: `ratelimit/sql/readme_test.go`
- 생성: `docs/images/readme-diagrams/postgres-ratelimit-token-bucket-sequence.svg`
- 생성: `docs/images/readme-diagrams/postgres-ratelimit-token-bucket-sequence.png`
- Modify: `ratelimit/sql/doc.go`
- Modify: exported-symbol Go docs in `ratelimit/sql/*.go`
- Modify: `ratelimit/README.md`
- Modify: `ratelimit/README.ko.md`

- [ ] **단계 1: 추가 RED example 및 documentation 계약 테스트**

`ExampleNew` must compile this ownership shape: 호출자 opens/closes pgx `*sql.DB`; migration owner
executes `SchemaSQL`; runtime calls `New`; `Al낮음` ignores every result when 오류 is non-nil;
commit-unknown is 아님 replayed; 호출자 invokes bounded `Cleanup` from its own scheduler 및 discards
the returned count on every cleanup 오류 rather than replaying the same batch. The example must
아님 require a live database to produce output.

`readme_test.go` checks both README files contain: the 공유
`postgres-ratelimit-token-bucket-sequence.png` asset, `SchemaSQL`, `New`, `Al낮음`, `Cleanup`,
`ErrConfigurationMismatch`, `ErrCommitUnknown`, 호출자-owned DB/migration/scheduler,
moderate-QPS/non-Redis-replacement guidance, fixed relation, least-privilege grants, primary-만
routing, configuration migration/namespace rotation, 없음 automatic replay, 및 language switches.
The SQL README assertions also require: cleanup 오류 returns count zero although up to `limit` rows
may already be deleted; retry advances current expired work 및 is 아님 idempotent batch replay;
local/Redis/SQL quota state is 아님 공유; simultaneous mixed-provider serving is prohibited because
each provider can grant a full burst; canaries use independent namespaces/cohorts; 및 cutover
quiesces the old provider 및 waits a full-refill window 또는 records an approved extra-burst budget.

- [ ] **단계 2: Observe RED**

```bash
go test -count=1 ./ratelimit/sql -run 'ExampleNew|TestReadmeContract'
```

예상: FAIL because the example 및 README files do 아님 exist.

- [ ] **단계 3: Write 영문 및 natural 한국어 패키지 documentation**

유지 the two README files source-equivalent. Include architecture 및 one-statement behavior;
installation/import; schema bootstrap 및 full catalog preflight; minimum grants; API/config table;
arbitrary byte key 및 cardinality constraints; cleanup scheduling/pressure controls; result/오류
handling; failure matrix; primary/proxy/HA/RPO requirements; observability without key/namespace
metric labels; configuration changes; provider cutover/rollback; 및 explicit unsupported ORM,
호출자 transaction, auto-migration, background cleanup, non-PostgreSQL, 및 높음-QPS claims.

업데이트 the parent `ratelimit` README pair 함께 a local/Redis/PostgreSQL selection table 및 공유
`ratelimit.OperationError`/`ErrCommitUnknown` example. Require the pair to repeat that provider
quota state is 아님 공유, mixed simultaneous serving can grant multiple full bursts, 및 safe
canary/cutover uses independent namespaces/cohorts plus quiesce-및-wait 또는 an approved extra-burst
budget. 유지 the benchmark chart N/A because issue #529 makes 없음 measured capacity comparison. 추가 a
source-backed sequence diagram using the repository best-practices reference. It must answer where
dispatch begins, where same-key atomicity is established, which outcomes debit quota, 및 why a
commit-unknown result must 아님 be replayed. Share one 영문-label PNG between both provider
READMEs 및 retain the matching SVG source.

- [ ] **단계 4: 검증 docs parity 및 commit**

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

예상: compile-checked example 및 README 계약 테스트 PASS; exported symbols have 영문 Go
docs; 영문/한국어 documents cover the same operational decisions; the SVG parses, its PNG is
rendered at 2x, diagram audits report 없음 geometry/endpoint/style failure, 및 the full-size PNG has
없음 clipped text, overlap, 또는 unreadable label.

### 작업 8: 업데이트 Public Index, Changelog, 및 Release Runbook

**복잡도:** Medium release-facing documentation.

**Skills:** `bluetape-writer` for 한국어 parity. The provider sequence asset is owned by 작업 7.

**파일:**
- Modify: `README.md`
- Modify: `README.ko.md`
- Modify: `CHANGELOG.md`
- Modify: `docs/release/v0.19.0-provider-conformance-runbook.md`

- [ ] **단계 1: 추가 the provider to both root indexes 및 0.19.0 changelog**

추가 adjacent `ratelimit/sql` rows describing PostgreSQL atomic token buckets for moderate-QPS,
database-만 deployments. 유지 영문 및 한국어 패키지 indexes aligned. 추가 an 영문
0.19.0 changelog bullet naming the 호출자-owned schema/cleanup boundary, 공유 root 오류
inspection, 및 non-Redis-replacement caveat.

- [ ] **단계 2: Extend the provider runbook 함께 executable deployment gates**

추가 exact commands/queries in this mandatory order: verify `public` schema ownership; revoke
`CREATE ON SCHEMA public FROM PUBLIC`; verify runtime/PUBLIC/inherited effective CREATE is false;
set bounded migration `lock_timeout`/`statement_timeout`; apply `SchemaSQL` as migration owner; run
the full catalog preflight; then grant minimum runtime DML. Fol낮음 함께 writable-primary endpoint checks
(`pg_is_in_recovery()=false`, `transaction_read_only=off`, server identity/timeline); serial
conformance/race commands; bounded cleanup; 및 rollback.

정의 deployment-recorded gates relative to a stable baseline: bounded-cardinality Al낮음
latency/outcome/오류 categories, `DBStats` wait/in-use, statement/row-lock latency, cleanup
count/duration/오류/backlog/oldest expiry, live/dead tuples, table/index size, autovacuum lag, 및
WAL growth. Require a predeclared consecutive-breach count 및 minimum canary observation window,
without inventing universal numeric thresholds. Prohibit key, namespace, DSN, endpoint, 및 raw
오류 in metric labels/logs; al낮음 redacted `KeyID` 만 in sampled 진단.

Specify an executable 호출자 scheduler 계약: cadence shorter than configured `IdleTTL`; a
fresh bounded context per run; `Cleanup` limit in `1..1000`; database lock/statement timeouts;
predeclared maximum batches 및 elapsed budget per run; jitter; 및 small bounded worker
concurrency. Pause cleanup when predeclared WAL, row-lock, pool-wait, 또는 autovacuum pressure gates
breach. On any cleanup 오류 the returned count is zero although up to `limit` rows may already be
deleted; retry advances the currently expired work 및 must 아님 claim to replay an idempotent batch.

Cutover uses an independent canary namespace/cohort, then quiesces the old provider 및 waits a
full-refill window 또는 records an approved extra-burst budget 전에 single-provider activation.
롤백 mirrors that boundary. Require controlled failover proof for old-writer fencing,
durability/RPO, 없음 statement replay, 및 commit-unknown 없음-replay 전에 production promotion.
롤백 retains the SQL relation, expiry index, 및 grants. Destructive table/grant removal is a
separate migration al낮음ed 만 후 a predeclared observation window shows zero SQL-provider
binary deployment 및 zero SQL-provider traffic; record a pre-removal rollback point 및 verify
the objects 및 privileges are absent 만 후 that migration succeeds.

- [ ] **단계 3: 추가 및 run a release-runbook 계약 테스트**

Extend `ratelimit/sql/readme_test.go` to read the release runbook 및 assert the presence 및 order
of these markers: 공개-schema ownership verification; `REVOKE CREATE ... FROM PUBLIC`; bounded
`lock_timeout`/`statement_timeout`; `SchemaSQL`; catalog preflight; runtime grants; writable-primary
checks; cleanup cadence/limit/run budget/pause/uncertain count; baseline-relative promotion window;
independent canary; old-writer fencing 및 RPO; rollback retention; zero-usage observation; 및
separate destructive migration. Compare marker indexes so the migration steps cannot be reordered.

실행:

```bash
go test -count=1 ./ratelimit/sql -run '^TestReleaseRunbookContract$'
```

예상: PASS 만 when the executable migration, cleanup, promotion, rollback, 및 removal
contracts are present in the required order.

- [ ] **단계 4: 검증 공개-doc parity 및 commit**

```bash
rg -n 'ratelimit/sql|PostgreSQL|moderate-QPS|ErrCommitUnknown|Cleanup' README.md README.ko.md CHANGELOG.md docs/release/v0.19.0-provider-conformance-runbook.md
go test -count=1 ./ratelimit/sql -run '^TestReleaseRunbookContract$'
git diff --check
git add README.md README.ko.md CHANGELOG.md docs/release/v0.19.0-provider-conformance-runbook.md ratelimit/sql/readme_test.go
git commit -m "docs: add SQL rate limiter rollout guidance"
```

예상: 모든 four 공개 surfaces discover the provider 및 its caveats; bilingual root entries
are aligned; the runbook contains measurable promotion, rollback, HA, 및 cleanup evidence.

### 작업 9: Final 검증, Acceptance Mapping, 및 Pre-PR 리뷰

**복잡도:** High integration gate; 없음 feature edits unless a failure is traced 및 repaired.

**Skills:** `verification-before-completion`, `requesting-code-review`, 및
`bluetape-full-feature` 단계 6-R 함께 모든 six review lenses plus main integration.

**파일:**
- 리뷰: 모든 files changed from `origin/develop...HEAD`
- 생성/update: the issue #529 단계 6-R review artifact under `docs/superpowers/reviews/`
- 생성 if durable learning warrants it: `docs/lessons/2026-07-13-issue-529-sql-rate-limiter.md`

- [ ] **단계 1: 실행 cheap/static gate**

```bash
gofmt -w ratelimit/errors.go ratelimit/errors_test.go ratelimit/redis/*.go ratelimit/sql/*.go
make fmt-check
make tidy-check
make vet
make lint
git diff --check
```

예상: 모든 commands exit 0, 없음 `go.mod`/`go.sum` drift, 및 없음 formatting changes remain.

- [ ] **단계 2: 실행 targeted, race, 및 repository gates serially**

```bash
go test -p 1 -count=1 ./ratelimit/... ./redis/...
go test -race -p 1 -count=1 ./ratelimit/sql ./ratelimit/redis
go test -p 1 -count=10 ./ratelimit/sql -run 'TestPostgresRateLimiterConformance|TestMultiPoolExactAdmission|TestCleanupAllowPoolContention'
make ci
```

예상: every command exits 0. `make ci` is the authoritative final local gate. Lost process
handles 또는 missing exit codes are 아님 evidence; rerun such a command from scratch.

- [ ] **단계 3: Map every acceptance criterion to evidence**

기록 a table in the 단계 6-R artifact mapping spec criteria 1..11 to exact 테스트, docs, 및
commands. Explicitly record N/A evidence: 없음 new runtime dependency, module/BOM/CI registration,
ORM/Spring/Exposed/coroutine/streaming/JDK-preview work, 및 benchmark/chart. 기록 the sequence
diagram as PASS 함께 the source/render paths, audit output, 및 full-size inspection evidence.

- [ ] **단계 4: 실행 the six review lenses 및 main integration**

리뷰 the exact `origin/develop...HEAD` diff for 성능, 안정성, 보안, 운영자/Ops,
개발자/API, 및 사용자/호출자. Fix every P0/P1, rerun the affected targeted proof 및 lens, 및
record P2/P3 repair 또는 explicit fol낮음-up rationale. Main integration checks SQL correctness,
오류 evidence integrity, docs parity, release readiness, 및 absence of unsupported claims.

Expected exit verdict: `P0=0 P1=0`, 없음 unresolved finding, 없음 placeholder, 및 clean status except
the review/lesson artifact being committed.

- [ ] **단계 5: 커밋 final evidence 및 prepare the PR handoff**

```bash
git add docs/superpowers/reviews docs/lessons
git commit -m "docs: record PostgreSQL rate limiter verification"
git status --short --branch
git log --oneline origin/develop..HEAD
```

예상: branch is clean 및 ahead of `origin/develop`; implementation, 테스트, docs, 및 review
evidence are committed. Stop 전에 push/PR if authorization has 아님 yet been given. When a PR is
authorized, copy issue #529 milestone `0.19.0`, assignee `debop`, 및 labels; end the PR body 함께
`## DoD Status`; wait for live CI/review 및 an explicit merge decision.

## 인수 추적성

| Spec acceptance criterion | Plan evidence |
|---|---|
| 1. Constructor-만 `New(*sql.DB, Options)` 및 root interface | 작업 2 constructor/interface 테스트 및 Go docs |
| 2. Caller-owned `SchemaSQL` 및 bounded `Cleanup` | Tasks 2, 5, 7, 및 8 |
| 3. One-statement server-time atomic refill/debit | 작업 3 query/integration proof |
| 4. Full `ratelimittest.Run` without skips | Tasks 4 및 6 |
| 5. Exact multi-pool stress 및 race | 작업 6 및 작업 9 repeated/race commands |
| 6. Cancellation/lost-response debit boundary | 작업 4 deterministic gates 및 작업 6 conformance |
| 7. Configuration mismatch quota 없음-op | 작업 3 row/config/`xmin` assertions |
| 8. Least privilege 및 schema/catalog 계약 | 작업 6 보안 fixture 및 작업 8 runbook |
| 9. 영문/한국어 docs, 공유 sequence diagram, root index, changelog, runbook | Tasks 7 및 8 |
| 10. Targeted/race/static/`make ci` verification | 작업 9 |
| 11. Cutover/rollback, HA fencing/RPO, telemetry gate | 작업 8 plus 운영자 review in 작업 9 |

## DoD 및 Conditional 리뷰 Coverage

- Spec 및 plan review artifacts must end at P0=0/P1=0 전에 작업 0/implementation.
- TDD RED 및 GREEN commands are named for every production task; commits fol낮음 each green slice.
- Performance covers one DB round trip, exact numeric cost, lock/pool pressure, expiry-index write
  amplification, bounded cleanup, 및 absence of unsupported benchmark claims.
- Stability covers pre/in/post dispatch cancellation, response loss, cleanup/Al낮음 races, pool 및
  container ownership, worker completion, 없음 retry/goroutine leaks, 및 repeated conformance.
- Security covers fixed qualified SQL, positional values, arbitrary bytes, hard bounds, redaction,
  schema ownership, effective privileges, RLS/triggers, 및 hostile pre-기존 objects.
- Operator/Ops covers migration, cleanup, telemetry, writable-primary routing, failover fencing/RPO,
  cutover/rollback, 및 removal 만 후 zero SQL-binary usage.
- Developer/API covers additive 패키지 shape, root 오류 compatibility, nil/zero safety, Go docs,
  example, 및 없음 new dependency.
- User/호출자 covers result-on-오류 handling, 없음 unknown replay, configuration migration,
  호출자-owned DB/schema/scheduler, provider selection, 및 unsupported behavior.
- New module/BOM/CI registration, Spring, Exposed, coroutines, streaming, JDK preview, 및 benchmark
  charts are N/A 함께 the concrete repository/scope evidence above; they are 아님 silently skipped.
  The sequence diagram is required evidence for the distributed execution 및 retry boundary.
