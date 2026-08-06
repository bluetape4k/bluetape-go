package etcdleader

import "sync/atomic"

var (
	liveEtcdSessions      atomic.Int64
	publishedEtcdMonitors atomic.Int64
	inFlightEtcdProclaims atomic.Int64
)
