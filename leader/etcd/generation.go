package etcdleader

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

type generation struct {
	id      uint64
	ctx     context.Context
	cancel  context.CancelFunc
	leaseID clientv3.LeaseID

	ttl            time.Duration
	published      bool
	ops            etcdOps
	testHook       func(operation, phase string) error
	session        *concurrency.Session
	sessionTracked bool
	election       *concurrency.Election
	key            string
	createRev      int64
	// proclaimRev 값은 leader backend election 동작의 세부 조건을 설명한다.
	proclaimRev      int64
	monitorDone      chan struct{}
	shutdownOnce     sync.Once
	shutdownDone     chan struct{}
	shutdownErr      error
	cleanupMu        sync.Mutex
	monitorPublished atomic.Bool
	monitorFinished  atomic.Bool
}

func (g *generation) runTestHook(operation, phase string) error {
	if g == nil || g.testHook == nil {
		return nil
	}
	return g.testHook(operation, phase)
}

func (g *generation) publishMonitor() {
	if g == nil || g.monitorFinished.Load() {
		return
	}
	if g.monitorPublished.CompareAndSwap(false, true) {
		publishedEtcdMonitors.Add(1)
	}
	if g.monitorFinished.Load() && g.monitorPublished.CompareAndSwap(true, false) {
		publishedEtcdMonitors.Add(-1)
	}
}

func (g *generation) finishMonitor() {
	if g == nil {
		return
	}
	g.monitorFinished.Store(true)
	if g.monitorPublished.CompareAndSwap(true, false) {
		publishedEtcdMonitors.Add(-1)
	}
}

func (e *Elector) publishGenerationTTL(generation *generation, seconds int64) error {
	ttl, err := ttlDuration(seconds)
	if err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if generation == nil || e.current != generation {
		return errors.New("etcd leader generation is no longer active")
	}
	generation.ttl = ttl
	generation.published = true
	e.lastTTL = ttl
	return nil
}

func (g *generation) shutdown(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	g.shutdownOnce.Do(func() {
		defer close(g.shutdownDone)
		defer func() {
			if g.sessionTracked {
				g.sessionTracked = false
				liveEtcdSessions.Add(-1)
			}
		}()
		if g.cancel != nil {
			g.cancel()
		}
		if g.session == nil {
			return
		}
		if g.ops.orphanSession == nil || g.ops.sessionDone == nil {
			g.shutdownErr = errors.New("etcd leader session shutdown operations are unavailable")
			return
		}

		g.shutdownErr = g.ops.orphanSession(g.session)
		done := g.ops.sessionDone(g.session)
		if done == nil {
			g.shutdownErr = errors.Join(g.shutdownErr, errors.New("etcd leader session Done is nil"))
			return
		}
		select {
		case <-done:
		case <-ctx.Done():
			g.shutdownErr = errors.Join(g.shutdownErr, ctx.Err())
		}
	})
	<-g.shutdownDone
	return g.shutdownErr
}

func waitForMonitor(ctx context.Context, g *generation) error {
	if g == nil || g.monitorDone == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-g.monitorDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
