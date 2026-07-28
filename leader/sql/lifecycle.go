package sqlleader

import (
	"context"
	"errors"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
)

var _ leader.Elector = (*Elector)(nil)

// Campaign은 elector가 leadership을 획득하거나 ctx가 끝날 때까지 기다린다.
//
// 충돌하는 local state에는 leader.ErrAlreadyLeader, leader.ErrCampaignInProgress,
// leader.ErrCleanupPending을 반환한다. reconciliation이 ctx 만료 후 ownership을 확인할 수 있으므로
// 호출자는 elector를 유지하고, protected work 전에 ctx.Err를 확인하며, bounded cleanup을 수행해야 한다.
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

// Resign은 renewal을 중지하고 이 elector의 owner token을 조건부로 삭제한다.
//
// delete 결과가 불명확하면 leader.ErrCommitUnknown을 반환한다. 같은 elector에서 fresh bounded context로
// Resign을 재시도한 뒤, 최종 fallback으로 full-lease expiry를 사용한다.
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
	resolved := false
	defer func() { e.finishResign(generation, resolved) }()
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
	if err := e.runTestHook("resign", "before"); err != nil {
		return err
	}
	if err := e.deleteOwner(ctx); err != nil {
		return errors.Join(err, leader.ErrCommitUnknown)
	}
	if err := e.runTestHook("resign", "after"); err != nil {
		return errors.Join(err, leader.ErrCommitUnknown)
	}
	resolved = true
	return nil
}

// IsLeader 이 elector가 현재 lease를 소유한다고 판단하는지 알려준다.
func (e *Elector) IsLeader() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.owned
}

// Leader PostgreSQL에 기록된 active owner token을 반환한다.
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
	if err := e.runTestHook("campaign", "before"); err != nil {
		return false, err
	}
	if err := e.runTestHook("campaign", "attempt"); err != nil {
		return false, err
	}
	attemptCtx, cancel := context.WithTimeout(ctx, e.attemptBudget())
	defer cancel()
	acquired, operationErr := e.tryAcquire(attemptCtx)
	internalTimeout := attemptCtx.Err() != nil && ctx.Err() == nil
	if operationErr == nil && acquired {
		operationErr = e.runTestHook("campaign", "after")
	}
	if operationErr == nil && attemptCtx.Err() != nil {
		operationErr = attemptCtx.Err()
		internalTimeout = ctx.Err() == nil
	}
	if operationErr == nil {
		return acquired, nil
	}

	reconcileCtx, reconcileCancel := context.WithTimeout(context.Background(), e.attemptBudget())
	defer reconcileCancel()
	var owner string
	probeErr := e.runTestHook("campaign", "reconcile")
	if probeErr == nil {
		owner, probeErr = e.lookupOwner(reconcileCtx)
	}
	switch {
	case probeErr == nil && owner == e.token:
		return true, nil
	case probeErr == nil && internalTimeout:
		return false, nil
	case probeErr == nil:
		return false, operationErr
	default:
		e.markCleanupPending()
		return false, errors.Join(operationErr, leader.ErrCommitUnknown)
	}
}

func (e *Elector) attemptBudget() time.Duration {
	return min(min(e.opts.RenewInterval, e.opts.Lease/4), time.Second)
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
			if err := e.runTestHook("renew", "before"); err != nil {
				e.clearOwnershipAfterLoss(generation, done, true)
				return
			}
			renewCtx, cancel := context.WithTimeout(ctx, e.renewalBudget())
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

func (e *Elector) renewalBudget() time.Duration {
	remainingMargin := e.opts.Lease - e.opts.RenewInterval
	return min(e.opts.RenewInterval, remainingMargin/2)
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
	e.resigning++
	return generation, cancel, done, true
}

func (e *Elector) finishResign(generation uint64, resolved bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.generation != generation || e.resigning == 0 {
		return
	}
	if resolved {
		e.resolved = true
	}
	e.resigning--
	if e.resigning == 0 && e.resolved {
		e.cleanup = false
		e.resolved = false
		e.cancel = nil
		e.done = nil
	}
}

func (e *Elector) clearOwnershipAfterLoss(generation uint64, done chan struct{}, cleanup bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.generation != generation || e.done != done {
		return
	}
	e.owned = false
	e.cleanup = cleanup || e.resigning > 0
	if e.resigning == 0 {
		e.cancel = nil
		e.done = nil
	}
}

func (e *Elector) markCleanupPending() {
	e.mu.Lock()
	e.owned = false
	e.cleanup = true
	e.mu.Unlock()
}

func (e *Elector) runTestHook(operation, phase string) error {
	if e.testHook == nil {
		return nil
	}
	if err := e.testHook(operation, phase); err != nil {
		return leader.NewOperationError("postgres", operation, err)
	}
	return nil
}
