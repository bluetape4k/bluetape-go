package concurrency

import (
	"context"

	"golang.org/x/sync/errgroup"
)

// Task func 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Task func(context.Context) error

// Group struct 공개 타입이다.
// 값의 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Group struct {
	group *errgroup.Group
	ctx   context.Context
}

// NewGroup NewGroup 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
func NewGroup(ctx context.Context) *Group {
	if ctx == nil {
		ctx = context.Background()
	}

	group, groupCtx := errgroup.WithContext(ctx)
	return &Group{
		group: group,
		ctx:   groupCtx,
	}
}

// Context Context 공개 API의 동작을 수행한다.
func (g *Group) Context() context.Context {
	return g.ctx
}

// SetLimit SetLimit 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - limit: SetLimit 동작에 필요한 limit 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func (g *Group) SetLimit(limit int) {
	g.group.SetLimit(limit)
}

// Go Go 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - task: Go 동작에 필요한 task 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func (g *Group) Go(task Task) {
	g.group.Go(func() error {
		return runTask(g.ctx, task)
	})
}

// TryGo TryGo 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - task: TryGo 동작에 필요한 task 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func (g *Group) TryGo(task Task) bool {
	return g.group.TryGo(func() error {
		return runTask(g.ctx, task)
	})
}

// Wait Wait 공개 API의 동작을 수행한다.
//
// 반환 오류는 입력 검증 실패, 취소, 외부 원인, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (g *Group) Wait() error {
	return g.group.Wait()
}
