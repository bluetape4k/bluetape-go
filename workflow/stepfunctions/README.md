# workflow/stepfunctions

[English](README.md) | [한국어](README.ko.md)

`workflow/stepfunctions` is a narrow bridge for observing AWS Step Functions
executions. It starts an execution, describes its status, optionally stops it
when the caller's client supports `StopExecution`, and waits with bounded,
cancellable polling.

The package does not define or deploy state machines. The caller owns SDK client
creation, credentials, region, endpoint, retry policy, IAM, and execution
lifecycle. The default tests and examples use fake clients and never require
live AWS credentials.

## Example

The SDK client is created and configured by the caller:

```go
ctx := context.Background()
client := sfn.NewFromConfig(cfg) // cfg and all credential policy remain caller-owned.

bridge, err := stepfunctions.New(stepfunctions.Options{Client: client})
if err != nil {
    return err
}

execution, err := bridge.Start(ctx, stepfunctions.StartRequest{
    StateMachineARN: "arn:aws:states:ap-northeast-2:123456789012:stateMachine:orders",
    Name:            "order-20260903-0001",
    Input:           []byte(`{"order_id": "42"}`),
})
if err != nil {
    return err
}

execution, err = bridge.Wait(ctx, execution.ExecutionARN, stepfunctions.WaitOptions{
    PollInterval:    time.Second,
    MaxPollInterval: 30 * time.Second,
    Timeout:         5 * time.Minute,
})
if err != nil {
    // execution still contains the last response for terminal failures.
    return err
}
fmt.Println(execution.Status)
```

## API boundaries

- `Start` validates the state-machine ARN, optional execution name, JSON input,
  and trace header before making an SDK call. Empty input is sent as `{}` and
  input bytes are preserved without reformatting.
- `Describe` preserves execution ARN, state-machine ARN, name, status, start and
  stop times, input/output, and provider failure metadata in an independent result
  copy. Unknown statuses fail closed with `ErrUnknownStatus`.
- `Stop` uses the optional `StopClient` capability. A client that only exposes
  `StartExecution` and `DescribeExecution` returns `ErrStopUnsupported`; waiting
  never stops or retries an execution implicitly.
- `Wait` performs one immediate `Describe`, then polls only while status is
  `RUNNING`. Timers are cancellable and backoff is capped. A positive `Timeout`
  is owned by the bridge and returns `ErrWaitTimeout` wrapping
  `context.DeadlineExceeded`; caller cancellation always takes precedence.

## Limits and idempotency

- Start/describe payloads default to the AWS 262,144-byte UTF-8 limit. Use
  `Options.MaxInputSize` to select a smaller bound; values above the AWS limit
  are rejected.
- Execution names are optional. When supplied, they are 1–80 ASCII bytes from
  `[A-Za-z0-9_-]`. The package does not generate names or retry
  `ExecutionAlreadyExists`; for `STANDARD`, the same name+input is idempotent
  while running, a closed execution blocks reuse until 90 days after closure,
  and `EXPRESS` is not idempotent (its names can be reused immediately). These
  are AWS service contracts, not package-level deduplication.
- State-machine and execution ARNs are bounded to 256 UTF-8 bytes. Trace
  headers are optional ASCII values bounded to 256 bytes.
- `DescribeExecution` is eventually consistent and does not support ordinary
  `EXPRESS` executions (Map Run-dispatched executions are an AWS exception).
  `Output` is returned only for successful executions; failed executions expose
  provider `Error`/`Cause` metadata instead. `StopExecution` is unsupported by
  `EXPRESS` state machines and accepts at most 256-byte `Error` and 32768-byte
  `Cause` values.
- Polling defaults to a 1-second first interval and a 30-second maximum. A
  custom `Backoff` receives a 1-based attempt number and previous interval;
  negative values fail and larger values are capped.

## Errors and security

Use `errors.Is` with the exported sentinels such as `ErrStartFailed`,
`ErrDescribeFailed`, `ErrExecutionFailed`, and `ErrWaitTimeout`. `*Error`
preserves the SDK cause for `errors.Is`/`errors.As` while its `Error()` and
`%+v` output contains only an allowlisted operation and known status. Provider
messages, payloads, ARNs, credentials, and trace headers are not copied into
error strings.

## Verification

The package has fake-first tests for request mapping, validation, malformed
responses, all terminal statuses, polling/backoff caps, timeout and
cancellation precedence, optional stop capability, concurrent isolation, and
error redaction:

```bash
go test ./workflow/stepfunctions
go test -race ./workflow/stepfunctions
go vet ./workflow/stepfunctions
golangci-lint run ./workflow/stepfunctions/...
```

The service-level limits and consistency/idempotency behavior follow the
[AWS `StartExecution`](https://docs.aws.amazon.com/step-functions/latest/apireference/API_StartExecution.html),
[`DescribeExecution`](https://docs.aws.amazon.com/step-functions/latest/apireference/API_DescribeExecution.html),
and [`StopExecution`](https://docs.aws.amazon.com/step-functions/latest/apireference/API_StopExecution.html)
contracts. The bridge does not provision state machines or perform live-AWS
smoke tests in default CI.
