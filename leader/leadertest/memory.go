package leadertest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode"

	"github.com/bluetape4k/bluetape-go/leader"
)

type memoryRecord struct {
	owner      string
	leaseUntil time.Time
	failures   map[Operation]error
	counts     map[Operation]int64
}

type memoryBackend struct {
	mu      sync.Mutex
	records map[string]*memoryRecord
	nextID  atomic.Uint64
}

type memoryElector struct {
	backend *memoryBackend
	opts    leader.Options
	key     string
	token   string

	mu          sync.RWMutex
	owned       bool
	campaigning bool
	cleanup     bool
	cancel      context.CancelFunc
	done        chan struct{}
}

// MemoryHarness는 race-safe in-memory reference implementation을 반환한다.
func MemoryHarness() Harness {
	backend := &memoryBackend{records: make(map[string]*memoryRecord)}
	return Harness{
		New: func(t testing.TB, opts leader.Options) (leader.Elector, error) {
			t.Helper()
			normalized, err := opts.Normalize()
			if err != nil {
				return nil, err
			}
			id := backend.nextID.Add(1)
			return &memoryElector{
				backend: backend,
				opts:    normalized,
				key:     identityKey(normalized),
				token:   fmt.Sprintf("member-%d", id),
			}, nil
		},
		Control: backend,
	}
}

func (b *memoryBackend) ReplaceOwner(ctx context.Context, opts leader.Options, owner string) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	normalized, err := opts.Normalize()
	if err != nil {
		return errors.New("leadertest: invalid options")
	}
	if !validOwner(owner) {
		return errors.New("leadertest: invalid owner")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	record := b.record(identityKey(normalized))
	record.owner = owner
	record.leaseUntil = time.Now().Add(normalized.Lease)
	return nil
}

func (b *memoryBackend) FailNext(ctx context.Context, opts leader.Options, operation Operation, cause error) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	normalized, err := opts.Normalize()
	if err != nil || !validOperation(operation) || cause == nil {
		return errors.New("leadertest: invalid failure injection")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.record(identityKey(normalized)).failures[operation] = cause
	return nil
}

func (b *memoryBackend) Owner(ctx context.Context, opts leader.Options) (string, error) {
	if err := validateContext(ctx); err != nil {
		return "", err
	}
	normalized, err := opts.Normalize()
	if err != nil {
		return "", errors.New("leadertest: invalid options")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	record := b.record(identityKey(normalized))
	if record.owner != "" && !record.leaseUntil.After(time.Now()) {
		record.owner = ""
		record.leaseUntil = time.Time{}
	}
	return record.owner, nil
}

func (b *memoryBackend) OperationCount(opts leader.Options, operation Operation) int64 {
	normalized, err := opts.Normalize()
	if err != nil || !validOperation(operation) {
		return 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.record(identityKey(normalized)).counts[operation]
}

func (b *memoryBackend) record(key string) *memoryRecord {
	record := b.records[key]
	if record == nil {
		record = &memoryRecord{
			failures: make(map[Operation]error),
			counts:   make(map[Operation]int64),
		}
		b.records[key] = record
	}
	return record
}

func (b *memoryBackend) campaign(key, token string, lease time.Duration) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	record := b.record(key)
	record.counts[OperationCampaign]++
	now := time.Now()
	if record.owner != "" && record.owner != token && record.leaseUntil.After(now) {
		return false, nil
	}
	record.owner = token
	record.leaseUntil = now.Add(lease)
	return true, takeFailure(record, OperationCampaign)
}

func (b *memoryBackend) renew(key, token string, lease time.Duration) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	record := b.record(key)
	record.counts[OperationRenew]++
	now := time.Now()
	if record.owner != token || !record.leaseUntil.After(now) {
		return false, nil
	}
	record.leaseUntil = now.Add(lease)
	return true, takeFailure(record, OperationRenew)
}

func (b *memoryBackend) resign(key, token string) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	record := b.record(key)
	record.counts[OperationResign]++
	deleted := record.owner == token
	if deleted {
		record.owner = ""
		record.leaseUntil = time.Time{}
	}
	return deleted, takeFailure(record, OperationResign)
}

func takeFailure(record *memoryRecord, operation Operation) error {
	err := record.failures[operation]
	delete(record.failures, operation)
	return err
}

func (e *memoryElector) Campaign(ctx context.Context) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	if err := e.beginCampaign(); err != nil {
		return err
	}
	defer e.endCampaign()

	retry := min(e.opts.RenewInterval, 10*time.Millisecond)
	for {
		acquired, err := e.backend.campaign(e.key, e.token, e.opts.Lease)
		if acquired {
			if err != nil {
				e.mu.Lock()
				e.cleanup = true
				e.mu.Unlock()
				return commitUnknown("campaign", err)
			}
			e.startRenewal()
			return nil
		}
		if err != nil {
			return leader.NewOperationError("memory", "campaign", err)
		}
		timer := time.NewTimer(retry)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (e *memoryElector) Resign(ctx context.Context) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	cancel, done, active := e.stopRenewal()
	if !active {
		return nil
	}
	if cancel != nil {
		cancel()
	}
	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			e.markCleanup()
			return ctx.Err()
		}
	}
	_, err := e.backend.resign(e.key, e.token)
	if err != nil {
		e.markCleanup()
		return commitUnknown("resign", err)
	}
	e.mu.Lock()
	e.cleanup = false
	e.mu.Unlock()
	return nil
}

func (e *memoryElector) IsLeader() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.owned
}

func (e *memoryElector) Leader(ctx context.Context) (string, error) {
	return e.backend.Owner(ctx, e.opts)
}

func (e *memoryElector) beginCampaign() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cleanup {
		return leader.ErrCleanupPending
	}
	if e.owned {
		return leader.ErrAlreadyLeader
	}
	if e.campaigning {
		return leader.ErrCampaignInProgress
	}
	e.campaigning = true
	return nil
}

func (e *memoryElector) endCampaign() {
	e.mu.Lock()
	e.campaigning = false
	e.mu.Unlock()
}

func (e *memoryElector) startRenewal() {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	e.mu.Lock()
	e.owned = true
	e.cleanup = false
	e.cancel = cancel
	e.done = done
	e.mu.Unlock()
	go e.renewLoop(ctx, done)
}

func (e *memoryElector) renewLoop(ctx context.Context, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(e.opts.RenewInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ok, err := e.backend.renew(e.key, e.token, e.opts.Lease)
			if err != nil || !ok {
				e.mu.Lock()
				e.owned = false
				e.cleanup = err != nil
				e.cancel = nil
				e.done = nil
				e.mu.Unlock()
				return
			}
		}
	}
}

func (e *memoryElector) stopRenewal() (context.CancelFunc, chan struct{}, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	active := e.owned || e.cleanup
	cancel := e.cancel
	done := e.done
	e.owned = false
	e.cancel = nil
	e.done = nil
	return cancel, done, active
}

func (e *memoryElector) markCleanup() {
	e.mu.Lock()
	e.cleanup = true
	e.mu.Unlock()
}

func commitUnknown(operation string, cause error) error {
	return errors.Join(
		leader.NewOperationError("memory", operation, cause),
		leader.ErrCommitUnknown,
	)
}

func identityKey(opts leader.Options) string {
	return opts.KeyPrefix + ":" + opts.Group
}

func validOwner(owner string) bool {
	if owner == "" || owner != strings.TrimSpace(owner) || len(owner) > 256 {
		return false
	}
	for _, r := range owner {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
