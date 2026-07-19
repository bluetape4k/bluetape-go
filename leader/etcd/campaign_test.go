package etcdleader

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

func TestCampaignPublishesOnlyAfterWatchCreated(t *testing.T) {
	elector, fake := newFakeElector(t)
	fake.send(clientv3.WatchResponse{Created: true})

	if err := elector.Campaign(context.Background()); err != nil {
		t.Fatal(err)
	}

	elector.mu.RLock()
	generation := elector.current
	elector.mu.RUnlock()
	if generation == nil || !generation.published {
		t.Fatal("Campaign returned without publishing the generation")
	}
	if got := elector.EffectiveTTL(); got != 3*time.Second {
		t.Fatalf("EffectiveTTL = %s, want 3s", got)
	}
	if fake.grantCalls.Load() != 1 || fake.campaignCalls.Load() != 1 ||
		fake.proclaimCalls.Load() != 1 || fake.watchCalls.Load() != 1 {
		t.Fatalf("operation counts grant=%d campaign=%d proclaim=%d watch=%d",
			fake.grantCalls.Load(), fake.campaignCalls.Load(),
			fake.proclaimCalls.Load(), fake.watchCalls.Load())
	}
	select {
	case <-fake.sessionDone:
		t.Fatal("successful Campaign orphaned its Session")
	default:
	}

	shutdownFakeGeneration(t, elector, generation)
}

func TestCampaignFailureStagesJoinAndRevoke(t *testing.T) {
	wantErr := errors.New("injected failure")
	tests := []struct {
		name       string
		configure  func(*fakeEtcdOps)
		wantOrphan bool
	}{
		{
			name: "grant",
			configure: func(fake *fakeEtcdOps) {
				fake.grantErr = wantErr
			},
		},
		{
			name: "session",
			configure: func(fake *fakeEtcdOps) {
				fake.sessionErr = wantErr
			},
		},
		{
			name: "invalid-granted-ttl",
			configure: func(fake *fakeEtcdOps) {
				fake.ttl = 0
			},
		},
		{
			name: "campaign",
			configure: func(fake *fakeEtcdOps) {
				fake.campaignErr = wantErr
			},
			wantOrphan: true,
		},
		{
			name: "proclaim",
			configure: func(fake *fakeEtcdOps) {
				fake.proclaimErr = wantErr
			},
			wantOrphan: true,
		},
		{
			name: "snapshot",
			configure: func(fake *fakeEtcdOps) {
				fake.snapshot = electionSnapshot{key: "wrong", createRev: 17, headerRev: 19}
			},
			wantOrphan: true,
		},
		{
			name: "watch-closed-before-created",
			configure: func(fake *fakeEtcdOps) {
				close(fake.watch)
			},
			wantOrphan: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			elector, fake := newFakeElector(t)
			tt.configure(fake)
			if tt.name != "watch-closed-before-created" {
				fake.send(clientv3.WatchResponse{Created: true})
			}

			err := elector.Campaign(context.Background())
			if err == nil {
				t.Fatal("Campaign succeeded")
			}
			var operationErr *leader.OperationError
			if !errors.As(err, &operationErr) {
				t.Fatalf("Campaign error = %T %v, want OperationError", err, err)
			}
			elector.mu.RLock()
			current := elector.current
			elector.mu.RUnlock()
			if current != nil {
				t.Fatal("resolved failure retained cleanup inventory")
			}
			if tt.name == "grant" {
				if fake.revokeCalls.Load() != 0 {
					t.Fatalf("Grant failure revoke calls = %d", fake.revokeCalls.Load())
				}
			} else if fake.revokeCalls.Load() != 1 {
				t.Fatalf("revoke calls = %d, want 1", fake.revokeCalls.Load())
			}
			if tt.wantOrphan {
				assertSessionJoined(t, fake)
			}
		})
	}
}

