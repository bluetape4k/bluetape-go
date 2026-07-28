package resilience

import (
	"context"
	"fmt"
)

// Operation func 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Operation[T any] func(context.Context) (T, error)

// Policy interface 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Policy[T any] interface {
	Apply(Operation[T]) Operation[T]
}

// PolicyFunc func 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type PolicyFunc[T any] func(Operation[T]) Operation[T]

// Apply Apply 공개 API의 동작을 수행하며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
//
// 매개변수:
//   - operation: Apply 동작에 필요한 operation 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func (fn PolicyFunc[T]) Apply(operation Operation[T]) Operation[T] {
	return fn(operation)
}

// Compose Compose 공개 API의 동작을 수행하며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
//
// 매개변수:
//   - policies: Compose 동작에 필요한 policies 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
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
//   - operation: Run 동작에 필요한 operation 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - policies: Run 동작에 필요한 policies 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, deadline, 상태 전이 실패, 또는 패키지 sentinel/typed error 계약을 보존한다.
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
