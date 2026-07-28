# Issue #407 Audit Publisher Adapter

Issue: #407
Milestone: 0.15.0
Decision date: 2026-07-07

## 결정

첫 audit publisher adapter는 `audit/sqloutbox/sqloutboxtest`로 구현한다. 이 package는 test, local example,
workshop adoption을 위한 deterministic helper package다.

의도적으로 durable broker adapter가 아니다. 이 package는 다음 helper로 기존 `sqloutbox.Publisher` contract를 구현한다.

- published output을 assert하지 않는 store/relay test용 `DiscardPublisher`.
- 작은 function adapter용 `PublisherFunc`.
- attempt-order assertion과 deterministic per-event failure injection용 `RecordingPublisher`.

## Source Context

이전 #405 research는 audit outbox contract와 broad broker candidate를 비교한 뒤 첫 adapter 방향을 선택했다. 이후 #406 relay
contract가 runtime behavior를 고정했다.

- at-least-once delivery.
- caller-owned context cancellation은 retry/dead-letter state가 되면 안 된다.
- non-cancellation publish error는 bounded failure text로 저장된다.
- duplicate publish attempt는 허용되며 `Record.EventID`와 `Record.IdempotencyKey`를 보존해야 한다.
- durable broker topology, authentication, retention, replay, redaction, consumer idempotency는 caller 또는 later-adapter
  responsibility로 남는다.

## Candidate Modules

| Candidate | Decision | Reason |
|---|---|---|
| `audit/sqloutbox/sqloutboxtest` | Selected | durable transport support를 암시하지 않고 현재 `Publisher` interface를 구현하는 좁은 package. |
| `audit/sqloutbox/publisher` | Rejected | 너무 generic해서 test/example support가 아니라 production adapter surface처럼 읽힌다. |
| `audit/publisher` | Rejected | storage-neutral audit value와 SQL outbox relay semantics를 흐린다. |
| `audit/sqloutbox/kafka` 또는 유사 broker package | Deferred | #407 범위를 넘는 topology, auth/TLS, retry/replay, idempotency, operator contract가 필요하다. |

## Diagram Decision

이 package에는 새 README diagram이 필요하지 않다. helper는 기존 `sqloutbox.Publisher` participant를 넘는 새 runtime topology나
sequence를 추가하지 않는다. source-backed reader question은 README prose와 기존 `audit/sqloutbox` class contract 및 relay sequence
diagram으로 답할 수 있다.

## Test Contract

implementation은 다음을 증명해야 한다.

- context cancellation과 nil helper surface가 bounded다.
- `PublisherFunc`가 function-owned behavior를 보존한다.
- `RecordingPublisher`가 defensive snapshot을 반환한다.
- failure injection이 retry attempt 전반에서 deterministic하다.
- concurrent `Publish` call이 `GoroutineStressTester` 아래 race-free로 남는다.
- `sqloutbox.Relay`가 PostgreSQL Testcontainers와 함께 helper를 통해 retry 및 dead-letter behavior를 drive할 수 있다.
