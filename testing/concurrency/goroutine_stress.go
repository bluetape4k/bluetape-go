package concurrencytest

import (
	"context"
	"testing"
)

// GoroutineStressTester는 struct 공개 타입이며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type GoroutineStressTester struct {
	options Options
}

// NewGoroutineStressTester는 NewGoroutineStressTester 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - options: NewGoroutineStressTester 동작에 필요한 options 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func NewGoroutineStressTester(options Options) GoroutineStressTester {
	return GoroutineStressTester{options: options}
}

// Run는 Run 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - tasks: Run 동작에 필요한 tasks 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, deadline, 상태 전이 실패, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (t GoroutineStressTester) Run(ctx context.Context, tasks ...Task) (Report, error) {
	return runAll(ctx, t.options, tasks)
}

// RunT는 RunT 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - tb: RunT 동작에 필요한 tb 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - tasks: RunT 동작에 필요한 tasks 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func (t GoroutineStressTester) RunT(tb testing.TB, tasks ...Task) Report {
	tb.Helper()
	report, err := t.Run(context.Background(), tasks...)
	return fail(tb, report, err)
}
