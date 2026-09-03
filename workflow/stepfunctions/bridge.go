// Package stepfunctions provides a narrow, caller-owned bridge to AWS Step Functions executions.
package stepfunctions

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go-v2/service/sfn"
)

const (
	defaultMaxInputSize = 262144
	maxARNSize          = 256
	maxExecutionName    = 80
	maxTraceHeaderSize  = 256
	maxStopErrorSize    = 256
	maxStopCauseSize    = 32768
	defaultPollInterval = time.Second
	defaultMaxPoll      = 30 * time.Second
)

// Client - bridge가 사용하는 AWS SDK surface다.
// client 생성, 자격 증명, endpoint, retry, timeout 수명은 호출자가 소유한다.
type Client interface {
	StartExecution(context.Context, *sfn.StartExecutionInput, ...func(*sfn.Options)) (*sfn.StartExecutionOutput, error)
	DescribeExecution(context.Context, *sfn.DescribeExecutionInput, ...func(*sfn.Options)) (*sfn.DescribeExecutionOutput, error)
}

// StopClient - 선택적 StopExecution capability를 표현한다.
type StopClient interface {
	StopExecution(context.Context, *sfn.StopExecutionInput, ...func(*sfn.Options)) (*sfn.StopExecutionOutput, error)
}

// Options - caller-owned AWS client와 입력 한도를 구성한다.
type Options struct {
	// Client는 bridge가 호출할 AWS SDK client다.
	Client Client
	// MaxInputSize는 Start/Describe payload의 최대 bytes다. 0이면 AWS 기본 262144다.
	MaxInputSize int
}

// StartRequest - Step Functions execution 시작에 필요한 입력이다.
type StartRequest struct {
	// StateMachineARN은 실행할 state machine ARN이다.
	StateMachineARN string
	// Name은 선택적 execution name이며 AWS idempotency 키의 일부다.
	Name string
	// Input은 JSON UTF-8 payload다. nil 또는 empty면 {}로 전송한다.
	Input []byte
	// TraceHeader는 선택적 X-Ray trace header다.
	TraceHeader string
}

// StopRequest - 실행 중인 execution 중지 요청이다.
type StopRequest struct {
	// ExecutionARN은 중지할 execution ARN이다.
	ExecutionARN string
	// Error는 provider가 기록할 선택적 오류 코드다.
	Error string
	// Cause는 provider가 기록할 선택적 중지 원인이다.
	Cause string
}

// WaitOptions - DescribeExecution polling의 bounded timing을 구성한다.
type WaitOptions struct {
	// PollInterval은 첫 polling 전 대기다. 0이면 1초다.
	PollInterval time.Duration
	// MaxPollInterval은 backoff 상한이다. 0이면 30초다.
	MaxPollInterval time.Duration
	// Timeout은 bridge가 소유하는 wait 상한이다. 0이면 caller context만 사용한다.
	Timeout time.Duration
	// Backoff는 polling 사이 간격을 계산한다. nil이면 capped exponential backoff다.
	Backoff Backoff
}

// Backoff - 직전 대기 간격과 1부터 시작하는 시도 횟수로 다음 간격을 계산한다.
// 반환값은 MaxPollInterval로 cap되며 음수는 ErrInvalidOptions로 처리된다.
type Backoff func(attempt int, previous time.Duration) time.Duration

// ExecutionStatus - Step Functions execution 상태 문자열이다.
type ExecutionStatus string

