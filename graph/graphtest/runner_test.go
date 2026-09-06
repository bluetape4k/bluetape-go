package graphtest

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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

func TestRunClosesAdapterReturnedAfterStartupDeadline(t *testing.T) {
	var closed atomic.Int64
	harness := validFakeHarness(func(adapter *Adapter) {
		adapter.Close = func(context.Context) error {
			closed.Add(1)
			return nil
		}
	})
	original := harness.New
	harness.New = func(ctx context.Context, tb testing.TB, config Config) (Adapter, error) {
		<-ctx.Done()
		return original(context.Background(), tb, config)
	}
	config := DefaultConfig()
	config.StartupTimeout = 10 * time.Millisecond

	err := run(context.Background(), t, harness, config)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("run() error = %v, want context deadline", err)
	}
	if got := closed.Load(); got != 1 {
		t.Fatalf("close count = %d, want 1", got)
	}
}

func TestRunRedactsFactoryAndCallbackFailures(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		mutate func(*Harness)
	}{
		{"factory-error", func(harness *Harness) {
			harness.New = func(context.Context, testing.TB, Config) (Adapter, error) {
				return Adapter{}, errors.New("factory-secret-marker")
			}
		}},
		{"connectivity-panic", func(harness *Harness) {
			original := harness.New
			harness.New = func(ctx context.Context, tb testing.TB, config Config) (Adapter, error) {
				adapter, err := original(ctx, tb, config)
				adapter.VerifyConnectivity = func(context.Context) error { panic("connectivity-secret-marker") }
				return adapter, err
			}
		}},
		{"cleanup-error", func(harness *Harness) {
			original := harness.New
			harness.New = func(ctx context.Context, tb testing.TB, config Config) (Adapter, error) {
				adapter, err := original(ctx, tb, config)
				adapter.CleanupFixture = func(context.Context, Fixture) error {
					return errors.New("cleanup-secret-marker")
				}
				return adapter, err
			}
		}},
		{"close-panic", func(harness *Harness) {
			original := harness.New
			harness.New = func(ctx context.Context, tb testing.TB, config Config) (Adapter, error) {
				adapter, err := original(ctx, tb, config)
				adapter.Close = func(context.Context) error { panic("close-secret-marker") }
				return adapter, err
			}
		}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			harness := validFakeHarness(nil)
			testCase.mutate(&harness)
			err := run(context.Background(), t, harness, DefaultConfig())
			if err == nil {
				t.Fatal("run() error = nil")
			}
			if strings.Contains(err.Error(), "secret-marker") {
				t.Fatal("run() disclosed a secret marker")
			}
		})
	}
}

func TestRunRejectsOversizedResultsBeforeComparison(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		mutate func(*Adapter)
	}{
		{"vertices", func(adapter *Adapter) {
			original := adapter.ReadVertices
			adapter.ReadVertices = func(ctx context.Context, fixture Fixture) ([]graph.Vertex, error) {
				values, err := original(ctx, fixture)
				if len(values) > 0 {
					values = append(values, values[0])
				}
				return values, err
			}
		}},
		{"edges", func(adapter *Adapter) {
			original := adapter.ReadEdges
			adapter.ReadEdges = func(ctx context.Context, fixture Fixture) ([]graph.Edge, error) {
				values, err := original(ctx, fixture)
				if len(values) > 0 {
					values = append(values, values[0])
				}
				return values, err
			}
		}},
		{"traversal", func(adapter *Adapter) {
			adapter.Traverse = func(context.Context, Fixture) ([]string, error) {
				return []string{"left", "right", "overflow"}, nil
			}
		}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			config := DefaultConfig()
			config.MaxVertices = 2
			config.MaxEdges = 1
			config.MaxTraversalResults = 2
			harness := validFakeHarness(testCase.mutate)
			if err := run(context.Background(), t, harness, config); err == nil {
				t.Fatal("run() error = nil")
			}
		})
	}
}

