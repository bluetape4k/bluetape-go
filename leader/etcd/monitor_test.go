package etcdleader

import (
	"context"
	"errors"
	"math"
	"sync/atomic"
	"testing"
	"time"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestMonitorOwnershipLossFailsClosedAndStopsTicker(t *testing.T) {
	tests := []struct {
		name string
		lose func(*fakeEtcdOps)
	}{
		{
			name: "session",
			lose: func(fake *fakeEtcdOps) {
				fake.sessionOnce.Do(func() { close(fake.sessionDone) })
			},
		},
		{
			name: "delete",
			lose: func(fake *fakeEtcdOps) {
				fake.send(watchEvent(fake, mvccpb.DELETE, ""))
			},
		},
		{
			name: "mismatched-put",
			lose: func(fake *fakeEtcdOps) {
				fake.send(watchEvent(fake, mvccpb.PUT, "other-owner"))
			},
		},
		{
			name: "compaction",
			lose: func(fake *fakeEtcdOps) {
				fake.send(clientv3.WatchResponse{CompactRevision: 20, Canceled: true})
			},
		},
		{
			name: "watch-closed",
			lose: func(fake *fakeEtcdOps) { close(fake.watch) },
		},
		{
			name: "proclaim",
			lose: func(fake *fakeEtcdOps) {
				fake.proclaimErr = errors.New("renew failed")
				fake.ticker.ticks <- time.Now()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			elector, fake := newFakeElector(t)
			fake.send(clientv3.WatchResponse{Created: true})
			if err := elector.Campaign(context.Background()); err != nil {
				t.Fatal(err)
			}
			elector.mu.RLock()
			generation := elector.current
			elector.mu.RUnlock()

			tt.lose(fake)
			waitClosed(t, generation.monitorDone, "monitor")
			if elector.IsLeader() {
				t.Fatal("monitor loss left IsLeader true")
			}
			elector.mu.RLock()
			current := elector.current
			elector.mu.RUnlock()
			if current != generation {
				t.Fatal("monitor loss cleared cleanup inventory")
			}
			if got := fake.ticker.stops.Load(); got != 1 {
				t.Fatalf("ticker stops = %d, want 1", got)
			}
			assertSessionJoined(t, fake)
		})
	}
}

func TestMonitorProclaimIsSingleFlight(t *testing.T) {
	elector, fake := newFakeElector(t)
	fake.send(clientv3.WatchResponse{Created: true})
	if err := elector.Campaign(context.Background()); err != nil {
		t.Fatal(err)
	}
	elector.mu.RLock()
	generation := elector.current
	elector.mu.RUnlock()

	var inFlight atomic.Int64
	var maxInFlight atomic.Int64
	release := make(chan struct{})
	started := make(chan struct{}, 2)
	fake.proclaimFunc = func(ctx context.Context) error {
		current := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			maximum := maxInFlight.Load()
			if current <= maximum || maxInFlight.CompareAndSwap(maximum, current) {
				break
			}
		}
		started <- struct{}{}
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	fake.ticker.ticks <- time.Now()
	<-started
	fake.ticker.ticks <- time.Now()
	select {
	case <-started:
		t.Fatal("second Proclaim overlapped the first")
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	for fake.proclaimCalls.Load() < 3 {
		select {
		case <-deadline.C:
			t.Fatalf("Proclaim calls = %d, want initial + 2 renewals", fake.proclaimCalls.Load())
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if got := maxInFlight.Load(); got != 1 {
		t.Fatalf("max in-flight Proclaim = %d, want 1", got)
	}

	shutdownFakeGeneration(t, elector, generation)
	if got := fake.ticker.stops.Load(); got != 1 {
		t.Fatalf("ticker stops = %d, want 1", got)
	}
}

func TestMonitorProclaimRateIsBoundedByRenewInterval(t *testing.T) {
	elector, fake := newFakeElector(t)
	elector.opts.RenewInterval = minimumProclaimInterval
	intervals := make(chan time.Duration, 1)
	elector.ops.newTicker = func(interval time.Duration) electorTicker {
		intervals <- interval
		return realElectorTicker{Ticker: time.NewTicker(interval)}
	}
	fake.send(clientv3.WatchResponse{Created: true})
	if err := elector.Campaign(context.Background()); err != nil {
		t.Fatal(err)
	}
	if interval := <-intervals; interval != minimumProclaimInterval {
		t.Fatalf("ticker interval = %s, want %s", interval, minimumProclaimInterval)
	}

	started := time.Now()
	deadline := started.Add(2 * time.Second)
	for fake.proclaimCalls.Load()-1 < 3 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	elapsed := time.Since(started)
	periodicCalls := fake.proclaimCalls.Load() - 1 // Exclude acquisition validation.
	wantMax := int64(math.Ceil(float64(elapsed)/float64(minimumProclaimInterval))) + 1
	if periodicCalls < 3 || periodicCalls > wantMax {
		t.Fatalf("periodic Proclaim calls = %d after %s, want 3..%d", periodicCalls, elapsed, wantMax)
	}
	if err := elector.Resign(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestMonitorStaleGenerationCannotClearCurrentOwner(t *testing.T) {
	elector, fake := newFakeElector(t)
	fake.send(clientv3.WatchResponse{Created: true})
	if err := elector.Campaign(context.Background()); err != nil {
		t.Fatal(err)
	}
	elector.mu.Lock()
	stale := elector.current
	ctx, cancel := context.WithCancel(context.Background())
	current := &generation{id: stale.id + 1, ctx: ctx, cancel: cancel, published: true, ttl: stale.ttl}
	elector.current = current
	elector.mu.Unlock()

	fake.send(watchEvent(fake, mvccpb.DELETE, ""))
	waitClosed(t, stale.monitorDone, "stale monitor")
	if !elector.IsLeader() {
		t.Fatal("stale monitor cleared the current owner")
	}

	cancel()
	elector.mu.Lock()
	elector.current = nil
	elector.mu.Unlock()
}

func watchEvent(fake *fakeEtcdOps, eventType mvccpb.Event_EventType, value string) clientv3.WatchResponse {
	return clientv3.WatchResponse{Events: []*clientv3.Event{{
		Type: eventType,
		Kv: &mvccpb.KeyValue{
			Key:            []byte(fake.snapshot.key),
			Value:          []byte(value),
			CreateRevision: fake.snapshot.createRev,
			Lease:          int64(fake.leaseID),
		},
	}}}
}

func waitClosed(t *testing.T, done <-chan struct{}, name string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("%s did not close", name)
	}
}