const (
	// StatusRunning - execution이 실행 중임을 나타낸다.
	StatusRunning ExecutionStatus = "RUNNING"
	// StatusSucceeded - execution이 성공했음을 나타낸다.
	StatusSucceeded ExecutionStatus = "SUCCEEDED"
	// StatusFailed - execution이 실패했음을 나타낸다.
	StatusFailed ExecutionStatus = "FAILED"
	// StatusTimedOut - execution이 시간 초과됐음을 나타낸다.
	StatusTimedOut ExecutionStatus = "TIMED_OUT"
	// StatusAborted - execution이 중단됐음을 나타낸다.
	StatusAborted ExecutionStatus = "ABORTED"
	// StatusPendingRedrive - execution이 redrive 대기임을 나타낸다.
	StatusPendingRedrive ExecutionStatus = "PENDING_REDRIVE"

	// ExecutionStatusRunning - AWS 이름과 대응하는 호환 alias다.
	ExecutionStatusRunning = StatusRunning
	// ExecutionStatusSucceeded - AWS 이름과 대응하는 호환 alias다.
	ExecutionStatusSucceeded = StatusSucceeded
	// ExecutionStatusFailed - AWS 이름과 대응하는 호환 alias다.
	ExecutionStatusFailed = StatusFailed
	// ExecutionStatusTimedOut - AWS 이름과 대응하는 호환 alias다.
	ExecutionStatusTimedOut = StatusTimedOut
	// ExecutionStatusAborted - AWS 이름과 대응하는 호환 alias다.
	ExecutionStatusAborted = StatusAborted
	// ExecutionStatusPendingRedrive - AWS 이름과 대응하는 호환 alias다.
	ExecutionStatusPendingRedrive = StatusPendingRedrive
)

// IsTerminal - 알려진 terminal execution 상태인지 반환한다.
func (s ExecutionStatus) IsTerminal() bool {
	switch s {
	case StatusSucceeded, StatusFailed, StatusTimedOut, StatusAborted, StatusPendingRedrive:
		return true
	default:
		return false
	}
}

// Execution - Start/Describe/Wait/Stop 결과를 caller가 관찰할 수 있는 값이다.
// Input, Output, Error, Cause는 provider response의 복사본이며 bridge 오류 문자열에는 포함되지 않는다.
type Execution struct {
	ExecutionARN    string
	StateMachineARN string
	Name            string
	Input           []byte
	Output          []byte
	Error           string
	Cause           string
	Status          ExecutionStatus
	StartedAt       time.Time
	StoppedAt       *time.Time
}

// Bridge - immutable 설정을 보유한 Step Functions execution adapter다.
type Bridge struct {
	client       Client
	maxInputSize int
}

var _ Client = (*sfn.Client)(nil)

// New - caller-owned SDK client와 bounded input 설정을 검증해 bridge를 생성한다.
func New(options Options) (*Bridge, error) {
	if isNilClient(options.Client) {
		return nil, newError(ErrNilClient, "validate options", "", nil)
	}
	maxInputSize := options.MaxInputSize
	if maxInputSize == 0 {
		maxInputSize = defaultMaxInputSize
	}
	if maxInputSize < 1 || maxInputSize > defaultMaxInputSize {
		return nil, newError(ErrInvalidOptions, "validate options", "", nil)
	}
	return &Bridge{client: options.Client, maxInputSize: maxInputSize}, nil
}

