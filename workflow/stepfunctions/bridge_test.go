package stepfunctions

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/aws/aws-sdk-go-v2/service/sfn/types"
)

const testStateMachineARN = "arn:aws:states:ap-northeast-2:123456789012:stateMachine:orders"

type fakeClient struct {
	mu sync.Mutex

	startFn    func(context.Context, *sfn.StartExecutionInput) (*sfn.StartExecutionOutput, error)
	describeFn func(context.Context, *sfn.DescribeExecutionInput) (*sfn.DescribeExecutionOutput, error)
	stopFn     func(context.Context, *sfn.StopExecutionInput) (*sfn.StopExecutionOutput, error)

	startInputs    []*sfn.StartExecutionInput
	describeInputs []*sfn.DescribeExecutionInput
	stopInputs     []*sfn.StopExecutionInput
}

func (f *fakeClient) StartExecution(ctx context.Context, input *sfn.StartExecutionInput, _ ...func(*sfn.Options)) (*sfn.StartExecutionOutput, error) {
	f.mu.Lock()
	f.startInputs = append(f.startInputs, cloneStartInput(input))
	fn := f.startFn
	f.mu.Unlock()
	if fn == nil {
		return nil, nil
	}
	return fn(ctx, input)
}

func (f *fakeClient) DescribeExecution(ctx context.Context, input *sfn.DescribeExecutionInput, _ ...func(*sfn.Options)) (*sfn.DescribeExecutionOutput, error) {
	f.mu.Lock()
	f.describeInputs = append(f.describeInputs, cloneDescribeInput(input))
	fn := f.describeFn
	f.mu.Unlock()
	if fn == nil {
		return nil, nil
	}
	return fn(ctx, input)
}

func (f *fakeClient) StopExecution(ctx context.Context, input *sfn.StopExecutionInput, _ ...func(*sfn.Options)) (*sfn.StopExecutionOutput, error) {
	f.mu.Lock()
	f.stopInputs = append(f.stopInputs, cloneStopInput(input))
	fn := f.stopFn
	f.mu.Unlock()
	if fn == nil {
		return nil, nil
	}
	return fn(ctx, input)
}

func (f *fakeClient) counts() (start, describe, stop int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.startInputs), len(f.describeInputs), len(f.stopInputs)
}

func (f *fakeClient) lastStart() *sfn.StartExecutionInput {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.startInputs) == 0 {
		return nil
	}
	return cloneStartInput(f.startInputs[len(f.startInputs)-1])
}

func (f *fakeClient) lastStop() *sfn.StopExecutionInput {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.stopInputs) == 0 {
		return nil
	}
	return cloneStopInput(f.stopInputs[len(f.stopInputs)-1])
}

func cloneStartInput(input *sfn.StartExecutionInput) *sfn.StartExecutionInput {
	if input == nil {
		return nil
	}
	return &sfn.StartExecutionInput{
		StateMachineArn: cloneString(input.StateMachineArn),
		Input:           cloneString(input.Input),
		Name:            cloneString(input.Name),
		TraceHeader:     cloneString(input.TraceHeader),
	}
}

func cloneDescribeInput(input *sfn.DescribeExecutionInput) *sfn.DescribeExecutionInput {
	if input == nil {
		return nil
	}
	return &sfn.DescribeExecutionInput{ExecutionArn: cloneString(input.ExecutionArn)}
}

func cloneStopInput(input *sfn.StopExecutionInput) *sfn.StopExecutionInput {
	if input == nil {
		return nil
	}
	return &sfn.StopExecutionInput{
		ExecutionArn: cloneString(input.ExecutionArn),
		Error:        cloneString(input.Error),
		Cause:        cloneString(input.Cause),
	}
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	valueCopy := *value
	return &valueCopy
}

func startOutput(arn string, startedAt time.Time) *sfn.StartExecutionOutput {
	return &sfn.StartExecutionOutput{ExecutionArn: &arn, StartDate: &startedAt}
}

