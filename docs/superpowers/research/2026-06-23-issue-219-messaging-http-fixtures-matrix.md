# Issue #219 Messaging, HTTP Mock, and Fault-Injection Matrix

Issue: [#219](https://github.com/bluetape4k/bluetape-go/issues/219)
Parent Epic: [#215](https://github.com/bluetape4k/bluetape-go/issues/215)
Date: 2026-06-23

## Current Baseline

- Existing wrappers: Kafka and NATS already cover the first messaging slice.
- Shared server contract from #217 is available in `testcontainers/server`.
- #218 added the first database/storage slice and proved the same wrapper shape
  can stay narrow.
- `docs/research/2026-06-21-issue-202-source-parity-matrix.md` routes
  messaging, HTTP mock, and fault-injection gaps to #219, but excludes broad
  catalog parity without package consumers.

## Roadmap Evidence

| Roadmap | Evidence | Fixture Implication |
|---|---|---|
| #46 / #56-#59 audit and outbox | Audit/outbox issues are still defining model, repository, publisher, and example boundaries. Kafka and NATS are already available. | Do not add RabbitMQ/Redpanda before the outbox adapter design selects a new broker. |
| #47 / #60-#64 AWS helpers | SQS/SNS-compatible local fixtures belong to AWS emulator selection. | Route ElasticMQ and SNS/SQS emulator choices to #220/#61-#64. |
| #221 / #224 integration recipes | Recipes should exercise corrected 0.6.x packages with Testcontainers-backed helper contracts and future-facing failure scenarios. | A fault-injection fixture can be reused by Redis, DB, Kafka, NATS, and later HTTP recipes without choosing a new broker. |
| Existing package tests | HTTP packages currently use local servers and package-local helpers. | Do not add WireMock or Nginx until a recipe or package needs external HTTP behavior that `httptest` cannot prove. |

## Testcontainers-Go Module Availability

Checked with:

```bash
go list -m -versions github.com/testcontainers/testcontainers-go/modules/<name>
```

| Candidate | Module Availability | Roadmap Need | Decision |
|---|---|---|---|
| Toxiproxy | `github.com/testcontainers/testcontainers-go/modules/toxiproxy` v0.37.0 through v0.43.0. | Failure injection for Redis, DB, Kafka, NATS, HTTP, and #224 recipes. | Implement first slice. |
| RabbitMQ | `github.com/testcontainers/testcontainers-go/modules/rabbitmq` v0.25.0 through v0.43.0. | Possible audit/outbox broker after #58 chooses adapter semantics. | Defer until #58 selects RabbitMQ. |
| Redpanda | `github.com/testcontainers/testcontainers-go/modules/redpanda` v0.20.0 through v0.43.0. | Kafka-compatible alternative; existing Kafka wrapper already covers current broker tests. | Defer until a package needs Redpanda-specific behavior. |
| Pulsar | `github.com/testcontainers/testcontainers-go/modules/pulsar` v0.19.0 through v0.43.0. | No live consumer in current roadmap. | Defer; do not add catalog parity. |
| WireMock | Module path resolves, but no version list surfaced in this check. | External HTTP mock server if #224 recipes need it. | Defer until `httptest` is insufficient. |
| Nginx | Module path resolves, but no version list surfaced in this check. | Reverse proxy/static HTTP behavior only if a recipe needs it. | Defer. |
| Mailpit | Module path resolves, but no version list surfaced in this check. | Email flows do not have a current package consumer. | Defer. |
| ElasticMQ | Module path resolves, but no version list surfaced in this check. | AWS SQS/SNS-compatible fixture. | Defer to #220/#61-#64. |

## First Slice

Implement `testcontainers/toxiproxy` only:

- uses image `ghcr.io/shopify/toxiproxy:2.12.0`, matching the current
  Testcontainers-Go module examples;
- exposes `Start(ctx, testing.TB, opts...)` and
  `StartServer(ctx, testing.TB, opts...)` with the shared
  `testcontainers/server` contract and upstream Testcontainers customizers;
- exposes `StartContainer(ctx, testing.TB, opts...)` for tests that need the
  upstream Toxiproxy container to read configured proxy endpoints;
- documents `ControlURIKey = "toxiproxy.control_uri"`;
- provides a helper to read a configured proxied endpoint as `host:port`;
- proves readiness through a live control API/proxy scenario;
- verifies failure injection against Redis in one representative integration
  test without making normal package tests flaky.

## Deferred Follow-Ups

- RabbitMQ/Redpanda/Pulsar: #58 must select delivery semantics and broker
  adapters before #219 adds another broker wrapper.
- WireMock/Nginx/Mailpit: #224 or a concrete package issue must prove package
  `httptest` helpers are insufficient.
- ElasticMQ/SNS/SQS emulators: #220 and #61-#64 own AWS emulator selection.