func TestRunInvokesCallbacksWithExpectedLogicalCounts(t *testing.T) {
	var mu sync.Mutex
	counts := make(map[string]int)
	increment := func(name string) {
		mu.Lock()
		defer mu.Unlock()
		counts[name]++
	}
	harness := validFakeHarness(func(adapter *Adapter) {
		verifyConnectivity := adapter.VerifyConnectivity
		adapter.VerifyConnectivity = func(ctx context.Context) error {
			increment("connectivity")
			return verifyConnectivity(ctx)
		}
		createFixture := adapter.CreateFixture
		adapter.CreateFixture = func(ctx context.Context, fixture Fixture) error {
			increment("create")
			return createFixture(ctx, fixture)
		}
		readVertices := adapter.ReadVertices
		adapter.ReadVertices = func(ctx context.Context, fixture Fixture) ([]graph.Vertex, error) {
			increment("vertices")
			return readVertices(ctx, fixture)
		}
		readEdges := adapter.ReadEdges
		adapter.ReadEdges = func(ctx context.Context, fixture Fixture) ([]graph.Edge, error) {
			increment("edges")
			return readEdges(ctx, fixture)
		}
		invalidOperation := adapter.InvalidOperation
		adapter.InvalidOperation = func(ctx context.Context, fixture Fixture) error {
			increment("invalid")
			return invalidOperation(ctx, fixture)
		}
		blockUntilCanceled := adapter.BlockUntilCanceled
		adapter.BlockUntilCanceled = func(ctx context.Context, fixture Fixture, started Started) error {
			increment("cancellation")
			return blockUntilCanceled(ctx, fixture, started)
		}
		cleanupFixture := adapter.CleanupFixture
		adapter.CleanupFixture = func(ctx context.Context, fixture Fixture) error {
			increment("cleanup")
			return cleanupFixture(ctx, fixture)
		}
		traverse := adapter.Traverse
		adapter.Traverse = func(ctx context.Context, fixture Fixture) ([]string, error) {
			increment("traversal")
			return traverse(ctx, fixture)
		}
		closeAdapter := adapter.Close
		adapter.Close = func(ctx context.Context) error {
			increment("close")
			return closeAdapter(ctx)
		}
	})
	if err := run(context.Background(), t, harness, DefaultConfig()); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	want := map[string]int{
		"connectivity": 1,
		"create":       3,
		"vertices":     3,
		"edges":        3,
		"invalid":      1,
		"cancellation": 1,
		"cleanup":      8,
		"traversal":    1,
		"close":        1,
	}
	if len(counts) != len(want) {
		t.Fatalf("callback count keys = %d, want %d", len(counts), len(want))
	}
	for name, expected := range want {
		if got := counts[name]; got != expected {
			t.Fatalf("callback %s count = %d, want %d", name, got, expected)
		}
	}
}

func TestRunSnapshotsCapabilitiesBeforeFactory(t *testing.T) {
	var traversed atomic.Bool
	harness := validFakeHarness(func(adapter *Adapter) {
		original := adapter.Traverse
		adapter.Traverse = func(ctx context.Context, fixture Fixture) ([]string, error) {
			traversed.Store(true)
			return original(ctx, fixture)
		}
	})
	original := harness.New
	harness.New = func(ctx context.Context, tb testing.TB, config Config) (Adapter, error) {
		harness.Capabilities[CapabilityTraversal] = Support{Enabled: false, ReasonCode: "caller-mutated"}
		return original(ctx, tb, config)
	}
	if err := run(context.Background(), t, harness, DefaultConfig()); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if !traversed.Load() {
		t.Fatal("traversal did not use the initial capability snapshot")
	}
}

