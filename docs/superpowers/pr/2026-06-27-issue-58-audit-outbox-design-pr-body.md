Closes #58.

## 이유

audit package에는 이제 model 및 repository/history contract가 있지만, Kafka,
NATS, Redis Stream, SQL adapter code를 추가하기 전에 outbox publisher 작업의
durable boundary를 결정해야 했다.

## 변경 사항

- 첫 번째 구체적인 durable publisher 대상으로 SQL outbox store와 relay
  contract를 선택한다.
- SQL audit outbox store 및 relay contract 구현 후속 issue #346을 생성한다.
- at-least-once delivery, event ID/idempotency key dedupe, retry와 dead-letter
  semantics, aggregate별 ordering 한계, serialization,
  application-owned transaction 책임을 문서화한다.
- audit README, research index, WIP, CHANGELOG, review, lesson 산출물을
  갱신한다.

## 보류

- Kafka, NATS, Redis Streams, RabbitMQ, Redpanda, Pulsar, direct Redis audit
  storage는 #346이 durable outbox contract를 증명하거나 구체적인 example이
  transport adapter를 요구할 때까지 보류한다.

## 검증

- `git diff --check`
- `rg -n '#346|2026-06-27-issue-58-audit-outbox-design|P0=0 P1=0|SQL outbox' ...`
- `go test -count=1 ./audit ./audit/audittest`
- `go test -race -count=1 ./audit ./audit/audittest`

## 검토

- 7-Tier 로컬 review 산출물:
  `docs/review/2026-06-27-issue-58-audit-outbox-design-review.md`
- 검토 결과: `P0=0 P1=0`

## DoD Status

| 단계 | 상태 | 증거 |
|---|---|---|
| 이전 PR 병합 | PASS | #345를 `de878226904b8b83ec3a4678478983ce81d41c50`으로 병합; #58 worktree 전에 root develop 동기화. |
| Route 및 discovery | PASS | #58 acceptance, #41 audit 범위, #57 lesson, #100 SQL 경계, audit README, 현재 0.9.0 milestone issue 검토. |
| 후속 issue | PASS | milestone `0.9.0`, assignee `debop`, label `type: task`, `priority: p1`, `area: audit`로 #346 생성. |
| Design 산출물 | PASS | `docs/research/2026-06-27-issue-58-audit-outbox-design.md`, review, lesson, README/WIP/CHANGELOG/index 갱신. |
| Review 게이트 | PASS | `docs/review/2026-06-27-issue-58-audit-outbox-design-review.md`에 `P0=0 P1=0` 기록. |
| 검증 | PASS | `git diff --check`; 대상 `rg`; `go test -count=1 ./audit ./audit/audittest`; `go test -race -count=1 ./audit ./audit/audittest`. |
