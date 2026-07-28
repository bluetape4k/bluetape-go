package etcdleader

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

type electionSnapshot struct {
	key       string
	createRev int64
	headerRev int64
}

type electorTicker interface {
	C() <-chan time.Time
	Stop()
}

type realElectorTicker struct {
	*time.Ticker
}

func (ticker realElectorTicker) C() <-chan time.Time { return ticker.Ticker.C }

type etcdOps struct {
	grant            func(context.Context, int64) (*clientv3.LeaseGrantResponse, error)
	newSession       func(context.Context, clientv3.LeaseID) (*concurrency.Session, error)
	campaign         func(context.Context, *concurrency.Election, string) error
	proclaim         func(context.Context, *concurrency.Election, string) error
	resign           func(context.Context, *concurrency.Election) error
	snapshotElection func(*concurrency.Election) (electionSnapshot, error)
	watch            func(context.Context, string, ...clientv3.OpOption) clientv3.WatchChan
	revoke           func(context.Context, clientv3.LeaseID) (*clientv3.LeaseRevokeResponse, error)
	get              func(context.Context, string, ...clientv3.OpOption) (*clientv3.GetResponse, error)
	sessionDone      func(*concurrency.Session) <-chan struct{}
	orphanSession    func(*concurrency.Session) error
	// 이 주석은 leader backend election의 backend 요구사항, cancellation, timeout, 오류 처리 세부사항을 설명한다.
	newTicker func(time.Duration) electorTicker
}

