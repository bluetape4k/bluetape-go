# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project uses semantic versioning once the first tag is published.

## [Unreleased]

### Added

- `leader.GroupElector` and Redis-backed `redisleader.NewGroup` for
  semaphore-style multi-leader election with ZSET slot tokens.

## [v0.1.0] - 2026-06-03

### Added

- Initial Go module with `core`, `testing`, `testcontainers/redis`, `leader`,
  and `leader/redis` packages.
- Redis-backed leader election with Testcontainers smoke coverage.
- Milestone research notes under `docs/research/`.
- English and Korean README files with roadmap, hero image, and architecture
  overview diagram.
- Project management scaffolding: `Makefile`, lint configuration, WIP log,
  package layout policy, and release guide.
- Nightly workflow that runs Testcontainers-backed tests on a scheduled smoke
  and full cadence.
- Core support helpers for validation, zero/default handling, pointers,
  strings, and small numeric checks.
- Collections helpers for chunking, grouping, distinct values, and error-aware
  slice transformations.
- Redis leader lifecycle tests for duplicate campaign, repeated resign, renewal
  loss, renewal failure, and leader lookup semantics.
- Testable Go examples for the `core`, `collections`, `codec`, `compression`,
  `concurrency`, `serialization`, and `testing/concurrency` packages.
- PostgreSQL, MySQL 8.4, NATS, and Kafka Testcontainers fixtures with smoke
  tests.
- Gomega-backed asynchronous test helpers for eventual and consistent
  conditions.
- Redis leader coordination examples for batch scheduling and migration gates.
- Redis leader key compatibility decision for Kotlin/Go mixed participants.

### Changed

- CI now validates formatting, module tidiness, vet, lint, tests, and race
  tests against real Testcontainers dependencies.
- `make test` and `make race` now pass `-count=1` so integration tests are not
  skipped by Go's test cache.
- `leader` API docs now define ownership, cancellation, idempotent resign,
  lost-leadership, and `errors.Is` comparison semantics.
