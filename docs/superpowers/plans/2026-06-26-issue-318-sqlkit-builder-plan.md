# Issue #318 SQLKit Builder Implementation Plan

> 한국어 운영 요약: 이 계획 문서는 사용자 협업용 실행 계획이다. 아래 원문에 포함된 명령, 경로, API 이름, issue/PR 번호, branch 이름, code block, test output은 추적성과 재현성을 위해 그대로 보존한다. 작업 순서, 위험, 검증, 롤백 판단은 한국어 독자가 바로 실행 경계를 이해할 수 있도록 이 메모를 우선 적용한다.
> 추가 한국어 요약: 이 문서의 실행 판단은 기존 순서를 따르며, 변경자는 작업 표와 검증 목록을 먼저 확인한 뒤 관련 테스트를 실행한다. 영어로 남은 항목은 코드 식별자 또는 재현 증거다.\n

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a PostgreSQL-first inspectable SQL builder and repository prototype to `sqlkit`.

**Architecture:** Keep the builder in the existing `sqlkit` package. Builders produce immutable `Statement` values with explicit SQL and args. Identifier validation/quoting and placeholder rewriting are local helpers; execution remains caller-owned through `database/sql` and existing context-aware helpers.

**Tech Stack:** Go `database/sql`, existing `sqlkit` helpers, existing `testcontainers/postgres` fixture, `pgx/v5/stdlib` driver, standard-library tests.

---

## File Structure

- Create `sqlkit/statement.go`: `Statement` value plus copied args helper.
- Create `sqlkit/identifier.go`: identifier validation and quoting.
- Create `sqlkit/builder.go`: SELECT/INSERT/UPDATE/DELETE builders and placeholder rewriting.
- Create `sqlkit/builder_test.go`: table-driven builder tests, RED/GREEN source.
- Create `sqlkit/repository_example_test.go`: PostgreSQL Testcontainers repository prototype.
- Modify `sqlkit/README.md` and `sqlkit/README.ko.md`: builder usage, PostgreSQL boundary, runtime-first non-goals.
- Modify `README.md` and `README.ko.md`: package table/roadmap wording.
- Create `docs/review/2026-06-26-issue-318-sqlkit-builder-review.md`.
- Create `docs/lessons/2026-06-26-sqlkit-builder-boundary.md`.

## Task 1: Builder RED Tests

**Complexity:** medium
**Skill:** Apply `bluetape-go-patterns`; use TDD.
**Files:**
- Create: `sqlkit/builder_test.go`

- [ ] Write tests for `SelectFrom`, `InsertInto`, `Update`, and `DeleteFrom`
      that assert exact SQL strings and args.
- [ ] Write tests for invalid identifiers, placeholder mismatch, empty
      UPDATE/DELETE where guards, and copied args.
- [ ] Run `go test -count=1 ./sqlkit`.
      Expected: FAIL because builder APIs do not exist.

## Task 2: Minimal Builder Implementation

**Complexity:** high
**Skill:** Apply `bluetape-go-patterns`; keep API small and Go-shaped.
**Files:**
- Create: `sqlkit/statement.go`
- Create: `sqlkit/identifier.go`
- Create: `sqlkit/builder.go`

- [ ] Implement `Statement` with copied `Args`.
- [ ] Implement identifier validation for dotted identifiers using ASCII
      letters, digits, and underscores; each segment must start with a letter or
      underscore.
- [ ] Implement PostgreSQL placeholder rewriting from `?` to `$n` with argument
      count validation.
- [ ] Implement SELECT, INSERT, UPDATE, DELETE builders.
- [ ] Run `gofmt` on new Go files.
- [ ] Run `go test -count=1 ./sqlkit`.
      Expected: PASS for builder unit tests and existing package tests.

## Task 3: PostgreSQL Repository Prototype

**Complexity:** medium
**Skill:** Apply `bluetape-go-patterns`; keep Testcontainers serial.
**Files:**
- Create: `sqlkit/repository_example_test.go`

- [ ] Add a small account repository in `sqlkit_test` package using the builder
      output and existing `QueryOne`, `QueryOptional`, `QueryAll`, and `WithTx`.
- [ ] Use `testcontainers/postgres.Start(ctx, t)` and `sql.Open("pgx", dsn)`.
- [ ] Prove create/read/update/delete, rollback visibility, and a relational
      account/event query.
- [ ] Run `go test -p 1 -count=1 ./sqlkit`.
      Expected: PASS with PostgreSQL Testcontainer.

## Task 4: Documentation

**Complexity:** low
**Skill:** Apply `bluetape-go-patterns` README guidance.
**Files:**
- Modify: `sqlkit/README.md`
- Modify: `sqlkit/README.ko.md`
- Modify: `README.md`
- Modify: `README.ko.md`

- [ ] Document builder usage with exact SQL/args inspection.
- [ ] Document PostgreSQL `$n` placeholder boundary and raw SQL fallback for
      unsupported dialect/features.
- [ ] Keep root README package table and planned package text in sync.
- [ ] Verify source/doc claims by grepping `SelectFrom`, `InsertInto`, `Update`,
      and `DeleteFrom`.

## Task 5: Verification And Review

**Complexity:** medium
**Skill:** Apply `verification-before-completion` and 7-tier review gate.
**Files:**
- Create: `docs/review/2026-06-26-issue-318-sqlkit-builder-review.md`
- Create: `docs/lessons/2026-06-26-sqlkit-builder-boundary.md`

- [ ] Run `git diff --check`.
- [ ] Run `go test -p 1 -count=1 ./sqlkit`.
- [ ] Run `go test -race -p 1 -count=1 ./sqlkit`.
- [ ] Run `make fmt-check`, `make tidy-check`, `make vet`, `make lint`,
      `go test -p 1 -count=1 ./...`, and `make ci`.
- [ ] Write Step 6-R review artifact with performance, stability, security,
      operator/Ops, developer/API, user/caller, and integration findings.
- [ ] Confirm `P0=0 P1=0`.
- [ ] Commit spec, plan, code, docs, review artifact, and lesson with Lore
      trailers before PR creation.
