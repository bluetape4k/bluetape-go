package mongoleader

import (
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

const defaultRetryDelay = 25 * time.Millisecond

type config struct {
	retryDelay time.Duration
	clock      func() time.Time
}

// Option는 leader backend election에서 설정값과 기본값 적용 방식을 설명한다.
type Option func(*config) error

// WithRetryDelay는 leader backend election에서 설정값과 기본값 적용 방식을 설명한다.
// 세부 조건은 backend별 lease, cleanup, retry 계약을 따른다.
func WithRetryDelay(delay time.Duration) Option {
	return func(cfg *config) error {
		if delay <= 0 {
			return errors.New("mongo leader retry delay must be positive")
		}
		cfg.retryDelay = delay
		return nil
	}
}

// WithClock는 leader backend election에서 설정값과 기본값 적용 방식을 설명한다.
//
// 이 주석은 backend lease, ownership, consistency, cancellation 조건을 설명한다.
// 세부 조건은 backend별 lease, cleanup, retry 계약을 따른다.
// 세부 조건은 backend별 lease, cleanup, retry 계약을 따른다.
func WithClock(clock func() time.Time) Option {
	return func(cfg *config) error {
		if clock == nil {
			return errors.New("mongo leader clock must not be nil")
		}
		cfg.clock = clock
		return nil
	}
}

func normalizeConfig(options []Option) (config, error) {
	cfg := config{
		retryDelay: defaultRetryDelay,
		clock:      time.Now,
	}
	for _, option := range options {
		if option == nil {
			return config{}, errors.New("mongo leader option must not be nil")
		}
		if err := option(&cfg); err != nil {
			return config{}, err
		}
	}
	return cfg, nil
}

func requireCollection(collection *mongo.Collection) error {
	if collection == nil {
		return errors.New("mongo leader collection must not be nil")
	}
	return nil
}
