package concurrency

import (
	"context"
	"fmt"
)

// PanicError는 struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type PanicError struct {
	Value any
}

func (e PanicError) Error() string {
	return fmt.Sprintf("task panicked: %v", e.Value)
}

func capturePanic(errp *error) {
	if recovered := recover(); recovered != nil {
		*errp = PanicError{Value: recovered}
	}
}

func runTask(ctx context.Context, task Task) (err error) {
	defer capturePanic(&err)

	if task == nil {
		return fmt.Errorf("task must not be nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return task(ctx)
}