func describeOutput(status types.ExecutionStatus, arn, stateMachineARN string, startedAt time.Time) *sfn.DescribeExecutionOutput {
	return &sfn.DescribeExecutionOutput{
		ExecutionArn:    &arn,
		StateMachineArn: &stateMachineARN,
		StartDate:       &startedAt,
		Status:          status,
	}
}

func TestNewRejectsNilAndTypedNilClient(t *testing.T) {
	if _, err := New(Options{}); !errors.Is(err, ErrNilClient) {
		t.Fatalf("New(nil) error = %v, want ErrNilClient", err)
	}

	var typedNil *fakeClient
	if _, err := New(Options{Client: typedNil}); !errors.Is(err, ErrNilClient) {
		t.Fatalf("New(typed nil) error = %v, want ErrNilClient", err)
	}
}

func TestStartMapsRequestAndDefaultInput(t *testing.T) {
	startedAt := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	fake := &fakeClient{startFn: func(context.Context, *sfn.StartExecutionInput) (*sfn.StartExecutionOutput, error) {
		return startOutput("arn:aws:states:ap-northeast-2:123456789012:execution:orders:run-1", startedAt), nil
	}}
	bridge, err := New(Options{Client: fake})
	if err != nil {
		t.Fatal(err)
	}

	request := StartRequest{StateMachineARN: testStateMachineARN, Name: "order-1", TraceHeader: "Root=1-abc"}
	execution, err := bridge.Start(context.Background(), request)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if execution.ExecutionARN == "" || !execution.StartedAt.Equal(startedAt) {
		t.Fatalf("execution = %+v, missing start response", execution)
	}
	if string(execution.Input) != "{}" || execution.StateMachineARN != testStateMachineARN || execution.Name != request.Name {
		t.Fatalf("execution mapping = %+v", execution)
	}

	got := fake.lastStart()
	if got == nil || got.StateMachineArn == nil || *got.StateMachineArn != testStateMachineARN || got.Input == nil || *got.Input != "{}" || got.Name == nil || *got.Name != request.Name || got.TraceHeader == nil || *got.TraceHeader != request.TraceHeader {
		t.Fatalf("StartExecution request = %+v", got)
	}
	request.Name = "mutated"
	if *got.Name != "order-1" {
		t.Fatalf("fake request was not isolated from caller mutation: %q", *got.Name)
	}
}

func TestStartRejectsInvalidRequestBeforeSDKCall(t *testing.T) {
	fake := &fakeClient{}
	bridge, err := New(Options{Client: fake})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		req  StartRequest
		want error
	}{
		{name: "missing ARN", req: StartRequest{}, want: ErrInvalidRequest},
		{name: "invalid name", req: StartRequest{StateMachineARN: testStateMachineARN, Name: "bad name"}, want: ErrInvalidName},
		{name: "invalid JSON", req: StartRequest{StateMachineARN: testStateMachineARN, Input: []byte("{")}, want: ErrInvalidRequest},
		{name: "invalid UTF-8", req: StartRequest{StateMachineARN: testStateMachineARN, Input: []byte{0xff}}, want: ErrInvalidRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := bridge.Start(context.Background(), tc.req); !errors.Is(err, tc.want) {
				t.Fatalf("Start() error = %v, want %v", err, tc.want)
			}
		})
	}
	start, _, _ := fake.counts()
	if start != 0 {
		t.Fatalf("invalid requests made %d SDK calls, want 0", start)
	}
}

func TestNewRejectsInvalidInputLimit(t *testing.T) {
	for _, limit := range []int{-1, defaultMaxInputSize + 1} {
		if _, err := New(Options{Client: &fakeClient{}, MaxInputSize: limit}); !errors.Is(err, ErrInvalidOptions) {
			t.Errorf("New(MaxInputSize=%d) error = %v, want ErrInvalidOptions", limit, err)
		}
	}
}

