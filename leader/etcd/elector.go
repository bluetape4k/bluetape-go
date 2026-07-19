package etcdleader

import (
	"errors"
	"sync"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// Elector coordinates a single etcd-backed leader election.
//
// An Elector must be constructed with [New]. Its zero value is unusable.
type Elector struct {
	client       *clientv3.Client
	opts         leader.Options
	paths        electionPath
	token        string
	requestedTTL int64
	ops          etcdOps

	mu             sync.RWMutex
	campaigning    bool
	current        *generation
	lastTTL        time.Duration
	nextGeneration uint64
	// testHook is package-private fault injection used only by real-server tests.
	testHook func(operation, phase string) error
}

var _ leader.Elector = (*Elector)(nil)

// New creates an etcd-backed elector over the caller-owned client.
//
// New performs no network I/O. After option normalization, RenewInterval must
// be less than Lease. The caller remains responsible for closing client.
func New(client *clientv3.Client, opts leader.Options) (*Elector, error) {
	if client == nil {
		return nil, errors.New("etcd leader client must not be nil")
	}

	normalized, err := opts.Normalize()
	if err != nil {
		return nil, err
	}
	if normalized.RenewInterval >= normalized.Lease {
		return nil, errors.New("etcd leader renew interval must be less than lease")
	}

	ttl, err := requestedTTL(normalized.Lease)
	if err != nil {
		return nil, err
	}
	token, err := ownerToken(normalized.MemberID)
	if err != nil {
		return nil, err
	}

	return &Elector{
		client:       client,
		opts:         normalized,
		paths:        electionPaths(normalized),
		token:        token,
		requestedTTL: ttl,
		ops:          productionEtcdOps(client),
	}, nil
}

// EffectiveTTL returns the last TTL published by etcd for the active or most
// recent generation. Before etcd publishes a grant, it returns the requested
// lease rounded up to whole seconds.
func (e *Elector) EffectiveTTL() time.Duration {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.current != nil && e.current.published {
		return e.current.ttl
	}
	if e.lastTTL > 0 {
		return e.lastTTL
	}
	return time.Duration(e.requestedTTL) * time.Second
}

// IsLeader reports whether the current generation still has locally observed
// ownership. Remote loss clears this state before monitor shutdown completes.
func (e *Elector) IsLeader() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.current != nil && e.current.published && e.current.ctx != nil && e.current.ctx.Err() == nil
}