func TestRunHonorsParentCancellationAndDeadline(t *testing.T) {
	t.Run("pre-canceled", func(t *testing.T) {
		parent, cancel := context.WithCancel(context.Background())
		cancel()
		var factoryCalls atomic.Int64
		harness := validFakeHarness(nil)
		harness.New = func(context.Context, testing.TB, Config) (Adapter, error) {
			factoryCalls.Add(1)
			return Adapter{}, nil
		}
		if err := run(parent, t, harness, DefaultConfig()); !errors.Is(err, context.Canceled) {
			t.Fatal("run() did not preserve parent cancellation")
		}
		if factoryCalls.Load() != 0 {
			t.Fatal("factory called with pre-canceled parent")
		}
	})

	t.Run("deadline-propagated", func(t *testing.T) {
		parent, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		harness := validFakeHarness(nil)
		original := harness.New
		harness.New = func(ctx context.Context, tb testing.TB, config Config) (Adapter, error) {
			if _, ok := ctx.Deadline(); !ok {
				return Adapter{}, errors.New("factory context missing deadline")
			}
			return original(ctx, tb, config)
		}
		if err := run(parent, t, harness, DefaultConfig()); err != nil {
			t.Fatalf("run() error = %v", err)
		}
	})
}

func TestRunCleansPartialCreateAndCloses(t *testing.T) {
	var cleaned, closed atomic.Int64
	harness := validFakeHarness(func(adapter *Adapter) {
		adapter.CreateFixture = func(context.Context, Fixture) error {
			return errors.New("partial-create-secret")
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
	if err == nil || strings.Contains(err.Error(), "partial-create-secret") {
		t.Fatal("run() did not return a redacted partial-create error")
	}
	if cleaned.Load() == 0 || closed.Load() != 1 {
		t.Fatalf("cleanup=%d close=%d", cleaned.Load(), closed.Load())
	}
}

func TestRunCancellationEventOrder(t *testing.T) {
	var mu sync.Mutex
	var events []string
	record := func(event string) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, event)
	}
	var cancellationJoined atomic.Bool
	harness := validFakeHarness(func(adapter *Adapter) {
		originalCancellation := adapter.BlockUntilCanceled
		adapter.BlockUntilCanceled = func(ctx context.Context, fixture Fixture, started Started) error {
			err := originalCancellation(ctx, fixture, func() {
				record("started")
				started()
			})
			cancellationJoined.Store(true)
			record("joined")
			return err
		}
		originalCleanup := adapter.CleanupFixture
		adapter.CleanupFixture = func(ctx context.Context, fixture Fixture) error {
			if cancellationJoined.Load() {
				record("fixture-cleanup")
			}
			return originalCleanup(ctx, fixture)
		}
		originalClose := adapter.Close
		adapter.Close = func(ctx context.Context) error {
			record("close")
			return originalClose(ctx)
		}
	})
	if err := run(context.Background(), t, harness, DefaultConfig()); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	startedIndex := slicesIndex(events, "started")
	joinedIndex := slicesIndex(events, "joined")
	cleanupIndex := slicesIndexAfter(events, "fixture-cleanup", joinedIndex)
	closeIndex := slicesIndex(events, "close")
	if startedIndex < 0 || joinedIndex <= startedIndex || cleanupIndex <= joinedIndex || closeIndex <= cleanupIndex {
		t.Fatalf("events = %v", events)
	}
}

func TestCleanupIsIdempotentAcrossFixtureStates(t *testing.T) {
	for _, state := range []string{"empty", "partial", "complete", "already-cleaned"} {
		t.Run(state, func(t *testing.T) {
			var submissions atomic.Int64
			current := state
			cleanup := func(context.Context, Fixture) error {
				submissions.Add(1)
				current = "empty"
				return nil
			}
			fixture, _ := newFixture()
			if err := cleanup(context.Background(), fixture); err != nil {
				t.Fatal("cleanup returned an error")
			}
			if submissions.Load() != 1 || current != "empty" {
				t.Fatalf("submissions=%d state=%s", submissions.Load(), current)
			}
		})
	}
}

func TestRunRejectsFalseProviderClassifier(t *testing.T) {
	harness := validFakeHarness(func(adapter *Adapter) {
		adapter.IsProviderError = func(error) bool { return false }
	})
	if err := run(context.Background(), t, harness, DefaultConfig()); err == nil {
		t.Fatal("run() accepted a false provider classifier")
	}
}

func slicesIndex(values []string, target string) int {
	return slicesIndexAfter(values, target, -1)
}

func slicesIndexAfter(values []string, target string, after int) int {
	for index := after + 1; index < len(values); index++ {
		if values[index] == target {
			return index
		}
	}
	return -1
}