func TestStartRejectsOversizedInputWithoutCallingSDK(t *testing.T) {
	fake := &fakeClient{}
	bridge, err := New(Options{Client: fake, MaxInputSize: 4})
	if err != nil {
		t.Fatal(err)
	}
	_, err = bridge.Start(context.Background(), StartRequest{StateMachineARN: testStateMachineARN, Input: []byte(`{"id":2}`)})
	if !errors.Is(err, ErrInputTooLarge) || !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Start() error = %v, want size/request sentinels", err)
	}
	start, _, _ := fake.counts()
	if start != 0 {
		t.Fatalf("oversized input made %d SDK calls, want 0", start)
	}
}

func TestStartContextCancellationWinsAfterProviderResponse(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	fake := &fakeClient{startFn: func(context.Context, *sfn.StartExecutionInput) (*sfn.StartExecutionOutput, error) {
		cancel()
		return startOutput("arn:aws:states:ap-northeast-2:123456789012:execution:orders:late", time.Now().UTC()), nil
	}}
	bridge, err := New(Options{Client: fake})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bridge.Start(ctx, StartRequest{StateMachineARN: testStateMachineARN}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Start() error = %v, want context.Canceled", err)
	}
}

func TestStartRejectsMalformedResponse(t *testing.T) {
	cases := []struct {
		name string
		out  *sfn.StartExecutionOutput
	}{
		{name: "nil output"},
		{name: "missing ARN", out: &sfn.StartExecutionOutput{StartDate: timePointer(time.Now().UTC())}},
		{name: "missing start date", out: &sfn.StartExecutionOutput{ExecutionArn: strptr("arn:aws:states:ap-northeast-2:123456789012:execution:orders:run")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeClient{startFn: func(context.Context, *sfn.StartExecutionInput) (*sfn.StartExecutionOutput, error) {
				return tc.out, nil
			}}
			bridge, err := New(Options{Client: fake})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := bridge.Start(context.Background(), StartRequest{StateMachineARN: testStateMachineARN}); !errors.Is(err, ErrMalformedOutput) {
				t.Fatalf("Start() error = %v, want ErrMalformedOutput", err)
			}
		})
	}
}

func TestDescribePreservesStatusPayloadAndFailureMetadata(t *testing.T) {
	startedAt := time.Date(2026, 9, 3, 12, 1, 0, 0, time.UTC)
	stoppedAt := startedAt.Add(2 * time.Second)
	executionARN := "arn:aws:states:ap-northeast-2:123456789012:execution:orders:run-2"
	fake := &fakeClient{describeFn: func(context.Context, *sfn.DescribeExecutionInput) (*sfn.DescribeExecutionOutput, error) {
		output := describeOutput(types.ExecutionStatusFailed, executionARN, testStateMachineARN, startedAt)
		output.Name = strptr("order-2")
		output.Input = strptr(`{"id":2}`)
		output.Output = strptr("")
		output.Error = strptr("States.TaskFailed")
		output.Cause = strptr("sensitive provider cause")
		output.StopDate = &stoppedAt
		return output, nil
	}}
	bridge, err := New(Options{Client: fake})
	if err != nil {
		t.Fatal(err)
	}

	execution, err := bridge.Describe(context.Background(), executionARN)
	if err != nil {
		t.Fatalf("Describe() error = %v", err)
	}
	if execution.Status != StatusFailed || string(execution.Input) != `{"id":2}` || execution.Error != "States.TaskFailed" || execution.Cause != "sensitive provider cause" || execution.StoppedAt == nil || !execution.StoppedAt.Equal(stoppedAt) {
		t.Fatalf("Describe() execution = %+v", execution)
	}
}

