package etcdleader

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	"github.com/bluetape4k/bluetape-go/leader/leadertest"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

func TestEtcdElectorConformance(t *testing.T) {
	fixture := newEtcdFixture(t)
	clients := newEtcdCaseClients(fixture.endpoints)
	control := newEtcdConformanceControl(fixture.client)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := control.close(ctx); err != nil {
			t.Errorf("close etcd conformance control: %v", err)
		}
	})

	harness := leadertest.Harness{
		New: func(tb testing.TB, opts leader.Options) (leader.Elector, error) {
			client, err := clients.clientFor(tb, opts)
			if err != nil {
				return nil, err
			}
			elector, err := New(client, opts)
			if err != nil {
				return nil, err
			}
			elector.testHook = control.hook(opts)
			return elector, nil
		},
		Control: control,
	}
	leadertest.RunWithConfig(t, harness, leadertest.Config{
		Timing: leadertest.Timing{
			Lease:         3 * time.Second,
			RenewInterval: time.Second,
			CaseTimeout:   12 * time.Second,
			WaitTimeout:   4 * time.Second,
			ResignTimeout: 2 * time.Second,
		},
		Abort: func(ctx context.Context, opts leader.Options) error {
			return clients.closeFor(ctx, opts)
		},
	})
}

type etcdCaseClients struct {
	endpoints []string
	mu        sync.Mutex
	clients   map[string]*clientv3.Client
}

func newEtcdCaseClients(endpoints []string) *etcdCaseClients {
	return &etcdCaseClients{
		endpoints: append([]string(nil), endpoints...),
		clients:   make(map[string]*clientv3.Client),
	}
}

func (clients *etcdCaseClients) clientFor(tb testing.TB, opts leader.Options) (*clientv3.Client, error) {
	tb.Helper()
	group, err := normalizedGroup(opts)
	if err != nil {
		return nil, err
	}
	clients.mu.Lock()
	defer clients.mu.Unlock()
	if client := clients.clients[group]; client != nil {
		return client, nil
	}
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   clients.endpoints,
		DialTimeout: 3 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("create case etcd client: %w", err)
	}
	clients.clients[group] = client
	tb.Cleanup(func() {
		if err := clients.closeGroup(group); err != nil {
			tb.Errorf("close case etcd client: %v", err)
		}
	})
	return client, nil
}

func (clients *etcdCaseClients) closeFor(ctx context.Context, opts leader.Options) error {
	group, normalizeErr := normalizedGroup(opts)
	if normalizeErr != nil {
		return normalizeErr
	}
	closeErr := clients.closeGroup(group)
	if ctx == nil {
		return errors.Join(leader.ErrInvalidContext, closeErr)
	}
	return errors.Join(ctx.Err(), closeErr)
}

func (clients *etcdCaseClients) closeGroup(group string) error {
	clients.mu.Lock()
	client := clients.clients[group]
	delete(clients.clients, group)
	clients.mu.Unlock()
	if client == nil {
		return nil
	}
	return client.Close()
}

type etcdConformanceControl struct {
	client *clientv3.Client
	mu     sync.Mutex

	failures map[string]map[leadertest.Operation]error
	counts   map[string]map[leadertest.Operation]int64
	leases   []clientv3.LeaseID
}

func newEtcdConformanceControl(client *clientv3.Client) *etcdConformanceControl {
	return &etcdConformanceControl{
		client:   client,
		failures: make(map[string]map[leadertest.Operation]error),
		counts:   make(map[string]map[leadertest.Operation]int64),
	}
}

func (control *etcdConformanceControl) ReplaceOwner(ctx context.Context, opts leader.Options, owner string) error {
	if err := validControlContext(ctx); err != nil {
		return err
	}
	normalized, err := opts.Normalize()
	if err != nil || strings.TrimSpace(owner) == "" {
		return errors.New("etcd leader conformance: invalid replacement")
	}
	paths := electionPaths(normalized)
	getOptions := append([]clientv3.OpOption{clientv3.WithRange(paths.end)}, clientv3.WithFirstCreate()...)
	current, err := control.client.Get(ctx, paths.root, getOptions...)
	if err != nil {
		return fmt.Errorf("read current etcd owner: %w", err)
	}
	if len(current.Kvs) > 0 && current.Kvs[0].Lease != 0 {
		if _, err := control.client.Revoke(ctx, clientv3.LeaseID(current.Kvs[0].Lease)); err != nil {
			return fmt.Errorf("revoke current etcd owner: %w", err)
		}
	}

	ttl, err := requestedTTL(normalized.Lease)
	if err != nil {
		return err
	}
	grant, err := control.client.Grant(ctx, ttl)
	if err != nil {
		return fmt.Errorf("grant replacement etcd lease: %w", err)
	}
	leaseID := grant.ID
	cleanup := true
	defer func() {
		if cleanup {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, _ = control.client.Revoke(cleanupCtx, leaseID)
		}
	}()
	session, err := concurrency.NewSession(control.client, concurrency.WithContext(ctx), concurrency.WithLease(leaseID))
	if err != nil {
		return fmt.Errorf("create replacement etcd session: %w", err)
	}
	election := concurrency.NewElection(session, paths.base)
	if err := election.Campaign(ctx, owner); err != nil {
		session.Orphan()
		return fmt.Errorf("campaign replacement etcd owner: %w", err)
	}
	session.Orphan()
	control.mu.Lock()
	control.leases = append(control.leases, leaseID)
	control.mu.Unlock()
	cleanup = false
	return nil
}

