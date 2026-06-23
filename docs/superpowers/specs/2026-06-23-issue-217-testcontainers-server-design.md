# Issue #217 Testcontainers Server Abstraction Design

Issue: [#217](https://github.com/bluetape4k/bluetape-go/issues/217)  
Parent Epic: [#215](https://github.com/bluetape4k/bluetape-go/issues/215)  
Milestone: `0.6.5`  
Date: 2026-06-23

## Goal

Add a small Go-native server abstraction for Testcontainers fixtures. The
abstraction should expose host, mapped ports, endpoint URLs, connection details,
cleanup, and optional environment export without changing existing wrapper
callers.

This is selective parity with bluetape4k's `GenericServer` and
`PropertyExportingServer`, not a JVM system-property or inheritance-style port.

## Current Evidence

- #217 requires a shared abstraction for host, mapped ports, URLs, connection
  details, start, terminate, cleanup, and reversible property export.
- #215 says #216 and #217 must land before broader service wrapper expansion.
- `docs/research/2026-06-21-issue-202-source-parity-matrix.md` routes generic
  server and property export gaps to #215/#217 and explicitly excludes JVM
  system-property export for Go.
- #216 already added stable wrapper keys:
  - `postgres.connection-string`
  - `mysql.dsn`
  - `redis.address`
  - `kafka.brokers`
  - `nats.url`
- Current wrappers under `testcontainers/{postgres,mysql,redis,kafka,nats}` use
  Testcontainers module `Run` helpers, register bounded cleanup through
  `internal/testcleanup.Register`, and return service-specific values.
- Testcontainers-Go v0.42.0 exposes the needed primitives through
  `testcontainers.Container`: `Host`, `MappedPort`, `PortEndpoint`, and
  `Terminate`.

## Non-Goals

- Do not remove or break existing `Start(ctx, testing.TB)` wrapper functions.
- Do not introduce package-level singleton servers or hidden global state.
- Do not port JVM `System.setProperty` behavior.
- Do not add new dependencies.
- Do not broaden this issue into new service wrappers; #218-#220 own expansion.
- Do not replace service-specific typed return values with generic maps.

## Brainstorming Options

### Option 1: Shared Contract Package Plus Adapter (Chosen)

Create `testcontainers/server` with a small `Server` interface, a concrete
adapter around `testcontainers.Container`, connection-detail helpers, and
explicit environment export. Existing wrappers add `StartServer(ctx, tb)` and
keep `Start(ctx, tb)` source-compatible.

Pros:

- Satisfies #217 without breaking current users.
- Keeps generic behavior isolated and testable without Docker.
- Lets existing wrappers migrate incrementally.
- Avoids hidden global state.

Cons:

- Adds one new public package and one additional wrapper function per service.
- Some callers may still prefer service-specific `Start` helpers for simplicity.

### Option 2: Replace Existing Wrapper Return Types

Change existing `Start` functions to return the new server abstraction.

Pros:

- Minimal duplication inside wrappers.
- All callers see the generic contract immediately.

Cons:

- Breaks every current caller expecting `string` or `[]string`.
- Violates #217's no-break condition unless a separate migration issue is
  approved.

### Option 3: Environment Export First

Keep wrappers as they are and add only an environment export helper that maps
connection details to env names.

Pros:

- Smallest immediate implementation.
- Directly solves reversible export.

Cons:

- Does not satisfy host/mapped-port/URL/terminate/cleanup abstraction criteria.
- Leaves wrapper lifecycle drift unresolved.

## Chosen Design

Use Option 1.

Add package:

```text
testcontainers/server
```

Package name:

```go
package server
```

Import examples should alias it when needed:

```go
import tcserver "github.com/bluetape4k/bluetape-go/testcontainers/server"
```

### Public Types

```go
type ConnectionDetails map[string]string

type Port struct {
    Name          string
    ContainerPort string
    Scheme        string
}

type Container interface {
    Host(context.Context) (string, error)
    MappedPort(context.Context, string) (network.Port, error)
    PortEndpoint(context.Context, string, string) (string, error)
    Terminate(context.Context, ...testcontainers.TerminateOption) error
}

type Server interface {
    Name() string
    Host(context.Context) (string, error)
    MappedPort(context.Context, string) (string, error)
    Endpoint(context.Context, string, string) (string, error)
    ConnectionDetails(context.Context) (ConnectionDetails, error)
    RegisterCleanup(context.Context, testing.TB)
    Terminate(context.Context) error
}

type Started struct {
    // unexported fields
}
```

### Constructor Shape

```go
func New(name string, container Container, options ...Option) (*Started, error)

func WithPort(name, containerPort, scheme string) Option

func WithConnectionDetails(func(context.Context, *Started) (ConnectionDetails, error)) Option
```

The constructor validates and stores metadata only. It does not start a
container and does not register cleanup by itself. The owning wrapper keeps
start and cleanup ordering explicit.

The `Container` interface is intentionally narrower than
`testcontainers.Container` so core contract tests can use fake containers
without implementing unrelated Docker operations. Testcontainers-Go module
containers satisfy this interface.

### ConnectionDetails Behavior

`ConnectionDetails` is a map because #217 needs named properties, but helpers
must make it hard to leak mutable internals:

```go
func (d ConnectionDetails) Clone() ConnectionDetails
func (d ConnectionDetails) Merge(other ConnectionDetails) ConnectionDetails
func (d ConnectionDetails) Get(key string) (string, bool)
func (d ConnectionDetails) Require(key string) (string, error)
```

`ConnectionDetails` keys are the existing package-specific key constants. Values
are strings. Multi-value data such as Kafka brokers uses a comma-separated
string in the generic details map while the existing `Start` helper still
returns `[]string`.

### Host, Port, and Endpoint Behavior

`Started.Host(ctx)` delegates to `testcontainers.Container.Host(ctx)`.

`Started.MappedPort(ctx, containerPort)` delegates to
`testcontainers.Container.MappedPort(ctx, containerPort)` and returns the mapped
host port as a string.

`Started.Endpoint(ctx, containerPort, scheme)` delegates to
`testcontainers.Container.PortEndpoint(ctx, containerPort, scheme)`.

Errors are wrapped with the server name and operation, for example:

```text
redis mapped port 6379/tcp: <cause>
```

This preserves causal errors for callers and gives operator-facing diagnostics.

### Cleanup and Termination

`Started.RegisterCleanup(ctx, tb)` delegates to `internal/testcleanup.Register`
using the server name and bounded termination timeout.

`Started.Terminate(ctx)` delegates to `internal/testcleanup.Terminate` using the
default timeout. It is for manual cleanup when callers do not use `testing.TB`
cleanup.

No package-level cleanup registry is introduced.

### Environment Export

Add:

```go
func ExportEnv(tb testing.TB, details ConnectionDetails, names map[string]string) error
```

`names` maps connection-detail keys to environment variable names:

```go
if err := tcserver.ExportEnv(t, details, map[string]string{
    redistestcontainer.AddressKey: "BLUETAPE_REDIS_ADDR",
}); err != nil {
    t.Fatal(err)
}
```

`ExportEnv` uses `tb.Setenv`, so Go owns cleanup and previous values are
restored after the test. It must call `tb.Helper()`.

Rules:

- Export is opt-in.
- The function validates the full mapping before mutating the environment.
- Missing detail keys and blank env names return errors that wrap package
  sentinel errors so callers can test them.
- Callers that want existing wrapper-style failures should use
  `if err := tcserver.ExportEnv(...); err != nil { t.Fatal(err) }`.
- Because `ExportEnv` uses `tb.Setenv`, it must not be used in tests that call
  `t.Parallel` or have parallel ancestors. Testcontainers wrapper tests remain
  serial.

No global export happens by default.

### Existing Wrapper Migration

Each wrapper gets a new function:

```go
func StartServer(ctx context.Context, tb testing.TB) *server.Started
```

The current `Start(ctx, tb)` remains and delegates to `StartServer` plus the
service-specific connection lookup:

- PostgreSQL: return `ConnectionStringKey`
- MySQL: return `DSNKey`
- Redis: return `AddressKey`
- Kafka: return brokers from the module directly or parse the generic
  `BrokersKey` only inside tests if needed
- NATS: return `URLKey`

The wrapper-specific connection detail functions should populate the generic
details map by calling the same module APIs used today.

Wrappers must fail the test if `server.New` rejects invalid configuration. This
keeps public `StartServer(ctx, tb)` ergonomic while preserving a normal Go error
contract for the reusable constructor.

### Fixed-Port vs Dynamic-Port Documentation

The default contract is dynamic host port mapping. `MappedPort` and `Endpoint`
must be read after the container starts.

Fixed host ports are not added in #217. Future wrappers may expose fixed-port
options only with documentation of collision risk and serial test requirements.

README files must document:

- dynamic mapped ports are the default;
- exported env vars point to mapped host ports, not container-internal ports;
- fixed host ports can collide and should be reserved for follow-up issues only
  when a real integration requires them.

## Risks and Mitigations

| Risk | Severity | Mitigation |
|---|---:|---|
| Generic abstraction becomes a framework larger than the wrappers. | P1 | Keep `testcontainers/server` to adapter, details, cleanup, and env export only. No builders for every Testcontainers option. |
| Existing callers break. | P1 | Preserve all current `Start(ctx, testing.TB)` signatures and return values. Add `StartServer` as an opt-in API. |
| Env export creates hidden global state. | P1 | Use `testing.TB.Setenv` only, require explicit key-to-env mapping, validate before mutation, and document `t.Parallel` limits. |
| Connection detail maps become mutable shared state. | P2 | Return cloned maps from `ConnectionDetails(ctx)` and add `Clone` tests. |
| Kafka loses typed broker list ergonomics. | P2 | Keep `Start` returning `[]string`; generic map uses comma-separated `kafka.brokers` only for export/reporting. |
| Docker contract tests slow the suite. | P2 | Core server package tests use fake containers; existing wrapper smoke tests remain serial Docker tests. |

## Acceptance Criteria Mapping

| #217 Criterion | Design Answer |
|---|---|
| Common interface exposes host, mapped ports, URLs, connection details, cleanup without global state. | `testcontainers/server.Server` plus `Started` adapter. |
| Optional environment/property export is explicit, reversible, documented. | `ExportEnv(tb, details, names)` validates input, uses `tb.Setenv`, returns errors, and does no default global export. |
| Existing wrappers migrate/adapt without breaking current users. | Add `StartServer`; preserve `Start`. |
| Add contract tests reused by all wrappers. | Add fake-container tests for `server.Started`; wrapper tests assert `StartServer` details and existing `Start` behavior. |
| Document fixed-port vs dynamic-port behavior and collision risks. | Update `testcontainers/*/README.md` and `.ko.md` plus new package docs. |

## DoD

- `testcontainers/server` package provides `Server`, `Started`,
  `ConnectionDetails`, `Port`, options, and `ExportEnv`.
- Existing five wrappers expose `StartServer(ctx, testing.TB)` and keep
  `Start(ctx, testing.TB)` source-compatible.
- Wrapper details use the key constants added by #216.
- Unit tests cover:
  - host, mapped port, endpoint delegation;
  - connection detail clone/require behavior;
  - cleanup registration on skipped tests;
  - manual terminate path;
  - env export, missing key, blank env name, and validate-before-mutate
    behavior.
- Docker smoke tests for existing wrappers still pass serially.
- README locale sets document dynamic mapping, env export, cleanup, and fixed
  port collision risk.
- Local validation passes:
  - `git diff --check`
  - targeted `go test -p 1 -count=1 ./testcontainers/server ./testcontainers/...`
  - targeted `go test -race -p 1 -count=1 ./testcontainers/server ./testcontainers/...`
  - `make fmt-check`
  - `make tidy-check`
  - `make vet`
  - `make lint`
