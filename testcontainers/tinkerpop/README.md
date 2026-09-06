# testcontainers/tinkerpop

[English](README.md) | [한국어](README.ko.md)

This package starts `tinkerpop/gremlin-server:3.8.1` from an immutable digest
for local `graph/gremlin` integration tests. It exposes a WebSocket endpoint
ending in `/gremlin` and registers deterministic cleanup with the test.

Run it serially because the fixture owns a network port and a JVM process:

```bash
go test -p 1 -count=1 -timeout=10m ./testcontainers/tinkerpop ./graph/gremlin
```

Neptune and other cloud endpoints are not started by this fixture.