func TestStartAndDescribeWrapTransportErrorsWithoutLeakingProviderText(t *testing.T) {
	providerErr := errors.New("credential=secret arn=" + testStateMachineARN)
	fake := &fakeClient{
		startFn: func(context.Context, *sfn.StartExecutionInput) (*sfn.StartExecutionOutput, error) {
			return nil, providerErr
		},
		describeFn: func(context.Context, *sfn.DescribeExecutionInput) (*sfn.DescribeExecutionOutput, error) {
			return nil, providerErr
		},
	}
	bridge, err := New(Options{Client: fake})
	if err != nil {
		t.Fatal(err)
	}
	startErr := func() error {
		_, err := bridge.Start(context.Background(), StartRequest{StateMachineARN: testStateMachineARN})
		return err
	}()
	if !errors.Is(startErr, ErrStartFailed) || !errors.Is(startErr, providerErr) || strings.Contains(fmt.Sprintf("%+v", startErr), providerErr.Error()) {
		t.Fatalf("Start() error = %q, missing safe wrapping", startErr)
	}
	describeErr := func() error {
		_, err := bridge.Describe(context.Background(), "arn:aws:states:ap-northeast-2:123456789012:execution:orders:run-1")
		return err
	}()
	if !errors.Is(describeErr, ErrDescribeFailed) || !errors.Is(describeErr, providerErr) || strings.Contains(fmt.Sprintf("%+v", describeErr), providerErr.Error()) {
		t.Fatalf("Describe() error = %q, missing safe wrapping", describeErr)
	}
}

func TestDescribeUnknownStatusFailsClosed(t *testing.T) {
	startedAt := time.Now().UTC()
	executionARN := "arn:aws:states:ap-northeast-2:123456789012:execution:orders:unknown"
	fake := &fakeClient{describeFn: func(context.Context, *sfn.DescribeExecutionInput) (*sfn.DescribeExecutionOutput, error) {
		return describeOutput(types.ExecutionStatus("NEW_STATUS"), executionARN, testStateMachineARN, startedAt), nil
	}}
	bridge, err := New(Options{Client: fake})
	if err != nil {
		t.Fatal(err)
	}
	execution, err := bridge.Describe(context.Background(), executionARN)
	if !errors.Is(err, ErrUnknownStatus) || execution == nil || execution.Status != ExecutionStatus("NEW_STATUS") {
		t.Fatalf("Describe() = %+v, %v; want preserved unknown status and ErrUnknownStatus", execution, err)
	}
}

func TestDescribeRejectsMalformedRequiredFields(t *testing.T) {
	startedAt := time.Now().UTC()
	validARN := "arn:aws:states:ap-northeast-2:123456789012:execution:orders:run-malformed"
	cases := []struct {
		name string
		out  *sfn.DescribeExecutionOutput
	}{
		{name: "nil output"},
		{name: "missing execution ARN", out: &sfn.DescribeExecutionOutput{StateMachineArn: strptr(testStateMachineARN), StartDate: &startedAt, Status: types.ExecutionStatusSucceeded}},
		{name: "missing state machine ARN", out: &sfn.DescribeExecutionOutput{ExecutionArn: strptr(validARN), StartDate: &startedAt, Status: types.ExecutionStatusSucceeded}},
		{name: "missing start date", out: &sfn.DescribeExecutionOutput{ExecutionArn: strptr(validARN), StateMachineArn: strptr(testStateMachineARN), Status: types.ExecutionStatusSucceeded}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeClient{describeFn: func(context.Context, *sfn.DescribeExecutionInput) (*sfn.DescribeExecutionOutput, error) {
				return tc.out, nil
			}}
			bridge, err := New(Options{Client: fake})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := bridge.Describe(context.Background(), validARN); !errors.Is(err, ErrMalformedOutput) {
				t.Fatalf("Describe() error = %v, want ErrMalformedOutput", err)
			}
		})
	}
}

func TestStopUsesOptionalCapabilityAndMapsRequest(t *testing.T) {
	stoppedAt := time.Date(2026, 9, 3, 12, 2, 0, 0, time.UTC)
	fake := &fakeClient{stopFn: func(context.Context, *sfn.StopExecutionInput) (*sfn.StopExecutionOutput, error) {
		return &sfn.StopExecutionOutput{StopDate: &stoppedAt}, nil
	}}
	bridge, err := New(Options{Client: fake})
	if err != nil {
		t.Fatal(err)
	}
	executionARN := "arn:aws:states:ap-northeast-2:123456789012:execution:orders:run-3"
	execution, err := bridge.Stop(context.Background(), StopRequest{ExecutionARN: executionARN, Error: "CancelledByCaller", Cause: "operator requested stop"})
	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if execution == nil || execution.ExecutionARN != executionARN || execution.StoppedAt == nil || !execution.StoppedAt.Equal(stoppedAt) {
		t.Fatalf("Stop() execution = %+v", execution)
	}
	got := fake.lastStop()
	if got == nil || got.ExecutionArn == nil || *got.ExecutionArn != executionARN || got.Error == nil || *got.Error != "CancelledByCaller" || got.Cause == nil || *got.Cause != "operator requested stop" {
		t.Fatalf("StopExecution request = %+v", got)
	}
}

