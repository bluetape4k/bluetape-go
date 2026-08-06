package concurrency

import (
	"context"
	"fmt"
)

// WorkerPool 패키지에서 공개하는 구조체다.
type WorkerPool[T any] struct {
	size    int
	handler func(context.Context, T) error
}

// NewWorkerPool WorkerPool 인스턴스를 생성한다.
//
// 매개변수:
//   - size: NewWorkerPool에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - handler: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func NewWorkerPool[T any](size int, handler func(context.Context, T) error) (*WorkerPool[T], error) {
	if size <= 0 {
		return nil, fmt.Errorf("size must be positive")
	}
	if handler == nil {
		return nil, fmt.Errorf("handler must not be nil")
	}

	return &WorkerPool[T]{
		size:    size,
		handler: handler,
	}, nil
}

// Run 작업을 실행하고 완료 또는 오류를 반환한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - jobs: Run에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func (p *WorkerPool[T]) Run(ctx context.Context, jobs <-chan T) error {
	if jobs == nil {
		return fmt.Errorf("jobs must not be nil")
	}

	group := NewGroup(ctx)
	for range p.size {
		group.Go(func(taskCtx context.Context) error {
			for {
				select {
				case <-taskCtx.Done():
					return taskCtx.Err()
				case job, ok := <-jobs:
					if !ok {
						return nil
					}
					if err := p.handler(taskCtx, job); err != nil {
						return err
					}
				}
			}
		})
	}

	return group.Wait()
}
