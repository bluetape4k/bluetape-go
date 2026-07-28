package batch

import "context"

// Reader batch 단계, checkpoint, writer 안전성, 재시작에서 사용하는 인터페이스이다.
type Reader[T any] interface {
	Open(context.Context) error
	Read(context.Context) (T, bool, error)
	Close(context.Context) error
}

// Processor batch 단계, checkpoint, writer 안전성, 재시작에서 사용하는 인터페이스이다.
type Processor[I any, O any] interface {
	Process(context.Context, I) (O, bool, error)
}

// ProcessorFunc batch 단계, checkpoint, writer 안전성, 재시작에서 사용하는 함수 타입이다.
type ProcessorFunc[I any, O any] func(context.Context, I) (O, bool, error)

// Process batch 단계, checkpoint, writer 안전성, 재시작의 상태를 변경한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - item: 처리할 단일 항목이다.
//
// 반환 오류는 입력 검증 실패, context 취소/deadline, 상태 전이 실패, 패키지 sentinel error와 typed error를 그대로 드러낸다.
func (f ProcessorFunc[I, O]) Process(ctx context.Context, item I) (O, bool, error) {
	return f(ctx, item)
}

// IdentityProcessor batch 단계, checkpoint, writer 안전성, 재시작 동작을 수행한다.
func IdentityProcessor[T any]() Processor[T, T] {
	return ProcessorFunc[T, T](func(ctx context.Context, item T) (T, bool, error) {
		if err := ctx.Err(); err != nil {
			var zero T
			return zero, false, err
		}
		return item, true, nil
	})
}

// Writer batch 단계, checkpoint, writer 안전성, 재시작에서 사용하는 인터페이스이다.
type Writer[T any] interface {
	Open(context.Context) error
	Write(context.Context, []T) error
	Close(context.Context) error
}