// Start - state machine execution을 시작하고 생성된 execution ARN을 반환한다.
func (b *Bridge) Start(ctx context.Context, request StartRequest) (*Execution, error) {
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := b.validate(); err != nil {
		return nil, err
	}
	normalized, err := b.validateStartRequest(request)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	input := &sfn.StartExecutionInput{
		StateMachineArn: stringPointer(normalized.StateMachineARN),
		Input:           stringPointer(string(normalized.Input)),
	}
	if normalized.Name != "" {
		input.Name = stringPointer(normalized.Name)
	}
	if normalized.TraceHeader != "" {
		input.TraceHeader = stringPointer(normalized.TraceHeader)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	output, callErr := b.client.StartExecution(ctx, input)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if callErr != nil {
		return nil, newError(ErrStartFailed, "start", "", callErr)
	}
	if output == nil || output.ExecutionArn == nil || output.StartDate == nil {
		return nil, newError(ErrMalformedOutput, "start", "", nil)
	}
	if err := validateARNValue(*output.ExecutionArn); err != nil {
		return nil, newError(ErrMalformedOutput, "start", "", nil)
	}
	startedAt := *output.StartDate
	return &Execution{
		ExecutionARN:    *output.ExecutionArn,
		StateMachineARN: normalized.StateMachineARN,
		Name:            normalized.Name,
		Input:           append([]byte(nil), normalized.Input...),
		Status:          StatusRunning,
		StartedAt:       startedAt,
	}, nil
}

// Describe - execution ARN으로 최신 execution 상태와 payload metadata를 조회한다.
func (b *Bridge) Describe(ctx context.Context, executionARN string) (*Execution, error) {
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := b.validate(); err != nil {
		return nil, err
	}
	if err := validateARN(executionARN); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	output, callErr := b.client.DescribeExecution(ctx, &sfn.DescribeExecutionInput{ExecutionArn: stringPointer(executionARN)})
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if callErr != nil {
		return nil, newError(ErrDescribeFailed, "describe", "", callErr)
	}
	if output == nil || output.ExecutionArn == nil || output.StateMachineArn == nil || output.StartDate == nil {
		return nil, newError(ErrMalformedOutput, "describe", "", nil)
	}
	if err := validateARNValue(*output.ExecutionArn); err != nil {
		return nil, newError(ErrMalformedOutput, "describe", "", nil)
	}
	if err := validateARNValue(*output.StateMachineArn); err != nil {
		return nil, newError(ErrMalformedOutput, "describe", "", nil)
	}
	status := ExecutionStatus(output.Status)
	execution := &Execution{
		ExecutionARN:    *output.ExecutionArn,
		StateMachineARN: *output.StateMachineArn,
		Status:          status,
		StartedAt:       *output.StartDate,
	}
	if output.Name != nil {
		if err := validateResponseName(*output.Name); err != nil {
			return nil, newError(ErrMalformedOutput, "describe", "", nil)
		}
		execution.Name = *output.Name
	}
	var err error
	if execution.Input, err = b.responsePayload(output.Input); err != nil {
		return nil, newError(ErrMalformedOutput, "describe", "", nil)
	}
	if execution.Output, err = b.responsePayload(output.Output); err != nil {
		return nil, newError(ErrMalformedOutput, "describe", "", nil)
	}
	if output.Error != nil {
		if err := validateBoundedUTF8(*output.Error, maxStopErrorSize); err != nil {
			return nil, newError(ErrMalformedOutput, "describe", "", nil)
		}
		execution.Error = *output.Error
	}
	if output.Cause != nil {
		if err := validateBoundedUTF8(*output.Cause, maxStopCauseSize); err != nil {
			return nil, newError(ErrMalformedOutput, "describe", "", nil)
		}
		execution.Cause = *output.Cause
	}
	if output.StopDate != nil {
		stoppedAt := *output.StopDate
		execution.StoppedAt = &stoppedAt
	}
	if !isKnownStatus(status) {
		return execution, newError(ErrUnknownStatus, "describe", status, nil)
	}
	return execution, nil
}

// Stop - StopExecution capability를 제공하는 client로 execution을 중지한다.
// StopExecution을 지원하지 않는 client에서는 ErrStopUnsupported를 반환한다.
func (b *Bridge) Stop(ctx context.Context, request StopRequest) (*Execution, error) {
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := b.validate(); err != nil {
		return nil, err
	}
	if err := validateARN(request.ExecutionARN); err != nil {
		return nil, err
	}
	if err := validateOptionalBoundedUTF8(request.Error, maxStopErrorSize); err != nil {
		return nil, newError(ErrInvalidRequest, "validate request", "", nil)
	}
	if err := validateOptionalBoundedUTF8(request.Cause, maxStopCauseSize); err != nil {
		return nil, newError(ErrInvalidRequest, "validate request", "", nil)
	}
	stopClient, ok := b.client.(StopClient)
	if !ok || isNilClient(stopClient) {
		return nil, newError(ErrStopUnsupported, "stop", "", nil)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	input := &sfn.StopExecutionInput{ExecutionArn: stringPointer(request.ExecutionARN)}
	if request.Error != "" {
		input.Error = stringPointer(request.Error)
	}
	if request.Cause != "" {
		input.Cause = stringPointer(request.Cause)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	output, callErr := stopClient.StopExecution(ctx, input)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if callErr != nil {
		return nil, newError(ErrStopFailed, "stop", "", callErr)
	}
	if output == nil || output.StopDate == nil {
		return nil, newError(ErrMalformedOutput, "stop", "", nil)
	}
	stoppedAt := *output.StopDate
	return &Execution{ExecutionARN: request.ExecutionARN, StoppedAt: &stoppedAt}, nil
}

// Wait - execution을 즉시 조회한 뒤 RUNNING 동안 bounded polling을 수행한다.
// terminal failure에서는 마지막 Execution과 상태별 오류를 함께 반환하며 자동 Stop/retry는 수행하지 않는다.
func (b *Bridge) Wait(ctx context.Context, executionARN string, options WaitOptions) (*Execution, error) {
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := b.validate(); err != nil {
		return nil, err
	}
	if err := validateARN(executionARN); err != nil {
		return nil, err
	}
	config, err := normalizeWaitOptions(options)
	if err != nil {
		return nil, err
	}
	waitCtx := ctx
	cancel := func() {}
	ownedTimeout := options.Timeout > 0
	if ownedTimeout {
		waitCtx, cancel = context.WithTimeout(ctx, options.Timeout)
	}
	defer cancel()

	var last *Execution
	delay := config.pollInterval
	attempt := 1
	for {
		if err := ctx.Err(); err != nil {
			return last, err
		}
		if err := waitCtx.Err(); err != nil {
			return last, waitError(ctx, waitCtx, err, ownedTimeout)
		}
		execution, describeErr := b.Describe(waitCtx, executionARN)
		last = execution
		if err := ctx.Err(); err != nil {
			return last, err
		}
		if describeErr != nil {
			return last, waitError(ctx, waitCtx, describeErr, ownedTimeout)
		}
		if execution == nil {
			return nil, newError(ErrMalformedOutput, "wait", "", nil)
		}
		switch execution.Status {
		case StatusRunning:
			if err := waitFor(waitCtx, ctx, delay); err != nil {
				return last, waitError(ctx, waitCtx, err, ownedTimeout)
			}
			if err := ctx.Err(); err != nil {
				return last, err
			}
			if err := waitCtx.Err(); err != nil {
				return last, waitError(ctx, waitCtx, err, ownedTimeout)
			}
			nextDelay := delay
			if config.backoff != nil {
				nextDelay = config.backoff(attempt, delay)
			}
			if nextDelay < 0 {
				return last, newError(ErrInvalidOptions, "wait", "", nil)
			}
			if nextDelay > config.maxPollInterval {
				nextDelay = config.maxPollInterval
			}
			delay = nextDelay
			attempt++
			continue
		case StatusSucceeded:
			return execution, nil
		case StatusFailed:
			return execution, newError(ErrExecutionFailed, "wait", execution.Status, nil)
		case StatusTimedOut:
			return execution, newError(ErrExecutionTimedOut, "wait", execution.Status, nil)
		case StatusAborted:
			return execution, newError(ErrExecutionAborted, "wait", execution.Status, nil)
		case StatusPendingRedrive:
			return execution, nil
		default:
			return execution, newError(ErrUnknownStatus, "wait", execution.Status, nil)
		}
	}
}

type startRequest struct {
	StateMachineARN string
	Name            string
	Input           []byte
	TraceHeader     string
}

type waitConfig struct {
	pollInterval    time.Duration
	maxPollInterval time.Duration
	backoff         Backoff
}

func (b *Bridge) validate() error {
	if b == nil || isNilClient(b.client) || b.maxInputSize < 1 || b.maxInputSize > defaultMaxInputSize {
		return newError(ErrInvalidOptions, "validate options", "", nil)
	}
	return nil
}

func (b *Bridge) validateStartRequest(request StartRequest) (startRequest, error) {
	if err := validateARN(request.StateMachineARN); err != nil {
		return startRequest{}, err
	}
	if request.Name != "" {
		if err := validateExecutionName(request.Name); err != nil {
			return startRequest{}, err
		}
	}
	if err := validateTraceHeader(request.TraceHeader); err != nil {
		return startRequest{}, newError(ErrInvalidRequest, "validate request", "", nil)
	}
	input := request.Input
	if len(input) == 0 {
		input = []byte("{}")
	}
	if len(input) > b.maxInputSize {
		return startRequest{}, newError(ErrInputTooLarge, "validate request", "", nil)
	}
	if !utf8.Valid(input) || !json.Valid(input) {
		return startRequest{}, newError(ErrInvalidRequest, "validate request", "", nil)
	}
	return startRequest{
		StateMachineARN: request.StateMachineARN,
		Name:            request.Name,
		Input:           append([]byte(nil), input...),
		TraceHeader:     request.TraceHeader,
	}, nil
}

func (b *Bridge) responsePayload(value *string) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	if len(*value) > b.maxInputSize || !utf8.ValidString(*value) {
		return nil, ErrMalformedOutput
	}
	return []byte(*value), nil
}

func normalizeWaitOptions(options WaitOptions) (waitConfig, error) {
	pollInterval := options.PollInterval
	if pollInterval == 0 {
		pollInterval = defaultPollInterval
	}
	maxPollInterval := options.MaxPollInterval
	if maxPollInterval == 0 {
		maxPollInterval = defaultMaxPoll
	}
	if pollInterval < 0 || maxPollInterval < 0 || options.Timeout < 0 || maxPollInterval < pollInterval {
		return waitConfig{}, newError(ErrInvalidOptions, "wait", "", nil)
	}
	backoff := options.Backoff
	if backoff == nil {
		backoff = defaultBackoff
	}
	return waitConfig{pollInterval: pollInterval, maxPollInterval: maxPollInterval, backoff: backoff}, nil
}

func defaultBackoff(attempt int, previous time.Duration) time.Duration {
	if attempt <= 0 || previous <= 0 {
		return previous
	}
	if previous > time.Duration(1<<63-1)/2 {
		return time.Duration(1<<63 - 1)
	}
	return previous * 2
}

func waitFor(waitCtx, parent context.Context, delay time.Duration) error {
	if delay <= 0 {
		if err := parent.Err(); err != nil {
			return err
		}
		if err := waitCtx.Err(); err != nil {
			return err
		}
		return nil
	}
	timer := time.NewTimer(delay)
	defer func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}()
	select {
	case <-timer.C:
		return nil
	case <-parent.Done():
		return parent.Err()
	case <-waitCtx.Done():
		if err := parent.Err(); err != nil {
			return err
		}
		return waitCtx.Err()
	}
}

func waitError(parent, waitCtx context.Context, err error, ownedTimeout bool) error {
	if parentErr := parent.Err(); parentErr != nil {
		return parentErr
	}
	if ownedTimeout && errors.Is(waitCtx.Err(), context.DeadlineExceeded) && errors.Is(err, context.DeadlineExceeded) {
		return newError(ErrWaitTimeout, "wait", "", context.DeadlineExceeded)
	}
	return err
}

func validateARN(value string) error {
	if err := validateARNValue(value); err != nil {
		return newError(ErrInvalidRequest, "validate request", "", nil)
	}
	return nil
}

func validateARNValue(value string) error {
	if !utf8.ValidString(value) || strings.TrimSpace(value) == "" || len(value) > maxARNSize {
		return ErrInvalidRequest
	}
	return nil
}

func validateExecutionName(value string) error {
	if len(value) < 1 || len(value) > maxExecutionName || !isASCII(value) {
		return newError(ErrInvalidName, "validate request", "", nil)
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '-' && char != '_' {
			return newError(ErrInvalidName, "validate request", "", nil)
		}
	}
	return nil
}

func validateResponseName(value string) error {
	if value == "" {
		return nil
	}
	if err := validateExecutionName(value); err != nil {
		return err
	}
	return nil
}

func validateTraceHeader(value string) error {
	if value == "" {
		return nil
	}
	if len(value) > maxTraceHeaderSize || !isASCII(value) {
		return ErrInvalidRequest
	}
	return nil
}

func validateOptionalBoundedUTF8(value string, limit int) error {
	if value == "" {
		return nil
	}
	return validateBoundedUTF8(value, limit)
}

func validateBoundedUTF8(value string, limit int) error {
	if !utf8.ValidString(value) || len(value) > limit {
		return ErrInvalidRequest
	}
	return nil
}

func isKnownStatus(status ExecutionStatus) bool {
	switch status {
	case StatusRunning, StatusSucceeded, StatusFailed, StatusTimedOut, StatusAborted, StatusPendingRedrive:
		return true
	default:
		return false
	}
}

func isASCII(value string) bool {
	for index := 0; index < len(value); index++ {
		if value[index] > 0x7f {
			return false
		}
	}
	return true
}

func isNilClient(value any) bool {
	if value == nil {
		return true
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}

func stringPointer(value string) *string { return &value }

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
