package ratelimittest

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
)

// Config는 struct 공개 타입이며 rate-limit conformance harness의 quota/result ownership 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Config struct {
	RatePerSecond float64
	Burst         int64
	IdleTTL       time.Duration
}

// Result는 struct 공개 타입이며 rate-limit conformance harness의 quota/result ownership 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Result struct {
	Allowed    bool
	Requested  int64
	Remaining  int64
	RetryAfter time.Duration
	ResetAfter time.Duration
}

// AllowFunc는 func 공개 타입이며 rate-limit conformance harness의 quota/result ownership 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type AllowFunc func(context.Context, string, int64) (Result, error)

// Factory는 func 공개 타입이며 rate-limit conformance harness의 quota/result ownership 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Factory func(testing.TB, Config) (AllowFunc, error)

// Phase는 string 공개 타입이며 rate-limit conformance harness의 quota/result ownership 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Phase string

const (
	// PhaseBeforeLinearize pauses before debit dispatch.
	PhaseBeforeLinearize Phase = "before-linearize"
	// PhaseAfterLinearize pauses after the debit commits.
	PhaseAfterLinearize Phase = "after-linearize"
)

// Gate는 interface 공개 타입이며 rate-limit conformance harness의 quota/result ownership 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Gate interface {
	AwaitStarted(context.Context) error
	Resume()
}

// Control는 interface 공개 타입이며 rate-limit conformance harness의 quota/result ownership 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Control interface {
	GateNext(context.Context, string, Phase) (Gate, error)
	FailNext(context.Context, string, error) error
	OperationCount(string) int64
}

// ErrorClassifier는 func 공개 타입이며 rate-limit conformance harness의 quota/result ownership 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type ErrorClassifier func(error) bool

// Harness는 struct 공개 타입이며 rate-limit conformance harness의 quota/result ownership 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Harness struct {
	New             Factory
	Control         Control
	IsProviderError ErrorClassifier
}

var errInvalidInput = errors.New("ratelimittest: invalid input")

func validateConfig(config Config) error {
	if config.RatePerSecond <= 0 || math.IsNaN(config.RatePerSecond) || math.IsInf(config.RatePerSecond, 0) || config.Burst <= 0 || config.IdleTTL < 0 {
		return errInvalidInput
	}
	if config.IdleTTL > 0 && config.IdleTTL < fullRefill(config) {
		return errInvalidInput
	}
	return nil
}

func validateKey(key string) error {
	if strings.TrimSpace(key) == "" {
		return errInvalidInput
	}
	return nil
}

func validateContext(ctx context.Context) error {
	if ctx == nil {
		return errInvalidInput
	}
	return ctx.Err()
}

func validateHarness(h Harness) error {
	if h.New == nil || h.Control == nil || h.IsProviderError == nil {
		return errors.New("ratelimittest: incomplete harness")
	}
	for _, err := range []error{nil, context.Canceled, errInvalidInput, errors.New("raw-cause")} {
		matched, panicValue := classifySafely(h.IsProviderError, err)
		if panicValue != nil || matched {
			return errors.New("ratelimittest: invalid provider error classifier")
		}
	}
	return nil
}

func validatePositiveClassifier(t *testing.T, h Harness) error {
	t.Helper()
	config := Config{RatePerSecond: 100, Burst: 10, IdleTTL: time.Second}
	allow, err := h.New(t, config)
	if err != nil || allow == nil {
		return errors.New("ratelimittest: classifier probe factory failed")
	}
	key := fmt.Sprintf("ratelimittest-classifier-%d", runnerID.Add(1))
	if err := h.Control.FailNext(context.Background(), key, errors.New("classifier-probe")); err != nil {
		return errors.New("ratelimittest: classifier probe injection failed")
	}
	_, err = allow(context.Background(), key, 1)
	if err == nil {
		return errors.New("ratelimittest: classifier probe returned nil error")
	}
	matched, panicValue := classifySafely(h.IsProviderError, err)
	if panicValue != nil || !matched {
		return errors.New("ratelimittest: classifier rejected provider error")
	}
	matched, panicValue = classifySafely(h.IsProviderError, fmt.Errorf("nested: %w", err))
	if panicValue != nil || !matched {
		return errors.New("ratelimittest: classifier rejected nested provider error")
	}
	return nil
}

func classifySafely(classifier ErrorClassifier, err error) (matched bool, panicValue any) {
	defer func() { panicValue = recover() }()
	return classifier(err), nil
}

func fullRefill(config Config) time.Duration {
	return time.Duration(math.Ceil(float64(config.Burst) / config.RatePerSecond * float64(time.Second)))
}

type fixtureError struct{ cause error }

func (*fixtureError) Error() string { return "ratelimittest allow failed" }
func (e *fixtureError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}
