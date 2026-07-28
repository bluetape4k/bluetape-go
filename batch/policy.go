package batch

import (
	"context"
	"errors"
	"fmt"
)

// ErrorPredicate batch 단계, checkpoint, writer 안전성, 재시작에서 사용하는 함수 타입이다.
type ErrorPredicate func(error) bool

// RetryPolicy batch 단계, checkpoint, writer 안전성, 재시작에서 사용하는 구조체다.
type RetryPolicy struct {
	MaxAttempts int
	RetryIf     ErrorPredicate
}

// SkipPolicy batch 단계, checkpoint, writer 안전성, 재시작에서 사용하는 구조체다.
type SkipPolicy struct {
	MaxSkips int
	SkipIf   ErrorPredicate
}

// RetryErrors batch 단계, checkpoint, writer 안전성, 재시작 오류 처리 정책을 만든다.
//
// 매개변수:
//   - maxAttempts: 허용할 최대 재시도 횟수다.
//   - retryIf: 오류를 재시도 대상으로 볼지 판정하는 함수다.
//
// 반환 오류는 입력 검증 실패, context 취소/deadline, 상태 전이 실패, 패키지 sentinel error와 typed error를 그대로 드러낸다.
func RetryErrors(maxAttempts int, retryIf ErrorPredicate) (RetryPolicy, error) {
	if maxAttempts <= 0 {
		return RetryPolicy{}, fmt.Errorf("max attempts must be positive")
	}
	return RetryPolicy{MaxAttempts: maxAttempts, RetryIf: retryIf}, nil
}

// SkipErrors batch 단계, checkpoint, writer 안전성, 재시작 오류 처리 정책을 만든다.
//
// 매개변수:
//   - maxSkips: 허용할 최대 skip 횟수다.
//   - skipIf: 오류를 skip 대상으로 볼지 판정하는 함수다.
//
// 반환 오류는 입력 검증 실패, context 취소/deadline, 상태 전이 실패, 패키지 sentinel error와 typed error를 그대로 드러낸다.
func SkipErrors(maxSkips int, skipIf ErrorPredicate) (SkipPolicy, error) {
	if maxSkips <= 0 {
		return SkipPolicy{}, fmt.Errorf("max skips must be positive")
	}
	return SkipPolicy{MaxSkips: maxSkips, SkipIf: skipIf}, nil
}

func (p RetryPolicy) normalize() (RetryPolicy, error) {
	if p.MaxAttempts == 0 {
		p.MaxAttempts = 1
	}
	if p.MaxAttempts < 0 {
		return p, fmt.Errorf("max attempts must be positive")
	}
	if p.RetryIf == nil && p.MaxAttempts > 1 {
		p.RetryIf = nonContextError
	}
	return p, nil
}

func (p RetryPolicy) shouldRetry(err error, attempt int) bool {
	if err == nil || attempt >= p.MaxAttempts || isContextError(err) {
		return false
	}
	return p.RetryIf != nil && p.RetryIf(err)
}

func (p SkipPolicy) normalize() (SkipPolicy, error) {
	if p.MaxSkips < 0 {
		return p, fmt.Errorf("max skips must not be negative")
	}
	if p.SkipIf == nil && p.MaxSkips > 0 {
		p.SkipIf = nonContextError
	}
	return p, nil
}

func (p SkipPolicy) shouldSkip(err error, used int, cost int) bool {
	if err == nil || cost <= 0 || p.MaxSkips <= 0 || isContextError(err) {
		return false
	}
	if used+cost > p.MaxSkips {
		return false
	}
	return p.SkipIf != nil && p.SkipIf(err)
}

func nonContextError(err error) bool {
	return err != nil && !isContextError(err)
}

func isContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
