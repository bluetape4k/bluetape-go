package etcdleader

import (
	"context"
	"errors"

	"github.com/bluetape4k/bluetape-go/leader"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// Leader leader backend election에서 반환값과 오류 의미를 설명한다.
// 이 주석은 backend lease, ownership, consistency, cancellation 조건을 설명한다.
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
