package etcdleader

import (
	"context"
	"errors"
	"testing"

	"github.com/bluetape4k/bluetape-go/leader"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestResignClearsOnlyAfterCleanupProof(t *testing.T) {
	elector, fake := newFakeElector(t)
	fake.send(clientv3.WatchResponse{Created: true})
	if err := elector.Campaign(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := elector.Resign(context.Background()); err != nil {
		t.Fatal(err)
	}
	if elector.IsLeader() {
		t.Fatal("Resign left IsLeader true")
	}
	elector.mu.RLock()
	current := elector.current
	elector.mu.RUnlock()
	if current != nil {
		t.Fatal("proved cleanup retained generation")
	}
	if fake.resignCalls.Load() != 1 || fake.revokeCalls.Load() != 1 {
		t.Fatalf("resign=%d revoke=%d", fake.resignCalls.Load(), fake.revokeCalls.Load())
	}
}

func TestResignRetainsUnknownCleanupAndAllowsRetry(t *testing.T) {
	elector, fake := newFakeElector(t)
	fake.send(clientv3.WatchResponse{Created: true})
	if err := elector.Campaign(context.Background()); err != nil {
		t.Fatal(err)
	}
	fake.revokeErr = errors.New("revoke lost")
	fake.getErr = errors.New("get unavailable")
	if err := elector.Resign(context.Background()); !errors.Is(err, leader.ErrCommitUnknown) {
		t.Fatalf("Resign error = %v", err)
	}
	if err := elector.Campaign(context.Background()); !errors.Is(err, leader.ErrCleanupPending) {
		t.Fatalf("Campaign error = %v, want cleanup pending", err)
	}

	fake.revokeErr = nil
	fake.getErr = nil
	if err := elector.Resign(context.Background()); err != nil {
		t.Fatalf("retry Resign error = %v", err)
	}
}

func TestResignContextAndIdempotence(t *testing.T) {
	elector, _ := newFakeElector(t)
	if err := elector.Resign(nil); !errors.Is(err, leader.ErrInvalidContext) { //nolint:staticcheck // nil is the contract input under test.
		t.Fatalf("nil context error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := elector.Resign(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled context error = %v", err)
	}
	if err := elector.Resign(context.Background()); err != nil {
		t.Fatal(err)
	}
}
