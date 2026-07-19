package etcdleader_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bluetape4k/bluetape-go/leader"
	etcdleader "github.com/bluetape4k/bluetape-go/leader/etcd"
)

func TestZeroElectorFailsDeterministically(t *testing.T) {
	var elector etcdleader.Elector
	if elector.IsLeader() {
		t.Fatal("zero Elector is leader")
	}
	if err := elector.Resign(context.Background()); err != nil {
		t.Fatalf("zero Resign error = %v", err)
	}
	if err := elector.Campaign(context.Background()); err == nil {
		t.Fatal("zero Campaign succeeded")
	}
	if _, err := elector.Leader(context.Background()); err == nil {
		t.Fatal("zero Leader succeeded")
	}
	if err := elector.Campaign(nil); !errors.Is(err, leader.ErrInvalidContext) { //nolint:staticcheck // nil is the contract input under test.
		t.Fatalf("zero nil Campaign error = %v", err)
	}
}
