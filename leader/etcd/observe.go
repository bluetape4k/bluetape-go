package etcdleader

import (
	"context"
	"errors"

	"github.com/bluetape4k/bluetape-go/leader"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// Leader returns the oldest candidate value in this elector's encoded range.
// It returns an empty string when the election has no candidates.
func (e *Elector) Leader(ctx context.Context) (string, error) {
	if ctx == nil {
		return "", leader.ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if e.client == nil || e.ops.get == nil {
		return "", leader.NewOperationError("etcd", "lookup", errors.New("etcd leader Elector is not initialized"))
	}

	opts := append([]clientv3.OpOption{clientv3.WithRange(e.paths.end)}, clientv3.WithFirstCreate()...)
	response, err := e.ops.get(ctx, e.paths.root, opts...)
	if err != nil {
		return "", leader.NewOperationError("etcd", "lookup", err)
	}
	if response == nil {
		return "", leader.NewOperationError("etcd", "lookup", errors.New("etcd leader lookup returned nil"))
	}
	if len(response.Kvs) == 0 {
		return "", nil
	}
	if response.Kvs[0] == nil {
		return "", leader.NewOperationError("etcd", "lookup", errors.New("etcd leader lookup returned invalid candidate"))
	}
	return string(response.Kvs[0].Value), nil
}
