# Issue #408 audit publisher adoption 교훈

일자: 2026-07-07
범위: `examples/audit`, `audit/sqloutbox`

## 교훈

Audit publisher adoption은 새 abstraction이 아니라 example boundary documentation에
속한다. Durable path는 `Store.Enqueue -> Relay.RunOnce/Run -> Publisher.Publish ->
downstream dedupe`로 읽혀야 하며, transaction ownership과 duplicate delivery가 보여야 한다.

## 패턴

- `examples/audit`는 작고 runnable하게 유지한다.
- Example replay path와 production `Store.Enqueue` 사이의 경계는 `EntrySink`로 둔다.
- Relay lifecycle 선택을 문서화한다. `RunOnce`는 scheduler-owned polling용이고, `Run`은
  service-owned worker용이다.
- Publisher adapter prose에서는 stable `Record.EventID`와 `Record.IdempotencyKey`를
  보존한다.
- Runnable cross-repo example이 필요하면 workshop follow-up issue를 연결한다.
- README diagram이 이미 reader question을 소유한다면 parallel diagram을 추가하지 말고 그
  단일 asset만 update하고 rerender한다.

## 후속 작업

Durable broker adapter에는 topology, authentication/TLS, retention, replay, redaction,
consumer idempotency를 위한 별도 package와 operator runbook이 여전히 필요하다.