func TestStopWithoutCapabilityDoesNotCallSDK(t *testing.T) {
	fake := &describeOnlyClient{}
	bridge, err := New(Options{Client: fake})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bridge.Stop(context.Background(), StopRequest{ExecutionARN: "arn:aws:states:ap-northeast-2:123456789012:execution:orders:run-4"}); !errors.Is(err, ErrStopUnsupported) {
		t.Fatalf("Stop() error = %v, want ErrStopUnsupported", err)
	}
}

func TestStopWrapsTransportAndRejectsMalformedResponse(t *testing.T) {
	providerErr := errors.New("secret provider message")
	cases := []struct {
		name string
		fn   func(context.Context, *sfn.StopExecutionInput) (*sfn.StopExecutionOutput, error)
		want error
	}{
		{name: "transport", fn: func(context.Context, *sfn.StopExecutionInput) (*sfn.StopExecutionOutput, error) {
			return nil, providerErr
		}, want: ErrStopFailed},
		{name: "nil output", fn: func(context.Context, *sfn.StopExecutionInput) (*sfn.StopExecutionOutput, error) { return nil, nil }, want: ErrMalformedOutput},
		{name: "missing stop date", fn: func(context.Context, *sfn.StopExecutionInput) (*sfn.StopExecutionOutput, error) {
			return &sfn.StopExecutionOutput{}, nil
		}, want: ErrMalformedOutput},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeClient{stopFn: tc.fn}
			bridge, err := New(Options{Client: fake})
			if err != nil {
				t.Fatal(err)
			}
			err = func() error {
				_, err := bridge.Stop(context.Background(), StopRequest{ExecutionARN: "arn:aws:states:ap-northeast-2:123456789012:execution:orders:run-stop"})
				return err
			}()
			if !errors.Is(err, tc.want) {
				t.Fatalf("Stop() error = %v, want %v", err, tc.want)
			}
			if tc.name == "transport" && strings.Contains(err.Error(), providerErr.Error()) {
				t.Fatalf("Stop() leaked provider error: %v", err)
			}
		})
	}
}

