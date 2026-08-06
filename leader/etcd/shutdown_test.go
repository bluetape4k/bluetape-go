package etcdleader

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"google.golang.org/grpc"
)

func TestResignAndMonitorLossJoinOneGeneration(t *testing.T) {
	elector, fake := newFakeElector(t)
	fake.send(clientv3.WatchResponse{Created: true})
	if err := elector.Campaign(context.Background()); err != nil {
		t.Fatal(err)
	}
	elector.mu.RLock()
	generation := elector.current
	elector.mu.RUnlock()

	shutdownStarted := make(chan struct{})
	releaseShutdown := make(chan struct{})
	var startOnce sync.Once
	originalOrphan := generation.ops.orphanSession
	generation.ops.orphanSession = func(session *concurrency.Session) error {
		startOnce.Do(func() { close(shutdownStarted) })
		<-releaseShutdown
		return originalOrphan(session)
	}

	resignResult := make(chan error, 1)
	go func() { resignResult <- elector.Resign(context.Background()) }()
	<-shutdownStarted
	lossDone := make(chan struct{})
	go func() {
		defer close(lossDone)
		elector.loseGeneration(generation)
	}()
	close(releaseShutdown)
	if err := <-resignResult; err != nil {
		t.Fatalf("Resign() error = %v", err)
	}
	<-lossDone
	if elector.IsLeader() {
		t.Fatal("interleaved monitor loss retained leadership")
	}
	elector.mu.RLock()
	current := elector.current
	elector.mu.RUnlock()
	if current != nil {
		t.Fatal("interleaved cleanup retained a proved generation")
	}
}

func TestCleanupReconciliationRejectsABA(t *testing.T) {
	generation := &generation{
		leaseID:   41,
		key:       "/election/candidate",
		createRev: 101,
	}
	const token = "member:token"
	tests := []struct {
		name     string
		response *clientv3.GetResponse
		wantGone bool
	}{
		{
			name: "same generation",
			response: &clientv3.GetResponse{Kvs: []*mvccpb.KeyValue{{
				Key: []byte(generation.key), Value: []byte(token), Lease: int64(generation.leaseID), CreateRevision: generation.createRev,
			}}},
			wantGone: false,
		},
		{
			name: "ABA replacement with same identity",
			response: &clientv3.GetResponse{Kvs: []*mvccpb.KeyValue{{
				Key: []byte(generation.key), Value: []byte(token), Lease: int64(generation.leaseID), CreateRevision: generation.createRev + 1,
			}}},
			wantGone: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := generationAbsentOrReplaced(tt.response, generation, token); got != tt.wantGone {
				t.Fatalf("generationAbsentOrReplaced() = %v, want %v", got, tt.wantGone)
			}
		})
	}
}

func TestBlockedOfficialCampaignCleanupRequiresClientHardStop(t *testing.T) {
	fixture := newEtcdFixture(t)
	opts := integrationOptions(t.Name())
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	owner := newIntegrationElector(t, fixture.client, opts)
	if err := owner.Campaign(ctx); err != nil {
		t.Fatalf("owner Campaign() error = %v", err)
	}

	cleanupBlocked := make(chan struct{})
	var blockedOnce sync.Once
	var txnCalls atomic.Int64
	caseClient, err := clientv3.New(clientv3.Config{
		Endpoints:   fixture.endpoints,
		DialTimeout: 3 * time.Second,
		DialOptions: []grpc.DialOption{grpc.WithChainUnaryInterceptor(
			func(
				callCtx context.Context,
				method string,
				req, reply any,
				connection *grpc.ClientConn,
				invoker grpc.UnaryInvoker,
				opts ...grpc.CallOption,
			) error {
				if method == "/etcdserverpb.KV/Txn" && txnCalls.Add(1) > 1 {
					blockedOnce.Do(func() { close(cleanupBlocked) })
					<-callCtx.Done()
					return callCtx.Err()
				}
				return invoker(callCtx, method, req, reply, connection, opts...)
			},
		)},
	})
	if err != nil {
		t.Fatalf("create hard-stop etcd client: %v", err)
	}
	t.Cleanup(func() { _ = caseClient.Close() })
	waiter := newIntegrationElector(t, caseClient, opts)
	campaignCtx, cancelCampaign := context.WithCancel(ctx)
	result := make(chan error, 1)
	go func() { result <- waiter.Campaign(campaignCtx) }()

	waitForExactCandidateCount(ctx, t, fixture.client, waiter.paths, 2)
	waiter.mu.RLock()
	generation := waiter.current
	waiter.mu.RUnlock()
	if generation == nil || generation.key == "" {
		t.Fatal("blocked campaign did not retain candidate inventory")
	}
	cancelCampaign()
	select {
	case <-cleanupBlocked:
	case <-time.After(2 * time.Second):
		t.Fatal("official Campaign cleanup did not block on client context")
	}
	if err := caseClient.Close(); err != nil {
		t.Fatalf("close case-dedicated client: %v", err)
	}

	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) || !errors.Is(err, leader.ErrCommitUnknown) {
			t.Fatalf("Campaign() error = %v, want cancellation plus commit unknown", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Campaign did not join after client hard stop")
	}
	select {
	case <-generation.shutdownDone:
	default:
		t.Fatal("Session shutdown did not join")
	}
	if generation.monitorDone != nil {
		t.Fatal("blocked Campaign unexpectedly published a monitor")
	}
	waiter.mu.RLock()
	retained := waiter.current
	waiter.mu.RUnlock()
	if retained != generation || generation.published {
		t.Fatal("hard stop did not preserve unresolved cleanup inventory")
	}

	waitForExactKeyAbsence(ctx, t, fixture.client, generation.key)
	restartGate := make(chan struct{})
	close(restartGate)
	select {
	case <-restartGate:
	default:
		t.Fatal("restart gate cleared without linearizable absence proof")
	}
	if err := owner.Resign(ctx); err != nil {
		t.Fatalf("owner Resign() error = %v", err)
	}
}

func waitForExactCandidateCount(
	ctx context.Context,
	t *testing.T,
	client *clientv3.Client,
	paths electionPath,
	want int,
) {
	t.Helper()
	waitForIntegrationCondition(ctx, t, func() bool {
		response, err := client.Get(ctx, paths.root, clientv3.WithRange(paths.end))
		return err == nil && len(response.Kvs) == want
	})
}

func waitForExactKeyAbsence(ctx context.Context, t *testing.T, client *clientv3.Client, key string) {
	t.Helper()
	waitForIntegrationCondition(ctx, t, func() bool {
		response, err := client.Get(ctx, key)
		return err == nil && len(response.Kvs) == 0
	})
}
