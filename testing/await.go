package bttesting

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// AwaitStatus int 공개 타입이며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type AwaitStatus int

const (
	// AwaitContinue keeps polling until the timeout, context cancellation, or a
	// later terminal status.
	AwaitContinue AwaitStatus = iota
	// AwaitSuccess stops polling successfully.
	AwaitSuccess
	// AwaitFailure stops polling with an immediate diagnostic failure.
	AwaitFailure
)

// AwaitProbe func 공개 타입이며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type AwaitProbe[T any] func(context.Context) (T, error)

// AwaitCheck func 공개 타입이며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type AwaitCheck[T any] func(T, error) AwaitStatus

// AwaitErrorProbe func 공개 타입이며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type AwaitErrorProbe func(context.Context) error

// AwaitResult struct 공개 타입이며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type AwaitResult[T any] struct {
	// Value is the final value returned by the probe.
	Value T
	// Err is the final error returned by the probe.
	Err error
	// Attempts is the number of probe calls.
	Attempts int
	// Elapsed is the wall-clock time spent polling.
	Elapsed time.Duration
}

// CheckAwait CheckAwait 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - timeout: CheckAwait 동작에 필요한 timeout 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - interval: CheckAwait 동작에 필요한 interval 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - probe: CheckAwait 동작에 필요한 probe 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - check: CheckAwait 동작에 필요한 check 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, deadline, 상태 전이 실패, 또는 패키지 sentinel/typed error 계약을 보존한다.
func CheckAwait[T any](
	ctx context.Context,
	timeout time.Duration,
	interval time.Duration,
	probe AwaitProbe[T],
	check AwaitCheck[T],
) (AwaitResult[T], error) {
	var result AwaitResult[T]
	started := time.Now()

	if timeout <= 0 {
		return result, fmt.Errorf("timeout must be positive")
	}
	if interval <= 0 {
		return result, fmt.Errorf("interval must be positive")
	}
	if probe == nil {
		return result, fmt.Errorf("probe must not be nil")
	}
	if check == nil {
		return result, fmt.Errorf("check must not be nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return result, awaitObservationError("await context ended before first attempt", result, err)
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		value, err := probe(runCtx)
		result.Value = value
		result.Err = err
		result.Attempts++
		result.Elapsed = time.Since(started)

		if isContextCancellation(err) {
			return result, awaitObservationError("await probe returned context cancellation", result, err)
		}

		switch status := check(value, err); status {
		case AwaitSuccess:
			return result, nil
		case AwaitFailure:
			return result, awaitObservationError("await failed", result, err)
		case AwaitContinue:
		default:
			return result, awaitObservationError("unknown await status", result, nil)
		}

		select {
		case <-runCtx.Done():
			result.Elapsed = time.Since(started)
			if ctxErr := ctx.Err(); ctxErr != nil {
				return result, awaitObservationError("await context ended", result, ctxErr)
			}
			return result, awaitObservationError("await timed out", result, context.DeadlineExceeded)
		case <-ticker.C:
		}
	}
}

// RequireAwait RequireAwait 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - tb: RequireAwait 동작에 필요한 tb 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - timeout: RequireAwait 동작에 필요한 timeout 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - interval: RequireAwait 동작에 필요한 interval 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - probe: RequireAwait 동작에 필요한 probe 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - check: RequireAwait 동작에 필요한 check 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func RequireAwait[T any](
	ctx context.Context,
	tb testing.TB,
	timeout time.Duration,
	interval time.Duration,
	probe AwaitProbe[T],
	check AwaitCheck[T],
) AwaitResult[T] {
	tb.Helper()

	result, err := CheckAwait(ctx, timeout, interval, probe, check)
	if err != nil {
		tb.Fatalf("await assertion failed: %v", err)
	}
	return result
}