func TestStopContextCancellationWinsAfterProviderResponse(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	fake := &fakeClient{stopFn: func(context.Context, *sfn.StopExecutionInput) (*sfn.StopExecutionOutput, error) {
		cancel()
		return &sfn.StopExecutionOutput{StopDate: timePointer(time.Now().UTC())}, nil
	}}
	bridge, err := New(Options{Client: fake})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bridge.Stop(ctx, StopRequest{ExecutionARN: "arn:aws:states:ap-northeast-2:123456789012:execution:orders:run-stop-late"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Stop() error = %v, want context.Canceled", err)
	}
}

type describeOnlyClient struct{}

func (describeOnlyClient) StartExecution(context.Context, *sfn.StartExecutionInput, ...func(*sfn.Options)) (*sfn.StartExecutionOutput, error) {
	return startOutput("arn:aws:states:ap-northeast-2:123456789012:execution:orders:run", time.Now()), nil
}

func (describeOnlyClient) DescribeExecution(context.Context, *sfn.DescribeExecutionInput, ...func(*sfn.Options)) (*sfn.DescribeExecutionOutput, error) {
	return nil, nil
}

func TestWaitPollsWithCappedBackoffAndReturnsSuccess(t *testing.T) {
	startedAt := time.Now().UTC()
	executionARN := "arn:aws:states:ap-northeast-2:123456789012:execution:orders:run-5"
	statuses := []types.ExecutionStatus{types.ExecutionStatusRunning, types.ExecutionStatusRunning, types.ExecutionStatusSucceeded}
	var mu sync.Mutex
	fake := &fakeClient{describeFn: func(context.Context, *sfn.DescribeExecutionInput) (*sfn.DescribeExecutionOutput, error) {
		mu.Lock()
		defer mu.Unlock()
		status := statuses[0]
		statuses = statuses[1:]
		return describeOutput(status, executionARN, testStateMachineARN, startedAt), nil
	}}
	bridge, err := New(Options{Client: fake})
	if err != nil {
		t.Fatal(err)
	}
	var calls []string
	execution, err := bridge.Wait(context.Background(), executionARN, WaitOptions{
		PollInterval:    time.Millisecond,
		MaxPollInterval: 3 * time.Millisecond,
		Backoff: func(attempt int, previous time.Duration) time.Duration {
			calls = append(calls, fmt.Sprintf("%d:%s", attempt, previous))
			return previous * 4
		},
	})
	if err != nil || execution == nil || execution.Status != StatusSucceeded {
		t.Fatalf("Wait() = %+v, %v; want success", execution, err)
	}
	if !reflect.DeepEqual(calls, []string{"1:1ms", "2:3ms"}) {
		t.Fatalf("backoff calls = %v, want deterministic capped transitions", calls)
	}
	_, describe, _ := fake.counts()
	if describe != 3 {
		t.Fatalf("DescribeExecution calls = %d, want immediate + 2 polls", describe)
	}
}

func TestWaitMapsTerminalFailureAndDoesNotStop(t *testing.T) {
	startedAt := time.Now().UTC()
	executionARN := "arn:aws:states:ap-northeast-2:123456789012:execution:orders:run-6"
	fake := &fakeClient{describeFn: func(context.Context, *sfn.DescribeExecutionInput) (*sfn.DescribeExecutionOutput, error) {
		output := describeOutput(types.ExecutionStatusFailed, executionARN, testStateMachineARN, startedAt)
		output.Error = strptr("States.Failed")
		output.Cause = strptr("provider details")
		return output, nil
	}, stopFn: func(context.Context, *sfn.StopExecutionInput) (*sfn.StopExecutionOutput, error) {
		t.Fatal("Wait must not implicitly stop execution")
		return nil, nil
	}}
	bridge, err := New(Options{Client: fake})
	if err != nil {
		t.Fatal(err)
	}
	execution, err := bridge.Wait(context.Background(), executionARN, WaitOptions{PollInterval: time.Millisecond})
	if !errors.Is(err, ErrExecutionFailed) || execution == nil || execution.Status != StatusFailed || execution.Cause != "provider details" {
		t.Fatalf("Wait() = %+v, %v; want failed execution", execution, err)
	}
	_, describe, stop := fake.counts()
	if describe != 1 || stop != 0 {
		t.Fatalf("Wait() calls = describe:%d stop:%d, want 1/0", describe, stop)
	}
}

func TestWaitMapsEachTerminalFailureStatus(t *testing.T) {
	startedAt := time.Now().UTC()
	executionARN := "arn:aws:states:ap-northeast-2:123456789012:execution:orders:run-terminal"
	cases := []struct {
		status types.ExecutionStatus
		want   error
	}{
		{status: types.ExecutionStatusFailed, want: ErrExecutionFailed},
		{status: types.ExecutionStatusTimedOut, want: ErrExecutionTimedOut},
		{status: types.ExecutionStatusAborted, want: ErrExecutionAborted},
	}
	for _, tc := range cases {
		t.Run(string(tc.status), func(t *testing.T) {
			fake := &fakeClient{describeFn: func(context.Context, *sfn.DescribeExecutionInput) (*sfn.DescribeExecutionOutput, error) {
				return describeOutput(tc.status, executionARN, testStateMachineARN, startedAt), nil
			}}
			bridge, err := New(Options{Client: fake})
			if err != nil {
				t.Fatal(err)
			}
			execution, err := bridge.Wait(context.Background(), executionARN, WaitOptions{PollInterval: time.Millisecond})
			if !errors.Is(err, tc.want) || execution == nil || execution.Status != ExecutionStatus(tc.status) {
				t.Fatalf("Wait() = %+v, %v; want %v", execution, err, tc.want)
			}
		})
	}
}

func TestWaitRejectsNegativeBackoffWithoutAdditionalDescribe(t *testing.T) {
	startedAt := time.Now().UTC()
	executionARN := "arn:aws:states:ap-northeast-2:123456789012:execution:orders:run-backoff"
	fake := &fakeClient{describeFn: func(context.Context, *sfn.DescribeExecutionInput) (*sfn.DescribeExecutionOutput, error) {
		return describeOutput(types.ExecutionStatusRunning, executionARN, testStateMachineARN, startedAt), nil
	}}
	bridge, err := New(Options{Client: fake})
	if err != nil {
		t.Fatal(err)
	}
	_, err = bridge.Wait(context.Background(), executionARN, WaitOptions{
		PollInterval:    time.Millisecond,
		MaxPollInterval: 10 * time.Millisecond,
		Backoff: func(int, time.Duration) time.Duration {
			return -time.Millisecond
		},
		Timeout: 100 * time.Millisecond,
	})
	if !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("Wait() error = %v, want ErrInvalidOptions", err)
	}
	_, describe, _ := fake.counts()
	if describe != 1 {
		t.Fatalf("DescribeExecution calls = %d, want one before invalid backoff", describe)
	}
}

