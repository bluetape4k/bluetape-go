package concurrency

import (
	"context"
	"fmt"
)

// Go는 Go 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - task: Go 동작에 필요한 task 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func Go(ctx context.Context, task Task) <-chan error {
	result := make(chan error, 1)
	go func() {
		result <- runTask(ctx, task)
		close(result)
	}()
	return result
}

// ForEach는 ForEach 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - values: ForEach가 읽거나 복사하는 values 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
//   - limit: ForEach 동작에 필요한 limit 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - worker: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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

// Map는 Map 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - values: Map가 읽거나 복사하는 values 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
//   - limit: Map 동작에 필요한 limit 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - mapper: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
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
