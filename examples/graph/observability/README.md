# Observability Graph Example

[English](README.md) | [한국어](README.ko.md)

This package ports the observability incident graph from
`bluetape4k-graph/examples/observability-graph-examples` into an idiomatic Go
example. It uses only `graph` values and `graph/graphio` records, so the example
is runnable before a Neo4j or Memgraph adapter is introduced.

![Observability incident graph topology](../../../docs/images/readme-diagrams/graph-observability-incident-topology.png)

## What It Proves

The seed fixture models a checkout outage:

- two public API vertices that depend on the edge API,
- service dependencies from edge API to checkout, payment, and Postgres,
- latency and error alerts linked to affected services,
- an incident root-cause edge to `payment-service`,
- ownership edges to `payments-team`.

The package answers the caller questions that make the graph useful:

| Question | Go API |
|---|---|
| What depends on a failing service? | `UpstreamImpactedServices("payment-service", 3)` |
| Which public APIs are affected? | `AffectedAPIs("payment-service", 5)` |
| What does a service depend on? | `DownstreamDependencies("checkout-service", 2)` |
| Which services are in the alert boundary? | `AlertBoundary([]string{...}, 2)` |
| Which team owns the incident boundary? | `OwningTeams("payment-service")` |

## Seed Data

Seed data lives in `SeedIncidentGraph`. It mirrors the source CSV fixture with
10 vertices and 10 edges. The public IDs returned by queries are domain IDs such
as `payment-service`, `checkout-api`, and `payments-team`, while graph element
IDs remain stable transport IDs such as `svc-payment`.

The same graph can be exported and imported through `graph/graphio` NDJSON with
`WriteNDJSON` and `ReadIncidentGraphNDJSON`.

## Test

```bash
go test -count=1 ./examples/graph/observability
go test -race -count=1 ./examples/graph/observability
```

## Production Omissions

This example intentionally omits backend sessions, persistence, Cypher/Gremlin,
online schema migration, alert ingestion, incident lifecycle state machines,
authorization, metrics, and traversal-performance claims. Those belong in
adapter-backed follow-ups after the first Neo4j proof is implemented.

Only this observability example is ported for `0.10.0`. Code dependency, fraud,
knowledge, social, recommendation, and Ktor examples remain deferred because
they either need a backend traversal contract, a larger domain model, or a
JVM/Ktor shape that should not be copied into Go. The next high-value follow-up
is tracked by [#368](https://github.com/bluetape4k/bluetape-go/issues/368) for
an IAM/access graph example.
