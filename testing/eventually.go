package bttesting

import (
	"testing"
	"time"

	"github.com/onsi/gomega"
)

const defaultPollInterval = 25 * time.Millisecond

// Eventually 는 condition이 timeout 안에 true가 될 때까지 평가한다.
func Eventually(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()

	EventuallyWithPolling(t, timeout, defaultPollInterval, condition)
}

// EventuallyWithPolling 은 지정한 polling 간격으로 condition을 평가한다.
func EventuallyWithPolling(t *testing.T, timeout time.Duration, polling time.Duration, condition func() bool) {
	t.Helper()

	gomega.NewWithT(t).
		Eventually(condition).
		WithTimeout(timeout).
		WithPolling(polling).
		Should(gomega.BeTrue())
}

// Consistently 는 condition이 duration 동안 true로 유지되는지 확인한다.
func Consistently(t *testing.T, duration time.Duration, condition func() bool) {
	t.Helper()

	ConsistentlyWithPolling(t, duration, defaultPollInterval, condition)
}

// ConsistentlyWithPolling 은 지정한 polling 간격으로 condition이 유지되는지 확인한다.
func ConsistentlyWithPolling(t *testing.T, duration time.Duration, polling time.Duration, condition func() bool) {
	t.Helper()

	gomega.NewWithT(t).
		Consistently(condition).
		WithTimeout(duration).
		WithPolling(polling).
		Should(gomega.BeTrue())
}
