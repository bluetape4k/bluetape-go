package concurrencytest

import (
	"context"
	"testing"
)

// GoroutineStressTester 테스트 helper의 timeout, cancellation, cleanup에서 사용하는 구조체다.
type GoroutineStressTester struct {
	options Options
}

// NewGoroutineStressTester 테스트 helper의 timeout, cancellation, cleanup에 사용할 값을 생성한다.
//
// 매개변수:
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
func NewGoroutineStressTester(options Options) GoroutineStressTester {
	return GoroutineStressTester{options: options}
}

// Run 테스트 helper의 timeout, cancellation, cleanup의 쓰기 동작을 수행한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - tasks: Run에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소/deadline, 상태 전이 실패, 패키지 sentinel error와 typed error를 그대로 드러낸다.
func (t GoroutineStressTester) Run(ctx context.Context, tasks ...Task) (Report, error) {
	return runAll(ctx, t.options, tasks)
}

// RunT 테스트 helper의 timeout, cancellation, cleanup 동작을 수행한다.
//
// 매개변수:
//   - tb: 실패를 보고할 testing 객체다.
//   - tasks: RunT에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func (t GoroutineStressTester) RunT(tb testing.TB, tasks ...Task) Report {
	tb.Helper()
	report, err := t.Run(context.Background(), tasks...)
	return fail(tb, report, err)
}