func TestWaitTimeoutAndCallerCancellationAreBounded(t *testing.T) {
	startedAt := time.Now().UTC()
	executionARN := "arn:aws:states:ap-northeast-2:123456789012:execution:orders:run-7"
	fake := &fakeClient{describeFn: func(ctx context.Context, _ *sfn.DescribeExecutionInput) (*sfn.DescribeExecutionOutput, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
			return describeOutput(types.ExecutionStatusRunning, executionARN, testStateMachineARN, startedAt), nil
		}
	}}
	bridge, err := New(Options{Client: fake})
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	_, timeoutErr := bridge.Wait(context.Background(), executionARN, WaitOptions{PollInterval: time.Millisecond, Timeout: 5 * time.Millisecond})
	if !errors.Is(timeoutErr, ErrWaitTimeout) || !errors.Is(timeoutErr, context.DeadlineExceeded) || time.Since(started) > 80*time.Millisecond {
		t.Fatalf("Wait(timeout) error = %v, elapsed = %s", timeoutErr, time.Since(started))
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, cancelErr := bridge.Wait(ctx, executionARN, WaitOptions{PollInterval: time.Millisecond})
	if !errors.Is(cancelErr, context.Canceled) || errors.Is(cancelErr, ErrWaitTimeout) {
		t.Fatalf("Wait(canceled) error = %v, want caller cancellation precedence", cancelErr)
	}
}

func TestWaitCancellationDuringPollingTimerReturnsPromptly(t *testing.T) {
	startedAt := time.Now().UTC()
	executionARN := "arn:aws:states:ap-northeast-2:123456789012:execution:orders:run-cancel"
	fake := &fakeClient{describeFn: func(context.Context, *sfn.DescribeExecutionInput) (*sfn.DescribeExecutionOutput, error) {
		return describeOutput(types.ExecutionStatusRunning, executionARN, testStateMachineARN, startedAt), nil
	}}
	bridge, err := New(Options{Client: fake})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()
	started := time.Now()
	_, err = bridge.Wait(ctx, executionARN, WaitOptions{PollInterval: time.Second})
	if !errors.Is(err, context.Canceled) || time.Since(started) > 200*time.Millisecond {
		t.Fatalf("Wait() error = %v, elapsed = %s; want prompt cancellation", err, time.Since(started))
	}
}

func TestWaitResponseCancellationWinsOverSuccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	executionARN := "arn:aws:states:ap-northeast-2:123456789012:execution:orders:run-late-cancel"
	fake := &fakeClient{describeFn: func(context.Context, *sfn.DescribeExecutionInput) (*sfn.DescribeExecutionOutput, error) {
		cancel()
		return describeOutput(types.ExecutionStatusSucceeded, executionARN, testStateMachineARN, time.Now().UTC()), nil
	}}
	bridge, err := New(Options{Client: fake})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bridge.Wait(ctx, executionARN, WaitOptions{PollInterval: time.Millisecond}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Wait() error = %v, want context.Canceled", err)
	}
}

