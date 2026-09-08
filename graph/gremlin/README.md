# graph/gremlin

[English](README.md) | [한국어](README.ko.md)

`graph/gremlin` is a narrow remote adapter for Apache TinkerPop Gremlin-Go
`v3.8.1`. It collects a bounded result channel and converts official
`gremlingo.Vertex`/`gremlingo.Edge` values to the existing `graph` model.

```go
client, err := gremlin.NewRemoteClient("ws://localhost:8182/gremlin")
if err != nil {
	return err
}
defer client.Close(ctx)

result, err := client.Query(ctx, "g.V().limit(10)")
if err != nil {
	return err
}
_ = result.Values
```

## Boundary

`NewClient` and `NewConnectionClient` wrap caller-owned executors and never
close them. `NewRemoteClient` creates the official
`DriverRemoteConnection`; the returned adapter owns that connection and closes
it idempotently. TLS, authentication, traversal source, and connection pool
settings stay caller-owned through `WithConnectionConfiguration`.

Gremlin-Go `v3.8.1` does not expose `context.Context` on remote `Submit` or
`ResultSet` consumption. The adapter checks context before submission, selects
against the result channel during collection, checks again before publishing,
and closes the result stream. Cancellation therefore bounds local waiting; it
does not claim immediate server-side traversal cancellation.

## Supported scope

- remote traversal submission with optional bindings and evaluation timeout;
- bounded result collection, provider/error redaction, and result conversion;
- vertex, edge, and path/logical-key reads for the `graph` model;
- explicit unsupported-capability errors for transactions and other server
  features outside this adapter.

Transactions, embedded TinkerGraph, a universal Gremlin dialect, ORM/OGM,
implicit retries, and Neptune/cloud credentials are non-goals. Neptune is
live-opt-in only: configure the official connection yourself and keep
credentials outside logs and public errors.

## Test

Unit tests use a fake result channel. The local TinkerPop fixture is
digest-pinned and runs serially:

```bash
go test -race -count=1 ./graph/gremlin
go test -p 1 -count=1 -timeout=10m ./testcontainers/tinkerpop ./graph/gremlin
```
