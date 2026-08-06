package resilience

import (
	"context"
	"fmt"
)

// Operation func 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
type Operation[T any] func(context.Context) (T, error)

// Policy interface 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
type Policy[T any] interface {
	Apply(Operation[T]) Operation[T]
}

// PolicyFunc func 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
type PolicyFunc[T any] func(Operation[T]) Operation[T]

// Apply Apply 공개 API의 동작을 수행하며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
//
// 매개변수:
//   - operation: 보호 정책 안에서 실행할 작업이다.
func (fn PolicyFunc[T]) Apply(operation Operation[T]) Operation[T] {
	return fn(operation)
}

// Compose Compose 공개 API의 동작을 수행하며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
//
// 매개변수:
//   - policies: 순서대로 적용할 policy 목록이다.
func Compose[T any](policies ...Policy[T]) Policy[T] {
	return PolicyFunc[T](func(operation Operation[T]) Operation[T] {
		wrapped := operation
		for index := len(policies) - 1; index >= 0; index-- {
			if policies[index] == nil {
				continue
			}
			wrapped = policies[index].Apply(wrapped)
		}
		return wrapped
	})
}

// Run Run 공개 API의 동작을 수행하며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - operation: 보호 정책 안에서 실행할 작업이다.
//   - policies: 순서대로 적용할 policy 목록이다.
//
// 반환 오류는 입력 검증 실패, context 취소/deadline, 상태 전이 실패, 패키지 sentinel error와 typed error를 그대로 드러낸다.
func Run[T any](ctx context.Context, operation Operation[T], policies ...Policy[T]) (T, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	wrapped := Compose(policies...).Apply(operation)
	return runOperation(ctx, wrapped)
}

func runOperation[T any](ctx context.Context, operation Operation[T]) (T, error) {
	var zero T
	if operation == nil {
		return zero, fmt.Errorf("operation must not be nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return operation(ctx)
}
