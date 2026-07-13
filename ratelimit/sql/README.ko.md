# ratelimit/sql

[English](README.md) | [한국어](README.ko.md)

`ratelimit/sql`은 PostgreSQL을 사용하는 moderate-QPS, database-only 배포용 token
bucket입니다. 여러 application instance가 이미 하나의 writable PostgreSQL primary를
공유하고 rate limit만을 위해 Redis를 운영하기 어려울 때 적합합니다. High-QPS 또는
latency가 중요한 트래픽에서 Redis를 대체하는 구현은 아닙니다(not a Redis replacement).

각 `Allow` 호출은 하나의 `INSERT ... ON CONFLICT DO UPDATE ... RETURNING` 문으로
refill, 검사, token 소비, 결과 반환을 수행합니다. PostgreSQL server time을 사용하며
같은 key의 호출은 primary-key row lock으로 직렬화됩니다. `Cleanup`은
`FOR UPDATE SKIP LOCKED`를 포함한 하나의 `DELETE` CTE를 실행합니다. `limit`은 lock과
delete하는 row 수를 제한하고 expiry index를 통해 가장 이른 expired row에서 탐색을
멈춥니다. 이미 lock된 row는 추가로 scan하고 건너뛸 수 있으므로 caller의 실행 시간과
pressure budget은 여전히 필요합니다.

## 설치

```go
import sqlratelimit "github.com/bluetape4k/bluetape-go/ratelimit/sql"
```

## 소유권과 Schema

Database pool, migration, cleanup scheduler는 caller-owned입니다. `New`는 I/O나
migration을 수행하지 않고 pool을 닫지 않습니다. Runtime traffic을 시작하기 전에
migration owner로 고정된 `SchemaSQL` relation을 적용합니다.

```go
if _, err := migrationDB.ExecContext(ctx, sqlratelimit.SchemaSQL); err != nil {
    return err
}
```

고정 relation은 `public.bluetape_ratelimit_buckets`이고 고정 expiry index는
`public.bluetape_ratelimit_buckets_expires_at_idx`입니다. Custom schema/table name,
ORM mapping, caller transaction은 지원하지 않습니다. Runtime grant 전에 column의
name/type/nullability/check, `(namespace, bucket_key)` primary key, expiry index, owner를
확인하고 예상하지 않은 trigger, row-level security, policy가 없는지 catalog preflight로
검증해야 합니다.

Migration role과 runtime role을 분리합니다. least-privilege runtime role에는 schema
usage와 table DML만 부여합니다.

```sql
grant usage on schema public to app_runtime;
grant select, insert, update, delete
on table public.bluetape_ratelimit_buckets to app_runtime;
```

Runtime role에 table ownership, schema `CREATE`, `TRUNCATE`, `REFERENCES`, `TRIGGER`를
부여하지 않습니다. Role inheritance나 `PUBLIC`을 통해 schema 생성 권한이 다시
생기지 않았는지도 확인합니다.

## 사용법

