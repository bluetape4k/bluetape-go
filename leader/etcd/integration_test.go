package etcdleader

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestEtcdIntegration(t *testing.T) {
	fixture := newEtcdFixture(t)

	t.Run("acquire and observe", func(t *testing.T) {
		elector := newIntegrationElector(t, fixture.client, integrationOptions(t.Name()))
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := elector.Campaign(ctx); err != nil {
			t.Fatalf("Campaign() error = %v", err)
		}
		if !elector.IsLeader() {
			t.Fatal("IsLeader() = false after campaign")
		}
		owner, err := elector.Leader(ctx)
		if err != nil {
			t.Fatalf("Leader() error = %v", err)
		}
		if owner == "" {
			t.Fatal("Leader() returned an empty owner")
		}
		if err := elector.Resign(ctx); err != nil {
			t.Fatalf("Resign() error = %v", err)
		}
	})

	t.Run("sixteen contenders have one winner", func(t *testing.T) {
		const contenders = 16
		opts := integrationOptions(t.Name())
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		campaignCtx, stopCampaigns := context.WithCancel(ctx)
		defer stopCampaigns()

		type result struct {
			elector *Elector
			err     error
		}
		results := make(chan result, contenders)
		var start sync.WaitGroup
		start.Add(contenders)
		ready := make(chan struct{})
		for range contenders {
			elector := newIntegrationElector(t, fixture.client, opts)
			go func() {
				start.Done()
				<-ready
				results <- result{elector: elector, err: elector.Campaign(campaignCtx)}
			}()
		}
		start.Wait()
		close(ready)

		first := <-results
		if first.err != nil {
			t.Fatalf("first campaign result = %v", first.err)
		}
		winner := first.elector
		stopCampaigns()
		successes := 1
		for range contenders - 1 {
			result := <-results
			if result.err == nil {
				successes++
			}
		}
		if successes != 1 {
			t.Fatalf("campaign winners = %d, want 1", successes)
		}
		if err := winner.Resign(ctx); err != nil {
			t.Fatalf("winner Resign() error = %v", err)
		}
	})

	t.Run("canceled waiter leaves no late candidate", func(t *testing.T) {
		opts := integrationOptions(t.Name())
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		owner := newIntegrationElector(t, fixture.client, opts)
		if err := owner.Campaign(ctx); err != nil {
			t.Fatalf("owner Campaign() error = %v", err)
		}
		waiter := newIntegrationElector(t, fixture.client, opts)
		waitCtx, stopWait := context.WithTimeout(ctx, 250*time.Millisecond)
		err := waiter.Campaign(waitCtx)
		stopWait()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("waiter Campaign() error = %v, want deadline exceeded", err)
		}
		if err := owner.Resign(ctx); err != nil {
			t.Fatalf("owner Resign() error = %v", err)
		}
		time.Sleep(150 * time.Millisecond)
		response, err := fixture.client.Get(ctx, waiter.paths.root, clientv3.WithRange(waiter.paths.end))
		if err != nil {
			t.Fatalf("read election candidates: %v", err)
		}
		if len(response.Kvs) != 0 {
			t.Fatalf("remaining candidates = %d, want 0", len(response.Kvs))
		}
		if waiter.IsLeader() {
			t.Fatal("canceled waiter became leader")
		}
	})

	t.Run("keepalive survives the requested lease", func(t *testing.T) {
		opts := integrationOptions(t.Name())
		elector := newIntegrationElector(t, fixture.client, opts)
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		if err := elector.Campaign(ctx); err != nil {
			t.Fatalf("Campaign() error = %v", err)
		}
		timer := time.NewTimer(elector.EffectiveTTL() + 500*time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			t.Fatalf("wait beyond lease: %v", ctx.Err())
		case <-timer.C:
		}
		if !elector.IsLeader() {
			t.Fatal("leadership expired despite keepalive")
		}
		if err := elector.Resign(ctx); err != nil {
			t.Fatalf("Resign() error = %v", err)
		}
	})

	t.Run("external lease revoke clears leadership", func(t *testing.T) {
		elector := newIntegrationElector(t, fixture.client, integrationOptions(t.Name()))
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		if err := elector.Campaign(ctx); err != nil {
			t.Fatalf("Campaign() error = %v", err)
		}
		elector.mu.RLock()
		leaseID := elector.current.leaseID
		elector.mu.RUnlock()
		if _, err := fixture.client.Revoke(ctx, leaseID); err != nil {
			t.Fatalf("external Revoke() error = %v", err)
		}
		waitForIntegrationCondition(ctx, t, func() bool { return !elector.IsLeader() })
		if err := elector.Resign(ctx); err != nil {
			t.Fatalf("Resign() after revoke error = %v", err)
		}
	})

	t.Run("exact candidate deletion clears leadership", func(t *testing.T) {
		elector := newIntegrationElector(t, fixture.client, integrationOptions(t.Name()))
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		if err := elector.Campaign(ctx); err != nil {
			t.Fatalf("Campaign() error = %v", err)
		}
		elector.mu.RLock()
		key := elector.current.key
		elector.mu.RUnlock()
		if _, err := fixture.client.Delete(ctx, key); err != nil {
			t.Fatalf("delete candidate key: %v", err)
		}
		waitForIntegrationCondition(ctx, t, func() bool { return !elector.IsLeader() })
		if err := elector.Resign(ctx); err != nil {
			t.Fatalf("Resign() after key deletion error = %v", err)
		}
	})

	t.Run("watch interruption fails closed", func(t *testing.T) {
		client, err := clientv3.New(clientv3.Config{Endpoints: fixture.endpoints, DialTimeout: 3 * time.Second})
		if err != nil {
			t.Fatalf("create dedicated etcd client: %v", err)
		}
		t.Cleanup(func() { _ = client.Close() })
		elector := newIntegrationElector(t, client, integrationOptions(t.Name()))
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		if err := elector.Campaign(ctx); err != nil {
			t.Fatalf("Campaign() error = %v", err)
		}
		if err := client.Close(); err != nil {
			t.Fatalf("close dedicated etcd client: %v", err)
		}
		waitForIntegrationCondition(ctx, t, func() bool { return !elector.IsLeader() })
	})

	t.Run("resign is idempotent", func(t *testing.T) {
		elector := newIntegrationElector(t, fixture.client, integrationOptions(t.Name()))
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := elector.Campaign(ctx); err != nil {
			t.Fatalf("Campaign() error = %v", err)
		}
		if err := elector.Resign(ctx); err != nil {
			t.Fatalf("first Resign() error = %v", err)
		}
		if err := elector.Resign(ctx); err != nil {
			t.Fatalf("second Resign() error = %v", err)
		}
	})

	t.Run("caller client remains reusable", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		first := newIntegrationElector(t, fixture.client, integrationOptions(t.Name()+"-first"))
		if err := first.Campaign(ctx); err != nil {
			t.Fatalf("first Campaign() error = %v", err)
		}
		if err := first.Resign(ctx); err != nil {
			t.Fatalf("first Resign() error = %v", err)
		}
		second := newIntegrationElector(t, fixture.client, integrationOptions(t.Name()+"-second"))
		if err := second.Campaign(ctx); err != nil {
			t.Fatalf("second Campaign() error = %v", err)
		}
		if err := second.Resign(ctx); err != nil {
			t.Fatalf("second Resign() error = %v", err)
		}
	})

	t.Run("single node restart restores elections", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		before := newIntegrationElector(t, fixture.client, integrationOptions(t.Name()+"-before"))
		if err := before.Campaign(ctx); err != nil {
			t.Fatalf("Campaign() before restart error = %v", err)
		}
		stopTimeout := 5 * time.Second
		if err := fixture.container.Stop(ctx, &stopTimeout); err != nil {
			t.Fatalf("stop etcd container: %v", err)
		}
		waitForIntegrationCondition(ctx, t, func() bool { return !before.IsLeader() })
		if err := fixture.container.Start(ctx); err != nil {
			t.Fatalf("restart etcd container: %v", err)
		}
		restartedEndpoints, err := fixture.container.ClientEndpoints(ctx)
		if err != nil {
			t.Fatalf("resolve restarted etcd endpoints: %v", err)
		}
		restartedClient, err := clientv3.New(clientv3.Config{Endpoints: restartedEndpoints, DialTimeout: 3 * time.Second})
		if err != nil {
			t.Fatalf("create restarted etcd client: %v", err)
		}
		t.Cleanup(func() { _ = restartedClient.Close() })
		waitForEtcdReady(t, restartedClient, restartedEndpoints)
		after := newIntegrationElector(t, restartedClient, integrationOptions(t.Name()+"-after"))
		if err := after.Campaign(ctx); err != nil {
			t.Fatalf("Campaign() after restart error = %v", err)
		}
		if err := after.Resign(ctx); err != nil {
			t.Fatalf("Resign() after restart error = %v", err)
		}
	})
}

func integrationOptions(name string) leader.Options {
	return leader.Options{
		Group:         fmt.Sprintf("integration-%d", time.Now().UnixNano()),
		MemberID:      name,
		Lease:         3 * time.Second,
		RenewInterval: time.Second,
		KeyPrefix:     "etcd-test",
	}
}

func newIntegrationElector(t *testing.T, client *clientv3.Client, opts leader.Options) *Elector {
	t.Helper()
	elector, err := New(client, opts)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return elector
}

func waitForIntegrationCondition(ctx context.Context, t *testing.T, condition func() bool) {
	t.Helper()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		if condition() {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("condition not met: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}
