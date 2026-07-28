package bttesting

import (
	"testing"
	"time"

	"github.com/onsi/gomega"
)

const defaultPollInterval = 25 * time.Millisecond

// Eventually Eventually 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - t: Eventually에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - timeout: 대기 또는 실행을 제한할 시간이다. 0의 의미는 함수별 기본값을 따른다.
//   - condition: Eventually에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func Eventually(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()

	EventuallyWithPolling(t, timeout, defaultPollInterval, condition)
}

// EventuallyWithPolling EventuallyWithPolling 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - t: EventuallyWithPolling에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - timeout: 대기 또는 실행을 제한할 시간이다. 0의 의미는 함수별 기본값을 따른다.
//   - polling: EventuallyWithPolling에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - condition: EventuallyWithPolling에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func EventuallyWithPolling(t *testing.T, timeout time.Duration, polling time.Duration, condition func() bool) {
	t.Helper()

	gomega.NewWithT(t).
		Eventually(condition).
		WithTimeout(timeout).
		WithPolling(polling).
		Should(gomega.BeTrue())
}

// Consistently Consistently 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - t: Consistently에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - duration: Consistently에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - condition: Consistently에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func Consistently(t *testing.T, duration time.Duration, condition func() bool) {
	t.Helper()

	ConsistentlyWithPolling(t, duration, defaultPollInterval, condition)
}

// ConsistentlyWithPolling ConsistentlyWithPolling 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - t: ConsistentlyWithPolling에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - duration: ConsistentlyWithPolling에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - polling: ConsistentlyWithPolling에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//   - condition: ConsistentlyWithPolling에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
func ConsistentlyWithPolling(t *testing.T, duration time.Duration, polling time.Duration, condition func() bool) {
	t.Helper()

	gomega.NewWithT(t).
		Consistently(condition).
		WithTimeout(duration).
		WithPolling(polling).
		Should(gomega.BeTrue())
}
