# graph/graphtest

[English](README.md) | [한국어](README.ko.md)

Graph storage 구현을 `graph.Vertex`와 `graph.Edge` 의미로 검증하는 strict,
backend-neutral conformance test-support입니다. Production repository, query,
session, transaction, schema abstraction은 제공하지 않습니다.

## 계약

- `Run`은 모든 필드가 채워진 기본 `Config`를 사용합니다. `RunWithConfig`는 zero
  또는 partial config를 provider factory 호출 전에 거절합니다.
- Connectivity, empty read, create/read, cancellation, provider error, cleanup,
  close callback은 필수이며 skip할 수 없습니다.
- Traversal은 optional입니다. 비활성 traversal에는 검증된 stable `ReasonCode`가
  필요하고, 활성 traversal에는 `Adapter.Traverse`가 필요합니다.
- Provider가 container 생성, credential, endpoint 정책, readiness를 소유합니다.
  반환된 adapter는 client 또는 driver만 소유하며 정확히 한 번 close합니다.
- Read와 traversal adapter는 fixed query/column, bound parameter, materialization
  전 `limit+1` 요청을 사용해야 합니다. Runner는 sort 전에 result limit을 한 번 더
  방어적으로 검사합니다.
- Error와 log에는 검증된 provider metadata, phase, status, category, timeout,
  duration만 기록합니다. Raw query, credential, parameter, fixture payload는
  출력하지 않습니다.

Lifecycle 순서는 다음과 같습니다.

```text
callback join -> fixture cleanup -> adapter close -> Run return -> container terminate
```

Cancellation을 무시하는 callback도 분리하지 않고 join합니다. 바깥의
`go test -timeout`이 fail-stop 경계이므로 active callback과 cleanup 또는 driver
close가 경합하지 않습니다.

## 새 backend 연결

전체 compile-checked fake backend는
[`example_test.go`](example_test.go)에 있습니다. 이 파일의 `exampleHarness` 구성을
복사하고 in-memory callback을 provider 전용 fixed query로 바꾼 뒤 다음처럼
호출하세요.

```go
func TestBackend(t *testing.T) {
	graphtest.Run(t, exampleHarness())
}
```

`RunWithConfig`에는 모든 필드가 유효한 positive 값인 config만 전달합니다.
`DefaultConfig`에서 시작해 필요한 상한만 바꾼 뒤 완전한 값을 넘기세요.

## 테스트

Harness self-test는 Docker를 사용하지 않습니다.

```bash
go test -race -count=10 ./graph/graphtest
```

Provider package의 Testcontainers suite는 명시적인 process timeout을 두고 직렬로
실행해야 합니다.
