# Issue #346 교훈: SQL audit outbox

## 결정

첫 durable audit publisher boundary는 `audit/sqloutbox`의 PostgreSQL-backed outbox
store와 relay다.

## 교훈

- Source transaction ownership은 보이게 둔다. Store method에 `*sql.Tx` 또는 `*sql.DB`를
  전달하는 방식이 transaction hook을 outbox repository 안에 숨기는 방식보다 명확하다.
- Per-aggregate ordering은 단순한 `order by`가 아니라 claim-time protection이 필요하다.
  이전 revision이 pending 또는 claimed 상태로 남아 있으면 이후 revision은 제외한다.
- `RunOnce`와 `Run`은 owner가 다르다. Scheduler는 batch 하나를 poll할 수 있고, service
  worker는 context cancellation까지 실행할 수 있다.
- Retry/dead-letter state는 outbox row에 둔다. Broker-specific adapter logic은 나중에
  추가한다.

## 후속 작업

- Concrete broker publisher adapter는 SQL outbox contract가 example을 통해 검증된 뒤에만
  추가한다.
- Retention, replay tooling, migration rollout, PII policy는 명시적인
  application/operator responsibility로 남긴다.
