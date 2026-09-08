# testcontainers/tinkerpop

[English](README.md) | [한국어](README.ko.md)

이 package는 `graph/gremlin` local integration test를 위해
`tinkerpop/gremlin-server:3.8.1`을 immutable digest로 시작합니다. `/gremlin`으로
끝나는 WebSocket endpoint를 노출하고 test cleanup에 deterministic하게 등록합니다.

Fixture가 network port와 JVM process를 소유하므로 직렬로 실행하세요.

```bash
go test -p 1 -count=1 -timeout=10m ./testcontainers/tinkerpop ./graph/gremlin
```

Neptune과 다른 cloud endpoint는 이 fixture에서 시작하지 않습니다.