// CheckAwaitValue CheckAwaitValue 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - timeout: CheckAwaitValue 동작에 필요한 timeout 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - interval: CheckAwaitValue 동작에 필요한 interval 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - probe: CheckAwaitValue 동작에 필요한 probe 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - want: CheckAwaitValue 동작에 필요한 want 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, deadline, 상태 전이 실패, 또는 패키지 sentinel/typed error 계약을 보존한다.
func CheckAwaitValue[T comparable](
	ctx context.Context,
	timeout time.Duration,
	interval time.Duration,
	probe AwaitProbe[T],
	want T,
) (AwaitResult[T], error) {
	return CheckAwait(ctx, timeout, interval, probe, func(value T, err error) AwaitStatus {
		if err == nil && value == want {
			return AwaitSuccess
		}
		return AwaitContinue
	})
}

// RequireAwaitValue RequireAwaitValue 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - tb: RequireAwaitValue 동작에 필요한 tb 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - timeout: RequireAwaitValue 동작에 필요한 timeout 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - interval: RequireAwaitValue 동작에 필요한 interval 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - probe: RequireAwaitValue 동작에 필요한 probe 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - want: RequireAwaitValue 동작에 필요한 want 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func RequireAwaitValue[T comparable](
	ctx context.Context,
	tb testing.TB,
	timeout time.Duration,
	interval time.Duration,
	probe AwaitProbe[T],
	want T,
) AwaitResult[T] {
	tb.Helper()

	result, err := CheckAwaitValue(ctx, timeout, interval, probe, want)
	if err != nil {
		tb.Fatalf("await value assertion failed: %v", err)
	}
	return result
}

// CheckAwaitError CheckAwaitError 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - timeout: CheckAwaitError 동작에 필요한 timeout 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - interval: CheckAwaitError 동작에 필요한 interval 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - probe: CheckAwaitError 동작에 필요한 probe 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - target: 검사하거나 감쌀 오류 값이다.
//
// 반환 오류는 입력 검증 실패, 취소, deadline, 상태 전이 실패, 또는 패키지 sentinel/typed error 계약을 보존한다.
func CheckAwaitError(
	ctx context.Context,
	timeout time.Duration,
	interval time.Duration,
	probe AwaitErrorProbe,
	target error,
) (AwaitResult[struct{}], error) {
	var result AwaitResult[struct{}]
	if target == nil {
		return result, fmt.Errorf("target error must not be nil")
	}
	if probe == nil {
		return result, fmt.Errorf("probe must not be nil")
	}

	return CheckAwait(ctx, timeout, interval, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, probe(ctx)
	}, func(_ struct{}, err error) AwaitStatus {
		if errors.Is(err, target) {
			return AwaitSuccess
		}
		return AwaitContinue
	})
}

// RequireAwaitError RequireAwaitError 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - tb: RequireAwaitError 동작에 필요한 tb 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - timeout: RequireAwaitError 동작에 필요한 timeout 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - interval: RequireAwaitError 동작에 필요한 interval 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - probe: RequireAwaitError 동작에 필요한 probe 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - target: 검사하거나 감쌀 오류 값이다.
func RequireAwaitError(
	ctx context.Context,
	tb testing.TB,
	timeout time.Duration,
	interval time.Duration,
	probe AwaitErrorProbe,
	target error,
) AwaitResult[struct{}] {
	tb.Helper()

	result, err := CheckAwaitError(ctx, timeout, interval, probe, target)
	if err != nil {
		tb.Fatalf("await error assertion failed: %v", err)
	}
	return result
}

func isContextCancellation(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func awaitObservationError[T any](prefix string, result AwaitResult[T], cause error) error {
	message := fmt.Sprintf(
		"%s after %d attempts: last value %#v; last error %s",
		prefix,
		result.Attempts,
		result.Value,
		formatAwaitError(result.Err),
	)
	if cause == nil {
		return errors.New(message)
	}
	if result.Err != nil && !errors.Is(result.Err, cause) {
		return fmt.Errorf("%s: %w", message, errors.Join(cause, result.Err))
	}
	return fmt.Errorf("%s: %w", message, cause)
}

func formatAwaitError(err error) string {
	if err == nil {
		return "<nil>"
	}
	return err.Error()
}
