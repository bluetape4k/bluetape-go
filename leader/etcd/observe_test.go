package etcdleader

import (
	"context"
	"errors"
	"testing"

	"github.com/bluetape4k/bluetape-go/leader"
	"go.etcd.io/etcd/api/v3/mvccpb"
)

func TestLeaderReturnsOldestCandidate(t *testing.T) {
	elector, fake := newFakeElector(t)
	fake.getResponse.Kvs = []*mvccpb.KeyValue{{Value: []byte("member:token")}}
	got, err := elector.Leader(context.Background())
	if err != nil || got != "member:token" {
		t.Fatalf("Leader = %q, %v", got, err)
	}
}

func TestLeaderContextAndOperationError(t *testing.T) {
	elector, fake := newFakeElector(t)
	if _, err := elector.Leader(nil); !errors.Is(err, leader.ErrInvalidContext) { //nolint:staticcheck // nil is the contract input under test.
		t.Fatalf("nil context error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := elector.Leader(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled context error = %v", err)
	}
	fake.getErr = errors.New("endpoint password token")
	_, err := elector.Leader(context.Background())
	var operationErr *leader.OperationError
	if !errors.As(err, &operationErr) || operationErr.Operation() != "lookup" {
		t.Fatalf("Leader error = %v", err)
	}
	if err != nil && err.Error() == "endpoint password token" {
		t.Fatal("Leader exposed raw cause")
	}
}
