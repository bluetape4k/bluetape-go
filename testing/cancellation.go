package bttesting

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// ContextOperation 테스트 helper의 timeout, cancellation, cleanup에서 사용하는 함수 타입이다.
type ContextOperation func(context.Context) error

// WaiterProbe 테스트 helper의 timeout, cancellation, cleanup에서 사용하는 함수 타입이다.
type WaiterProbe func(context.Context, func()) error

// CleanupProbe 테스트 helper의 timeout, cancellation, cleanup에서 사용하는 함수 타입이다.
type CleanupProbe func(context.Context, func(), func()) error

// CheckContextCanceled 테스트 helper의 timeout, cancellation, cleanup 동작을 수행한다.
//
// 매개변수:
//   - operation: 보호 정책 안에서 실행할 작업이다.
//
// 반환 오류는 입력 검증 실패, context 취소/deadline, 상태 전이 실패, 패키지 sentinel error와 typed error를 그대로 드러낸다.
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

// RequireContextCanceled 테스트 helper의 timeout, cancellation, cleanup 동작을 수행한다.
//
// 매개변수:
//   - tb: 실패를 보고할 testing 객체다.
//   - operation: 보호 정책 안에서 실행할 작업이다.
func RequireContextCanceled(tb testing.TB, operation ContextOperation) {
	tb.Helper()

	if err := CheckContextCanceled(operation); err != nil {
		tb.Fatalf("context cancellation assertion failed: %v", err)
	}
}

// CheckDeadlineExceeded 테스트 helper의 timeout, cancellation, cleanup 동작을 수행한다.
//
// 매개변수:
//   - timeout: 대기 또는 실행을 제한할 시간이다. 0의 의미는 함수별 기본값을 따른다.
//   - operation: 보호 정책 안에서 실행할 작업이다.
//
// 반환 오류는 입력 검증 실패, context 취소/deadline, 상태 전이 실패, 패키지 sentinel error와 typed error를 그대로 드러낸다.
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

// RequireDeadlineExceeded 테스트 helper의 timeout, cancellation, cleanup 동작을 수행한다.
//
// 매개변수:
//   - tb: 실패를 보고할 testing 객체다.
//   - timeout: 대기 또는 실행을 제한할 시간이다. 0의 의미는 함수별 기본값을 따른다.
//   - operation: 보호 정책 안에서 실행할 작업이다.
func RequireDeadlineExceeded(tb testing.TB, timeout time.Duration, operation ContextOperation) {
	tb.Helper()

	if err := CheckDeadlineExceeded(timeout, operation); err != nil {
		tb.Fatalf("deadline assertion failed: %v", err)
	}
}

// CheckWaiterReleased 테스트 helper의 timeout, cancellation, cleanup 동작을 수행한다.
//
// 매개변수:
//   - timeout: 대기 또는 실행을 제한할 시간이다. 0의 의미는 함수별 기본값을 따른다.
//   - waiter: CheckWaiterReleased에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소/deadline, 상태 전이 실패, 패키지 sentinel error와 typed error를 그대로 드러낸다.
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

// RequireWaiterReleased 테스트 helper의 timeout, cancellation, cleanup 동작을 수행한다.
//
// 매개변수:
//   - tb: 실패를 보고할 testing 객체다.
//   - timeout: 대기 또는 실행을 제한할 시간이다. 0의 의미는 함수별 기본값을 따른다.
//   - waiter: RequireWaiterReleased에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func RequireWaiterReleased(tb testing.TB, timeout time.Duration, waiter WaiterProbe) {
	tb.Helper()

	if err := CheckWaiterReleased(timeout, waiter); err != nil {
		tb.Fatalf("waiter release assertion failed: %v", err)
	}
}

// CheckCleanupOnCancel 테스트 helper의 timeout, cancellation, cleanup 동작을 수행한다.
//
// 매개변수:
//   - timeout: 대기 또는 실행을 제한할 시간이다. 0의 의미는 함수별 기본값을 따른다.
//   - probe: 상태를 읽어올 함수다.
//
// 반환 오류는 입력 검증 실패, context 취소/deadline, 상태 전이 실패, 패키지 sentinel error와 typed error를 그대로 드러낸다.
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

// RequireCleanupOnCancel 테스트 helper의 timeout, cancellation, cleanup 동작을 수행한다.
//
// 매개변수:
//   - tb: 실패를 보고할 testing 객체다.
//   - timeout: 대기 또는 실행을 제한할 시간이다. 0의 의미는 함수별 기본값을 따른다.
//   - probe: 상태를 읽어올 함수다.
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
