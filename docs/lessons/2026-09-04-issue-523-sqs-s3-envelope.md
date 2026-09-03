# #523 SQS S3 envelope 교훈

## 결정

Go에서는 SQS 전체 client를 감싸기보다 caller-owned SQS/S3 SDK method subset을
주입하는 얇은 `messaging/sqsextended` adapter가 적절하다. payload object의
bucket/key는 caller identity로 보존하고, provider는 credential·retry·timeout·
logging·queue topology·lifecycle·DLQ·replay를 소유하지 않는다.

## 재사용할 패턴

- S3 `PutObject` 성공 뒤 SQS `SendMessage`가 실패할 수 있으므로 자동 보상
  삭제를 시도하지 않고 `OrphanedObject()`와 explicit `DeleteObject` capability로
  불확실한 orphan 수명을 caller/operator에게 돌린다.
- SQS 수신은 envelope, declared size, response body close와 SHA-256을 검증하지만
  acknowledge하지 않는다. visibility timeout과 processing budget은 caller가
  함께 설계해야 한다.
- 삭제 순서를 SQS first로 고정하면 S3 cleanup 실패가 queue ack 이후에 발생한다.
  `QueueDeleted()`를 통해 repair/lifecycle 경로가 이미 지워진 queue message를
  구분할 수 있다.
- canonical JSON은 field/map order를 고정하고 unknown/duplicate/trailing field,
  invalid UTF-8과 bound 초과를 거부해야 재시도·보관·다른 언어 reader 사이의
  wire ambiguity를 줄일 수 있다.
- 외부 SDK error는 `errors.Is`/`errors.As`로 원인과 상태를 관찰하되 `Error()`와
  `%+v`에는 provider URL, object key, checksum, payload와 raw diagnostic을
  포함하지 않는다.
- side effect 직후 cancellation은 단순 `context.Canceled`로 버리지 말고
  `ErrCanceled`와 `OrphanedObject`/`QueueDeleted` 상태를 함께 보존해야 caller가
  중복 ack나 cleanup 누락 없이 reconciliation할 수 있다.
- fake는 SDK request와 response body를 복사하고 context/order/call count를
  기록해야 live AWS credential 없이 partial success, output anomaly, cancellation,
  race를 재현할 수 있다.

- Receive batch는 개별 payload bound만으로는 여러 near-limit object가 수 GiB를
  동시에 보관할 수 있다. bounded aggregate budget을 먼저 계산하고 초과 batch는
  S3 dispatch 전에 거부해 partial read와 peak memory를 함께 제한한다.

## 유예 범위

LocalStack/Floci live test, orphan background sweeper, visibility extension,
IAM/queue/bucket provisioning, replay/DLQ tooling과 exactly-once delivery는 이
slice에서 구현하지 않는다. 해당 요구가 실제 call site로 생기면 별도 issue와
운영 계약을 먼저 만든다.

## 검증

`go test -count=1`, `go test -race -count=1`, `go test -run Example`,
`go vet`, `make fmt-check`, `make tidy-check`, `make vet`와 `make lint`는
package와 repository 정적 검증에서 통과했다. `make test`는 새 package 실행은
통과했지만 기존 `leader/sql.TestPostgresLifecycle/renewal`이 1.001초 경계에서
timeout되어 전체 target은 실패했다. live AWS와 emulator는 실행하지 않았으며,
기본 CI가 credential/Docker를 요구하지 않는다는 점 자체를 계약으로 유지한다.
