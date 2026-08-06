package etcdleader

import (
	"bufio"
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestEtcdContenderResourceContainment(t *testing.T) {
	const contenders = 32
	fixture := newEtcdFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	client, err := clientv3.New(clientv3.Config{Endpoints: fixture.endpoints, DialTimeout: 3 * time.Second})
	if err != nil {
		t.Fatalf("create resource-test etcd client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	baselineLeases := etcdLeaseCount(ctx, t, fixture.client)
	baselineWatchers := etcdWatcherCount(ctx, t, fixture.endpoints[0])
	baselineSessions := liveEtcdSessions.Load()
	baselineMonitors := publishedEtcdMonitors.Load()
	baselineProclaims := inFlightEtcdProclaims.Load()

	opts := integrationOptions(t.Name())
	campaignCtx, stopCampaigns := context.WithCancel(ctx)
	type campaignResult struct {
		elector *Elector
		err     error
	}
	results := make(chan campaignResult, contenders)
	for range contenders {
		elector := newIntegrationElector(t, client, opts)
		go func() {
			results <- campaignResult{elector: elector, err: elector.Campaign(campaignCtx)}
		}()
	}

	var winner *Elector
	select {
	case result := <-results:
		if result.err != nil {
			t.Fatalf("first contender error = %v", result.err)
		}
		winner = result.elector
	case <-ctx.Done():
		t.Fatalf("no contention winner: %v", ctx.Err())
	}
	waitForExactCandidateCount(ctx, t, fixture.client, winner.paths, contenders)
	waitForIntegrationCondition(ctx, t, func() bool {
		return liveEtcdSessions.Load()-baselineSessions == contenders &&
			publishedEtcdMonitors.Load()-baselineMonitors == 1
	})

	leaseDelta := etcdLeaseCount(ctx, t, fixture.client) - baselineLeases
	watcherDelta := etcdWatcherCount(ctx, t, fixture.endpoints[0]) - baselineWatchers
	sessionDelta := liveEtcdSessions.Load() - baselineSessions
	monitorDelta := publishedEtcdMonitors.Load() - baselineMonitors
	proclaimDelta := inFlightEtcdProclaims.Load() - baselineProclaims
	if leaseDelta < 1 || leaseDelta > contenders {
		t.Fatalf("live lease delta = %d, want 1..%d", leaseDelta, contenders)
	}
	if watcherDelta < 1 || watcherDelta > contenders {
		t.Fatalf("server watcher delta = %d, want 1..%d", watcherDelta, contenders)
	}
	if sessionDelta != contenders {
		t.Fatalf("live Session delta = %d, want %d", sessionDelta, contenders)
	}
	if monitorDelta != 1 {
		t.Fatalf("published monitor delta = %d, want 1", monitorDelta)
	}
	if proclaimDelta < 0 || proclaimDelta > 1 {
		t.Fatalf("in-flight Proclaim delta = %d, want 0..1", proclaimDelta)
	}

	stopCampaigns()
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cleanupCancel()
	if err := winner.Resign(cleanupCtx); err != nil {
		t.Fatalf("winner Resign() error = %v", err)
	}
	for range contenders - 1 {
		select {
		case result := <-results:
			if result.err == nil || (!errors.Is(result.err, context.Canceled) && !errors.Is(result.err, leader.ErrCommitUnknown)) {
				t.Fatalf("losing contender error = %v", result.err)
			}
		case <-cleanupCtx.Done():
			t.Fatalf("contenders did not join: %v", cleanupCtx.Err())
		}
	}
	if err := client.Close(); err != nil {
		t.Fatalf("close resource-test etcd client: %v", err)
	}

	waitForIntegrationCondition(cleanupCtx, t, func() bool {
		return etcdLeaseCountNoFail(cleanupCtx, fixture.client) == baselineLeases &&
			etcdWatcherCountNoFail(cleanupCtx, fixture.endpoints[0]) == baselineWatchers &&
			liveEtcdSessions.Load() == baselineSessions &&
			publishedEtcdMonitors.Load() == baselineMonitors &&
			inFlightEtcdProclaims.Load() == baselineProclaims
	})
}

func etcdLeaseCount(ctx context.Context, t *testing.T, client *clientv3.Client) int64 {
	t.Helper()
	count := etcdLeaseCountNoFail(ctx, client)
	if count < 0 {
		t.Fatal("read etcd leases")
	}
	return count
}

func etcdLeaseCountNoFail(ctx context.Context, client *clientv3.Client) int64 {
	response, err := client.Leases(ctx)
	if err != nil {
		return -1
	}
	return int64(len(response.Leases))
}

func etcdWatcherCount(ctx context.Context, t *testing.T, endpoint string) int64 {
	t.Helper()
	count := etcdWatcherCountNoFail(ctx, endpoint)
	if count < 0 {
		t.Fatal("scrape etcd watcher metric")
	}
	return count
}

func etcdWatcherCountNoFail(ctx context.Context, endpoint string) int64 {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(endpoint, "/")+"/metrics", nil)
	if err != nil {
		return -1
	}
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return -1
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return -1
	}
	scanner := bufio.NewScanner(response.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "etcd_debugging_mvcc_watcher_total ") {
			value := strings.TrimSpace(strings.TrimPrefix(line, "etcd_debugging_mvcc_watcher_total "))
			parsed, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return -1
			}
			return int64(parsed)
		}
	}
	return -1
}
