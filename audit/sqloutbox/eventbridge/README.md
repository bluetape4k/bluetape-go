# audit/sqloutbox/eventbridge

[English](README.md) | [한국어](README.ko.md)

`audit/sqloutbox/eventbridge` publishes one claimed `sqloutbox.Record` per
`PutEvents` call using a caller-owned AWS SDK for Go v2 EventBridge client.

## Import and usage

```go
import (
	awseventbridge "github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/bluetape4k/bluetape-go/audit/sqloutbox/eventbridge"
)

publisher, err := eventbridge.New(eventbridge.Options{
	Client:     awseventbridge.NewFromConfig(cfg),
	EventBusName: "audit",
	Source:     "com.example.billing",
	DetailType: "AuditRecorded",
})
if err != nil {
	return err
}

relay, err := sqloutbox.NewRelay(store, publisher, sqloutbox.RelayOptions{
	ClaimLimit: 10,
	MaxAttempts: 3,
})
```

`Client` is the smallest SDK method subset needed by this package. The caller
creates, configures, times out, retries, observes, and closes the AWS client;
the publisher never loads credentials, creates a client, or installs a logger.
The child package includes a compile-checked `ExampleNew` using a fake client.

## EventBridge entry and detail

Every `Publish` sends exactly one `PutEventsRequestEntry`. `Source` and
`DetailType` are required and preserved byte-for-byte. An empty `EventBusName`
omits the request pointer and selects the AWS default event bus; a non-empty bus
name or ARN is passed unchanged. `Time` is `Record.OccurredAt`.

The JSON `Detail` is a bounded object with the same stable envelope names as
the Redis Streams publisher:

| Field | Meaning |
|---|---|
| `record_id`, `status`, `attempts` | SQL outbox diagnostic state |
| `aggregate_type`, `aggregate_id`, `revision` | aggregate ordering identity |
| `event_id`, `idempotency_key`, `event_type` | stable downstream identity and event contract |
| `occurred_at`, `recorded_at`, `schema_version` | UTC RFC3339Nano and schema metadata |
| `entry_json` | validated full `audit.Entry` JSON object |

The adapter validates the record and entry before the SDK call. `MaxDetailSize`
defaults to `256 << 10`; the preflight also adds UTF-8 `Source`, `DetailType`,
and optional bus bytes and rejects an EventBridge entry at or above the 256 KiB
limit. Oversized or invalid records make zero SDK calls.

## Failure and cancellation

- A pre-dispatch or post-response caller cancellation returns
  `context.Canceled` or `context.DeadlineExceeded` and is not retried or
  dead-lettered by `sqloutbox.Relay`.
- A transport/client error returns a safe `*eventbridge.Error` matching
  `ErrPublishFailed`; `errors.Is` can still observe the injected cause.
- A non-nil response with `FailedEntryCount`, per-entry `ErrorCode`, or
  `ErrorMessage` returns `ErrPartialFailure` with bounded failure count/code.
- Nil output, an entry-count mismatch, an impossible failure count, or a
  successful result without `EventId` returns `ErrMalformedOutput`.

`Error()` and `%+v` never include EventBridge `ErrorMessage`, detail, payload,
credentials, bus/source values, or response `EventId`. The response `EventId`
is not the outbox `event_id` and is never used for deduplication. Consumers must
deduplicate retries with the detail's stable `event_id` or `idempotency_key`.

## Operational boundaries

- Event bus, rule, target, endpoint, IAM, authentication, retention, and
  production rollout are caller/operator responsibilities.
- AWS config, credentials, client lifecycle, timeout, retry/backoff, metrics,
  logs, and hooks are caller responsibilities.
- This package does not batch entries, retry SDK calls, delete accepted events,
  or provide downstream idempotency storage.
- The default CI uses fake clients only; no live AWS smoke test is implied.

## Tests

```bash
go test -count=1 ./audit/sqloutbox/eventbridge
go test -race -count=1 ./audit/sqloutbox/eventbridge
go vet ./audit/sqloutbox/eventbridge
```

The fake client deep-copies requests, records logical calls and contexts,
supports blocking/cancellation and output-plus-error responses, and proves
constructor validation, stable detail, size bounds, failure mapping, redaction,
relay semantics, and concurrent request isolation without Docker or AWS.
