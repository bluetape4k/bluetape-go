# Issue #346 Plan: SQL Audit Outbox

> 한국어 운영 요약: 이 계획 문서는 사용자 협업용 실행 계획이다. 아래 원문에 포함된 명령, 경로, API 이름, issue/PR 번호, branch 이름, code block, test output은 추적성과 재현성을 위해 그대로 보존한다. 작업 순서, 위험, 검증, 롤백 판단은 한국어 독자가 바로 실행 경계를 이해할 수 있도록 이 메모를 우선 적용한다.
> 추가 한국어 요약: 이 문서의 실행 판단은 기존 순서를 따르며, 변경자는 작업 표와 검증 목록을 먼저 확인한 뒤 관련 테스트를 실행한다. 영어로 남은 항목은 코드 식별자 또는 재현 증거다.\n

## Steps

1. Add failing PostgreSQL-backed tests for store and relay behavior.
2. Implement `audit/sqloutbox` with explicit `sqlkit.Session` boundaries.
3. Add visible PostgreSQL DDL and `FOR UPDATE SKIP LOCKED` claim SQL.
4. Implement retry/dead-letter transitions and a cancellable relay loop.
5. Document public boundaries in package, audit, root, and changelog docs.
6. Run targeted package tests, race tests, formatting, vet, and local CI where
   practical.

## Design Decisions

- `Store.Enqueue` accepts `sqlkit.Execer` so callers can pass `*sql.Tx` and own
  source-write coupling.
- `Store.Claim` accepts `sqlkit.Session` because claiming needs query and update
  behavior in one statement.
- Claim SQL sets a bounded lease, can reclaim expired claimed rows, and excludes
  later revisions while lower revisions for the same aggregate are still pending
  or claimed.
- Publish/failure marking checks the current claim attempt so stale workers do
  not mutate reclaimed rows.
- `RunOnce` supports scheduler-owned polling; `Run` supports service-owned
  worker lifecycle with context cancellation.

## Risk Controls

- Bound stored `entry_json` before calling `audit.DecodeEntryJSON`.
- Keep failure state to bounded text and avoid storing payload copies as error
  metadata.
- Leave migrations explicit through optional `CreateSchema`; production rollout
  remains application-owned.
