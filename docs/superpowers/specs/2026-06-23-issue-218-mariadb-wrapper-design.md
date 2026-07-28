# Issue #218 MariaDB Wrapper Design

> 한국어 요구사항 경계: 이 spec/design/test-spec 문서는 한국어 독자가 요구사항을 추적할 수 있도록 목적과 검증 경계를 한국어로 보강한다. API 이름, command, code identifier, issue/PR 번호, compatibility matrix, acceptance keyword, DoD/test evidence는 요구사항 약화를 막기 위해 원문 그대로 보존한다. 변경자는 아래 literal contract를 삭제하거나 의미를 약하게 바꾸지 않아야 한다.
> 추가 한국어 검증 메모: 영어로 남은 항목은 대부분 code/API/evidence literal이다. 구현 전에는 한국어 경계 문장과 원문 acceptance checklist를 함께 읽고, 검증 gate가 줄어들지 않았는지 확인한다.\n

Issue: [#218](https://github.com/bluetape4k/bluetape-go/issues/218)  
Parent Epic: [#215](https://github.com/bluetape4k/bluetape-go/issues/215)  
Milestone: `0.6.5`  
Date: 2026-06-23

## 목표

Add the first narrow #218 database/storage slice: a MariaDB Testcontainers
wrapper that follows the existing MySQL wrapper and the #217 shared server
contract.

## Chosen Scope

Create:

```text
testcontainers/mariadb
```

Package name:

```go
package mariadbtestcontainer
```

Public API:

```go
const DSNKey = "mariadb.dsn"

func Start(ctx context.Context, tb testing.TB) string
func StartServer(ctx context.Context, tb testing.TB) *server.Started
```

Defaults:

- image: `mariadb:11.0.3`
- database: `bluetape`
- username: `bluetape`
- password: `bluetape`
- DSN query arg: `parseTime=true`

## Behavior

- `Start` returns a MariaDB DSN string for `database/sql` with
  `github.com/go-sql-driver/mysql`.
- `StartServer` returns the #217 shared server adapter with
  `ConnectionDetails(ctx)[DSNKey]`.
- Cleanup is registered through `server.Started.RegisterCleanup`.
- If `server.New` fails after the container starts, the wrapper immediately
  terminates the container through `internal/testcleanup.Terminate` before
  failing the test.
- The wrapper keeps Docker tests serial and uses the Testcontainers module
  readiness behavior.

## Non-Goals

- Do not implement MongoDB, MinIO, DynamoDB Local, CockroachDB, ClickHouse,
  Trino, PostGIS, pgvector, or AGE in this PR.
- Do not add a generic database builder or new connection abstraction.
- Do not change existing PostgreSQL/MySQL wrappers.
- Do not add a new SQL DSL or repository helper here.

## Acceptance Mapping

| #218 Criterion | Design Answer |
|---|---|
| Matrix maps candidates to roadmap issues. | `docs/superpowers/research/2026-06-23-issue-218-db-storage-matrix.md`. |
| Implementation order justified by live roadmap issues. | MariaDB directly supports #100/#101 and is MySQL-adjacent. |
| First narrow slice uses #217 lifecycle/property contract. | `StartServer` returns `*server.Started` and exports `mariadb.dsn`. |
| Each selected wrapper has README, example test, connection detail contract, readiness, cleanup. | MariaDB README pair plus Docker smoke test using `database/sql`; readiness comes from the Testcontainers module; cleanup uses #217. |
| Deferred servers are linked. | Matrix links MongoDB to #198, AWS/storage to #220/#61-#64, graph to #220/#50, SQL dialect breadth to #100/#101. |

## 검증

- `go test -p 1 -count=1 ./testcontainers/mariadb`
- `go test -race -p 1 -count=1 ./testcontainers/mariadb`
- `go test -p 1 -count=1 ./testcontainers/server ./testcontainers/mariadb`
- `make fmt-check`
- `make tidy-check`
- `make vet`
- `make lint`

