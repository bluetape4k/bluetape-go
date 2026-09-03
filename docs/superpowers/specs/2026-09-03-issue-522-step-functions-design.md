# 이슈 #522 Step Functions 실행 bridge 설계

## 설계 상태

- 상태: 구현 승인 범위로 고정
- 작업 유형: Type-A full feature
- 기준 repository: `bluetape4k/bluetape-go`
- 기준 branch: `feat/issue-522-step-functions` (`origin/develop` 기준)
- 연결 epic: #517
- 조사 자료: #513, [AWS research gate](https://github.com/bluetape4k/bluetape4k-wiki/blob/develop/research/2026-07-09-bluetape-go-aws-research-gate.md)
- 외부 기준일: 2026-09-03에 AWS API 문서와 AWS SDK for Go v2 `service/sfn` 모듈을 재확인

## 문제와 목표

현재 `workflow` package는 프로세스 안에서 `workreport`를 조합하는 가벼운
runner다. AWS Step Functions 실행을 같은 `Runner` 구현으로 포팅하면 durable
state-machine, 배포, retry, 운영 lifecycle을 숨기게 되어 기존 경계를 넓힌다.
이 이슈는 그 대신 외부 execution을 호출하고 관찰하는 최소 bridge를 제공한다.

목표는 다음 네 가지다.

1. caller-owned AWS SDK for Go v2 client로 `StartExecution`을 호출한다.
2. execution ARN으로 `DescribeExecution`을 호출하고 상태·입출력·실패 metadata를
   caller가 읽을 수 있는 typed value로 보존한다.
3. client가 capability를 제공할 때만 `StopExecution`을 호출한다.
4. bounded polling `Wait`에서 timeout, backoff, cancellation, terminal status를
   결정론적으로 처리한다.

## 조사 근거와 확정된 외부 계약

