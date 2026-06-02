# Package Layout Policy

## Goals

- Keep public packages small, idiomatic, and independently useful.
- Avoid catch-all utility packages.
- Prefer Go standard library and proven dependencies before adding bluetape-go
  wrappers.
- Keep implementation details private until there is a clear public contract.

## Public Packages

Top-level directories are public packages when they represent stable user-facing
APIs:

- `core`
- `testing`
- `testcontainers/...`
- `leader`
- `leader/redis`

Future public packages should follow the same rule: create a top-level package
only when it has a clear domain, examples, package docs, and tests.

## `internal`

Use `internal/` for code shared by packages but not intended as public API.

Good candidates:

- shared test helpers not useful to users;
- implementation details for serializers, compressors, locks, or retry
  policies;
- compatibility shims while public APIs are still settling.

Do not put user-facing packages under `internal/`. If users must import it, it
belongs in a public package with docs and tests.

## Package Documentation

Every public package should have package documentation before it is considered
release-ready:

- package-level purpose;
- primary API example;
- concurrency and context behavior when relevant;
- error semantics and sentinel errors when exposed;
- compatibility notes for bluetape4k Kotlin/JVM behavior when applicable.

## Source Comments

Write source comments in Korean for this repository. Keep them short and
Go-native: exported declarations still start with the exported identifier and a
space before any Korean particle so `go doc`, pkg.go.dev, and linters can
associate the comment with the API.

## Examples

Prefer examples that solve real backend problems:

- leader-guarded scheduler;
- cache warmer;
- migration gate;
- Redis near-cache invalidation;
- resilient HTTP client/server calls;
- LocalStack-backed AWS examples.
