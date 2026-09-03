# workflow/stepfunctions

[English](README.md) | [한국어](README.ko.md)

`workflow/stepfunctions`는 AWS Step Functions execution을 관찰하기 위한 좁은
bridge입니다. Execution을 시작하고 상태를 조회하며, caller의 client가
`StopExecution`을 지원할 때 선택적으로 중지하고, bounded·cancellable polling으로
기다립니다.

State machine을 정의하거나 배포하지 않습니다. SDK client 생성, credential, region,
endpoint, retry policy, IAM, execution lifecycle은 caller가 소유합니다. 기본 test와
example은 fake client만 사용하므로 live AWS credential이 필요하지 않습니다.

## 예제

SDK client의 생성과 설정은 caller가 담당합니다.

```go
ctx := context.Background()
client := sfn.NewFromConfig(cfg) // cfg와 credential policy는 caller 소유입니다.

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
    // terminal failure에서도 execution에는 마지막 response가 남습니다.
    return err
}
fmt.Println(execution.Status)
```

## API 경계

- `Start`는 SDK 호출 전에 state-machine ARN, 선택적 execution name, JSON input,
  trace header를 검증합니다. 빈 input은 `{}`로 보내며 input bytes를 재정렬하지
  않습니다.
- `Describe`는 execution ARN, state-machine ARN, name, status, 시작·중지 시각,
  input/output, provider 실패 metadata를 독립 복사본으로 보존합니다. 알 수 없는
  status는 `ErrUnknownStatus`로 fail closed 합니다.
- `Stop`은 선택적 `StopClient` capability를 사용합니다. `StartExecution`과
  `DescribeExecution`만 제공하는 client는 `ErrStopUnsupported`를 반환합니다.
  `Wait`는 execution을 암묵적으로 stop하거나 retry하지 않습니다.
- `Wait`는 `Describe`를 즉시 한 번 호출한 다음 `RUNNING`일 때만 polling합니다.
  Timer는 취소할 수 있고 backoff는 상한을 가집니다. 양수 `Timeout`은 bridge가
  소유하며 `context.DeadlineExceeded`를 감싼 `ErrWaitTimeout`을 반환합니다.
  Caller cancellation은 항상 우선합니다.

## 한도와 멱등성

- Start/describe payload 기본 한도는 AWS UTF-8 262,144 bytes입니다.
  `Options.MaxInputSize`로 더 작은 한도를 선택할 수 있으며 AWS 한도보다 큰 값은
  거부합니다.
- Execution name은 선택 사항입니다. 지정하면 1–80 ASCII bytes와
  `[A-Za-z0-9_-]`만 허용합니다. 이름을 생성하거나 `ExecutionAlreadyExists`를
  retry하지 않으며, standard workflow의 name+input 멱등성은 AWS service 계약으로
  남깁니다.
- State-machine/execution ARN은 256 UTF-8 bytes, trace header는 선택적 256 ASCII
  bytes로 제한합니다.
- Polling 기본값은 첫 1초, 최대 30초입니다. Custom `Backoff`는 1부터 시작하는
  시도 횟수와 직전 간격을 받고 음수는 실패하며 상한을 넘으면 cap됩니다.

## 오류와 보안

`errors.Is`로 `ErrStartFailed`, `ErrDescribeFailed`, `ErrExecutionFailed`,
`ErrWaitTimeout` 같은 exported sentinel을 확인합니다. `*Error`는 SDK 원인을
`errors.Is`/`errors.As`로 관찰하게 하지만 `Error()`와 `%+v`에는 allowlist operation과
known status만 출력합니다. Provider message, payload, ARN, credential, trace
header는 오류 문자열에 복사하지 않습니다.

## 검증

Request mapping, validation, malformed response, terminal status, polling/backoff
cap, timeout·cancellation 우선순위, optional stop capability, concurrent isolation,
error redaction을 fake-first로 검증합니다.

```bash
go test ./workflow/stepfunctions
go test -race ./workflow/stepfunctions
go vet ./workflow/stepfunctions
golangci-lint run ./workflow/stepfunctions/...
```