- [AWS `StartExecution` API](https://docs.aws.amazon.com/step-functions/latest/apireference/API_StartExecution.html)는 `stateMachineArn`을 필수로 요구하고, `input`을 UTF-8 bytes 기준 최대 `262144` bytes로 제한한다. `name`은 최대 80 bytes이며 `STANDARD` workflow에서 같은 name+input의 실행은 idempotent하고, 같은 name의 다른 input은 `ExecutionAlreadyExists`가 된다.
- [AWS `DescribeExecution` API](https://docs.aws.amazon.com/step-functions/latest/apireference/API_DescribeExecution.html)는 eventually consistent이며 `RUNNING`, `SUCCEEDED`, `FAILED`, `TIMED_OUT`, `ABORTED`, `PENDING_REDRIVE` 상태와 input/output/error/cause를 반환한다. `EXPRESS` 실행은 일반적으로 이 API의 대상이 아니다.
- [AWS `StopExecution` API](https://docs.aws.amazon.com/step-functions/latest/apireference/API_StopExecution.html)는 `EXPRESS` state machine에 지원되지 않고 execution ARN이 필수다. error/cause는 호출자가 선택적으로 보낸다.
- SDK 표면은 AWS SDK for Go v2 `service/sfn`의 `StartExecution`, `DescribeExecution`, `StopExecution` method subset만 사용한다. client 생성, credentials, endpoint, retry, timeout, logger, IAM과 state-machine provisioning은 caller/operator가 소유한다.

## 대안 비교

### A. 좁은 `workflow/stepfunctions` execution bridge (선택)

`StartExecution`·`DescribeExecution`을 필수 client interface로 두고,
`StopExecution`은 별도 capability interface로 type assertion한다. 요청 검증과
상태 mapping은 package가 소유하고 SDK client lifecycle과 운영 정책은 호출자가
소유한다.

- 장점: 기존 `workflow` runner와 책임이 섞이지 않고 fake-first 테스트가 쉽다.
- 장점: 한 AWS client를 start/describe/stop에 재사용하면서도 stop 미지원 fake와
  Express 제한을 명시할 수 있다.
- 단점: polling은 service의 eventual consistency와 caller timeout을 문서화해야
  한다.

### B. `workflow.Runner` 구현으로 직접 포팅 (기각)

Step Functions execution을 `workreport.Report`로 즉시 변환한다.

- 기각 이유: report는 프로세스 안 child work의 결과이고 Step Functions는 외부
  durable execution이다. ARN, eventual consistency, provider error와 stop
  capability가 사라져 잘못된 성공/실패 의미를 만든다.

### C. 범용 AWS workflow facade (기각)

state-machine definition, deploy, provisioning, retry registry와 여러 AWS
workflow service를 하나의 facade로 묶는다.

- 기각 이유: issue 범위를 초과하고 SDK가 제공하는 API를 중복 포장한다. IAM,
  state-machine, retry와 운영 lifecycle의 caller ownership도 침해한다.

## API와 책임 경계

패키지 경로는 `workflow/stepfunctions`, package 이름은 `stepfunctions`다.

```go
type Client interface {
    StartExecution(context.Context, *sfn.StartExecutionInput, ...func(*sfn.Options)) (*sfn.StartExecutionOutput, error)
    DescribeExecution(context.Context, *sfn.DescribeExecutionInput, ...func(*sfn.Options)) (*sfn.DescribeExecutionOutput, error)
}

type StopClient interface {
    StopExecution(context.Context, *sfn.StopExecutionInput, ...func(*sfn.Options)) (*sfn.StopExecutionOutput, error)
}

type Options struct {
    Client        Client
    MaxInputSize  int // zero: AWS 262144-byte limit
}

type StartRequest struct {
    StateMachineARN string
    Name            string
    Input           []byte // nil/empty: "{}"
    TraceHeader     string
}

type WaitOptions struct {
    PollInterval    time.Duration
    MaxPollInterval time.Duration
    Timeout         time.Duration // zero: caller context/deadline only
    Backoff         Backoff
}
```

실제 구현은 요청 slice를 SDK request의 string으로 복사하고, constructor가
검증한 immutable 설정만 보유한다. zero-value `Bridge`는 명시적인
`ErrInvalidOptions`를 반환하며 panic이나 global client를 만들지 않는다.

### 입력 계약

- `StateMachineARN`은 공백이 아닌 유효 UTF-8이고 최대 256 bytes여야 한다.
- `Name`은 생략할 수 있다. 지정하면 1~80 bytes의 ASCII
  `[A-Za-z0-9_-]`만 허용해 AWS 금지 문자를 조기에 거부한다. package는 name을
  생성하거나 재시도하지 않으며, 동일 name+input의 idempotency는 AWS 계약으로
  남긴다.
- `Input`은 nil/empty일 때 `{}`로 보내고, 유효한 JSON이며 UTF-8이고 configured
  maximum(기본 262144 bytes) 이하이어야 한다. JSON을 재정렬하지 않아 caller
  input bytes를 그대로 보존한다.
- `TraceHeader`는 비어 있을 수 있고, 비어 있지 않으면 ASCII 및 256 bytes
  이하로 검증한다.

### 응답 값

`Execution`은 `ExecutionARN`, `StateMachineARN`, `Name`, `Input`, `Output`,
`Error`, `Cause`, `Status`, `StartedAt`, `StoppedAt`을 가진다. provider가
반환한 `Error`/`Cause`/payload는 caller가 명시적으로 조회할 수 있지만,
package error의 `Error()`나 내부 log에는 복사하지 않는다. `Status`는 AWS
상태 문자열을 보존하며 알려진 terminal 상태는 `SUCCEEDED`, `FAILED`,
`TIMED_OUT`, `ABORTED`, `PENDING_REDRIVE`다.

`Start`는 ARN과 start time이 없는 성공 응답을 malformed로 거부한다.
`Describe`는 ARN, state-machine ARN, status, start time이 없는 응답을
malformed로 거부한다. `Stop`은 stop time이 없는 응답을 malformed로 거부한다.

### Wait와 cancellation

`Wait(ctx, executionARN, WaitOptions)`는 즉시 한 번 `Describe`한 뒤
`RUNNING`일 때만 timer를 사용해 polling한다.

- `PollInterval` 기본값은 1초, `MaxPollInterval` 기본값은 30초다.
- 기본 backoff는 attempt별 exponential 증가 후 max interval에서 cap한다.
  custom backoff는 음수를 거부하고 max interval을 넘지 않도록 cap한다.
- `Timeout`이 양수면 child context deadline을 만들고, caller context의
  cancellation/deadline이 동시에 발생하면 caller 오류가 우선한다. 명시적
  timeout으로 종료하면 `ErrWaitTimeout`과 `context.DeadlineExceeded`를
  `errors.Is`로 관찰할 수 있다.
- timer는 `select`로 context와 함께 종료한다. goroutine을 추가해 SDK 호출을
  강제 종료하지 않으며, 대기 취소가 자동 `StopExecution`이나 retry를
  발생시키지 않는다.
- unknown status는 `ErrUnknownStatus`로 fail closed 한다. terminal failure는
  마지막 `Execution`을 반환하면서 `ErrExecutionFailed`,
  `ErrExecutionTimedOut`, `ErrExecutionAborted` 중 하나를 반환한다.

모든 public method는 dispatch 전과 SDK response 직후 context를 확인한다.
따라서 늦게 도착한 성공 response가 caller cancellation을 덮지 않는다.

## 오류 계약과 보안

다음 sentinel과 typed `*Error`를 제공한다.

- `ErrNilClient`, `ErrInvalidOptions`, `ErrInvalidRequest`, `ErrInputTooLarge`,
  `ErrInvalidName`, `ErrStartFailed`, `ErrDescribeFailed`, `ErrStopFailed`,
  `ErrStopUnsupported`, `ErrMalformedOutput`, `ErrUnknownStatus`,
  `ErrExecutionFailed`, `ErrExecutionTimedOut`, `ErrExecutionAborted`,
  `ErrWaitTimeout`.

`*Error`는 allowlisted operation/status만 `Error()`에 출력하고 AWS SDK error
message, ARN, input/output, error/cause, credential와 trace header를 출력하지
않는다. SDK 원인 오류는 `Unwrap`으로 `errors.Is`/`errors.As`를 보존한다.

## 실패 모드

1. **사전 검증 실패**: 잘못된 ARN/name/JSON/UTF-8/size는 SDK 호출 0회로
   `ErrInvalidRequest` 계열을 반환한다.
2. **transport 실패**: SDK 오류는 `ErrStartFailed`, `ErrDescribeFailed` 또는
   `ErrStopFailed`로 감싸고 provider text는 redaction한다.
3. **malformed output**: nil output, 필수 field 누락, 알 수 없는 status는
   typed error로 종료하며 성공으로 간주하지 않는다.
4. **실행 실패**: `FAILED`/`TIMED_OUT`/`ABORTED` 상태와 response metadata를
   반환하고 상태별 typed error를 함께 반환한다.
5. **eventual consistency와 timeout**: polling은 caller가 정한 bounded
   interval/deadline까지만 실행하고, timeout 뒤 추가 describe나 자동 stop을
   하지 않는다.
6. **cancellation 경계**: 호출 전·후 context checkpoint에서 cancellation이
   우선하며 timer를 즉시 해제한다.

## 테스트와 수용 기준

fake client는 request를 deep-copy하고 method별 call count, 입력, context를
기록한다. 다음 table-driven 테스트와 compile-checked `ExampleNew`를 기본 CI에서
실행한다.

- start success/default `{}`/custom input/name/trace header와 request mapping;
- invalid ARN/name/JSON/UTF-8/oversized input 및 zero provider calls;
- start/describe/stop transport error, nil output, required field 누락;
- describe의 각 known status, unknown status와 provider Error/Cause 보존;
- wait immediate success, running→success, failed/timed-out/aborted terminal
  mapping;
- deterministic custom backoff 호출 순서와 max cap;
- explicit timeout, caller cancellation 중 timer 대기와 response 직후
  cancellation;
- stop capability 없음, stop request error/cause와 no implicit stop;
- concurrent Start/Describe/Wait request isolation과 `go test -race`;
- provider error `Error()`/`%+v` redaction 및 no live AWS credential proof.

수용 기준은 issue #522의 API 좁음/context-aware, state-machine
definition/build/deploy 비포함, fake-only 기본 CI, 문서화된 size/timeout/backoff,
성공·실패·timeout·cancellation 테스트를 모두 충족하는 것이다.

## 호환성·운영·롤백

- 새 package와 `service/sfn` dependency만 추가하며 기존 `workflow` API를
  변경하지 않는다.
- AWS client, credential, region, endpoint, retry/timeout, IAM, state-machine
  lifecycle, CloudWatch logging/metrics와 live smoke는 caller/operator가
  소유한다.
- dependency 또는 API 문제가 발견되면 새 package와 `go.mod`/`go.sum`을
  feature branch에서 되돌리고 기존 `workflow` package는 그대로 유지한다.
- 문서에는 AWS STANDARD/EXPRESS 차이, 90-day name reuse, eventual
  consistency와 downstream retry/deduplication 책임을 명시한다.

## DoD

- [ ] `workflow/stepfunctions`의 좁은 client/API와 zero-value/error 계약 구현
- [ ] 입력 bound, context checkpoint, terminal status, bounded backoff/timeout
      검증
- [ ] fake-first normal/failure/malformed/terminal/timeout/cancellation/race
      테스트와 compile-checked example
- [ ] package README EN/KO 및 root README index parity
- [ ] implementation review, lesson, PR body에 `GO-HARD-08` 증거 기록
- [ ] `make ci`, exact-head remote CI, fresh merge approval과 local cleanup

## Writer gate (SPW)

- SPW-01: PASS — 독자(Go caller/operator), 목적, issue/research/API source와
  unsupported live/provisioning 범위를 고정했다.
- SPW-02: PASS — 대안, API/data flow, failure/cancellation, testing,
  compatibility, rollback, DoD를 포함했다.
- SPW-03: PASS — Korean technical register를 사용하고 code/API/commands/URLs와
  숫자·상태 token을 보존했다.
- SPW-04: PASS — issue #522, research gate, AWS 공식 API와 local `workflow`/
  `audit/sqloutbox/eventbridge` 패턴을 대조했다.
- SPW-05: PASS — placeholder/모순/범위 누락을 read-back으로 확인했다.
