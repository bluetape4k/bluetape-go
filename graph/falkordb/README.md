# graph/falkordb

`graph/falkordb` is a deliberately narrow OpenCypher adapter over a
caller-owned `redis.UniversalClient`. It sends `GRAPH.QUERY` with deterministic
parameter headers, validates bounded RESP results, and maps explicit rows to
the existing `graph.Vertex` and `graph.Edge` values.

```go
redisClient := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
client, _ := falkordb.NewClient(redisClient, "orders")
result, err := client.Query(ctx, "RETURN 1", nil)
```

The official `falkordb-go/v2` high-level API is compiled into the module graph,
but its package-global `context.Background()` boundary is not hidden here. This
adapter therefore uses `Do(ctx, ...)`; `Close` never closes the shared Redis
client, which remains caller-owned. Server-side traversal cancellation is not
claimed beyond the Redis context and local checkpoints.

Supported scope is query/result mapping, graph deletion, and a small
vertex/edge row shape. ORM/OGM, transactions, TinkerPop, implicit retries, and
performance parity are non-goals. The default tests use a fake Redis command;
the serial integration test uses a digest-pinned FalkorDB container.

```bash
go test -race -count=1 ./graph/falkordb
go test -p 1 -count=1 -timeout=10m ./testcontainers/falkordb ./graph/falkordb
```
