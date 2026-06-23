# MariaDB Testcontainers Wrapper Implementation Plan

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

