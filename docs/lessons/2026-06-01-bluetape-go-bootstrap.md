# bluetape-go Bootstrap

## Context

The Go plan moved from a narrow `leader-go` idea to one `bluetape-go`
repository that can host shared packages such as `core`, `testing`, and
`testcontainers` alongside domain packages such as `leader`.

## Decision

Bootstrap `github.com/bluetape4k/bluetape-go` as a single Go module. Keep the
first implementation small: shared validation, an eventually test helper, a
Redis Testcontainers fixture, and Redis-backed leader election.

## Outcome

The repository now has an initial module structure, CI workflow, README, MIT
license, and Redis leader smoke tests.

## Verification

- `gofmt -w core testing testcontainers leader`
- `go mod tidy`
- `go test ./...`

## Future Guard

Do not split `leader-go`, `core-go`, or `testcontainers-go` into separate
repositories until the package boundaries and release cadence are proven inside
`bluetape-go`.
