# Issue #319 SQL generator guidance review

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

날짜: 2026-06-26
브랜치: `docs/issue-319-sql-guidance`
Baseline: `2c6a7a2 feat: add inspectable sqlkit builders (#326)`
범위: documentation-only SQL generator and migration guidance.

## 검토

| Tier | Verdict | Evidence |
|---|---|---|
| Performance | PASS | Documentation-only change; no Go code or runtime dependency changed. |
| Stability | PASS | Examples write only under `.tmp/sql-guidance/*` and keep generated code out of normal package paths. |
| Security | PASS | Atlas example requires caller-supplied disposable dev database URL; no credentials are committed. |
| Operator/Ops | PASS | Atlas is documented as an external CI/CD or runbook boundary, not a hidden repository helper. |
| Developer/API | PASS | Selection matrix covers direct `database/sql`, `sqlkit`, sqlc, Jet, ent, Bun, GORM, goqu, and Atlas. |
| User/Caller | PASS | README and `sqlkit` README link to the new English/Korean guidance pages. |
| Integration | PASS | Guidance cross-links #100 research and #101 epic, and keeps core `sqlkit` dependency-free. |

P0=0 P1=0

## 검증

- `git diff --check`
- targeted `rg` for guidance links, `sqlc`, `Jet`, `Atlas`, and #100/#101 anchors
- Web evidence preserved in `bluetape4k-wiki` commit `d5d6828`

## 잔여 위험

The sqlc, Jet, and Atlas CLIs are intentionally not executed locally because the
examples require those external tools and, for Jet/Atlas, a caller-owned
database boundary. The commands are copied from current official documentation
patterns and isolated under `.tmp/sql-guidance`.
