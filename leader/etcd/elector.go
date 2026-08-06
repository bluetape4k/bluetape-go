package etcdleader

import (
	"errors"
	"sync"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const minimumProclaimInterval = 100 * time.Millisecond

// Elector leader backend election에서 leader election 선택과 조정 계약을 설명한다.
//
// 이 주석은 backend lease, ownership, consistency, cancellation 조건을 설명한다.
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
	// testHook 값은 leader backend election 동작의 세부 조건을 설명한다.
	testHook func(operation, phase string) error
}

var _ leader.Elector = (*Elector)(nil)

// New leader backend election에서 생성과 초기화 계약을 설명한다.
//
// New leader backend election에서 동작과 caller-visible 계약을 설명한다.
// 세부 조건은 backend별 lease, cleanup, retry 계약을 따른다.
// 세부 조건은 backend별 lease, cleanup, retry 계약을 따른다.
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
	if normalized.RenewInterval < minimumProclaimInterval {
		return nil, errors.New("etcd leader renew interval must be at least 100 milliseconds")
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

// EffectiveTTL leader backend election에서 반환값과 오류 의미를 설명한다.
// 세부 조건은 backend별 lease, cleanup, retry 계약을 따른다.
// 세부 조건은 backend별 lease, cleanup, retry 계약을 따른다.
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

// IsLeader leader backend election에서 반환값과 오류 의미를 설명한다.
// 이 주석은 leader backend election의 backend 요구사항, cancellation, timeout, 오류 처리 세부사항을 설명한다.
func (e *Elector) IsLeader() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.current != nil && e.current.published && e.current.ctx != nil && e.current.ctx.Err() == nil
}
