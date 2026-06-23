Part of #218

## Summary

- Add the #218 database/storage roadmap matrix and explicitly route broad candidates to their consumer issues.
- Add the first narrow database/storage wrapper slice: `testcontainers/mariadb`.
- Expose `Start(ctx, testing.TB)`, `StartServer(ctx, testing.TB)`, and `mariadb.dsn` connection details using the #217 server contract.
- Add English/Korean MariaDB README docs with dynamic port, env export, cleanup, and fixed-port collision notes.

## Deferred Scope

- MongoDB remains tied to #198 until the MongoDB backend/package boundary starts.
- MinIO, DynamoDB Local, and broader AWS emulator work remain tied to #220 and #61-#64.
- CockroachDB, ClickHouse, and Trino remain tied to #100/#101 SQL dialect decisions.
- AGE and graph databases remain tied to #220/#50.

## Review Evidence

- Step 2-R spec review: P0=0 P1=0.
- Step 6-R code review: P0=0 P1=0.

## Validation

- `go test -p 1 -count=1 ./testcontainers/mariadb`
- `go test -race -p 1 -count=1 ./testcontainers/mariadb`
- `go test -p 1 -count=1 ./testcontainers/server ./testcontainers/mariadb`
- `go test -race -p 1 -count=1 ./testcontainers/server ./testcontainers/mariadb`
- `make fmt-check`
- `make tidy-check`
- `make vet`
- `make lint`
- `make test`
- `make race`
- `git diff --check`

## DoD Status

- [x] Server matrix maps database/storage candidates to current roadmap issues.
- [x] First narrow implementation slice uses shared lifecycle/property contracts from #217.
- [x] MariaDB wrapper has README, example smoke test, connection detail contract, readiness through Testcontainers module, and cleanup.
- [x] Deferred database/storage servers are linked to concrete follow-up issues or roadmap epics.
- [x] Docker-heavy tests pass serially locally.
