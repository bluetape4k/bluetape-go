package concurrency

import (
	"context"

	"golang.org/x/sync/errgroup"
)

// Task func 공개 타입이다.
type Task func(context.Context) error

// Group 패키지에서 공개하는 구조체다.
type Group struct {
	group *errgroup.Group
	ctx   context.Context
}

// NewGroup Group 인스턴스를 생성한다.
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

// Context Group에 연결된 context를 반환한다.
func (g *Group) Context() context.Context {
	return g.ctx
}

// SetLimit 병렬 작업 실행 흐름을 제어한다.
//
// 매개변수:
//   - limit: SetLimit에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func (g *Group) SetLimit(limit int) {
	g.group.SetLimit(limit)
}

// Go task를 새 goroutine에서 실행하고 결과를 수집한다.
//
// 매개변수:
//   - task: Go에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func (g *Group) Go(task Task) {
	g.group.Go(func() error {
		return runTask(g.ctx, task)
	})
}

// TryGo 병렬 작업 실행 흐름을 제어한다.
//
// 매개변수:
//   - task: TryGo에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func (g *Group) TryGo(task Task) bool {
	return g.group.TryGo(func() error {
		return runTask(g.ctx, task)
	})
}

// Wait 병렬 작업 실행 흐름을 제어한다.
//
// 반환 오류는 입력 검증 실패와 패키지에서 정의한 sentinel error/typed error를 그대로 드러낸다.
func (g *Group) Wait() error {
	return g.group.Wait()
}
