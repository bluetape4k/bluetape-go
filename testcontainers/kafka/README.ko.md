# testcontainers/kafka

[English](README.md) | [한국어](README.ko.md)

`testcontainers/kafka`는 integration test를 위해 Kafka container를 시작하고 하나 이상의 broker address를 반환합니다.

## 가져오기

```go
import kafkatestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/kafka"
```

## 사용 예

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
t.Cleanup(cancel)

brokers := kafkatestcontainer.Start(ctx, t)
writer := &kafka.Writer{
    Addr:  kafka.TCP(brokers...),
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

## 운영 경계

- Docker 또는 다른 Testcontainers-compatible runtime이 필요합니다.
- Kafka startup은 작은 fixture보다 느릴 수 있으므로 start context에 explicit test timeout을 사용하세요.

## 테스트

```bash
go test -count=1 ./testcontainers/kafka
```
