# Testcontainers Fixtures

Issue #16 establishes the shared fixture shape for bluetape-go infrastructure
tests. Keep service fixtures under `testcontainers/<service>` and expose a small
`Start(ctx, t)` helper that registers cleanup through `t.Cleanup`.

Fixture smoke tests should connect to the real service, not only assert that a
container started. Use `t.Parallel()` when the service binds dynamic host ports
and has no shared mutable state, so the package proves the fixture is
parallel-safe.

Use the Testcontainers module helper when it provides a connection string. For
PostgreSQL this is `PostgresContainer.ConnectionString(ctx, "sslmode=disable")`;
for MySQL this is `MySQLContainer.ConnectionString(ctx, "parseTime=true")`;
for NATS this is `NATSContainer.ConnectionString(ctx)`; for Kafka this is
`KafkaContainer.Brokers(ctx)`.

Redis does not expose a connection-string helper in the current local fixture;
the fixture returns `host:port`. Keep its package-local smoke test on a real
`go-redis` client and verify both `PING` and a simple `SET`/`GET` round-trip so
the package contract is covered directly, not only through leader integration
tests.
