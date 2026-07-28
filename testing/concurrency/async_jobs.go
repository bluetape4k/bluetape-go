package concurrencytest

import (
	"context"
	"testing"
)

// AsyncJobTester struct 공개 타입이며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type AsyncJobTester struct {
	options Options
}

// NewAsyncJobTester NewAsyncJobTester 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
func NewAsyncJobTester(options Options) AsyncJobTester {
	return AsyncJobTester{options: options}
}

// Run Run 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//   - jobs: Run에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 입력 검증 실패, context 취소/deadline, 상태 전이 실패, 패키지 sentinel error와 typed error를 그대로 드러낸다.
func (t AsyncJobTester) Run(ctx context.Context, jobs ...Task) (Report, error) {
	return runAll(ctx, t.options, jobs)
}

// RunT RunT 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - tb: 실패를 보고할 testing 객체다.
//   - jobs: RunT에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func (t AsyncJobTester) RunT(tb testing.TB, jobs ...Task) Report {
	tb.Helper()
	report, err := t.Run(context.Background(), jobs...)
	return fail(tb, report, err)
}
