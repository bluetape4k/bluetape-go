package concurrency

import (
	"context"
	"fmt"
)

// Go task를 새 goroutine에서 실행하고 결과를 수집한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - task: Go에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func Go(ctx context.Context, task Task) <-chan error {
	result := make(chan error, 1)
	go func() {
		result <- runTask(ctx, task)
		close(result)
	}()
	return result
}

// ForEach 값 목록을 제한된 병렬도로 처리한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - values: 처리할 값 목록이다. nil과 빈 슬라이스는 함수별 입력 규칙에 따라 빈 입력으로 다룬다.
//   - limit: ForEach에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - worker: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func ForEach[T any](ctx context.Context, values []T, limit int, worker func(context.Context, T) error) error {
	if limit <= 0 {
		return fmt.Errorf("limit must be positive")
	}
	if worker == nil {
		return fmt.Errorf("worker must not be nil")
	}

	group := NewGroup(ctx)
	group.SetLimit(limit)

	for _, value := range values {
		value := value
		group.Go(func(taskCtx context.Context) error {
			return worker(taskCtx, value)
		})
	}

	return group.Wait()
}

// Map 값 목록을 제한된 병렬도로 변환한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - values: 처리할 값 목록이다. nil과 빈 슬라이스는 함수별 입력 규칙에 따라 빈 입력으로 다룬다.
//   - limit: Map에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - mapper: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func Map[T any, R any](ctx context.Context, values []T, limit int, mapper func(context.Context, T) (R, error)) ([]R, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be positive")
	}
	if mapper == nil {
		return nil, fmt.Errorf("mapper must not be nil")
	}
	if values == nil {
		return nil, nil
	}

	results := make([]R, len(values))
	group := NewGroup(ctx)
	group.SetLimit(limit)

	for index, value := range values {
		index, value := index, value
		group.Go(func(taskCtx context.Context) error {
			mapped, err := mapper(taskCtx, value)
			if err != nil {
				return err
			}
			results[index] = mapped
			return nil
		})
	}

	if err := group.Wait(); err != nil {
		return nil, err
	}
	return results, nil
}
