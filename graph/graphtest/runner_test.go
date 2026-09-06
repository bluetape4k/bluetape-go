package graphtest

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bluetape4k/bluetape-go/graph"
)

func TestRunExecutesCoreAndTraversalWithExactlyOnceClose(t *testing.T) {
	var closed atomic.Int64
	harness := validFakeHarness(func(adapter *Adapter) {
		adapter.Close = func(context.Context) error {
			closed.Add(1)
			return nil
		}
	})
	if err := run(context.Background(), t, harness, DefaultConfig()); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got := closed.Load(); got != 1 {
		t.Fatalf("close count = %d, want 1", got)
	}
}

func TestRunStopsAfterCoreFailureButCleansAndCloses(t *testing.T) {
	var cleaned, closed, laterCalls atomic.Int64
	harness := validFakeHarness(func(adapter *Adapter) {
		adapter.ReadVertices = func(context.Context, Fixture) ([]graph.Vertex, error) {
			return nil, errors.New("secret-marker")
		}
		adapter.ReadEdges = func(context.Context, Fixture) ([]graph.Edge, error) {
			laterCalls.Add(1)
			return nil, nil
		}
		adapter.CleanupFixture = func(context.Context, Fixture) error {
			cleaned.Add(1)
			return nil
		}
		adapter.Close = func(context.Context) error {
			closed.Add(1)
			return nil
		}
	})
	err := run(context.Background(), t, harness, DefaultConfig())
	if err == nil {
		t.Fatal("run() error = nil")
	}
	if strings.Contains(err.Error(), "secret-marker") {
		t.Fatal("run() disclosed the secret marker")
	}
	if cleaned.Load() == 0 || closed.Load() != 1 || laterCalls.Load() != 0 {
		t.Fatalf("cleanup=%d close=%d later=%d", cleaned.Load(), closed.Load(), laterCalls.Load())
	}
}

func TestRunRedactsFactoryPanic(t *testing.T) {
	harness := validFakeHarness(nil)
	harness.New = func(context.Context, testing.TB, Config) (Adapter, error) {
		panic("factory-secret")
	}
	err := run(context.Background(), t, harness, DefaultConfig())
	if err == nil || strings.Contains(err.Error(), "factory-secret") {
		t.Fatalf("run() error = %v", err)
	}
}