func TestMethodsRejectZeroValueBridge(t *testing.T) {
	var bridge Bridge
	if _, err := bridge.Start(context.Background(), StartRequest{StateMachineARN: testStateMachineARN}); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("zero Bridge Start() error = %v", err)
	}
	if _, err := bridge.Describe(context.Background(), "arn:aws:states:ap-northeast-2:123456789012:execution:orders:run"); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("zero Bridge Describe() error = %v", err)
	}
}

func TestConcurrentStartRequestIsolation(t *testing.T) {
	startedAt := time.Now().UTC()
	fake := &fakeClient{startFn: func(context.Context, *sfn.StartExecutionInput) (*sfn.StartExecutionOutput, error) {
		return startOutput("arn:aws:states:ap-northeast-2:123456789012:execution:orders:concurrent", startedAt), nil
	}}
	bridge, err := New(Options{Client: fake})
	if err != nil {
		t.Fatal(err)
	}
	const workers = 24
	var wg sync.WaitGroup
	for index := 0; index < workers; index++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			input := []byte(fmt.Sprintf(`{"id":%d}`, index))
			if _, err := bridge.Start(context.Background(), StartRequest{
				StateMachineARN: testStateMachineARN,
				Name:            fmt.Sprintf("run-%d", index),
				Input:           input,
			}); err != nil {
				t.Errorf("Start(%d) error = %v", index, err)
			}
			input[0] = 'x'
		}(index)
	}
	wg.Wait()
	start, _, _ := fake.counts()
	if start != workers {
		t.Fatalf("StartExecution calls = %d, want %d", start, workers)
	}
}

func TestErrorExposesOnlySafeMetadata(t *testing.T) {
	providerErr := errors.New("credential=secret arn=" + testStateMachineARN)
	err := newError(ErrStartFailed, "start", "", providerErr)
	if !errors.Is(err, ErrStartFailed) || !errors.Is(err, providerErr) {
		t.Fatalf("error matching failed: %v", err)
	}
	var typed *Error
	if !errors.As(err, &typed) || typed.Operation() != "start" {
		t.Fatalf("errors.As/Operation failed: %v", err)
	}
	formatted := fmt.Sprintf("%+v", err)
	if strings.Contains(formatted, providerErr.Error()) || strings.Contains(formatted, testStateMachineARN) {
		t.Fatalf("formatted error leaked provider values: %q", formatted)
	}
	statusErr := newError(ErrExecutionFailed, "wait", StatusFailed, nil)
	if statusErr.Status() != StatusFailed || !strings.Contains(statusErr.Error(), string(StatusFailed)) {
		t.Fatalf("status error = %v, status = %q", statusErr, statusErr.Status())
	}
}

func TestExecutionStatusTerminalSet(t *testing.T) {
	for _, status := range []ExecutionStatus{StatusSucceeded, StatusFailed, StatusTimedOut, StatusAborted, StatusPendingRedrive} {
		if !status.IsTerminal() {
			t.Errorf("%q IsTerminal() = false", status)
		}
	}
	if StatusRunning.IsTerminal() || ExecutionStatus("future").IsTerminal() {
		t.Fatal("RUNNING and unknown statuses must not be terminal")
	}
}

func TestTypedNilStopCapabilityIsUnsupported(t *testing.T) {
	var fake *fakeClient
	_, err := New(Options{Client: fake})
	if err == nil || !errors.Is(err, ErrNilClient) {
		t.Fatalf("New(typed nil) = %v, want ErrNilClient", err)
	}
	if reflect.ValueOf(fake).Kind() != reflect.Pointer {
		t.Fatal("test setup lost typed nil")
	}
}

func strptr(value string) *string { return &value }

func timePointer(value time.Time) *time.Time { return &value }
