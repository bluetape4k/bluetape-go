## 요약

- `audit` package에 `AggregateID`, `Revision`, `DomainEvent`, `Entry`,
  `SnapshotMetadata`, `ChangeMetadata`, `AggregateRecorder`, `History`를
  추가한다.
- 검증 항목: schema version, aggregate/revision consistency, missing snapshot payload,
  metadata taxonomy, redacted validation error, duplicate event/idempotency
  처리, pending-event ack 동작을 포함하여 constructor와 JSON decode 경로를
  검증한다.
- bilingual audit README, root README 항목, CHANGELOG/WIP 갱신, Step 6-R
  review 증거, lesson을 추가한다.

## 경계

- 다음은 #56 범위에서 제외한다: Repository interface, history query storage,
  outbox publisher, SQL/Redis/Kafka/NATS adapter, JaVers 방식 object diffing.
- Recorder pending event는 process 내부 retry 보조 수단일 뿐이다. 이후
  repository 및 outbox issue는 event를 ack하기 전에 durable transaction,
  rollback, outbox, reconciliation semantics를 제공해야 한다.

Closes #56

## 검증

- `go test -count=1 ./audit`
- `go test -race -count=1 ./audit`
- `go test -run '^$' -bench 'BenchmarkAggregateRecorder(Record|PendingEvents|AckThrough)' -benchmem ./audit`
- `make lint`
- `make ci`
- `git diff --check`
- Step 6-R review: `docs/superpowers/reviews/2026-06-27-issue-56-audit-model-step-6r-review.md`, `P0=0 P1=0`

## DoD Status

| 게이트 | 상태 | 증거 |
|---|---|---|
| 이슈 범위 | PASS | #56 model/recording/history 기본 기능을 구현하고 repository/outbox/diffing을 범위에서 제외. |
| Test | PASS | 대상 audit test, race test, benchmark 명령, `make ci` 통과. |
| Review | PASS | snapshot payload P1을 수정한 뒤 `P0=0 P1=0`으로 Step 6-R seven-tier review 종료. |
| Docs | PASS | `audit/README.md`, `audit/README.ko.md`, root README pair, CHANGELOG, WIP, review, lesson 갱신. |
| PR metadata | PASS | Assignee, milestone, label이 #56과 일치. |
