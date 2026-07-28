package leadertest

import (
	"context"
	"errors"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
)

// Timing은 provider conformance case가 사용하는 시간 경계를 설정한다.
type Timing struct {
	// Lease case lease duration을 설정한다.
	Lease time.Duration
	// RenewInterval은 case renewal cadence를 설정한다.
	RenewInterval time.Duration
	// CaseTimeout은 cancellation과 containment가 시작되기 전 evaluator work의 한도를 정한다.
	// final provider join은 제한하지 않는다. 호출자는 provider work를 unblock할 Abort와,
	// join할 수 없는 provider를 fail-stop할 외부 go test timeout을 설정해야 한다.
	CaseTimeout time.Duration
	// WaitTimeout은 case 안의 backend-state observation 한도를 정한다.
	WaitTimeout time.Duration
	// ResignTimeout은 abort containment 전에 수행하는 normal cleanup 한도를 정한다.
	ResignTimeout time.Duration
	_             struct{}
}

// AbortFunc cancellation grace period 동안 evaluator work가 join되는 경우를 포함해
// 모든 case timeout 뒤 provider work를 contain한다. RunWithConfig가 join할 수 있도록
// provider operation을 unblock해야 하며, containment가 진전되지 않을 때 외부 go test timeout이 최종 fail-stop이다.
type AbortFunc func(context.Context, leader.Options) error

// Config conformance run을 설정한다.
type Config struct {
	// Timing은 zero-valued conformance timing field를 override한다.
	Timing Timing
	// Abort evaluator가 grace 동안 join되거나 provider hard stop이 필요한 경우 모두에서
	// root cancellation 뒤 timeout case를 contain한다.
	Abort AbortFunc
	_     struct{}
}

func normalizeConfig(config Config) (Config, error) {
	timing := config.Timing
	if timing.Lease == 0 {
		timing.Lease = 300 * time.Millisecond
	}
	if timing.RenewInterval == 0 {
		timing.RenewInterval = 50 * time.Millisecond
	}
	if timing.CaseTimeout == 0 {
		timing.CaseTimeout = 5 * time.Second
	}
	if timing.WaitTimeout == 0 {
		timing.WaitTimeout = 2 * time.Second
	}
	if timing.ResignTimeout == 0 {
		timing.ResignTimeout = 250 * time.Millisecond
	}
	if timing.Lease < 0 || timing.RenewInterval < 0 || timing.CaseTimeout < 0 ||
		timing.WaitTimeout < 0 || timing.ResignTimeout < 0 {
		return Config{}, errors.New("leadertest: timing durations must be positive")
	}
	if timing.RenewInterval >= timing.Lease {
		return Config{}, errors.New("leadertest: renewal interval must be shorter than lease")
	}

	joinGrace := min(timing.ResignTimeout, timing.CaseTimeout/10)
	abortBudget := min(timing.ResignTimeout, time.Second)
	fits := func(first time.Duration) bool {
		return first < timing.CaseTimeout &&
			joinGrace < timing.CaseTimeout-first &&
			abortBudget < timing.CaseTimeout-first-joinGrace
	}
	if !fits(timing.WaitTimeout) || !fits(timing.ResignTimeout) {
		return Config{}, errors.New("leadertest: timing cannot contain a timed out case")
	}

	config.Timing = timing
	return config, nil
}
