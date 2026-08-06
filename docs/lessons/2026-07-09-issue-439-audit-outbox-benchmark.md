# Issue #439 Audit Outbox Benchmark 교훈

Audit와 outbox benchmark 작업은 raw output, result table, chart, written
interpretation이 함께 움직일 때만 완료로 본다.

- benchmark command와 raw output을 `docs/research/outputs/` 아래 보존해 follow-up
  adapter issue가 summary-only claim 대신 측정 evidence를 link할 수 있게 한다.
- in-memory audit repository row와 PostgreSQL/Testcontainers outbox row를 분리한다.
  둘은 다른 boundary를 측정하므로 interchangeable delivery option처럼 ranking해서는
  안 된다.
- Database-backed benchmark에는 명시적 opt-in env flag, serial command, fixture
  image/version, bounded operation context가 필요하다.
- single-digit `ns/op` 같은 불가능한 Testcontainers row는 timer hygiene failure로
  취급한다. setup, reset, seed work는 제외할 수 있지만 SQL operation 자체는
  `StartTimer` / `StopTimer` 안에 있어야 한다.
- local benchmark snapshot은 regression과 planning evidence이지 production
  throughput guarantee가 아니다. 별도 design issue 없이 delivery semantics, retry
  ownership, idempotency, dead-letter behavior를 바꾸는 근거로 쓰지 않는다.
