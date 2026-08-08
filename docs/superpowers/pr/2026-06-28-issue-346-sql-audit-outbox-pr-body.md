Closes #346.

## 요약

- PostgreSQL 기반 enqueue, claim lease, claim-attempt-guarded publish/failure
  mark, retry, dead-letter, relay API를 제공하는 `audit/sqloutbox`를 추가한다.
- caller가 전달한 `database/sql` session을 받아 transaction ownership을
  명시적으로 유지한다.
- at-least-once delivery, aggregate별 claim ordering, idempotency, migration,
  redaction, operator 경계를 문서화한다.

## 검증

- `go test -count=1 ./audit/sqloutbox`
- `make ci`

## DoD Status

- [x] 이슈 #346 범위를 구현했다.
- [x] store, claim lease, stale mark rejection, relay 동작을 대상으로
      PostgreSQL Testcontainers coverage를 추가했다.
- [x] concurrent claim 및 relay lifecycle에 stress/cancellation helper를
      사용했다.
- [x] README, changelog, spec, plan, review, lesson, PR 산출물을 갱신했다.
- [x] P0=0 P1=0 review를 기록했다.
- [x] 로컬 CI를 통과했다.
