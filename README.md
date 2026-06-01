# bluetape-go

[English](README.md) | [한국어](README.ko.md)

![bluetape-go hero](docs/assets/bluetape-go-hero.png)

Go backend utilities and distributed infrastructure packages for the bluetape
ecosystem.

`bluetape-go` complements the Kotlin/JVM bluetape4k libraries. It is not a
rewrite of bluetape4k. It provides idiomatic Go packages for teams that prefer
Go for backend infrastructure, service coordination, test fixtures, resilience,
caching, workflow, batch, graph, text, audit, and AWS-adjacent service code.

## Current Status

`bluetape-go` is in early `0.1.0` development. The repository already contains
the first foundation packages and Redis-backed leader election, and the rest of
the roadmap is tracked through milestones and research notes.

## Packages

| Package | Status | Purpose |
|---|---:|---|
| `core` | initial | Small shared validation and support helpers. |
| `testing` | initial | Common test helpers for eventual consistency checks. |
| `testcontainers/redis` | initial | Redis fixture helpers based on Testcontainers for Go. |
| `leader` | initial | Leader election API. |
| `leader/redis` | initial | Redis-backed leader election using `SET NX PX` and TTL renewal. |

Planned package families include `collections`, `concurrency`, `serialization`,
`resilience`, `cache`, `workflow`, `batch`, `id`, `jwt`, `graph`, `text`,
`audit`, and AWS helper/example packages.

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
will be decided before the first stable tag.

## Roadmap

| Milestone | Theme |
|---|---|
| `0.1.0` | Core support, collections, goroutine helpers, codecs, compression, Redis leader election, Testcontainers. |
| `0.2.0` | Resilience primitives: retry, timeout, circuit breaker, bulkhead, HTTP middleware. |
| `0.3.0` | Cache and coordination: near cache, Redis locks, token-bucket rate limiting. |
| `0.4.0` | State machine and lightweight workflow primitives. |
| `0.5.0` | Batch processing with checkpoints and leader-guarded examples. |
| `0.6.0` | Portable utilities: ID generation, JWT, measured values, money, probabilistic structures, rule engine. |
| `0.7.0` | Research gate for larger domains. |
| `0.8.0` | Graph packages and examples. |
| `0.9.0` | Text search, blockword masking, tokenizer research. |
| `0.10.0` | Audit and event packages inspired by bluetape4k-javers patterns. |
| `0.11.0` | AWS helper packages and LocalStack examples. |

See the [GitHub milestones](https://github.com/bluetape4k/bluetape-go/milestones)
and [`docs/research`](docs/research/) for the current planning record.

## Development

```bash
make test
make ci
```

Common commands:

| Command | Purpose |
|---|---|
| `make fmt` | Format Go sources with `gofmt`. |
| `make fmt-check` | Fail when Go sources are not formatted. |
| `make tidy` | Run `go mod tidy`. |
| `make tidy-check` | Fail when `go.mod` or `go.sum` drift after `go mod tidy`. |
| `make vet` | Run `go vet ./...`. |
| `make lint` | Run `golangci-lint run ./...`. |
| `make test` | Run `go test ./...`. |
| `make race` | Run `go test -race ./...`. |
| `make ci` | Run the local CI gate. |

Redis integration tests use Testcontainers and require Docker.

## Project Management

- [Changelog](CHANGELOG.md)
- [Current WIP](WIP.md)
- [Research index](docs/research/README.md)
- [Package layout policy](docs/package-layout.md)
- [Release guide](docs/release.md)

## Project Rules

- Keep APIs idiomatic to Go. Do not mechanically copy Kotlin extension-style
  APIs.
- Prefer small packages with clear service value over catch-all utility bags.
- Use proven Go dependencies where they reduce risk, but avoid wrapping mature
  SDKs without a bluetape-specific reason.
- Add Testcontainers-backed smoke tests for infrastructure packages.
