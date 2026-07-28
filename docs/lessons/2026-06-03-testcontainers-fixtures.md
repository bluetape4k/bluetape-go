# Testcontainers Fixtures

Issue #16은 bluetape-go infrastructure test의 shared fixture shape를 정했다. service
fixture는 `testcontainers/<service>` 아래에 두고, `t.Cleanup`으로 cleanup을 등록하는
작은 `Start(ctx, t)` helper를 노출한다.

fixture smoke test는 container가 시작됐다는 사실만 assert하지 말고 실제 service에
접속해야 한다. service가 dynamic host port를 bind하고 shared mutable state가 없으면
`t.Parallel()`을 사용해 fixture가 parallel-safe임을 증명한다.

Testcontainers module helper가 connection string을 제공하면 이를 사용한다.
PostgreSQL은 `PostgresContainer.ConnectionString(ctx, "sslmode=disable")`, MySQL은
`MySQLContainer.ConnectionString(ctx, "parseTime=true")`, NATS는
`NATSContainer.ConnectionString(ctx)`, Kafka는 `KafkaContainer.Brokers(ctx)`다.

현재 Redis local fixture는 connection-string helper를 노출하지 않으므로 `host:port`를
반환한다. package-local smoke test는 실제 `go-redis` client로 `PING`과 간단한
`SET`/`GET` round-trip을 검증해 package contract를 직접 보장한다.
