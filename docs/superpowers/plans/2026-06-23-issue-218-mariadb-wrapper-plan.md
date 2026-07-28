# MariaDB Testcontainers Wrapper Implementation Plan

> 한국어 운영 요약: 이 계획 문서는 사용자 협업용 실행 계획이다. 아래 원문에 포함된 명령, 경로, API 이름, issue/PR 번호, branch 이름, code block, test output은 추적성과 재현성을 위해 그대로 보존한다. 작업 순서, 위험, 검증, 롤백 판단은 한국어 독자가 바로 실행 경계를 이해할 수 있도록 이 메모를 우선 적용한다.
> 추가 한국어 요약: 이 문서의 실행 판단은 기존 순서를 따르며, 변경자는 작업 표와 검증 목록을 먼저 확인한 뒤 관련 테스트를 실행한다. 영어로 남은 항목은 코드 식별자 또는 재현 증거다.\n

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the first #218 database/storage fixture slice with a MariaDB Testcontainers wrapper.

**Architecture:** Mirror `testcontainers/mysql` for caller ergonomics and use the #217 `testcontainers/server` package for generic server details, cleanup, and env export. Keep all other database/storage candidates deferred through the matrix.

**Tech Stack:** Go 1.26, Testcontainers-Go MariaDB module v0.42.0, `database/sql`, `github.com/go-sql-driver/mysql`, `internal/testcleanup`, `testcontainers/server`.

---

## File Structure

- Create `testcontainers/mariadb/doc.go`: package documentation.
- Create `testcontainers/mariadb/mariadb.go`: `DSNKey`, defaults, `Start`, `StartServer`, detail helper.
- Create `testcontainers/mariadb/mariadb_test.go`: Docker smoke test and key contract test.
- Create `testcontainers/mariadb/README.md`: English usage, `StartServer`, env export, dynamic port notes.
- Create `testcontainers/mariadb/README.ko.md`: Korean equivalent.
- Modify `go.mod` / `go.sum`: add `github.com/testcontainers/testcontainers-go/modules/mariadb v0.42.0`.

## Task 1: TDD Wrapper

- [ ] Write `mariadb_test.go` first. The smoke test must call `StartServer`, read `ConnectionDetails(ctx)`, require `DSNKey`, open `database/sql` with MySQL driver, and run `select 1`.
- [ ] Run `go test -p 1 -count=1 ./testcontainers/mariadb`. Expected failure: package/API missing.
- [ ] Implement `doc.go` and `mariadb.go`.
- [ ] Run `go test -p 1 -count=1 ./testcontainers/mariadb`. Expected: PASS.
- [ ] Run `go test -race -p 1 -count=1 ./testcontainers/mariadb`. Expected: PASS.

## Task 2: Documentation

- [ ] Add README and README.ko.md with import, usage, `StartServer`, `tcserver.ExportEnv`, behavior, operational boundaries, and test command.
- [ ] Run `git diff --check`. Expected: PASS.

## Task 3: Validation

- [ ] Run `go test -p 1 -count=1 ./testcontainers/server ./testcontainers/mariadb`.
- [ ] Run `go test -race -p 1 -count=1 ./testcontainers/server ./testcontainers/mariadb`.
- [ ] Run `make fmt-check`, `make tidy-check`, `make vet`, and `make lint`.
- [ ] Record Step 6-R code review with P0=0 P1=0 before PR.

