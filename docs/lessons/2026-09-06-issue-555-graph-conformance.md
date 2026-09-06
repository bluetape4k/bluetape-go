# Issue #555 Graph Backend Conformance 교훈

## Timeout은 반환값 ownership을 없애지 않는다

Callback을 abandon하지 않고 join하는 runner에서는 deadline 뒤에도 반환값을 받을 수
있다. 오류 판정을 먼저 하면 늦게 반환된 client/driver owner를 잃는다. Factory 결과의
`Close` 가능 여부를 먼저 확인하고 cleanup을 등록한 다음 timeout/error를 판정해야 한다.
이 순서는 성공, provider error, deadline에서 동일해야 한다.

## Fail-stop과 goroutine leak은 다른 계약이다

취소를 무시하는 callback을 background에 남긴 채 cleanup을 시작하면 active I/O와
fixture/driver 회수가 경합한다. Conformance runner는 callback을 join하고 외부
`go test -timeout`을 process fail-stop 경계로 사용한다. Unit process를 직접 멈추는
case는 짧은 subprocess test로 검증해야 한다.

## Redaction과 운영 진단을 함께 설계한다

Raw provider error, query, URI, credential을 출력하지 않는 것만으로 충분하지 않다.
검증된 provider/version/digest와 phase, status, category, timeout, duration처럼
low-cardinality allowlist를 남겨야 실패 원인을 재현할 수 있다. Subprocess output test는
요구 필드 존재와 secret marker 부재를 동시에 고정한다.

## Migration 삭제 전 live 참조를 다시 찾는다

계획은 legacy helper 삭제를 지시했지만 `benchmark_test.go`가
`waitForMemgraphConnectivity`와 `memgraphBoltPort`를 계속 사용했다. Side-by-side parity
뒤에도 `rg`로 live 참조를 확인하고, 중복 integration body만 제거해야 한다. 계획의 파일
목록보다 현재 tree의 호출 관계가 우선이다.

## Conformance 문서는 실행 가능한 entrypoint를 가리킨다

부분 snippet을 compile-checked example이라고 부르면 backend 작성자가 존재하지 않는
helper를 복사할 수 있다. 전체 fake harness는 실제 `example_test.go`에 두고 README는 그
파일과 짧은 invocation을 연결한다. Testcontainers 명령에는 직렬 실행과 process
timeout을 함께 적고, 상위 package recipe에도 Docker-free harness self-test를 포함한다.

## Go doc은 identifier를 정확히 보존한다

한국어 조사 앞에서도 exported identifier 뒤에 ASCII space를 둔다. 예를 들어
`// Config 값은`처럼 쓰면 golint의 identifier prefix와 한국어 가독성을 함께 지킨다.
이 규칙은 작업 중 `$bluetape-go-patterns`의 durable source와 live skill에 추가해 다음
Go 작업에서도 재사용할 수 있게 했다.
