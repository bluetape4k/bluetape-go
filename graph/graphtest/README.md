# graph/graphtest

[English](README.md) | [한국어](README.ko.md)

Strict, backend-neutral conformance support for implementations that map graph
storage into `graph.Vertex` and `graph.Edge`. This is a test-support package,
not a production repository, query, session, transaction, or schema
abstraction.

## Contract

- `Run` uses a complete default `Config`; `RunWithConfig` rejects zero or
  partial configurations before calling the provider factory.
- Connectivity, empty read, create/read, cancellation, provider-error,
  cleanup, and close callbacks are mandatory and cannot be skipped.
- Traversal is optional. A disabled traversal must use a validated, stable
  `ReasonCode`; an enabled traversal must provide `Adapter.Traverse`.
- Providers own container creation, credentials, endpoint policy, and
  readiness. The returned adapter owns only its client or driver and closes it
  exactly once.
- Read and traversal adapters must use fixed queries and columns, bound
  parameters, and a `limit+1` request before materializing results. The runner
  performs a second defensive result-limit check before sorting.
- Rendered errors and logs contain only validated provider metadata, phase,
  status, category, timeout, and duration. Raw queries, credentials,
  parameters, and fixture payloads must not be rendered.

The lifecycle order is:

```text
callback join -> fixture cleanup -> adapter close -> Run return -> container terminate
```

A callback that ignores cancellation is joined instead of being abandoned.
The surrounding `go test -timeout` is the fail-stop boundary, so cleanup and
driver close never race an active callback.

## Add a backend

The complete, compile-checked fake backend is in
[`example_test.go`](example_test.go). Copy its `exampleHarness` shape, replace
the in-memory callbacks with provider-specific fixed queries, and invoke it as
follows:

```go
func TestBackend(t *testing.T) {
	graphtest.Run(t, exampleHarness())
}
```

Use `RunWithConfig` only with every field set to a valid positive value. Start
from `DefaultConfig`, change the required bounds, and pass the complete value.

## Test

The harness self-test is Docker-free:

```bash
go test -race -count=10 ./graph/graphtest
```

Provider packages should run their Testcontainers-backed suites serially and
with an explicit process timeout.
