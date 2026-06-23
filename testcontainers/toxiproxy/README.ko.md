# testcontainers/toxiproxy

[English](README.md) | [한국어](README.ko.md)

`testcontainers/toxiproxy`는 네트워크 장애 주입이 필요한 통합 테스트를
위해 Toxiproxy 컨테이너를 시작합니다.

![testcontainers helper flow](../../docs/images/readme-diagrams/testcontainers-helper-flow.png)

## Import

```go
import toxiproxytestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/toxiproxy"
```

## Usage

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
t.Cleanup(cancel)

controlURI := toxiproxytestcontainer.Start(ctx, t)
details := map[string]string{
    toxiproxytestcontainer.ControlURIKey: controlURI,
}
client := toxiproxy.NewClient(details[toxiproxytestcontainer.ControlURIKey])
_ = client
```

## Shared Server API

Toxiproxy control URI만 필요하면 `Start(ctx, t)`를 사용합니다. 공유
Testcontainers server contract가 필요한 테스트는 `StartServer(ctx, t)`를
사용합니다. 이 API는 host 조회, mapped port, endpoint, connection details,
cleanup, manual termination, 명시적 env export를 제공합니다.

아래 예제는 `tcserver`가 `github.com/bluetape4k/bluetape-go/testcontainers/server`의 alias라고 가정합니다.

```go
srv := toxiproxytestcontainer.StartServer(ctx, t)
details, err := srv.ConnectionDetails(ctx)
if err != nil {
    t.Fatalf("toxiproxy details: %v", err)
}
if err := tcserver.ExportEnv(t, details, map[string]string{
    toxiproxytestcontainer.ControlURIKey: "BLUETAPE_TOXIPROXY_CONTROL_URI",
}); err != nil {
    t.Fatal(err)
}
```

`tcserver.ExportEnv`는 `testing.TB.Setenv`를 사용합니다. `t.Parallel`을
사용하거나 parallel ancestor가 있는 테스트에서는 호출하지 마세요.

## Proxying Another Container

구성된 proxy endpoint를 읽기 위해 upstream Toxiproxy container가 필요한
테스트는 `StartContainer(ctx, t, opts...)`를 사용합니다.

```go
nw, err := network.New(ctx)
if err != nil {
    t.Fatalf("create network: %v", err)
}
t.Cleanup(func() { _ = nw.Remove(ctx) })

redisContainer, err := redis.Run(ctx, "redis:7.4-alpine", network.WithNetwork([]string{"redis"}, nw))
if err != nil {
    t.Fatalf("start redis: %v", err)
}
t.Cleanup(func() { _ = testcontainers.TerminateContainer(redisContainer) })

toxiproxyContainer := toxiproxytestcontainer.StartContainer(
    ctx,
    t,
    tctoxiproxy.WithProxy("redis", "redis:6379"),
    network.WithNetwork([]string{"toxiproxy"}, nw),
)
endpoint := toxiproxytestcontainer.ProxiedEndpoint(ctx, t, toxiproxyContainer, 8666)
redisClient := redisclient.NewClient(&redisclient.Options{
    Addr:        endpoint,
    DialTimeout: 500 * time.Millisecond,
    ReadTimeout: 500 * time.Millisecond,
})
t.Cleanup(func() { _ = redisClient.Close() })
```

장애 주입은 upstream `github.com/Shopify/toxiproxy/v2` client로 구성합니다.
일반 CI에서는 latency window assertion보다 deterministic enable/disable
테스트를 선호하세요.

## Behavior

- `ghcr.io/shopify/toxiproxy:2.12.0` 이미지를 사용합니다.
- Toxiproxy control API URI를 반환합니다.
- `t.Cleanup`으로 컨테이너 종료를 등록합니다.
- control URI key는 `toxiproxytestcontainer.ControlURIKey`
  (`toxiproxy.control_uri`)입니다.
- `tctoxiproxy.WithProxy(...)`, `network.WithNetwork(...)` 같은 upstream
  Testcontainers customizer를 받습니다.
- 시작 실패는 Docker unavailable, image pull failure, readiness timeout,
  context cancellation, wrapper failure로 분류됩니다.

## Operational Boundaries

- Docker 또는 Testcontainers 호환 runtime이 필요합니다.
- Toxiproxy는 테스트용입니다. 이 helper로 production traffic을 라우팅하지
  마세요.
- dynamic host port mapping이 기본입니다. mapped port와 exported env 값은
  컨테이너 시작 이후에 읽어야 하며, container-internal port가 아니라 host
  port를 가리킵니다.
- 장애 주입 테스트는 bounded client timeout과 context deadline을 사용해야
  합니다. 일반 CI에서 broad, always-on failure test는 피하세요.
- fixed host port는 parallel local run과 CI job에서 충돌할 수 있으므로 이
  helper는 노출하지 않습니다.
- Docker-backed Testcontainers package는 resource나 port를 공유할 때 serial로
  실행하세요.
- Docker가 없는 CI job은 `./testcontainers/...`를 skip해야 합니다. 이
  helper를 검증하는 CI job은 package를 `-p 1`로 실행하세요.

## Deferred Scope

- RabbitMQ, Redpanda, Pulsar는 outbox adapter delivery semantics가 broker를
  고르는 #58까지 보류합니다.
- WireMock, Nginx, Mailpit은 `httptest`로 부족하다는 점을 #224나 구체
  package issue가 증명할 때까지 보류합니다.
- ElasticMQ와 SNS/SQS-compatible emulator는 #220 및 #61-#64로 보류합니다.

## Test

```bash
go test -p 1 -count=1 ./testcontainers/toxiproxy
```
