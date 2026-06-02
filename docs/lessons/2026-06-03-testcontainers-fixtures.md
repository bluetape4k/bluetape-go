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
for NATS this is `NATSContainer.ConnectionString(ctx)`.