func (control *etcdConformanceControl) FailNext(
	ctx context.Context,
	opts leader.Options,
	operation leadertest.Operation,
	cause error,
) error {
	if err := validControlContext(ctx); err != nil {
		return err
	}
	key, err := normalizedControlKey(opts)
	if err != nil || cause == nil || !validConformanceOperation(operation) {
		return errors.New("etcd leader conformance: invalid failure injection")
	}
	control.mu.Lock()
	defer control.mu.Unlock()
	if control.failures[key] == nil {
		control.failures[key] = make(map[leadertest.Operation]error)
	}
	control.failures[key][operation] = cause
	return nil
}

func (control *etcdConformanceControl) Owner(ctx context.Context, opts leader.Options) (string, error) {
	if err := validControlContext(ctx); err != nil {
		return "", err
	}
	normalized, err := opts.Normalize()
	if err != nil {
		return "", errors.New("etcd leader conformance: invalid options")
	}
	paths := electionPaths(normalized)
	getOptions := append([]clientv3.OpOption{clientv3.WithRange(paths.end)}, clientv3.WithFirstCreate()...)
	response, err := control.client.Get(ctx, paths.root, getOptions...)
	if err != nil {
		return "", err
	}
	if len(response.Kvs) == 0 {
		return "", nil
	}
	return string(response.Kvs[0].Value), nil
}

func (control *etcdConformanceControl) OperationCount(opts leader.Options, operation leadertest.Operation) int64 {
	key, err := normalizedControlKey(opts)
	if err != nil || !validConformanceOperation(operation) {
		return 0
	}
	control.mu.Lock()
	defer control.mu.Unlock()
	return control.counts[key][operation]
}

func (control *etcdConformanceControl) hook(opts leader.Options) func(string, string) error {
	key, _ := normalizedControlKey(opts)
	return func(operation, phase string) error {
		mapped, ok := conformanceOperation(operation)
		if !ok {
			return nil
		}
		control.mu.Lock()
		defer control.mu.Unlock()
		if control.counts[key] == nil {
			control.counts[key] = make(map[leadertest.Operation]int64)
		}
		if (mapped == leadertest.OperationCampaign && phase == "before") ||
			(mapped != leadertest.OperationCampaign && phase == "after") {
			control.counts[key][mapped]++
		}
		if phase != "after" {
			return nil
		}
		failure := control.failures[key][mapped]
		delete(control.failures[key], mapped)
		return failure
	}
}

func (control *etcdConformanceControl) close(ctx context.Context) error {
	control.mu.Lock()
	leases := append([]clientv3.LeaseID(nil), control.leases...)
	control.leases = nil
	control.mu.Unlock()
	var errs []error
	for _, leaseID := range leases {
		if _, err := control.client.Revoke(ctx, leaseID); err != nil && !errors.Is(err, rpctypes.ErrLeaseNotFound) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func normalizedGroup(opts leader.Options) (string, error) {
	normalized, err := opts.Normalize()
	if err != nil {
		return "", errors.New("etcd leader conformance: invalid options")
	}
	return normalized.Group, nil
}

func normalizedControlKey(opts leader.Options) (string, error) {
	normalized, err := opts.Normalize()
	if err != nil {
		return "", errors.New("etcd leader conformance: invalid options")
	}
	return electionPaths(normalized).base, nil
}

func validControlContext(ctx context.Context) error {
	if ctx == nil {
		return leader.ErrInvalidContext
	}
	return ctx.Err()
}

func validConformanceOperation(operation leadertest.Operation) bool {
	switch operation {
	case leadertest.OperationCampaign, leadertest.OperationRenew, leadertest.OperationResign:
		return true
	default:
		return false
	}
}

func conformanceOperation(operation string) (leadertest.Operation, bool) {
	mapped := leadertest.Operation(operation)
	return mapped, validConformanceOperation(mapped)
}
