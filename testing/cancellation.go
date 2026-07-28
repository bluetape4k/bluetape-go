package bttesting

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// ContextOperation func 공개 타입이며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type ContextOperation func(context.Context) error

// WaiterProbe func 공개 타입이며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type WaiterProbe func(context.Context, func()) error

// CleanupProbe func 공개 타입이며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type CleanupProbe func(context.Context, func(), func()) error

// CheckContextCanceled CheckContextCanceled 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - operation: CheckContextCanceled 동작에 필요한 operation 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, deadline, 상태 전이 실패, 또는 패키지 sentinel/typed error 계약을 보존한다.
func CheckContextCanceled(operation ContextOperation) error {
	if operation == nil {
		return fmt.Errorf("operation must not be nil")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := operation(ctx)
	if !errors.Is(err, context.Canceled) {
		return expectedError("context.Canceled", err)
	}
	return nil
}

// RequireContextCanceled RequireContextCanceled 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - tb: RequireContextCanceled 동작에 필요한 tb 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - operation: RequireContextCanceled 동작에 필요한 operation 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func RequireContextCanceled(tb testing.TB, operation ContextOperation) {
	tb.Helper()

	if err := CheckContextCanceled(operation); err != nil {
		tb.Fatalf("context cancellation assertion failed: %v", err)
	}
}

// CheckDeadlineExceeded CheckDeadlineExceeded 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - timeout: CheckDeadlineExceeded 동작에 필요한 timeout 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - operation: CheckDeadlineExceeded 동작에 필요한 operation 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, deadline, 상태 전이 실패, 또는 패키지 sentinel/typed error 계약을 보존한다.
func CheckDeadlineExceeded(timeout time.Duration, operation ContextOperation) error {
	if timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	if operation == nil {
		return fmt.Errorf("operation must not be nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- operation(ctx)
	}()

	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			return expectedError(context.DeadlineExceeded.Error(), err)
		}
		return nil
	case <-time.After(timeout * 2):
		return fmt.Errorf("operation did not return after %s deadline", timeout)
	}
}

// RequireDeadlineExceeded RequireDeadlineExceeded 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - tb: RequireDeadlineExceeded 동작에 필요한 tb 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - timeout: RequireDeadlineExceeded 동작에 필요한 timeout 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - operation: RequireDeadlineExceeded 동작에 필요한 operation 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func RequireDeadlineExceeded(tb testing.TB, timeout time.Duration, operation ContextOperation) {
	tb.Helper()

	if err := CheckDeadlineExceeded(timeout, operation); err != nil {
		tb.Fatalf("deadline assertion failed: %v", err)
	}
}

// CheckWaiterReleased CheckWaiterReleased 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - timeout: CheckWaiterReleased 동작에 필요한 timeout 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - waiter: CheckWaiterReleased 동작에 필요한 waiter 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, deadline, 상태 전이 실패, 또는 패키지 sentinel/typed error 계약을 보존한다.
func CheckWaiterReleased(timeout time.Duration, waiter WaiterProbe) error {
	if timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	if waiter == nil {
		return fmt.Errorf("waiter must not be nil")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ready := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- waiter(ctx, closeOnce(ready))
	}()

	select {
	case <-ready:
	case err := <-done:
		return wrapProbeError("waiter returned before signaling ready", err)
	case <-time.After(timeout):
		return fmt.Errorf("waiter did not signal ready within %s", timeout)
	}

	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			return expectedError("context.Canceled after waiter cancellation", err)
		}
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("waiter did not return within %s after cancellation", timeout)
	}
}

// RequireWaiterReleased RequireWaiterReleased 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - tb: RequireWaiterReleased 동작에 필요한 tb 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - timeout: RequireWaiterReleased 동작에 필요한 timeout 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - waiter: RequireWaiterReleased 동작에 필요한 waiter 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func RequireWaiterReleased(tb testing.TB, timeout time.Duration, waiter WaiterProbe) {
	tb.Helper()

	if err := CheckWaiterReleased(timeout, waiter); err != nil {
		tb.Fatalf("waiter release assertion failed: %v", err)
	}
}

// CheckCleanupOnCancel CheckCleanupOnCancel 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - timeout: CheckCleanupOnCancel 동작에 필요한 timeout 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - probe: CheckCleanupOnCancel 동작에 필요한 probe 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, deadline, 상태 전이 실패, 또는 패키지 sentinel/typed error 계약을 보존한다.
func CheckCleanupOnCancel(timeout time.Duration, probe CleanupProbe) error {
	if timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	if probe == nil {
		return fmt.Errorf("probe must not be nil")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ready := make(chan struct{})
	cleaned := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- probe(ctx, closeOnce(ready), closeOnce(cleaned))
	}()

	select {
	case <-ready:
	case err := <-done:
		return wrapProbeError("probe returned before signaling ready", err)
	case <-time.After(timeout):
		return fmt.Errorf("probe did not signal ready within %s", timeout)
	}

	cancel()

	var observedCleanup bool
	var observedReturn bool
	var runErr error
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for !observedCleanup || !observedReturn {
		select {
		case <-cleaned:
			observedCleanup = true
			cleaned = nil
		case runErr = <-done:
			observedReturn = true
			done = nil
		case <-timer.C:
			if !observedCleanup {
				return fmt.Errorf("cleanup was not observed within %s after cancellation", timeout)
			}
			return fmt.Errorf("probe did not return within %s after cancellation", timeout)
		}
	}

	if !errors.Is(runErr, context.Canceled) {
		return expectedError("context.Canceled after cleanup cancellation", runErr)
	}
	return nil
}

// RequireCleanupOnCancel RequireCleanupOnCancel 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - tb: RequireCleanupOnCancel 동작에 필요한 tb 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - timeout: RequireCleanupOnCancel 동작에 필요한 timeout 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - probe: RequireCleanupOnCancel 동작에 필요한 probe 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func RequireCleanupOnCancel(tb testing.TB, timeout time.Duration, probe CleanupProbe) {
	tb.Helper()

	if err := CheckCleanupOnCancel(timeout, probe); err != nil {
		tb.Fatalf("cleanup assertion failed: %v", err)
	}
}

func closeOnce(ch chan struct{}) func() {
	var once sync.Once
	return func() {
		once.Do(func() { close(ch) })
	}
}

func expectedError(want string, got error) error {
	if got == nil {
		return fmt.Errorf("expected %s, got <nil>", want)
	}
	return fmt.Errorf("expected %s, got %w", want, got)
}

func wrapProbeError(prefix string, err error) error {
	if err == nil {
		return fmt.Errorf("%s: <nil>", prefix)
	}
	return fmt.Errorf("%s: %w", prefix, err)
}