func productionEtcdOps(client *clientv3.Client) etcdOps {
	return etcdOps{
		grant: func(ctx context.Context, ttl int64) (*clientv3.LeaseGrantResponse, error) {
			return client.Grant(ctx, ttl)
		},
		newSession: func(ctx context.Context, leaseID clientv3.LeaseID) (*concurrency.Session, error) {
			return concurrency.NewSession(
				client,
				concurrency.WithContext(ctx),
				concurrency.WithLease(leaseID),
			)
		},
		campaign: func(ctx context.Context, election *concurrency.Election, token string) error {
			return election.Campaign(ctx, token)
		},
		proclaim: func(ctx context.Context, election *concurrency.Election, token string) error {
			return election.Proclaim(ctx, token)
		},
		resign: func(ctx context.Context, election *concurrency.Election) error {
			return election.Resign(ctx)
		},
		snapshotElection: snapshotOfficialElection,
		watch: func(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan {
			return client.Watch(ctx, key, opts...)
		},
		revoke: func(ctx context.Context, leaseID clientv3.LeaseID) (*clientv3.LeaseRevokeResponse, error) {
			return client.Revoke(ctx, leaseID)
		},
		get: func(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
			return client.Get(ctx, key, opts...)
		},
		sessionDone: func(session *concurrency.Session) <-chan struct{} { return session.Done() },
		orphanSession: func(session *concurrency.Session) error {
			session.Orphan()
			return nil
		},
		newTicker: func(interval time.Duration) electorTicker {
			return realElectorTicker{Ticker: time.NewTicker(interval)}
		},
	}
}

func snapshotOfficialElection(election *concurrency.Election) (electionSnapshot, error) {
	if election == nil {
		return electionSnapshot{}, errors.New("etcd leader election is nil")
	}
	header := election.Header()
	if header == nil {
		return electionSnapshot{}, errors.New("etcd leader election header is nil")
	}
	return electionSnapshot{
		key:       election.Key(),
		createRev: election.Rev(),
		headerRev: header.Revision,
	}, nil
}

// Campaign는 leader backend election에서 실행, cancellation, cleanup 계약을 설명한다.
func (e *Elector) Campaign(ctx context.Context) error {
	if ctx == nil {
		return leader.ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if e.client == nil || e.ops.grant == nil {
		return leader.NewOperationError("etcd", "campaign", errors.New("etcd leader Elector is not initialized"))
	}

	generation, err := e.beginCampaign()
	if err != nil {
		return err
	}
	defer e.endCampaign()

	caller := newCallerCancellation(ctx, generation.cancel)
	if err := generation.ctx.Err(); err != nil {
		caller.detach()
		e.clearGeneration(generation)
		return ctx.Err()
	}

	grant, err := generation.ops.grant(generation.ctx, e.requestedTTL)
	if err != nil {
		return e.failCampaign(generation, caller, err)
	}
	if grant == nil || grant.ID == clientv3.NoLease {
		return e.failCampaign(generation, caller, errors.New("etcd leader Grant returned no lease"))
	}
	generation.leaseID = grant.ID
	generation.key = e.paths.root + fmt.Sprintf("%x", grant.ID)
	grantedTTL, err := ttlDuration(grant.TTL)
	if err != nil {
		return e.failCampaign(generation, caller, err)
	}
	generation.ttl = grantedTTL

	session, err := generation.ops.newSession(generation.ctx, generation.leaseID)
	if err != nil {
		return e.failCampaign(generation, caller, err)
	}
	if session == nil {
		return e.failCampaign(generation, caller, errors.New("etcd leader NewSession returned nil"))
	}
	generation.session = session
	generation.sessionTracked = true
	liveEtcdSessions.Add(1)
	generation.election = concurrency.NewElection(session, e.paths.base)

	if err := generation.runTestHook("campaign", "before"); err != nil {
		return e.failCampaign(generation, caller, err)
	}
	if err := generation.ops.campaign(generation.ctx, generation.election, e.token); err != nil {
		return e.failCampaign(generation, caller, err)
	}

	proclaimCtx, cancelProclaim := context.WithTimeout(generation.ctx, e.operationBudget(generation.ttl))
	err = generation.ops.proclaim(proclaimCtx, generation.election, e.token)
	cancelProclaim()
	if err != nil {
		return e.failCampaign(generation, caller, err)
	}

	snapshot, err := generation.ops.snapshotElection(generation.election)
	if err != nil {
		return e.failCampaign(generation, caller, err)
	}
	if err := validateElectionSnapshot(snapshot, generation.key); err != nil {
		return e.failCampaign(generation, caller, err)
	}
	generation.key = snapshot.key
	generation.createRev = snapshot.createRev
	generation.proclaimRev = snapshot.headerRev

	watch := generation.ops.watch(
		generation.ctx,
		generation.key,
		clientv3.WithRev(snapshot.headerRev+1),
		clientv3.WithCreatedNotify(),
	)
	monitor := startGenerationMonitor(e, generation, watch, e.token)
	if err := monitor.waitCreated(generation.ctx); err != nil {
		return e.failCampaign(generation, caller, err)
	}

	e.mu.Lock()
	publicationErr := e.publicationErrorLocked(generation, monitor)
	if publicationErr == nil && caller.detach() {
		generation.published = true
		e.lastTTL = generation.ttl
		monitor.publish()
	} else if publicationErr == nil {
		publicationErr = context.Canceled
	}
	e.mu.Unlock()
	if publicationErr != nil {
		return e.failCampaign(generation, caller, publicationErr)
	}
	if err := generation.runTestHook("campaign", "after"); err != nil {
		return errors.Join(leader.NewOperationError("etcd", "campaign", err), leader.ErrCommitUnknown)
	}
	return nil
}

func (e *Elector) beginCampaign() (*generation, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.campaigning {
		return nil, leader.ErrCampaignInProgress
	}
	if e.current != nil {
		if e.current.published {
			return nil, leader.ErrAlreadyLeader
		}
		return nil, leader.ErrCleanupPending
	}

	ctx, cancel := context.WithCancel(context.Background())
	e.nextGeneration++
	generation := &generation{
		id:           e.nextGeneration,
		ctx:          ctx,
		cancel:       cancel,
		ops:          e.ops,
		testHook:     e.testHook,
		shutdownDone: make(chan struct{}),
	}
	e.campaigning = true
	e.current = generation
	return generation, nil
}

func (e *Elector) endCampaign() {
	e.mu.Lock()
	e.campaigning = false
	e.mu.Unlock()
}

func (e *Elector) publicationErrorLocked(generation *generation, monitor *generationMonitor) error {
	if e.current != generation {
		return errors.New("etcd leader generation is no longer current")
	}
	if err := generation.ctx.Err(); err != nil {
		return err
	}
	select {
	case err := <-monitor.terminal:
		if err == nil {
			return errors.New("etcd leader monitor terminated")
		}
		return err
	default:
		return nil
	}
}

func validateElectionSnapshot(snapshot electionSnapshot, expectedKey string) error {
	if snapshot.key == "" || snapshot.key != expectedKey {
		return errors.New("etcd leader election key mismatch")
	}
	if snapshot.createRev <= 0 || snapshot.headerRev <= 0 || snapshot.headerRev < snapshot.createRev {
		return errors.New("etcd leader election revision is invalid")
	}
	if snapshot.headerRev == math.MaxInt64 {
		return errors.New("etcd leader election revision overflows watch start")
	}
	return nil
}

func (e *Elector) failCampaign(generation *generation, caller *callerCancellation, cause error) error {
	caller.detach()
	cause = errors.Join(cause, caller.err())
	shutdownErr := generation.shutdown(context.Background())
	monitorErr := waitForMonitor(context.Background(), generation)
	cause = errors.Join(cause, shutdownErr, monitorErr)

	proved, cleanupErr := e.cleanupFailedGeneration(generation)
	cause = errors.Join(cause, cleanupErr)
	operationErr := leader.NewOperationError("etcd", "campaign", cause)
	if !proved {
		return errors.Join(operationErr, leader.ErrCommitUnknown)
	}
	e.clearGeneration(generation)
	return operationErr
}

func (e *Elector) cleanupFailedGeneration(generation *generation) (bool, error) {
	if generation.leaseID == clientv3.NoLease {
		return true, nil
	}
	cleanupCtx, cancel := context.WithTimeout(context.Background(), e.operationBudget(generation.ttl))
	defer cancel()

	_, revokeErr := generation.ops.revoke(cleanupCtx, generation.leaseID)
	if revokeErr == nil {
		return true, nil
	}
	if generation.key == "" {
		return false, revokeErr
	}
	response, getErr := generation.ops.get(cleanupCtx, generation.key)
	if getErr != nil {
		return false, errors.Join(revokeErr, getErr)
	}
	if generationAbsentOrReplaced(response, generation, e.token) {
		return true, revokeErr
	}
	return false, revokeErr
}

func generationAbsentOrReplaced(response *clientv3.GetResponse, generation *generation, token string) bool {
	if response == nil {
		return false
	}
	if len(response.Kvs) == 0 {
		return true
	}
	keyValue := response.Kvs[0]
	if keyValue == nil || string(keyValue.Key) != generation.key || string(keyValue.Value) != token ||
		clientv3.LeaseID(keyValue.Lease) != generation.leaseID {
		return true
	}
	return generation.createRev > 0 && keyValue.CreateRevision != generation.createRev
}

func (e *Elector) clearGeneration(generation *generation) {
	e.mu.Lock()
	if e.current == generation {
		e.current = nil
	}
	e.mu.Unlock()
}

func (e *Elector) operationBudget(ttl time.Duration) time.Duration {
	budget := time.Second
	if e.opts.RenewInterval > 0 && e.opts.RenewInterval < budget {
		budget = e.opts.RenewInterval
	}
	if ttl > 0 && ttl/4 < budget {
		budget = ttl / 4
	}
	if budget <= 0 {
		return time.Millisecond
	}
	return budget
}

type callerCancellation struct {
	ctx  context.Context
	stop func() bool
	done <-chan struct{}
}

func newCallerCancellation(ctx context.Context, cancel context.CancelFunc) *callerCancellation {
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		cancel()
		close(done)
	})
	return &callerCancellation{ctx: ctx, stop: stop, done: done}
}

func (c *callerCancellation) detach() bool {
	if c.stop() {
		return true
	}
	<-c.done
	return false
}

func (c *callerCancellation) err() error {
	if c == nil || c.ctx == nil {
		return nil
	}
	return c.ctx.Err()
}
