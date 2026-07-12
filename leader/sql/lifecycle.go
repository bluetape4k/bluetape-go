package sqlleader

import (
	"context"
	"errors"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
)

var _ leader.Elector = (*Elector)(nil)

// Campaign waits until the elector acquires leadership or ctx ends.
func (e *Elector) Campaign(ctx context.Context) error {
	if ctx == nil {
		return leader.ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := e.beginCampaign(); err != nil {
		return err
	}
	defer e.endCampaign()

	backoff := newBackoff(e.token, e.opts.Lease)
	for {
		acquired, err := e.acquireAttempt(ctx)
		if err != nil {
			return err
		}
		if acquired {
			e.startRenewal()
			return nil
		}
		if err := backoff.wait(ctx); err != nil {
			return err
		}
	}
}

// Resign stops renewal and conditionally deletes this elector's owner token.
func (e *Elector) Resign(ctx context.Context) error {
	if ctx == nil {
		return leader.ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	generation, cancel, done, active := e.clearOwnership()
	if !active {
		return nil
	}
	if cancel != nil {
		cancel()
	}
	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if err := e.deleteOwner(ctx); err != nil {
		return errors.Join(err, leader.ErrCommitUnknown)
	}

	e.mu.Lock()
	if e.generation == generation && e.done == done {
		e.cleanup = false
		e.cancel = nil
		e.done = nil
	}
	e.mu.Unlock()
	return nil
}

// IsLeader reports whether this elector currently believes it owns the lease.
func (e *Elector) IsLeader() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.owned
}

// Leader returns the active owner token recorded in PostgreSQL.
func (e *Elector) Leader(ctx context.Context) (string, error) {
	if ctx == nil {
		return "", leader.ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return e.lookupOwner(ctx)
}

func (e *Elector) acquireAttempt(ctx context.Context) (bool, error) {
	attemptCtx, cancel := context.WithTimeout(ctx, e.attemptBudget())
	defer cancel()
	return e.tryAcquire(attemptCtx)
}

func (e *Elector) attemptBudget() time.Duration {
	return max(100*time.Millisecond, min(e.opts.RenewInterval, time.Second))
}

func (e *Elector) beginCampaign() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cleanup {
		return leader.ErrCleanupPending
	}
	if e.owned {
		return leader.ErrAlreadyLeader
	}
	if e.campaigning {
		return leader.ErrCampaignInProgress
	}
	e.campaigning = true
	return nil
}

func (e *Elector) endCampaign() {
	e.mu.Lock()
	e.campaigning = false
	e.mu.Unlock()
}

func (e *Elector) startRenewal() {
	renewCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	e.mu.Lock()
	e.generation++
	generation := e.generation
	e.owned = true
	e.cleanup = false
	e.cancel = cancel
	e.done = done
	e.mu.Unlock()

	go e.renewLoop(renewCtx, generation, done)
}

func (e *Elector) renewLoop(ctx context.Context, generation uint64, done chan struct{}) {
	defer close(done)
	ticker := time.NewTicker(e.opts.RenewInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			renewCtx, cancel := context.WithTimeout(ctx, e.opts.RenewInterval)
			ok, err := e.renew(renewCtx)
			cancel()
			if err == nil && ok {
				err = e.runTestHook("renew", "after")
			}
			if err != nil || !ok {
				e.clearOwnershipAfterLoss(generation, done, err != nil)
				return
			}
		}
	}
}

func (e *Elector) clearOwnership() (uint64, context.CancelFunc, chan struct{}, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.owned && !e.cleanup {
		return e.generation, nil, nil, false
	}
	generation := e.generation
	cancel := e.cancel
	done := e.done
	e.owned = false
	e.cleanup = true
	return generation, cancel, done, true
}

func (e *Elector) clearOwnershipAfterLoss(generation uint64, done chan struct{}, cleanup bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.generation != generation || e.done != done {
		return
	}
	e.owned = false
	e.cleanup = cleanup
	e.cancel = nil
	e.done = nil
}

func (e *Elector) runTestHook(operation, phase string) error {
	if e.testHook == nil {
		return nil
	}
	return e.testHook(operation, phase)
}
