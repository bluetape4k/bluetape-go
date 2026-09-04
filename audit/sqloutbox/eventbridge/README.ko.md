# audit/sqloutbox/eventbridge

[English](README.md) | [한국어](README.ko.md)

`audit/sqloutbox/eventbridge`는 caller-owned AWS SDK for Go v2 EventBridge
client를 사용해 `sqloutbox.Record` 하나를 `PutEvents` 호출 하나로 전달합니다.

## Import와 사용

```go
import (
	awseventbridge "github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/bluetape4k/bluetape-go/audit/sqloutbox/eventbridge"
)

publisher, err := eventbridge.New(eventbridge.Options{
	Client:       awseventbridge.NewFromConfig(cfg),
	EventBusName: "audit",
	Source:       "com.example.billing",
	DetailType:   "AuditRecorded",
})
if err != nil {
	return err
}

relay, err := sqloutbox.NewRelay(store, publisher, sqloutbox.RelayOptions{
	ClaimLimit:  10,
	MaxAttempts: 3,
})
```

`Client`는 이 package에 필요한 최소 SDK method subset입니다. AWS client 생성,
설정, timeout, retry, 관측, close는 caller가 소유하며 publisher는 credential을
로드하거나 client를 만들거나 logger를 설치하지 않습니다. child package에는
fake client를 사용하는 compile-checked `ExampleNew`가 있습니다.

## EventBridge entry와 detail

각 `Publish`는 `PutEventsRequestEntry`를 정확히 하나 전송합니다. `Source`와
`DetailType`은 필수이며 caller가 준 bytes를 그대로 보존합니다. `EventBusName`이
비어 있으면 request pointer를 생략해 AWS default event bus를 선택하고, 값이나
ARN이 있으면 정규화하지 않고 그대로 전달합니다. `Time`은
`Record.OccurredAt`입니다.

JSON `Detail`은 Redis Streams publisher와 같은 stable envelope 이름을 갖는
bounded object입니다.

| Field | 의미 |
|---|---|
| `record_id`, `status`, `attempts` | SQL outbox 진단 상태 |
| `aggregate_type`, `aggregate_id`, `revision` | aggregate ordering identity |
| `event_id`, `idempotency_key`, `event_type` | downstream stable identity와 event contract |
| `occurred_at`, `recorded_at`, `schema_version` | UTC RFC3339Nano와 schema metadata |
| `entry_json` | 검증된 전체 `audit.Entry` JSON object |

adapter는 SDK 호출 전에 record와 entry를 검증합니다. `MaxDetailSize` 기본값은
`256 << 10`이며, preflight에서 UTF-8 `Source`, `DetailType`, optional bus bytes를
더해 256 KiB 이상인 EventBridge entry를 거부합니다. 크기 초과 또는 invalid
record는 SDK 호출을 0회 수행합니다.

## Failure와 cancellation

- dispatch 전 또는 response 직후 caller cancellation은
  `context.Canceled`/`context.DeadlineExceeded`로 반환되고
  `sqloutbox.Relay`가 retry/dead-letter하지 않습니다.
- transport/client 오류는 `ErrPublishFailed`와 matching되는 안전한
  `*eventbridge.Error`로 반환되며 `errors.Is`로 주입 원인을 관찰할 수 있습니다.
- `FailedEntryCount`, entry `ErrorCode` 또는 `ErrorMessage`가 있는 non-nil
  response는 bounded failure count/code와 함께 `ErrPartialFailure`를 반환합니다.
- nil output, entry 수 불일치, 불가능한 failure count, `EventId`가 없는 성공
  result는 `ErrMalformedOutput`을 반환합니다.

`Error()`와 `%+v`에는 EventBridge `ErrorMessage`, detail, payload, credentials,
bus/source 값, response `EventId`가 들어가지 않습니다. Response `EventId`는
outbox `event_id`가 아니며 deduplication에 사용하지 않습니다. Consumer는
detail의 stable `event_id` 또는 `idempotency_key`로 retry 중복을 제거해야 합니다.

## 운영 경계

- Event bus, rule, target, endpoint, IAM, authentication, retention과
  production rollout은 caller/operator 책임입니다.
- AWS config, credential, client lifecycle, timeout, retry/backoff, metric,
  log와 hook은 caller 책임입니다.
- 이 package는 entry batching, SDK retry, accepted event 삭제, downstream
  idempotency storage를 제공하지 않습니다.
- 기본 CI는 fake client만 사용하며 live AWS smoke test를 의미하지 않습니다.

## Tests

```bash
go test -count=1 ./audit/sqloutbox/eventbridge
go test -race -count=1 ./audit/sqloutbox/eventbridge
go vet ./audit/sqloutbox/eventbridge
```

Fake client는 request를 deep-copy하고 logical call/context를 기록하며 blocking,
cancellation, output-plus-error 응답을 지원합니다. Constructor 검증, stable
detail, size bound, failure mapping, redaction, relay semantics와 concurrent
request isolation을 Docker나 AWS 없이 증명합니다.
