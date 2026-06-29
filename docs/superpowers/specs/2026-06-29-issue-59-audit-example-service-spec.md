# Issue #59 Spec: Audit Example Service

## Scope

- Repository: `bluetape4k/bluetape-go`
- Issue: #59 `Add audit event example service`
- Milestone: `0.9.0`

## Requirements

- Add a runnable Go example service with `go test`.
- Demonstrate aggregate changes, audit repository writes, audit history queries,
  and optional outbox replay.
- Keep the example Go-native and framework-free.
- Document that the example is not a full event-sourcing framework, JaVers-style
  object diff engine, or durable source-of-truth database.

## Source Evidence

Read source examples under `/Users/debop/work/bluetape4k/bluetape4k-javers`:

- `examples/javers-exposed-ddd/README.md`
- `examples/javers-ktor/README.md`
- `examples/javers-spring-boot4/README.md`

Shared source shape: command-side order state is the source of truth, audit
history is read through JaVers snapshots, and Kafka/Redis/framework wiring is
example-specific infrastructure. The Go example ports the boundary, not the JVM
framework integration.

## Acceptance

- `examples/audit` has an `ExampleOrderService` with stable output.
- Tests cover repository boundary failure, history queries, outbox replay,
  concurrent commands with `GoroutineStressTester`, and cancellation with
  `AsyncJobTester`.
- README and README.ko explain the non-goals and future `audit/sqloutbox` path.
