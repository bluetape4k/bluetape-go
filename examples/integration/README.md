# Integration Recipes

[English](README.md) | [한국어](README.ko.md)

This package contains compile-checked recipes that combine the corrected `0.6.x`
packages in application-shaped flows. It is intentionally placed under
`examples/` so package docs can show cross-package usage without turning those
flows into public helper APIs.

## Recipes

- `Example_batchWorkflowJWTCacheAndResilienceRecipe` creates a request ID,
  signs and parses a JWT, loads a profile through `cache.Memory` protected by
  retry and timeout policies, then runs a checkpointed batch import inside a
  `workflow.Sequential` runner.
- `TestConcurrentIDAndJWTRecipe` exercises UUID v7 generation and JWT
  compose/parse from multiple goroutines.
- `TestRedisCoordinationRecipeSmoke` starts Redis with `testcontainers/redis`,
  takes a Redis owner-token lock, campaigns for Redis-backed leadership, runs
  the batch recipe, and stores the outcome in Redis.

## Failure Paths

The batch recipe uses a temporary writer failure to prove retry accounting and
an invalid input item to prove skip accounting. The reader implements checkpoint
capture and restore, so a real job can move the checkpoint store to Redis,
Postgres, or another durable owner without changing the step contract.

The profile loader is protected by both retry and timeout policies. Production
callers should keep the parent `context.Context` deadline narrower than the
request budget and use policy event hooks for metrics or logs.

## Cleanup and Timeouts

Each recipe creates an explicit timeout context. The Redis smoke test registers
cleanup for the Redis client, Redis lock lease, leadership lease, and
Testcontainers-managed container. Run Testcontainers-backed packages serially
when Docker resources or ports are shared.

## Test

Compile-check and run the service-free recipes:

```bash
go test -count=1 ./examples/integration
```

Run the Docker-backed Redis coordination smoke test:

```bash
BLUETAPE_INTEGRATION_RECIPE_SMOKE=1 go test -p 1 -count=1 ./examples/integration
```

Run the race detector for the concurrency recipe:

```bash
go test -race -count=1 ./examples/integration
```

## Related Packages

- [`batch`](../../batch/README.md)
- [`cache`](../../cache/README.md)
- [`id`](../../id/README.md)
- [`jwt`](../../jwt/README.md)
- [`leader/redis`](../../leader/redis/README.md)
- [`lock/redis`](../../lock/redis/README.md)
- [`resilience`](../../resilience/README.md)
- [`testcontainers/redis`](../../testcontainers/redis/README.md)
- [`workflow`](../../workflow/README.md)
