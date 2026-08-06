package etcdleader

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"go.etcd.io/etcd/client/v3/concurrency"
)

func TestShutdownGenerationIsNilSessionSafe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	generation := &generation{
		ctx:          ctx,
		cancel:       cancel,
		shutdownDone: make(chan struct{}),
	}

	var callers sync.WaitGroup
	for range 8 {
		callers.Add(1)
		go func() {
			defer callers.Done()
			if err := generation.shutdown(context.Background()); err != nil {
				t.Errorf("shutdown error = %v", err)
			}
		}()
	}
	callers.Wait()

	select {
	case <-generation.shutdownDone:
	default:
		t.Fatal("shutdown did not close")
	}
}

func TestShutdownGenerationOrphansAndJoinsSessionOnce(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	var orphanCalls atomic.Int64
	var closeOnce sync.Once
	generation := &generation{
		ctx:          ctx,
		cancel:       cancel,
		session:      &concurrency.Session{},
		shutdownDone: make(chan struct{}),
		ops: etcdOps{
			orphanSession: func(*concurrency.Session) error {
				orphanCalls.Add(1)
				closeOnce.Do(func() { close(done) })
				return nil
			},
			sessionDone: func(*concurrency.Session) <-chan struct{} { return done },
		},
	}

	var callers sync.WaitGroup
	for range 8 {
		callers.Add(1)
		go func() {
			defer callers.Done()
			if err := generation.shutdown(context.Background()); err != nil {
				t.Errorf("shutdown error = %v", err)
			}
		}()
	}
	callers.Wait()

	if got := orphanCalls.Load(); got != 1 {
		t.Fatalf("orphan calls = %d, want 1", got)
	}
	select {
	case <-done:
	default:
		t.Fatal("shutdown returned before Session.Done closed")
	}
}

func TestShutdownGenerationPreservesOrphanErrorAfterJoin(t *testing.T) {
	wantErr := context.DeadlineExceeded
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	close(done)
	generation := &generation{
		ctx:          ctx,
		cancel:       cancel,
		session:      &concurrency.Session{},
		shutdownDone: make(chan struct{}),
		ops: etcdOps{
			orphanSession: func(*concurrency.Session) error { return wantErr },
			sessionDone:   func(*concurrency.Session) <-chan struct{} { return done },
		},
	}

	if err := generation.shutdown(context.Background()); !errors.Is(err, wantErr) {
		t.Fatalf("shutdown error = %v, want %v", err, wantErr)
	}
	if err := generation.shutdown(context.Background()); !errors.Is(err, wantErr) {
		t.Fatalf("second shutdown error = %v, want %v", err, wantErr)
	}
}
