# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project uses semantic versioning once the first tag is published.

## [Unreleased]

### Added

- Initial Go module with `core`, `testing`, `testcontainers/redis`, `leader`,
  and `leader/redis` packages.
- Redis-backed leader election with Testcontainers smoke coverage.
- Milestone research notes under `docs/research/`.
- English and Korean README files with roadmap and hero image.
- Project management scaffolding: `Makefile`, lint configuration, WIP log,
  package layout policy, and release guide.
- Nightly workflow that runs Testcontainers-backed tests on a scheduled smoke
  and full cadence.
- Core support helpers for validation, zero/default handling, pointers,
  strings, and small numeric checks.
- Issue #8 core support inventory that classifies implement/adopt/defer areas
  from `bluetape4k/core`.
- Collections helpers for chunking, grouping, distinct values, and error-aware
  slice transformations.
- Issue #9 collections inventory that classifies implement/adopt/defer areas
  from `bluetape4k/core` collection support.
- Redis leader lifecycle tests for duplicate campaign, repeated resign, renewal
  loss, renewal failure, and leader lookup semantics.
- Testable Go examples for the `core`, `collections`, `codec`, `compression`,
  `concurrency`, `serialization`, and `testing/concurrency` packages.
- PostgreSQL and NATS Testcontainers fixtures with smoke tests.

### Changed

- CI now validates formatting, module tidiness, vet, lint, tests, and race
  tests against real Testcontainers dependencies.
- `make test` and `make race` now pass `-count=1` so integration tests are not
  skipped by Go's test cache.
- `leader` API docs now define ownership, cancellation, idempotent resign,
  lost-leadership, and `errors.Is` comparison semantics.

### Removed

- Nothing yet.

## Release Notes

The first release target is `v0.1.0`. Until then, package APIs may change while
the foundation packages and leader semantics are stabilized.