func TestCampaignCreatedThenOwnershipLossFailsAndJoins(t *testing.T) {
	for _, eventType := range []mvccpb.Event_EventType{mvccpb.PUT, mvccpb.DELETE} {
		t.Run(eventType.String(), func(t *testing.T) {
			elector, fake := newFakeElector(t)
			fake.send(clientv3.WatchResponse{Created: true})
			fake.send(clientv3.WatchResponse{Events: []*clientv3.Event{{
				Type: eventType,
				Kv: &mvccpb.KeyValue{
					Key:            []byte(fake.snapshot.key),
					Value:          []byte("other-owner"),
					CreateRevision: fake.snapshot.createRev,
					Lease:          int64(fake.leaseID),
				},
			}}})

			if err := elector.Campaign(context.Background()); err == nil {
				t.Fatalf("Campaign published after %s", eventType)
			}
			assertSessionJoined(t, fake)
			if fake.revokeCalls.Load() != 1 {
				t.Fatalf("revoke calls = %d, want 1", fake.revokeCalls.Load())
			}
		})
	}
}

func TestCampaignBoundsProclaim(t *testing.T) {
	elector, fake := newFakeElector(t)
	elector.opts.RenewInterval = 20 * time.Millisecond
	fake.proclaimFunc = func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}

	started := time.Now()
	if err := elector.Campaign(context.Background()); err == nil {
		t.Fatal("Campaign succeeded after Proclaim deadline")
	}
	if elapsed := time.Since(started); elapsed > 300*time.Millisecond {
		t.Fatalf("Proclaim was not bounded: %s", elapsed)
	}
	assertSessionJoined(t, fake)
}

func TestCampaignCreatedNotifyTimeoutJoinsMonitorAndSession(t *testing.T) {
	elector, fake := newFakeElector(t)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()

	result := make(chan error, 1)
	go func() { result <- elector.Campaign(ctx) }()
	fake.waitForWatch(t)
	elector.mu.RLock()
	generation := elector.current
	elector.mu.RUnlock()
	if generation == nil {
		t.Fatal("Campaign did not retain its in-progress generation")
	}
	if err := <-result; err == nil {
		t.Fatal("Campaign succeeded without watch Created acknowledgement")
	}
	assertSessionJoined(t, fake)
	select {
	case <-generation.monitorDone:
	default:
		t.Fatal("Campaign returned before monitor joined")
	}
}

func TestCampaignRetainsCleanupWhenRevokeAndProofFail(t *testing.T) {
	for _, tt := range []struct {
		name      string
		configure func(*fakeEtcdOps)
	}{
		{
			name: "lookup-error",
			configure: func(fake *fakeEtcdOps) {
				fake.getErr = errors.New("lookup unavailable")
			},
		},
		{
			name: "nil-lookup-response",
			configure: func(fake *fakeEtcdOps) {
				fake.getResponse = nil
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			elector, fake := newFakeElector(t)
			fake.campaignErr = errors.New("campaign response lost")
			fake.revokeErr = errors.New("revoke response lost")
			tt.configure(fake)

			err := elector.Campaign(context.Background())
			if !errors.Is(err, leader.ErrCommitUnknown) {
				t.Fatalf("Campaign error = %v, want ErrCommitUnknown", err)
			}
			elector.mu.RLock()
			current := elector.current
			elector.mu.RUnlock()
			if current == nil || current.published {
				t.Fatal("Campaign did not retain unresolved cleanup inventory")
			}
			assertSessionJoined(t, fake)
		})
	}
}

