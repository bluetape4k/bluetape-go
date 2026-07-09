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

// Option configures a MongoDB elector.
type Option func(*config) error

// WithRetryDelay configures how long Campaign waits between failed acquisition
// attempts while another live owner exists.
func WithRetryDelay(delay time.Duration) Option {
	return func(cfg *config) error {
		if delay <= 0 {
			return errors.New("mongo leader retry delay must be positive")
		}
		cfg.retryDelay = delay
		return nil
	}
}

// WithClock configures the clock used for lease_until timestamps.
//
// It exists primarily for deterministic tests. Production callers should keep
// clocks synchronized across contenders and choose lease durations larger than
// expected skew plus MongoDB operation latency.
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
