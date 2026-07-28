package concurrencytest

import (
	"context"
	"testing"
)

// AsyncJobTester는 struct 공개 타입이며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type AsyncJobTester struct {
	options Options
}

// NewAsyncJobTester는 NewAsyncJobTester 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - options: NewAsyncJobTester 동작에 필요한 options 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func NewAsyncJobTester(options Options) AsyncJobTester {
	return AsyncJobTester{options: options}
}

// Run는 Run 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - jobs: Run 동작에 필요한 jobs 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, deadline, 상태 전이 실패, 또는 패키지 sentinel/typed error 계약을 보존한다.
func (t AsyncJobTester) Run(ctx context.Context, jobs ...Task) (Report, error) {
	return runAll(ctx, t.options, jobs)
}

// RunT는 RunT 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - tb: RunT 동작에 필요한 tb 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - jobs: RunT 동작에 필요한 jobs 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func (t AsyncJobTester) RunT(tb testing.TB, jobs ...Task) Report {
	tb.Helper()
	report, err := t.Run(context.Background(), jobs...)
	return fail(tb, report, err)
}