func TestCampaignRejectsConcurrentAndPublishedGeneration(t *testing.T) {
	elector, fake := newFakeElector(t)
	started := make(chan struct{})
	release := make(chan struct{})
	fake.campaignFunc = func(ctx context.Context) error {
		close(started)
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	fake.send(clientv3.WatchResponse{Created: true})

	result := make(chan error, 1)
	go func() { result <- elector.Campaign(context.Background()) }()
	<-started
	if err := elector.Campaign(context.Background()); !errors.Is(err, leader.ErrCampaignInProgress) {
		t.Fatalf("concurrent Campaign error = %v", err)
	}
	close(release)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	if err := elector.Campaign(context.Background()); !errors.Is(err, leader.ErrAlreadyLeader) {
		t.Fatalf("published Campaign error = %v", err)
	}

	elector.mu.RLock()
	generation := elector.current
	elector.mu.RUnlock()
	shutdownFakeGeneration(t, elector, generation)
}

func TestCampaignCopiesOperationsBeforeRemoteDispatch(t *testing.T) {
	elector, fake := newFakeElector(t)
	started := make(chan struct{})
	release := make(chan struct{})
	originalGrant := elector.ops.grant
	elector.ops.grant = func(ctx context.Context, ttl int64) (*clientv3.LeaseGrantResponse, error) {
		close(started)
		select {
		case <-release:
			return originalGrant(ctx, ttl)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	fake.send(clientv3.WatchResponse{Created: true})

	result := make(chan error, 1)
	go func() { result <- elector.Campaign(context.Background()) }()
	<-started
	elector.ops.campaign = func(context.Context, *concurrency.Election, string) error {
		return errors.New("mutated elector operations")
	}
	close(release)
	if err := <-result; err != nil {
		t.Fatalf("Campaign observed mutated operations: %v", err)
	}

	elector.mu.RLock()
	generation := elector.current
	elector.mu.RUnlock()
	shutdownFakeGeneration(t, elector, generation)
}

func TestSnapshotOfficialElectionRejectsMissingState(t *testing.T) {
	for _, election := range []*concurrency.Election{nil, {}} {
		if _, err := snapshotOfficialElection(election); err == nil {
			t.Fatalf("snapshotOfficialElection(%v) succeeded", election)
		}
	}
}

func TestPublicationAndCancellationHaveOneWinner(t *testing.T) {
	t.Run("cancellation", func(t *testing.T) {
		elector, fake := newFakeElector(t)
		ctx, cancel := context.WithCancel(context.Background())
		result := make(chan error, 1)
		go func() { result <- elector.Campaign(ctx) }()
		fake.waitForWatch(t)
		cancel()
		fake.send(clientv3.WatchResponse{Created: true})
		if err := <-result; err == nil {
			t.Fatal("Campaign published after cancellation won")
		}
		elector.mu.RLock()
		current := elector.current
		elector.mu.RUnlock()
		if current != nil {
			t.Fatal("cancellation winner retained resolved generation")
		}
		assertSessionJoined(t, fake)
	})

	t.Run("publication", func(t *testing.T) {
		elector, fake := newFakeElector(t)
		ctx, cancel := context.WithCancel(context.Background())
		fake.send(clientv3.WatchResponse{Created: true})
		if err := elector.Campaign(ctx); err != nil {
			t.Fatal(err)
		}
		cancel()
		elector.mu.RLock()
		generation := elector.current
		published := generation != nil && generation.published
		elector.mu.RUnlock()
		if !published {
			t.Fatal("caller cancellation remained attached after publication")
		}
		select {
		case <-fake.sessionDone:
			t.Fatal("caller cancellation orphaned a published Session")
		default:
		}
		shutdownFakeGeneration(t, elector, generation)
	})
}

type fakeEtcdOps struct {
	leaseID clientv3.LeaseID
	ttl     int64
	session *concurrency.Session

	grantErr    error
	sessionErr  error
	campaignErr error
	proclaimErr error
	revokeErr   error
	getErr      error
	getResponse *clientv3.GetResponse

	snapshot electionSnapshot
	watch    chan clientv3.WatchResponse
	ticker   *fakeElectorTicker

	sessionDone  chan struct{}
	sessionOnce  sync.Once
	campaignFunc func(context.Context) error
	proclaimFunc func(context.Context) error

	grantCalls    atomic.Int64
	campaignCalls atomic.Int64
	proclaimCalls atomic.Int64
	watchCalls    atomic.Int64
	revokeCalls   atomic.Int64
	orphanCalls   atomic.Int64
}

func newFakeElector(t *testing.T) (*Elector, *fakeEtcdOps) {
	t.Helper()
	elector, err := New(&clientv3.Client{}, testOptions())
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeEtcdOps{
		leaseID:     11,
		ttl:         3,
		session:     &concurrency.Session{},
		watch:       make(chan clientv3.WatchResponse, 8),
		sessionDone: make(chan struct{}),
		getResponse: &clientv3.GetResponse{},
		ticker:      newFakeElectorTicker(),
	}
	fake.snapshot = electionSnapshot{
		key:       elector.paths.root + fmt.Sprintf("%x", fake.leaseID),
		createRev: 17,
		headerRev: 19,
	}
	elector.ops = fake.ops()
	return elector, fake
}

func (fake *fakeEtcdOps) ops() etcdOps {
	return etcdOps{
		grant: func(context.Context, int64) (*clientv3.LeaseGrantResponse, error) {
			fake.grantCalls.Add(1)
			if fake.grantErr != nil {
				return nil, fake.grantErr
			}
			return &clientv3.LeaseGrantResponse{ID: fake.leaseID, TTL: fake.ttl}, nil
		},
		newSession: func(context.Context, clientv3.LeaseID) (*concurrency.Session, error) {
			if fake.sessionErr != nil {
				return nil, fake.sessionErr
			}
			return fake.session, nil
		},
		campaign: func(ctx context.Context, _ *concurrency.Election, _ string) error {
			fake.campaignCalls.Add(1)
			if fake.campaignFunc != nil {
				return fake.campaignFunc(ctx)
			}
			return fake.campaignErr
		},
		proclaim: func(ctx context.Context, _ *concurrency.Election, _ string) error {
			fake.proclaimCalls.Add(1)
			if fake.proclaimFunc != nil {
				return fake.proclaimFunc(ctx)
			}
			return fake.proclaimErr
		},
		snapshotElection: func(*concurrency.Election) (electionSnapshot, error) {
			return fake.snapshot, nil
		},
		watch: func(context.Context, string, ...clientv3.OpOption) clientv3.WatchChan {
			fake.watchCalls.Add(1)
			return fake.watch
		},
		revoke: func(context.Context, clientv3.LeaseID) (*clientv3.LeaseRevokeResponse, error) {
			fake.revokeCalls.Add(1)
			if fake.revokeErr != nil {
				return nil, fake.revokeErr
			}
			return &clientv3.LeaseRevokeResponse{}, nil
		},
		get: func(context.Context, string, ...clientv3.OpOption) (*clientv3.GetResponse, error) {
			if fake.getErr != nil {
				return nil, fake.getErr
			}
			return fake.getResponse, nil
		},
		sessionDone: func(*concurrency.Session) <-chan struct{} { return fake.sessionDone },
		orphanSession: func(*concurrency.Session) error {
			fake.orphanCalls.Add(1)
			fake.sessionOnce.Do(func() { close(fake.sessionDone) })
			return nil
		},
		newTicker: func(time.Duration) electorTicker { return fake.ticker },
	}
}

func (fake *fakeEtcdOps) send(response clientv3.WatchResponse) {
	fake.watch <- response
}

func (fake *fakeEtcdOps) waitForWatch(t *testing.T) {
	t.Helper()
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	for fake.watchCalls.Load() == 0 {
		select {
		case <-deadline.C:
			t.Fatal("watch did not start")
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

func assertSessionJoined(t *testing.T, fake *fakeEtcdOps) {
	t.Helper()
	if fake.orphanCalls.Load() != 1 {
		t.Fatalf("orphan calls = %d, want 1", fake.orphanCalls.Load())
	}
	select {
	case <-fake.sessionDone:
	default:
		t.Fatal("Campaign returned before Session.Done closed")
	}
}

func shutdownFakeGeneration(t *testing.T, elector *Elector, generation *generation) {
	t.Helper()
	if generation == nil {
		t.Fatal("missing generation")
	}
	if err := generation.shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := waitForMonitor(context.Background(), generation); err != nil {
		t.Fatal(err)
	}
	elector.mu.Lock()
	if elector.current == generation {
		elector.current = nil
	}
	elector.mu.Unlock()
}

type fakeElectorTicker struct {
	ticks chan time.Time
	stops atomic.Int64
}

func newFakeElectorTicker() *fakeElectorTicker {
	return &fakeElectorTicker{ticks: make(chan time.Time, 16)}
}

func (ticker *fakeElectorTicker) C() <-chan time.Time { return ticker.ticks }

func (ticker *fakeElectorTicker) Stop() { ticker.stops.Add(1) }
