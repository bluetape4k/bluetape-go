# testcontainers/toxiproxy

[English](README.md) | [한국어](README.ko.md)

`testcontainers/toxiproxy` starts a Toxiproxy container for integration tests
that need network failure injection.

![testcontainers helper flow](../../docs/images/readme-diagrams/testcontainers-helper-flow.png)

## Import

```go
import toxiproxytestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/toxiproxy"
```

## Usage

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
t.Cleanup(cancel)

controlURI := toxiproxytestcontainer.Start(ctx, t)
details := map[string]string{
    toxiproxytestcontainer.ControlURIKey: controlURI,
}
client := toxiproxy.NewClient(details[toxiproxytestcontainer.ControlURIKey])
_ = client
```

## Shared Server API

Use `Start(ctx, t)` when the Toxiproxy control URI is enough. Use
`StartServer(ctx, t)` when a test needs the shared Testcontainers server
contract: host lookup, mapped ports, endpoints, connection details, cleanup,
manual termination, or explicit env export.

The example assumes `tcserver` aliases `github.com/bluetape4k/bluetape-go/testcontainers/server`.

```go
srv := toxiproxytestcontainer.StartServer(ctx, t)
details, err := srv.ConnectionDetails(ctx)
if err != nil {
    t.Fatalf("toxiproxy details: %v", err)
}
if err := tcserver.ExportEnv(t, details, map[string]string{
    toxiproxytestcontainer.ControlURIKey: "BLUETAPE_TOXIPROXY_CONTROL_URI",
}); err != nil {
    t.Fatal(err)
}
```

`tcserver.ExportEnv` uses `testing.TB.Setenv`; do not call it from tests that
use `t.Parallel` or have parallel ancestors.

## Proxying Another Container

Use `StartContainer(ctx, t, opts...)` when a test needs the upstream
Toxiproxy container to read a configured proxy endpoint.

```go
nw, err := network.New(ctx)
if err != nil {
    t.Fatalf("create network: %v", err)
}
t.Cleanup(func() { _ = nw.Remove(ctx) })

redisContainer, err := redis.Run(ctx, "redis:7.4-alpine", network.WithNetwork([]string{"redis"}, nw))
if err != nil {
    t.Fatalf("start redis: %v", err)
}
t.Cleanup(func() { _ = testcontainers.TerminateContainer(redisContainer) })

toxiproxyContainer := toxiproxytestcontainer.StartContainer(
    ctx,
    t,
    tctoxiproxy.WithProxy("redis", "redis:6379"),
    network.WithNetwork([]string{"toxiproxy"}, nw),
)
endpoint := toxiproxytestcontainer.ProxiedEndpoint(ctx, t, toxiproxyContainer, 8666)
redisClient := redisclient.NewClient(&redisclient.Options{
    Addr:        endpoint,
    DialTimeout: 500 * time.Millisecond,
    ReadTimeout: 500 * time.Millisecond,
})
t.Cleanup(func() { _ = redisClient.Close() })
```

Configure fault injection with the upstream `github.com/Shopify/toxiproxy/v2`
client. Prefer deterministic enable/disable tests over latency-window
assertions in normal CI.

## Behavior

- Uses `ghcr.io/shopify/toxiproxy:2.12.0`.
- Returns the Toxiproxy control API URI.
- Registers container termination with `t.Cleanup`.
- Exposes the control URI key as `toxiproxytestcontainer.ControlURIKey`
  (`toxiproxy.control_uri`).
- Accepts upstream Testcontainers customizers such as
  `tctoxiproxy.WithProxy(...)` and `network.WithNetwork(...)`.
- Start failures are categorized as Docker unavailable, image pull failure,
  readiness timeout, context cancellation, or wrapper failure.

## Operational Boundaries

- Docker or another Testcontainers-compatible runtime must be available.
- Toxiproxy is for tests. Do not route production traffic through this helper.
- Dynamic host port mapping is the default. Read mapped ports and exported env
  values after the container starts; they point to host ports, not
  container-internal ports.
- Failure-injection tests must use bounded client timeouts and context
  deadlines. Avoid broad, always-on failure tests in normal CI.
- Fixed host ports are not exposed by this helper because they can collide in
  parallel local runs and CI jobs.
- Run Docker-backed Testcontainers packages serially when resources or ports are
  shared.
- CI jobs without Docker should skip `./testcontainers/...`; CI jobs that cover
  these helpers should run the packages with `-p 1`.

## Deferred Scope

- RabbitMQ, Redpanda, and Pulsar stay deferred to #58 until outbox adapter
  delivery semantics select a broker.
- WireMock, Nginx, and Mailpit stay deferred to #224 or another concrete package
  issue that proves `httptest` is insufficient.
- ElasticMQ and SNS/SQS-compatible emulators stay deferred to #220 and #61-#64.

## Test

```bash
go test -p 1 -count=1 ./testcontainers/toxiproxy
```
