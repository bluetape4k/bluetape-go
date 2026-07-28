# Issue #408 Audit Publisher Adoption

Issue: #408
Milestone: 0.15.0
Decision date: 2026-07-07

## 결정

새 framework-style example API를 추가하지 않고 기존 audit example 안에 audit publisher adoption을 문서화한다.

example이 계속 소유하는 범위는 다음뿐이다.

- command-side `audit.Entry` creation.
- source-state mutation 전 explicit `audit.Repository` append.
- repository boundary를 통한 history query.
- `EntrySink`로 replay.

production adoption은 명시적 handoff로 문서화한다.

1. caller-owned source write와 `audit/sqloutbox.Store.Enqueue`가 같은 `*sql.Tx`를 공유한다.
2. `sqloutbox.NewRelay`가 relay를 만든다.
3. `Relay.RunOnce` 또는 `Relay.Run`이 durable row를 claim한다.
4. `sqloutbox.Publisher.Publish`가 각 `sqloutbox.Record`를 받는다.
5. publisher adapter는 duplicate-safe downstream consumer를 위해 `Record.EventID`와 `Record.IdempotencyKey`를 보존한다.

## Source Checks

README 이름은 다음 source와 대조했다.

- `audit/sqloutbox/store.go`: `Store.Enqueue`, `Record.EventID`, `Record.IdempotencyKey`.
- `audit/sqloutbox/relay.go`: `Publisher`, `NewRelay`, `Relay.RunOnce`, `Relay.Run`.
- `audit/sqloutbox/sqloutboxtest/publisher.go`: `RecordingPublisher`, `NewRecordingPublisher`, `WithFailures`.

## Operator Contract

documentation은 operational contract를 계속 보이게 해야 한다.

- delivery는 at-least-once다.
- non-cancellation publish error는 retry/dead-letter state를 drive한다.
- context cancellation 및 deadline은 retry/dead-letter state를 mutate하지 않고 worker lifecycle을 멈춘다.
- persisted failure text는 operator-facing이므로 publisher error는 bounded 및 redacted여야 한다.
- duplicate delivery는 expected이며 consumer가 idempotency를 소유한다.

## Diagram Decision

두 번째 package-local diagram을 추가하지 않고 기존 `audit-example-service-flow` diagram을 갱신한다. 기존 image가 example service
narrative를 이미 소유하고 있으며, #408은 durable outbox row에서 relay, publisher adapter, downstream deduplication으로 이어지는
adoption path만 확장한다.

## Cross-Repo Follow-Up

workshop-facing runnable coverage는
[bluetape-go-workshop#57](https://github.com/bluetape4k/bluetape-go-workshop/issues/57)에서 추적한다.
