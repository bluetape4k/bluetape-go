# Issue #217 Testcontainers server abstraction research

Issue: [#217](https://github.com/bluetape4k/bluetape-go/issues/217)  
Parent Epic: [#215](https://github.com/bluetape4k/bluetape-go/issues/215)  
Milestone: `0.6.5`  
Date: 2026-06-23

## Current Go Evidence

- Existing wrappers live under:
  - `testcontainers/postgres`
  - `testcontainers/mysql`
  - `testcontainers/redis`
  - `testcontainers/kafka`
  - `testcontainers/nats`
- #216 hardened these wrappers first:
  - `internal/testcleanup.Terminate` provides bounded cleanup.
  - `internal/testcleanup.FormatStartError` categorizes start failures.
  - Each wrapper exposes a documented connection detail key.
  - Docker-backed package tests run serially.
- Current wrapper APIs return service-specific values directly:
  - PostgreSQL: connection string
  - MySQL: DSN
  - Redis: address
  - Kafka: broker list
  - NATS: URL
- Current wrappers fail through `testing.TB`, which matches Go test-helper style.
  They do not expose an error-returning public lifecycle API yet.

## Kotlin Parity Evidence

- GNO matched the relevant bluetape4k-projects design note:
  `docs/superpowers/specs/2026-04-03-testcontainers-design.md`.
- Kotlin `PropertyExportingServer` defines:
  - `propertyNamespace`
  - `propertyKeys()`
  - `properties()`
  - `registerSystemProperties()`
  - `writeToSystemProperties()`
- Kotlin `GenericServer` exposes `port` and `url`, backed by Testcontainers
  `ContainerState`.
- The Kotlin design fixed a JVM-specific problem: hidden global
  `System.setProperty` writes were not reversible. The final Kotlin interface
  uses reversible registration.

## Go Parity Decision

Do not port JVM system-property behavior directly.

Go should use:

- returned connection-detail maps as the primary export contract;
- optional environment export through `testing.TB.Setenv`, so cleanup is owned
  by the test and is reversible;
- explicit server values rather than singleton/global state;
- package-specific wrappers that keep their existing `Start(ctx, testing.TB)`
  functions source-compatible.

This matches `docs/research/2026-06-21-issue-202-source-parity-matrix.md`,
which explicitly routes generic server/property export to #215/#217 and says
JVM system property export is excluded for Go.

## Official Testcontainers-Go Evidence

Context7 resolved the official library as `/testcontainers/testcontainers-go`.
The installed module is `github.com/testcontainers/testcontainers-go v0.42.0`.

Relevant APIs:

- `testcontainers.Container`
  - `Host(context.Context) (string, error)`
  - `MappedPort(context.Context, string) (network.Port, error)`
  - `PortEndpoint(context.Context, port string, proto string) (string, error)`
  - `Endpoint(context.Context, proto string) (string, error)`
  - `Terminate(context.Context, ...TerminateOption) error`
- `testcontainers.GenericContainer(ctx, GenericContainerRequest)` starts
  generic containers from `ContainerRequest`.
- `testcontainers.CleanupContainer(testing.TB, Container)` is nil-safe and is
  intended to be called immediately after container creation, before checking
  the returned error.

The repo already owns bounded cleanup through `internal/testcleanup`, so #217
can reuse that helper rather than replacing it with raw `CleanupContainer`.

## Adopt / Borrow / Skip

| Source | Decision | Rationale |
|---|---|---|
| Kotlin `PropertyExportingServer` key contract | Borrow | Key discovery and value maps are useful across wrappers. |
| Kotlin `registerSystemProperties()` | Adapt | Go equivalent should be `testing.TB.Setenv`, not global JVM properties. |
| Kotlin `GenericServer` inheritance shape | Skip | Go should use small interfaces and composition, not a broad container framework. |
| Testcontainers-Go `Container` host/port APIs | Adopt | Official API already exposes host, mapped port, endpoint, and terminate. |
| Testcontainers-Go `CleanupContainer` | Borrow concept | Nil-safe cleanup is useful, but repo bounded cleanup remains the local contract. |

## Design Constraints

- Existing `Start(ctx, tb)` functions must remain source-compatible.
- New shared package must not introduce new dependencies.
- Environment export must be opt-in and reversible.
- Connection-detail keys must stay stable and documented.
- Docker-backed tests remain serial.
- Contract tests for the abstraction should not require Docker.
- Real wrapper smoke tests should continue to use Docker and `-p 1`.

## Research Summary for Spec

The smallest safe design is a new shared `testcontainers/server` package with:

- a `Server` interface for name, host, mapped ports, URLs/endpoints, connection
  details, cleanup, and termination;
- a concrete `Started` adapter around `testcontainers.Container`;
- `ConnectionDetails` helpers for clone/merge/string values;
- `ExportEnv(testing.TB, ConnectionDetails, map[string]string)` using
  `tb.Setenv`;
- reusable contract tests with fake containers;
- new `StartServer(ctx, tb)` functions in existing wrappers, while existing
  `Start(ctx, tb)` functions delegate or preserve their current return values.
