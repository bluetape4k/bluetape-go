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
//   - t: Eventually 동작에 필요한 t 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - timeout: Eventually 동작에 필요한 timeout 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - condition: Eventually 동작에 필요한 condition 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func Eventually(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()

	EventuallyWithPolling(t, timeout, defaultPollInterval, condition)
}

// EventuallyWithPolling EventuallyWithPolling 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - t: EventuallyWithPolling 동작에 필요한 t 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - timeout: EventuallyWithPolling 동작에 필요한 timeout 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - polling: EventuallyWithPolling 동작에 필요한 polling 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - condition: EventuallyWithPolling 동작에 필요한 condition 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
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
//   - t: Consistently 동작에 필요한 t 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - duration: Consistently 동작에 필요한 duration 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - condition: Consistently 동작에 필요한 condition 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func Consistently(t *testing.T, duration time.Duration, condition func() bool) {
	t.Helper()

	ConsistentlyWithPolling(t, duration, defaultPollInterval, condition)
}

// ConsistentlyWithPolling ConsistentlyWithPolling 공개 API의 동작을 수행하며 테스트 helper의 timeout, cancellation, cleanup 계약을 보존한다.
//
// 매개변수:
//   - t: ConsistentlyWithPolling 동작에 필요한 t 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - duration: ConsistentlyWithPolling 동작에 필요한 duration 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - polling: ConsistentlyWithPolling 동작에 필요한 polling 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - condition: ConsistentlyWithPolling 동작에 필요한 condition 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
func ConsistentlyWithPolling(t *testing.T, duration time.Duration, polling time.Duration, condition func() bool) {
	t.Helper()

	gomega.NewWithT(t).
		Consistently(condition).
		WithTimeout(duration).
		WithPolling(polling).
		Should(gomega.BeTrue())
}
