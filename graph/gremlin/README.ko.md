# graph/gremlin

[English](README.md) | [한국어](README.ko.md)

`graph/gremlin`은 Apache TinkerPop Gremlin-Go `v3.8.1`을 사용하는 좁은 원격
adapter입니다. Bounded result channel을 수집하고 공식 `gremlingo.Vertex`/
`gremlingo.Edge`를 기존 `graph` model로 변환합니다.

```go
client, err := gremlin.NewRemoteClient("ws://localhost:8182/gremlin")
if err != nil {
	return err
}
defer client.Close(ctx)

result, err := client.Query(ctx, "g.V().limit(10)")
if err != nil {
	return err
}
_ = result.Values
```

## 경계

`NewClient`와 `NewConnectionClient`는 caller-owned executor를 감싸며 닫지
않습니다. `NewRemoteClient`는 공식 `DriverRemoteConnection`을 생성하므로 반환된
adapter가 connection을 소유하고 idempotent하게 닫습니다. TLS, 인증, traversal
source, connection pool 설정은 `WithConnectionConfiguration`으로 caller가
소유합니다.

Gremlin-Go `v3.8.1`의 remote `Submit`과 `ResultSet` 소비에는
`context.Context`가 없습니다. Adapter는 submit 전 context, result channel 대기 중
select, publish 직전 context를 확인하고 result stream을 닫습니다. 따라서 취소는
local 대기 시간을 제한하지만 server traversal이 즉시 취소된다고 주장하지 않습니다.

## 지원 범위

- optional binding과 evaluation timeout을 가진 remote traversal 제출
- bounded result 수집, provider/error redaction, result 변환
- `graph` model을 위한 vertex, edge, path/logical-key read
- 이 adapter 밖의 transaction 등 server 기능에 대한 명시적 unsupported error

Transaction, embedded TinkerGraph, 범용 Gremlin dialect, ORM/OGM, 암묵적 retry,
Neptune/cloud credential은 비목표입니다. Neptune은 live-opt-in으로만 사용하세요.
공식 connection을 직접 설정하고 credential을 로그와 public error 밖에서 관리해야
합니다.

## Test

Unit test는 fake result channel을 사용합니다. Local TinkerPop fixture는 digest를
고정하고 직렬로 실행합니다.

```bash
go test -race -count=1 ./graph/gremlin
go test -p 1 -count=1 -timeout=10m ./testcontainers/tinkerpop ./graph/gremlin
```
