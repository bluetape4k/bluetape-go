Closes #217

## Summary

- Add `testcontainers/server` with a narrow started-container contract, cloned connection details, explicit `ExportEnv`, and bounded cleanup/termination.
- Add `StartServer(ctx, testing.TB)` to PostgreSQL, MySQL, Redis, Kafka, and NATS wrappers while preserving existing typed `Start` helpers.
- Document shared server usage, dynamic mapped ports, `testing.TB.Setenv` limits, and fixed host-port collision risks in English/Korean README pairs.

## Review Evidence

- Step 2-R spec review: P0=0 P1=0 after API corrections.
- Step 3-R plan review: P0=0 P1=0 after construction-failure cleanup guard.
- Step 6-R code review: P0=0 P1=0.

## Validation

- `go test -p 1 -count=1 ./testcontainers/server ./testcontainers/redis ./testcontainers/postgres ./testcontainers/mysql ./testcontainers/kafka ./testcontainers/nats`
- `go test -race -p 1 -count=1 ./testcontainers/server ./testcontainers/redis ./testcontainers/postgres ./testcontainers/mysql ./testcontainers/kafka ./testcontainers/nats`
- `make fmt-check`
- `make tidy-check`
- `make vet`
- `make lint`
- `make test`
- `make race`
- `git diff --check`

## DoD Status

- [x] Common server interface exposes host, mapped ports, endpoints, connection details, cleanup, and manual termination without global state.
- [x] Env export is explicit, reversible through `testing.TB.Setenv`, validation-first, and documented as unsafe for parallel tests.
- [x] Existing wrappers keep current `Start` return types and add opt-in `StartServer`.
- [x] Contract tests cover connection detail cloning, missing keys, env export validation, delegation, and termination.
- [x] Wrapper smoke tests pass serially with Docker.
- [x] English and Korean README files document dynamic mapped ports and fixed-port collision risks.
