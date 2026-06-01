# bluetape-go

Go backend utilities and distributed infrastructure packages for the bluetape
ecosystem.

`bluetape-go` complements the Kotlin/JVM bluetape4k libraries. It is not a
rewrite of bluetape4k; it provides idiomatic Go packages for teams that prefer
Go in backend infrastructure and service components.

## Packages

| Package | Status | Purpose |
|---|---:|---|
| `core` | initial | Small shared validation and support helpers. |
| `testing` | initial | Common test helpers for eventual consistency checks. |
| `testcontainers/redis` | initial | Redis fixture helpers based on Testcontainers for Go. |
| `leader` | initial | Leader election API. |
| `leader/redis` | initial | Redis-backed leader election using `SET NX PX` and TTL renewal. |

## Install

```bash
go get github.com/bluetape4k/bluetape-go
```

## Leader Election

```go
client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})

elector, err := redisleader.New(client, leader.Options{
    Group:    "billing-workers",
    MemberID: "worker-1",
})
if err != nil {
    return err
}

if err := elector.Campaign(ctx); err != nil {
    return err
}
defer elector.Resign(context.Background())
```

The Kotlin/JVM `bluetape4k-leader` repository remains supported separately.
Cross-language Redis key compatibility is still an explicit design question and
should be decided before the first stable tag.

## Development

```bash
go test ./...
```

Redis integration tests use Testcontainers and require Docker.

