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
- Docker resource나 port를 공유하는 Testcontainers package는 serial로
  실행하세요.
- Docker가 없는 CI job은 `./testcontainers/...`를 제외하고, 이 helper를 검증하는
  CI job은 `-p 1`로 실행하세요.

## 테스트

```bash
go test -p 1 -count=1 ./testcontainers/kafka
```
