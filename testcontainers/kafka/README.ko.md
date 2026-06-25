# testcontainers/kafka

[English](README.md) | [한국어](README.ko.md)

`testcontainers/kafka`는 integration test용 Kafka container를 시작하고 하나 이상의
broker address를 반환합니다.

![testcontainers helper flow](../../docs/images/readme-diagrams/testcontainers-helper-flow.png)

## 가져오기

```go
import kafkatestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/kafka"
```

## 사용 예

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
t.Cleanup(cancel)

brokers := kafkatestcontainer.Start(ctx, t)
details := map[string][]string{
    kafkatestcontainer.BrokersKey: brokers,
}
writer := &kafka.Writer{
    Addr:  kafka.TCP(details[kafkatestcontainer.BrokersKey]...),
    Topic: "bluetape-test",
}
t.Cleanup(func() {
    _ = writer.Close()
})
```

## Shared Server API

Broker slice만 필요하면 `Start(ctx, t)`를 사용하세요. Host lookup, mapped port,
endpoint, connection details, cleanup, manual termination, 명시적 env export가
필요하면 shared Testcontainers server contract를 반환하는 `StartServer(ctx, t)`를
사용하세요.

예제는 `tcserver`가 `github.com/bluetape4k/bluetape-go/testcontainers/server`를 alias import한다고 가정합니다.

```go
srv := kafkatestcontainer.StartServer(ctx, t)
details, err := srv.ConnectionDetails(ctx)
if err != nil {
    t.Fatalf("kafka details: %v", err)
}
if err := tcserver.ExportEnv(t, details, map[string]string{
    kafkatestcontainer.BrokersKey: "BLUETAPE_KAFKA_BROKERS",
}); err != nil {
    t.Fatal(err)
}
```

Generic `kafka.brokers` connection detail은 env export와 reporting용
comma-separated string입니다. `Start(ctx, t)`는 계속 `[]string`을 반환합니다.

`tcserver.ExportEnv`는 `testing.TB.Setenv`를 사용합니다. `t.Parallel`을
호출하거나 parallel ancestor가 있는 테스트에서는 사용하지 마세요.

## 동작

- `confluentinc/confluent-local:7.5.0`을 사용합니다.
- Cluster ID `bluetape-test-cluster`를 설정합니다.
- Testcontainers Kafka module에서 broker list를 반환합니다.
- Broker address가 없으면 test를 실패시킵니다.
- Container termination을 `t.Cleanup`에 등록합니다.
- Broker list key는 `kafkatestcontainer.BrokersKey` (`kafka.brokers`)로
  노출합니다.
- Start failure는 Docker unavailable, image pull failure, readiness timeout,
  context cancellation, wrapper failure로 구분해 보고합니다.

## 운영 경계

- Docker 또는 다른 Testcontainers-compatible runtime이 필요합니다.
- Kafka startup은 작은 fixture보다 느릴 수 있으므로 start context에 explicit test
  timeout을 사용하세요.
- Dynamic host port mapping이 기본입니다. Mapped port와 exported env value는
  container 시작 후 읽어야 하며, container-internal port가 아니라 host port를
  가리킵니다.
- Fixed host port는 parallel local run과 CI job에서 충돌할 수 있어 이 helper가
  노출하지 않습니다.
- Docker resource나 port를 공유하는 Testcontainers package는 serial로
  실행하세요.
- Docker가 없는 CI job은 `./testcontainers/...`를 제외하고, 이 helper를 검증하는
  CI job은 `-p 1`로 실행하세요.

## 테스트

```bash
go test -p 1 -count=1 ./testcontainers/kafka
```
