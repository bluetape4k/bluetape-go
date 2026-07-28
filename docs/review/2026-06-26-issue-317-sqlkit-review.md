# Issue 317 sqlkit Step 6-R Review

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

## 범위

- Baseline: `a21cc85` (`develop`)
- Changed package: `sqlkit`
- Supporting docs: root README/README.ko.md, `sqlkit` README/README.ko.md,
  #100 research note milestone wording

## 7-Tier 발견 사항

| Tier | Verdict | Evidence |
|---|---|---|
| Performance | PASS | `QueryOptional`/`QueryOne` now read at most two rows before returning `ErrTooManyRows`; this avoids unbounded cardinality checks. |
| Stability | PASS | `WithTx` commits only on nil callback error, rolls back on callback error, and preserves begin/commit/rollback errors. `QueryAll` closes rows through defer on success and failure. |
| Security | PASS | No SQL construction, interpolation, credentials, auth, or filesystem behavior added. SQL and args remain caller-owned. |
| Operator/Ops | PASS | PostgreSQL Testcontainers integration proves commit and rollback with a real driver path. No pool lifecycle ownership or background goroutines added. |
| Developer/API | PASS | API is small and Go-native: `WithTx`, `QueryAll`, `QueryOptional`, `QueryOne`, `ScanOne`, plus narrow `database/sql` interfaces. No ORM or code generation surface. |
| User/Caller | PASS | README and README.ko.md explain selection guide, direct SQL ownership, non-goals, and when to use direct `database/sql`, `pgx`, sqlc, Jet, ent, Bun, GORM, or goqu. |
| Integration | PASS | Targeted tests, race test, vet, lint, fmt, and full `go test ./...` are green. |

## 차단 항목 검토

- P0: 0
- P1: 0

## 검토 중 수정됨

- P1 candidate: `QueryOne`/`QueryOptional` originally delegated through
  `QueryAll`, so cardinality checks could read an unbounded result set. Fixed
  by adding a bounded internal row mapper that stops after two mapped rows.

## 검증

- `git diff --check`
- `go test -count=1 ./sqlkit`
- `go test -race -count=1 ./sqlkit`
- `make fmt-check`
- `make vet`
- `make lint`
- `go test -count=1 ./...`

## 잔여 위험

- `make tidy-check` should be rerun after committing because it checks
  `go.mod`/`go.sum` drift against the working tree; this branch intentionally
  records the new indirect `github.com/jackc/puddle/v2` tidy entry required by
  `pgx/v5/stdlib`.
