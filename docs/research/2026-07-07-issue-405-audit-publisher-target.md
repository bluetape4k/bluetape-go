# Issue #405 Audit Publisher Adapter Target

Issue #405는 0.15.0 audit publisher track의 첫 audit publisher adapter target을 선택한다.

## 결정

첫 adapter slice로 standard-library 기반 deterministic test/discard publisher package를 선택한다.

follow-up implementation issue #407은 좁게 유지한다.

- #406에서 최종 path를 정한 작은 audit publisher helper package를 추가하고, broker dependency 없이 기존
  `audit/sqloutbox.Publisher` boundary를 구현한다.
- unit test 및 runnable example을 위한 discard publisher, function adapter, recording publisher를 포함한다.
- context cancellation, duplicate publish attempt, failure injection, `sqloutbox.Relay`를 통한 retry/dead-letter handoff,
  deterministic shutdown behavior를 증명한다.
- 이 package는 contract test, local example, workshop adoption용이며 durable transport가 아니라고 문서화한다.

Kafka, NATS, Redis Streams는 later transport adapter로 남긴다. publisher contract와 example shape가 안정된 뒤
`audit/sqloutbox` record를 consume해야 한다.

## Evidence

- `docs/research/2026-06-27-issue-58-audit-outbox-design.md`는 첫 durable audit publisher boundary로 SQL outbox store와
  relay를 선택했다. Kafka, NATS, Redis Streams는 durable outbox contract가 증명될 때까지 명시적으로 미뤘다.
- `audit/sqloutbox`는 proven relay boundary를 제공한다. `Publisher`는 claimed `Record` 하나를 publish하고, `RunOnce`는
  publish success/failure를 mark하며, `Run`은 cancellation-aware worker loop를 소유한다.
- `audit/sqloutbox/README.md`는 at-least-once delivery, duplicate publish attempt, stable event ID/idempotency-key dedupe,
  retry state, caller-owned broker topology를 문서화한다.
- `examples/audit/README.md`는 현재 example을 framework-free로 유지하고 production code가 example `EntrySink`를
  `audit/sqloutbox`로 adapt할 수 있다고 설명한다.
- `bluetape-go-workshop`의
  `docs/superpowers/research/2026-06-23-issue-48-audit-aws-sql-candidates-research.md`는 durable outbox behavior를 먼저
  deterministic하게 유지하고, upstream package가 요구하기 전까지 Kafka-backed publisher test를 미루라고 한다.

## Candidate Comparison

| Candidate | Decision | Rationale |
|---|---|---|
| Standard-library test/discard publisher | Select first | broker lifecycle, Docker cost, topology policy 없이 publisher contract를 증명한다. #406/#407이 retry, duplicate, cancellation, shutdown behavior를 한 PR에서 실행 가능한 example로 제공할 수 있다. |
| Kafka publisher | Defer | ordered event fanout에는 강한 eventual transport지만 startup, partition/topic policy, producer ack configuration, schema/header choice, workshop Testcontainers cost가 첫 contract를 흐린다. |
| NATS publisher | Defer | fixture cost가 낮고 fanout semantics가 유용하지만 subject naming, JetStream vs core NATS durability, ack behavior, replay policy에는 settled publisher envelope이 먼저 필요하다. |
| Redis Streams publisher | Defer | repo에서 Redis가 흔하지만 stream trimming, consumer group, pending-entry recovery, projection vs durable audit semantics를 contract 안정 전 과장하기 쉽다. |

## Contract Consequences

- 첫 implementation은 generic message-bus abstraction을 도입하지 않는다.
- 첫 implementation은 기존 `sqloutbox.Record` shape와 `context.Context`에만 의존한다.
- transport adapter는 audit history store가 되면 안 된다.
- broker-specific package는 나중에 추가될 때 topology, retention, authentication, TLS, consumer idempotency,
  replay/poison-message ownership을 문서화해야 한다.

## 이 slice에서 거부한 항목

- Kotlin workshop에 transactional outbox example이 있다는 이유만으로 Kafka를 선택하지 않는다. Go workshop note는 broker
  emulation을 deterministic outbox behavior 뒤에 둔다.
- Redis fixture가 이미 있다는 이유만으로 Redis Streams를 선택하지 않는다. later issue가 Redis를 explicit audit source로
  선택하기 전까지 Redis stream semantics는 projection/fanout semantics다.
- fixture cost가 낮다는 이유만으로 NATS를 선택하지 않는다. 낮은 비용의 broker도 #406 contract 안정 전에 subject 및 delivery
  semantics를 추가한다.

## Follow-Up Scope

#407은 선택한 standard-library publisher helper를 먼저 구현한다. #407과 #408이 public docs와 example을 증명한 뒤, workshop 또는
application scenario가 real broker를 필요로 할 때만 별도 transport-specific issue를 만든다.

## Validation Plan

- documentation PR: `git diff --check`.
- traceability: `rg -n "Issue #405|test/discard publisher|Kafka publisher|Redis Streams publisher" docs/research docs/lessons`.
- 이 slice에는 code behavior change가 없으므로 #407 전까지 package test는 필요하지 않다.
