# Issue #219 Toxiproxy Wrapper Design

Issue: [#219](https://github.com/bluetape4k/bluetape-go/issues/219)
Parent Epic: [#215](https://github.com/bluetape4k/bluetape-go/issues/215)
Date: 2026-06-23

## Goal

Add the first #219 wrapper slice by introducing a Go-native Toxiproxy
Testcontainers helper. The slice should support future messaging, HTTP, Redis,
database, and recipe failure-path tests without adding a new broker or HTTP mock
catalog prematurely.

## Context

- Kafka and NATS wrappers already exist.
- #217 introduced the shared `testcontainers/server` contract.
- #218 showed the preferred first-slice shape: matrix, narrow wrapper, README
  pair, serial Docker tests, and explicit deferrals.
- #46/#56-#59 audit/outbox issues have not selected RabbitMQ, Redpanda, Pulsar,
  or another concrete broker.
- #224 integration recipes need realistic Testcontainers-backed failure
  scenarios.

## Considered Approaches

| Approach | Trade-off | Decision |
|---|---|---|
| Toxiproxy first | Adds broad failure-injection value for existing Redis, DB, Kafka, NATS, and later HTTP tests. Requires a Docker network in example tests. | Selected. |
| RabbitMQ or Redpanda first | Adds a concrete broker wrapper, but outbox semantics are not selected yet and Kafka/NATS already cover current messaging tests. | Rejected for this slice. |
| WireMock/Nginx/Mailpit first | Useful for HTTP/email recipes, but current packages can use `httptest` and there is no concrete consumer yet. | Rejected for this slice. |

## Public API

Create package `testcontainers/toxiproxy` with package name
`toxiproxytestcontainer`.

Public constants:

- `ControlURIKey = "toxiproxy.control_uri"`

Public functions:

- `Start(ctx context.Context, tb testing.TB, opts ...testcontainers.ContainerCustomizer) string`
  - starts Toxiproxy and returns the control URI.
- `StartServer(ctx context.Context, tb testing.TB, opts ...testcontainers.ContainerCustomizer) *tcserver.Started`
  - starts Toxiproxy and returns the shared server adapter.
- `ProxiedEndpoint(ctx context.Context, tb testing.TB, container *tctoxiproxy.Container, port int) string`
  - returns the proxied endpoint for a configured proxy as `host:port`.
  - fails the test with a clear message when the proxy port is not configured.

No public wrapper should expose a custom abstraction over the upstream
Toxiproxy client. Tests and recipes can use `github.com/Shopify/toxiproxy/v2`
directly when they need toxic controls.

## Implementation Shape

- Default image: `ghcr.io/shopify/toxiproxy:2.12.0`.
- `StartServer` calls `tctoxiproxy.Run(ctx, defaultImage, opts...)`.
- Callers pass upstream customizers such as `tctoxiproxy.WithProxy(...)` and
  `network.WithNetwork(...)`; the wrapper does not invent a second proxy DSL.
- `StartServer` stores `toxiproxy.control_uri` by calling `container.URI(ctx)`.
- `StartServer` uses `tcserver.New("toxiproxy", container, ...)` and registers
  bounded cleanup.
- On server construction failure after the container starts, terminate the
  container with `testcleanup.Terminate`.
- The variadic option surface is intentionally the upstream
  `testcontainers.ContainerCustomizer` contract, matching Testcontainers-Go and
  avoiding a first-party proxy DSL.

## Representative Test

Add an integration test that:

1. Creates a Testcontainers network.
2. Starts Redis with alias `redis`.
3. Starts Toxiproxy with a configured proxy to `redis:6379`.
4. Reads the proxied endpoint.
5. Connects to Redis through the proxy.
6. Writes and reads a value.
7. Disables the proxy through the upstream Toxiproxy client and verifies a
   bounded Redis command fails.
8. Re-enables the proxy and verifies Redis works again.

The test must use context timeouts and close clients/resources through
`testing.TB.Cleanup`.

## Documentation

Add `README.md` and `README.ko.md` for `testcontainers/toxiproxy`:

- Docker requirement.
- Dynamic host port behavior.
- Control URI connection detail and env export example.
- Example proxying Redis through a shared Docker network.
- Failure injection caveats: tests must use bounded client timeouts and avoid
  broad, always-on failure tests in normal CI.
- Deferred broker/HTTP/mock/mail/AWS fixture decisions and linked issues.

## Non-Goals

- Do not add RabbitMQ, Redpanda, Pulsar, WireMock, Nginx, Mailpit, ElasticMQ,
  or AWS emulator wrappers in this slice.
- Do not create a first-party Toxiproxy toxic DSL.
- Do not make normal tests flaky with sleeps or unbounded timing assertions.
- Do not replace package-local `httptest` usage where it already proves the
  contract.

## Validation

- `go test -p 1 -count=1 ./testcontainers/toxiproxy`
- `go test -race -p 1 -count=1 ./testcontainers/toxiproxy`
- `go test -p 1 -count=1 ./testcontainers/redis ./testcontainers/toxiproxy`
- `go test -race -p 1 -count=1 ./testcontainers/redis ./testcontainers/toxiproxy`
- `make fmt-check`
- `make tidy-check`
- `make vet`
- `make lint`
- `make test`
- `make race`
- `git diff --check`

## Open Questions

None. The first slice deliberately avoids selecting a new broker or external
HTTP mock server.

## Step DoD

| Requirement | Status |
|---|---|
| Prioritization references current package needs. | Done: Toxiproxy supports existing Redis/DB/Kafka/NATS and #224 recipes. |
| Wrapper has bounded readiness and cleanup. | Planned: Testcontainers module wait strategy plus `tcserver` cleanup. |
| Failure injection example is non-flaky. | Planned: disable/enable proxy behavior with bounded Redis timeouts, not latency timing assertions. |
| Messaging examples include cleanup and caveats. | Deferred: broker additions wait for #58 adapter selection. |
| AWS emulator duplication avoided. | Done: ElasticMQ/SQS/SNS stay in #220/#61-#64. |
