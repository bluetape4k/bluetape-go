package leadertest

import (
	"context"
	"errors"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
)

// Timing configures the time boundaries used by provider conformance cases.
type Timing struct {
	// Lease configures the case lease duration.
	Lease time.Duration
	// RenewInterval configures the case renewal cadence.
	RenewInterval time.Duration
	// CaseTimeout bounds evaluator work before cancellation and containment
	// starts. It does not bound the final provider join; callers must configure
	// Abort to unblock provider work and an outer go test timeout to fail-stop a
	// provider that cannot be joined.
	CaseTimeout time.Duration
	// WaitTimeout bounds backend-state observation within a case.
	WaitTimeout time.Duration
	// ResignTimeout bounds normal cleanup before abort containment.
	ResignTimeout time.Duration
	_             struct{}
}

// AbortFunc contains provider work after every case timeout, including when
// evaluator work joins during the cancellation grace period. It must unblock
// provider operations so RunWithConfig can join them; the outer go test timeout
// is the final fail-stop when containment cannot make progress.
type AbortFunc func(context.Context, leader.Options) error

// Config configures a conformance run.
type Config struct {
	// Timing overrides zero-valued conformance timing fields.
	Timing Timing
	// Abort contains every timed-out case after root cancellation, whether the
	// evaluator joins during grace or requires a provider hard stop.
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
