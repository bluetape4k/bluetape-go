package concurrency

import (
	"context"
	"fmt"
)

// PanicError 패키지에서 공개하는 구조체다.
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
