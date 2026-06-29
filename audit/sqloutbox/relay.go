package sqloutbox

import (
	"context"
	"fmt"
	"time"

	"github.com/bluetape4k/bluetape-go/sqlkit"
)

const (
	defaultClaimLimit  = 10
	defaultMaxAttempts = 3
	defaultRetryDelay  = time.Second
	defaultIdleDelay   = 100 * time.Millisecond
)

// Publisher publishes one claimed outbox record.
type Publisher interface {
	Publish(context.Context, Record) error
}

// RelayOptions configures outbox relay behavior.
type RelayOptions struct {
	ClaimLimit  int
	MaxAttempts int
	RetryDelay  time.Duration
	IdleDelay   time.Duration
	Now         func() time.Time
}

// RelayResult summarizes one RunOnce batch.
type RelayResult struct {
	Claimed      int
	Published    int
	Failed       int
	DeadLettered int
}

// Relay claims pending records and sends them through a Publisher.
type Relay struct {
	store      *Store
	publisher  Publisher
	claimLimit int
	maxAttempt int
	retryDelay time.Duration
	idleDelay  time.Duration
	now        func() time.Time
}

// NewRelay creates a relay over a Store and Publisher.
func NewRelay(store *Store, publisher Publisher, options RelayOptions) (*Relay, error) {
	if store == nil {
		return nil, fmt.Errorf("%w: store must not be nil", ErrInvalidArgument)
	}
	if publisher == nil {
		return nil, fmt.Errorf("%w: publisher must not be nil", ErrInvalidArgument)
	}

	claimLimit := options.ClaimLimit
	if claimLimit == 0 {
		claimLimit = defaultClaimLimit
	}
	if claimLimit < 0 {
		return nil, fmt.Errorf("%w: claim limit must be positive", ErrInvalidArgument)
	}
	maxAttempts := options.MaxAttempts
	if maxAttempts == 0 {
		maxAttempts = defaultMaxAttempts
	}
	if maxAttempts < 0 {
		return nil, fmt.Errorf("%w: max attempts must be positive", ErrInvalidArgument)
	}
	retryDelay := options.RetryDelay
	if retryDelay == 0 {
		retryDelay = defaultRetryDelay
	}
	if retryDelay < 0 {
		return nil, fmt.Errorf("%w: retry delay must not be negative", ErrInvalidArgument)
	}
	idleDelay := options.IdleDelay
	if idleDelay == 0 {
		idleDelay = defaultIdleDelay
	}
	if idleDelay < 0 {
		return nil, fmt.Errorf("%w: idle delay must not be negative", ErrInvalidArgument)
	}
	now := options.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}

	return &Relay{
		store:      store,
		publisher:  publisher,
		claimLimit: claimLimit,
		maxAttempt: maxAttempts,
		retryDelay: retryDelay,
		idleDelay:  idleDelay,
		now:        now,
	}, nil
}

// RunOnce claims one batch and records publish success or failure state.
func (r *Relay) RunOnce(ctx context.Context, db sqlkit.Session) (RelayResult, error) {
	if r == nil {
		return RelayResult{}, fmt.Errorf("%w: relay must not be nil", ErrInvalidArgument)
	}
	if db == nil {
		return RelayResult{}, fmt.Errorf("%w: session must not be nil", ErrInvalidArgument)
	}
	now := r.now()
	records, err := r.store.Claim(ctx, db, ClaimOptions{Limit: r.claimLimit, Now: now})
	if err != nil {
		return RelayResult{}, err
	}

	result := RelayResult{Claimed: len(records)}
	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if err := r.publisher.Publish(ctx, record); err != nil {
			deadLetter := record.Attempts >= r.maxAttempt
			if markErr := r.store.MarkFailed(ctx, db, Failure{
				ID:          record.ID,
				Attempt:     record.Attempts,
				Err:         err,
				RetryAt:     now.Add(r.retryDelay),
				MaxAttempts: r.maxAttempt,
				Now:         now,
			}); markErr != nil {
				return result, markErr
			}
			if deadLetter {
				result.DeadLettered++
			} else {
				result.Failed++
			}
			continue
		}
		if err := r.store.MarkPublished(ctx, db, record); err != nil {
			return result, err
		}
		result.Published++
	}
	return result, nil
}

// Run keeps publishing batches until the context is cancelled or an internal
// store operation fails.
func (r *Relay) Run(ctx context.Context, db sqlkit.Session) error {
	if ctx == nil {
		ctx = context.Background()
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		result, err := r.RunOnce(ctx, db)
		if err != nil {
			return err
		}
		if result.Claimed > 0 {
			continue
		}
		timer := time.NewTimer(r.idleDelay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}
