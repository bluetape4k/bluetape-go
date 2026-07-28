package concurrency

import (
	"context"
	"fmt"
)

// WorkerPool struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type WorkerPool[T any] struct {
	size    int
	handler func(context.Context, T) error
}

// NewWorkerPool NewWorkerPool 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - size: NewWorkerPool 동작에 필요한 size 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - handler: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// Run Run 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - jobs: Run 동작에 필요한 jobs 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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
