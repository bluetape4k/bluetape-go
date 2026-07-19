package etcdleader

import (
	"context"
	"errors"

	"github.com/bluetape4k/bluetape-go/leader"
	"go.etcd.io/etcd/client/v3/concurrency"
)

// Resign releases this elector's current generation.
//
// It is idempotent when no generation exists. Remote cleanup inventory is
// retained until lease revocation or an exact-key lookup proves ownership is
// absent or replaced. An unpublished in-progress Campaign owns its cleanup;
// concurrent Resign returns nil without canceling or inspecting that generation.
func (e *Elector) Resign(ctx context.Context) error {
	if ctx == nil {
		return leader.ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	e.mu.Lock()
	generation := e.current
	if generation == nil {
		e.mu.Unlock()
		return nil
	}
	if e.campaigning && !generation.published {
		e.mu.Unlock()
		return nil
	}
	generation.published = false
	e.mu.Unlock()

	generation.cleanupMu.Lock()
	defer generation.cleanupMu.Unlock()
	e.mu.RLock()
	stillCurrent := e.current == generation
	e.mu.RUnlock()
	if !stillCurrent {
		return nil
	}

	shutdownErr := generation.shutdown(context.Background())
	monitorErr := waitForMonitor(context.Background(), generation)
	var resignErr error
	if generation.createRev > 0 && generation.session != nil && generation.ops.resign != nil {
		resignCtx, cancel := context.WithTimeout(ctx, e.operationBudget(generation.ttl))
		resumed := concurrency.ResumeElection(
			generation.session,
			e.paths.root,
			generation.key,
			generation.createRev,
		)
		resignErr = generation.ops.resign(resignCtx, resumed)
		cancel()
		if resignErr == nil {
			if hookErr := generation.runTestHook("resign", "after"); hookErr != nil {
				cause := errors.Join(hookErr, shutdownErr, monitorErr)
				return errors.Join(leader.NewOperationError("etcd", "resign", cause), leader.ErrCommitUnknown)
			}
		}
	}

	proved, cleanupErr := e.cleanupFailedGeneration(generation)
	cause := errors.Join(resignErr, shutdownErr, monitorErr)
	if !proved {
		cause = errors.Join(cause, cleanupErr)
		return errors.Join(leader.NewOperationError("etcd", "resign", cause), leader.ErrCommitUnknown)
	}
	e.clearGeneration(generation)
	if cause != nil {
		return leader.NewOperationError("etcd", "resign", cause)
	}
	return nil
}
