# Issue #346 Review: SQL Audit Outbox

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

## 판정

P0=0 P1=0

## 발견 사항

- No P0/P1 evidence-backed defects found in the current implementation window.
- Transaction ownership remains caller-visible: store methods accept
  `sqlkit.Execer` or `sqlkit.Session`; no hidden source-write transaction is
  created.
- Delivery semantics are explicit at-least-once. `Relay` marks failed attempts
  for retry or dead-letter and does not promise exactly-once delivery.
- Stored audit JSON is bounded before decode through `Options.MaxEntryBytes`.
- PostgreSQL claim SQL uses `FOR UPDATE SKIP LOCKED`, claim leases, expired
  claim reclamation, and excludes later aggregate revisions while earlier
  revisions remain pending or claimed.
- Publish and failure marking require the current claim attempt, preventing
  stale workers from overwriting a reclaimed row.

## 잔여 위험

- `CreateSchema` is intended for explicit setup and tests. Production migration
  rollout, table ownership, retention, and operator replay tooling remain
  application responsibilities.
- Failure text is bounded but not a general-purpose PII scrubber; callers must
  keep publisher errors redacted.
- Broker-specific publisher adapters are intentionally outside this PR.

## 증거

- `go test -count=1 ./audit/sqloutbox`
- `make ci`
