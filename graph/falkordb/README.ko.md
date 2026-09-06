# graph/falkordb

`graph/falkordb`는 caller-owned `redis.UniversalClient` 위에 제한된 OpenCypher
adapter를 제공합니다. `GRAPH.QUERY`에 deterministic parameter header를
붙이고 bounded RESP result를 검증한 뒤 기존 `graph.Vertex`와 `graph.Edge`로
명시적인 row를 변환합니다.

```go
redisClient := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
client, _ := falkordb.NewClient(redisClient, "orders")
result, err := client.Query(ctx, "RETURN 1", nil)
```

공식 `falkordb-go/v2`는 module graph에 compile되지만 package-global
`context.Background()` 경계를 숨기지 않습니다. 따라서 이 adapter는
`Do(ctx, ...)`를 사용합니다. `Close`는 shared Redis client를 닫지 않으며,
client 수명은 caller가 소유합니다. server-side traversal cancellation은
Redis context와 local checkpoint 이상을 보장한다고 주장하지 않습니다.

지원 범위는 query/result mapping, graph 삭제, 작은 vertex/edge row shape입니다.
ORM/OGM, transaction, TinkerPop, 암묵 retry, 성능 parity는 비목표입니다.
기본 테스트는 fake Redis command를 사용하고, serial integration test는
digest-pinned FalkorDB container를 사용합니다.

```bash
go test -race -count=1 ./graph/falkordb
go test -p 1 -count=1 -timeout=10m ./testcontainers/falkordb ./graph/falkordb
```
