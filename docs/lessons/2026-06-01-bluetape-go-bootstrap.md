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
license, Redis leader smoke tests, and a Nightly workflow that runs
Testcontainers-backed tests against real containers.

## Verification

- `gofmt -w core testing testcontainers leader`
- `go mod tidy`
- `go test ./...`
- `actionlint .github/workflows/ci.yml .github/workflows/nightly-tests.yml`
- `golangci-lint config verify`
- `make ci`

## Future Guard

Do not split `leader-go`, `core-go`, or `testcontainers-go` into separate
repositories until the package boundaries and release cadence are proven inside
`bluetape-go`.

Keep CI and Nightly test commands uncached with `go test -count=1` so
Testcontainers startup is exercised instead of hidden by Go's test cache.

For `core`, port concepts rather than Kotlin extension shapes. Prefer
error-returning validation, small generic helpers, and explicit documentation of
standard-library adoption or deferred areas.

For `collections`, avoid wrapping Go's standard `slices` and `maps` packages.
Add helpers only for repeated service transformations such as chunking,
grouping, distinct-by-key, and error-aware map/filter.
