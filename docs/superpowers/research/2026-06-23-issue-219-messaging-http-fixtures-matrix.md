# Issue #219 Messaging, HTTP Mock, and Fault-Injection Matrix

Issue: [#219](https://github.com/bluetape4k/bluetape-go/issues/219)
Parent Epic: [#215](https://github.com/bluetape4k/bluetape-go/issues/215)
Date: 2026-06-23

## 현재 기준선

- 기존 래퍼는 Kafka와 NATS로 첫 messaging slice를 이미 덮는다.
- #217의 shared server 계약은 `testcontainers/server`에서 사용할 수 있다.
- #218은 첫 database/storage slice를 추가했고 같은 래퍼 형태를 좁게
  유지할 수 있음을 증명했다.
- `docs/research/2026-06-21-issue-202-source-parity-matrix.md`는
  messaging, HTTP mock, fault-injection gap을 #219로 라우팅하지만, package
  consumer 없는 광범위 catalog parity는 제외한다.

## 로드맵 증거

| Roadmap | Evidence | Fixture Implication |
|---|---|---|
| #46 / #56-#59 audit and outbox | Audit/outbox issue들은 아직 model, repository, publisher, example boundary를 정의 중이다. Kafka와 NATS는 이미 사용 가능하다. | Outbox adapter 설계가 새 broker를 선택하기 전에는 RabbitMQ/Redpanda를 추가하지 않는다. |
| #47 / #60-#64 AWS helpers | SQS/SNS-compatible local fixture는 AWS emulator 선택에 속한다. | ElasticMQ와 SNS/SQS emulator 선택은 #220/#61-#64로 보낸다. |
| #221 / #224 integration recipes | Recipe는 수정된 0.6.x package와 Testcontainers-backed helper contract, 미래 failure scenario를 검증해야 한다. | Fault-injection fixture는 새 broker를 고르지 않고도 Redis, DB, Kafka, NATS, 이후 HTTP recipe에서 재사용할 수 있다. |
| Existing package tests | HTTP package는 현재 local server와 package-local helper를 사용한다. | `httptest`로 증명할 수 없는 외부 HTTP 동작이 recipe나 package에서 필요해질 때까지 WireMock/Nginx를 추가하지 않는다. |

## Testcontainers-Go 모듈 가용성

다음 명령으로 확인했다.

```bash
go list -m -versions github.com/testcontainers/testcontainers-go/modules/<name>
```

| Candidate | Module Availability | Roadmap Need | Decision |
|---|---|---|---|
| Toxiproxy | `github.com/testcontainers/testcontainers-go/modules/toxiproxy` v0.37.0부터 v0.43.0까지. | Redis, DB, Kafka, NATS, HTTP, #224 recipe의 failure injection. | 첫 slice로 구현한다. |
| RabbitMQ | `github.com/testcontainers/testcontainers-go/modules/rabbitmq` v0.25.0부터 v0.43.0까지. | #58이 adapter semantics를 선택한 뒤 가능한 audit/outbox broker. | #58이 RabbitMQ를 선택할 때까지 연기한다. |
| Redpanda | `github.com/testcontainers/testcontainers-go/modules/redpanda` v0.20.0부터 v0.43.0까지. | Kafka-compatible alternative. 기존 Kafka 래퍼가 현재 broker test를 이미 덮는다. | Package가 Redpanda-specific behavior를 필요로 할 때까지 연기한다. |
| Pulsar | `github.com/testcontainers/testcontainers-go/modules/pulsar` v0.19.0부터 v0.43.0까지. | 현재 roadmap에 live consumer가 없다. | 연기한다. Catalog parity를 추가하지 않는다. |
| WireMock | Module path는 해석되지만 이번 확인에서 version list가 드러나지 않았다. | #224 recipe가 필요로 할 때 external HTTP mock server. | `httptest`가 부족하다는 증거가 있을 때까지 연기한다. |
| Nginx | Module path는 해석되지만 이번 확인에서 version list가 드러나지 않았다. | Recipe가 필요한 경우에만 reverse proxy/static HTTP behavior. | 연기한다. |
| Mailpit | Module path는 해석되지만 이번 확인에서 version list가 드러나지 않았다. | Email flow에는 현재 package consumer가 없다. | 연기한다. |
| ElasticMQ | Module path는 해석되지만 이번 확인에서 version list가 드러나지 않았다. | AWS SQS/SNS-compatible fixture. | #220/#61-#64로 연기한다. |

## 첫 Slice

`testcontainers/toxiproxy`만 구현한다.

- 현재 Testcontainers-Go module example과 맞춰 image
  `ghcr.io/shopify/toxiproxy:2.12.0`을 사용한다.
- shared `testcontainers/server` contract와 upstream Testcontainers
  customizer를 사용해 `Start(ctx, testing.TB, opts...)`와
  `StartServer(ctx, testing.TB, opts...)`를 노출한다.
- 설정된 proxy endpoint를 읽기 위해 upstream Toxiproxy container가 필요한
  테스트용으로 `StartContainer(ctx, testing.TB, opts...)`를 노출한다.
- `ControlURIKey = "toxiproxy.control_uri"`를 문서화한다.
- 구성된 proxied endpoint를 `host:port`로 읽는 helper를 제공한다.
- live control API/proxy scenario로 readiness를 증명한다.
- 일반 package test를 flaky하게 만들지 않으면서 대표 integration test
  하나에서 Redis 대상 failure injection을 검증한다.

## 연기된 후속 작업

- RabbitMQ/Redpanda/Pulsar: #58이 delivery semantics와 broker adapter를
  선택해야 #219가 다른 broker wrapper를 추가할 수 있다.
- WireMock/Nginx/Mailpit: #224 또는 구체 package issue가 package
  `httptest` helper로 부족함을 증명해야 한다.
- ElasticMQ/SNS/SQS emulator: #220과 #61-#64가 AWS emulator 선택을
  소유한다.