`Allow`에는 caller-owned deadline을 전달합니다. Package는 dispatch 뒤에 자체 timeout이나
retry를 추가하지 않습니다.

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
    // 오류가 있으면 result를 버리고 commit-unknown 여부를 추측하지 않습니다.
    return err
}
if !result.Allowed {
    return fmt.Errorf("retry after %s", result.RetryAfter)
}
```

| Option | 계약 |
|---|---|
| `Namespace` | 공백이 아니고 최대 128 byte이며 기본값은 `default`입니다. |
| `RatePerSecond` | 양의 finite rate이며 내부에서는 microtoken으로 표현합니다. |
| `Burst` | 양의 정수 token capacity입니다. |
| `IdleTTL` | 최소 한 번의 full-refill window 이상이어야 하며 기본값은 최소 1분이자 두 refill window 이상입니다. |
| `MaxKeyBytes` | `1..1024`이고 기본값은 512입니다. |

Key는 Go string으로 전달되어 `bytea`에 저장되므로 NUL과 invalid UTF-8을 포함한
임의의 byte를 보존합니다. `MaxKeyBytes`를 제한하고 identity source를 인증하며 새
namespace 생성과 전체 row cardinality를 통제해야 합니다. Raw key와 namespace를
metric에 넣지 않습니다.

## 결과와 오류

요청 거절은 error가 아닌 정상 result입니다. Error가 하나라도 있으면 `Result` 전체를
버립니다. Provider-neutral interface로 진단 정보를 확인할 수 있습니다.

```go
result, err := limiter.Allow(ctx, key, 1)
if err != nil {
    var operation ratelimit.OperationError
    if errors.As(err, &operation) {
        recordBoundedCategory(operation.Family(), operation.Operation())
        // operation.KeyID()는 sampled diagnostic용이며 metric label이 아닙니다.
    }
    if errors.Is(err, ratelimit.ErrCommitUnknown) {
        // 한 번 debit됐을 수 있으므로 no automatic replay 원칙을 지킵니다.
    }
    return ratelimit.Result{}, err
}
```

| 조건 | 의미와 caller 조치 |
|---|---|
| `ErrConfigurationMismatch` | 기존 row의 rate, burst, idle TTL이 다릅니다. 해당 namespace traffic을 멈추고 configuration migration 또는 namespace rotation을 수행합니다. |
| `ErrCommitUnknown` | 전송한 statement가 commit됐을 수 있습니다. Result를 버리고 자동 재실행하지 않습니다. |
| `sqlratelimit.OpError` | Redacted typed failure입니다. 원인은 `errors.Is`/`errors.As`로 확인할 수 있습니다. |
| Commit unknown이 아닌 context/database error | 성공 result가 없으므로 원인과 service policy에 따라 처리합니다. |

Raw error, SQL, DSN, endpoint, key, namespace를 log에 남기지 않습니다. `KeyID`는
sampled diagnostic correlation에만 쓰며 metric label로 사용하지 않습니다.

## 제한된 Cleanup

Caller-owned scheduler가 매 실행마다 새 bounded context로 `Cleanup`을 호출합니다.
실행 주기는 `IdleTTL`보다 짧아야 합니다. `1..1000` 범위의 limit, bounded database
timeout, jitter, 작은 worker 수, 실행별 batch/time budget을 사용합니다.

```go
cleanupCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
defer cancel()
deleted, err := limiter.Cleanup(cleanupCtx, 100)
if err != nil {
    // Cleanup은 count zero를 반환하지만 up to `limit` rows가 이미 삭제됐을 수
    // 있습니다. 현재 expired work를 다시 처리하며 not an idempotent batch replay입니다.
    return err
}
recordCleanupCount(deleted)
```

WAL growth, row-lock latency, pool wait, autovacuum lag가 미리 정한 pressure gate를
넘으면 cleanup을 일시 중지합니다. Cleanup은 maintenance 작업이며 `Allow`의 correctness
조건은 아닙니다.

## PostgreSQL과 HA 경계

- `Allow`, `Cleanup`, catalog check, reconciliation probe는 모두 하나의 writable
  primary-only endpoint로 보냅니다. Read replica와 transaction replay proxy는 지원하지
  않습니다.
- Promotion 전에 `pg_is_in_recovery() = false`, `transaction_read_only = off`, server
  identity와 HA timeline을 확인합니다.
- Controlled failover에서 old-writer fencing, durability/RPO, statement replay 부재를
  입증합니다. Commit-unknown operation은 재실행하지 않습니다.
- Bounded outcome/error category, Allow latency, `sql.DBStats`, statement/row-lock latency,
  cleanup duration/count/error/backlog, oldest expiry, live/dead tuple, relation/index size,
  autovacuum lag, WAL growth를 관측합니다.

## 설정 변경과 Provider 전환

Rate, burst, idle TTL은 각 bucket에 저장됩니다. 값을 즉시 바꾸면
`ErrConfigurationMismatch`가 발생합니다. Traffic을 quiesce한 뒤 계획된 configuration
migration을 수행하거나 namespace rotation으로 배포합니다.

Local, Redis, SQL 사이에 quota state is not shared라는 경계가 있습니다. Provider를
동시에 섞어 서비스하면 각각이 full burst를 허용하여 multiple full bursts가 발생하므로
금지합니다. Canary에는 independent namespace와 independent cohort를 사용합니다. 전환과
rollback 때는 old provider를 quiesce하고 full-refill window를 기다린 뒤 정확히 하나의
새 provider를 활성화합니다. 겹치는 구간이 꼭 필요하면 이를 감당할
approved extra-burst budget을 사전에 기록합니다.

Binary rollback 중에는 SQL table, index, grant를 유지합니다. 미리 정한 observation
window 동안 SQL provider binary와 traffic이 모두 0임을 확인한 다음 별도 migration으로만
제거합니다.

## 지원하지 않는 범위

Auto-migration, background cleanup, caller-owned transaction 참여, custom relation, ORM
integration, reservation, waiting/FIFO fairness, distributed multi-primary semantics,
PostgreSQL 이외 database, high-QPS 성능 보장은 지원하지 않습니다.

## 테스트

Test는 Docker가 필요하며 PostgreSQL Testcontainer를 직렬로 실행합니다.

```bash
go test -count=1 ./ratelimit/sql
go test -race -p1 -count=1 ./ratelimit/sql
```

Production rollout에는 bilingual
[v0.19.0 provider runbook](../../docs/release/v0.19.0-provider-conformance-runbook.md)도
적용해야 합니다.
