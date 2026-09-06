package graphtest

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/graph"
)

// Run 함수는 t.Context를 parent로 사용해 기본 Config로 strict conformance suite를 실행한다.
func Run(t *testing.T, harness Harness) {
	t.Helper()
	RunWithConfig(t, harness, DefaultConfig())
}

// RunWithConfig 함수는 t.Context의 cancellation/deadline을 소유하지 않고 전달한다.
// Config의 zero value나 partial value는 허용하지 않는다.
func RunWithConfig(t *testing.T, harness Harness, config Config) {
	t.Helper()
	if err := run(t.Context(), t, harness, config); err != nil {
		t.Fatal(err)
	}
}

func run(parent context.Context, t *testing.T, harness Harness, config Config) (returnErr error) {
	if err := validateHarness(harness, config); err != nil {
		return err
	}
	capabilities := snapshotCapabilities(harness.Capabilities)
	startup := call(parent, config.StartupTimeout, func(ctx context.Context) (Adapter, error) {
		return harness.New(ctx, t, config)
	})
	adapter := startup.value
	if adapter.Close != nil {
		defer func() {
			if recover() != nil {
				returnErr = errors.Join(returnErr, errors.New("graphtest: runner panic"))
			}
			closeStarted := time.Now()
			closeResult := call(context.WithoutCancel(parent), config.CloseTimeout, func(ctx context.Context) (struct{}, error) {
				return struct{}{}, adapter.Close(ctx)
			})
			closeStatus, closeCategory, closeTimedOut := callbackStatus(closeResult)
			t.Logf(
				"graphtest provider=%s phase=close status=%s category=%s timeout=%t duration=%s",
				harness.Provider.Name,
				closeStatus,
				closeCategory,
				closeTimedOut,
				time.Since(closeStarted),
			)
			returnErr = errors.Join(returnErr, callbackError("close", closeResult))
		}()
	}
	if err := callbackError("factory", startup); err != nil {
		return err
	}
	if err := validateAdapter(adapter, capabilities); err != nil {
		return err
	}
	t.Logf(
		"graphtest provider=%s version=%s image_digest=%s phase=start",
		harness.Provider.Name,
		harness.Provider.Version,
		imageDigest(harness.Provider.ImageReference),
	)
	defer func(start time.Time) {
		t.Logf(
			"graphtest provider=%s version=%s phase=run duration=%s",
			harness.Provider.Name,
			harness.Provider.Version,
			time.Since(start),
		)
	}(time.Now())
	traversalSupport := capabilities[CapabilityTraversal]
	if traversalSupport.Enabled {
		t.Logf("graphtest provider=%s capability=%s status=enabled", harness.Provider.Name, CapabilityTraversal)
	} else {
		t.Logf(
			"graphtest provider=%s capability=%s status=disabled reason=%s",
			harness.Provider.Name,
			CapabilityTraversal,
			traversalSupport.ReasonCode,
		)
	}

	var scenarioErr error
	for _, testCase := range []struct {
		name string
		run  func(context.Context, Adapter, Fixture, Config) error
	}{
		{"connectivity", caseConnectivity},
		{"empty-read", caseEmptyRead},
		{"create-read", caseCreateRead},
		{"cancellation", caseCancellation},
		{"provider-error", caseProviderError},
		{"cleanup", caseCleanup},
	} {
		fixture, fixtureErr := newFixture()
		if fixtureErr != nil {
			scenarioErr = fixtureErr
			break
		}
		scenarioErr = runScenario(
			parent,
			t,
			harness.Provider.Name,
			testCase.name,
			adapter,
			fixture,
			config,
			testCase.run,
		)
		if scenarioErr != nil {
			break
		}
	}
	if scenarioErr == nil && capabilities[CapabilityTraversal].Enabled {
		fixture, fixtureErr := newFixture()
		if fixtureErr != nil {
			scenarioErr = fixtureErr
		} else {
			scenarioErr = runScenario(
				parent,
				t,
				harness.Provider.Name,
				"traversal",
				adapter,
				fixture,
				config,
				caseTraversal,
			)
		}
	}
	return scenarioErr
}

func runScenario(
	parent context.Context,
	t *testing.T,
	provider string,
	name string,
	adapter Adapter,
	fixture Fixture,
	config Config,
	fn func(context.Context, Adapter, Fixture, Config) error,
) (returnErr error) {
	started := time.Now()
	defer func() {
		if recover() != nil {
			returnErr = errors.Join(returnErr, errors.New("graphtest: scenario panic"))
		}
		cleanupStarted := time.Now()
		cleanupResult := call(context.WithoutCancel(parent), config.CleanupTimeout, func(ctx context.Context) (struct{}, error) {
			return struct{}{}, adapter.CleanupFixture(ctx, fixture)
		})
		cleanupStatus, cleanupCategory, cleanupTimedOut := callbackStatus(cleanupResult)
		t.Logf(
			"graphtest provider=%s phase=fixture-cleanup status=%s category=%s timeout=%t duration=%s",
			provider,
			cleanupStatus,
			cleanupCategory,
			cleanupTimedOut,
			time.Since(cleanupStarted),
		)
		returnErr = errors.Join(returnErr, callbackError("fixture cleanup", cleanupResult))
		scenarioStatus := "ok"
		if returnErr != nil {
			scenarioStatus = "error"
		}
		t.Logf(
			"graphtest provider=%s phase=%s status=%s duration=%s",
			provider,
			name,
			scenarioStatus,
			time.Since(started),
		)
	}()
	return fn(parent, adapter, fixture, config)
}

func bounded[T any](
	parent context.Context,
	timeout time.Duration,
	phase string,
	fn func(context.Context) (T, error),
) (T, error) {
	result := call(parent, timeout, fn)
	if err := callbackError(phase, result); err != nil {
		var zero T
		return zero, err
	}
	return result.value, nil
}

func caseConnectivity(ctx context.Context, adapter Adapter, _ Fixture, config Config) error {
	_, err := bounded(ctx, config.CaseTimeout, "connectivity", func(ctx context.Context) (struct{}, error) {
		return struct{}{}, adapter.VerifyConnectivity(ctx)
	})
	return err
}

func caseEmptyRead(ctx context.Context, adapter Adapter, fixture Fixture, config Config) error {
	vertices, err := bounded(ctx, config.CaseTimeout, "empty vertices", func(ctx context.Context) ([]graph.Vertex, error) {
		return adapter.ReadVertices(ctx, fixture)
	})
	if err != nil {
		return err
	}
	if len(vertices) != 0 {
		return errors.New("graphtest: empty vertex read was not empty")
	}
	edges, err := bounded(ctx, config.CaseTimeout, "empty edges", func(ctx context.Context) ([]graph.Edge, error) {
		return adapter.ReadEdges(ctx, fixture)
	})
	if err != nil {
		return err
	}
	if len(edges) != 0 {
		return errors.New("graphtest: empty edge read was not empty")
	}
	return nil
}

func caseCancellation(ctx context.Context, adapter Adapter, fixture Fixture, config Config) error {
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err := bounded(canceled, config.CaseTimeout, "pre-canceled read", func(ctx context.Context) ([]graph.Vertex, error) {
		return adapter.ReadVertices(ctx, fixture)
	})
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		return errors.New("graphtest: pre-canceled context was not preserved")
	}
	err = exerciseCancellation(ctx, adapter, fixture, config)
	if errors.Is(err, errCancellationReturnedBeforeStart) ||
		errors.Is(err, errCancellationStartTimeout) ||
		errors.Is(err, errCancellationDuplicateStart) {
		return err
	}
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		return errors.New("graphtest: in-flight cancellation was not preserved")
	}
	return nil
}

func caseProviderError(ctx context.Context, adapter Adapter, fixture Fixture, config Config) error {
	_, err := bounded(ctx, config.CaseTimeout, "provider error", func(ctx context.Context) (struct{}, error) {
		return struct{}{}, adapter.InvalidOperation(ctx, fixture)
	})
	if err == nil {
		return errors.New("graphtest: invalid operation returned nil")
	}
	return validatePositiveClassifier(adapter.IsProviderError, err)
}

func caseCreateRead(ctx context.Context, adapter Adapter, fixture Fixture, config Config) error {
	if _, err := bounded(ctx, config.CaseTimeout, "create fixture", func(ctx context.Context) (struct{}, error) {
		return struct{}{}, adapter.CreateFixture(ctx, fixture)
	}); err != nil {
		return err
	}
	vertices, err := bounded(ctx, config.CaseTimeout, "read vertices", func(ctx context.Context) ([]graph.Vertex, error) {
		return adapter.ReadVertices(ctx, fixture)
	})
	if err != nil {
		return err
	}
	actualVertices, err := canonicalVertices(vertices, config.MaxVertices)
	if err != nil {
		return err
	}
	expectedVertices, _ := canonicalVertices(fixture.Vertices(), config.MaxVertices)
	if len(actualVertices) != len(expectedVertices) {
		return errors.New("graphtest: vertex count mismatch")
	}
	actualIDs := make(map[string]string, len(actualVertices))
	for i := range actualVertices {
		actualKey, ok := logicalKey(actualVertices[i].Properties())
		if !ok {
			return errors.New("graphtest: vertex logical key missing")
		}
		expectedKey, _ := logicalKey(expectedVertices[i].Properties())
		if actualKey != expectedKey ||
			actualVertices[i].Label() != expectedVertices[i].Label() ||
			!reflect.DeepEqual(actualVertices[i].Properties(), expectedVertices[i].Properties()) {
			return errors.New("graphtest: vertex semantic mismatch")
		}
		actualIDs[actualVertices[i].ID().String()] = actualKey
	}
	edges, err := bounded(ctx, config.CaseTimeout, "read edges", func(ctx context.Context) ([]graph.Edge, error) {
		return adapter.ReadEdges(ctx, fixture)
	})
	if err != nil {
		return err
	}
	actualEdges, err := canonicalEdges(edges, config.MaxEdges)
	if err != nil {
		return err
	}
	expectedEdges, _ := canonicalEdges(fixture.Edges(), config.MaxEdges)
	if len(actualEdges) != len(expectedEdges) {
		return errors.New("graphtest: edge count mismatch")
	}
	for i := range actualEdges {
		if actualEdges[i].Label() != expectedEdges[i].Label() ||
			!reflect.DeepEqual(actualEdges[i].Properties(), expectedEdges[i].Properties()) ||
			actualIDs[actualEdges[i].StartID().String()] != expectedEdges[i].StartID().String() ||
			actualIDs[actualEdges[i].EndID().String()] != expectedEdges[i].EndID().String() {
			return errors.New("graphtest: edge semantic mismatch")
		}
	}
	return nil
}

func caseCleanup(ctx context.Context, adapter Adapter, fixture Fixture, config Config) error {
	if _, err := bounded(ctx, config.CaseTimeout, "create cleanup fixture", func(ctx context.Context) (struct{}, error) {
		return struct{}{}, adapter.CreateFixture(ctx, fixture)
	}); err != nil {
		return err
	}
	cleanupResult := call(context.WithoutCancel(ctx), config.CleanupTimeout, func(cleanupCtx context.Context) (struct{}, error) {
		return struct{}{}, adapter.CleanupFixture(cleanupCtx, fixture)
	})
	if err := callbackError("cleanup", cleanupResult); err != nil {
		return err
	}
	vertices, err := bounded(ctx, config.CaseTimeout, "cleanup read vertices", func(ctx context.Context) ([]graph.Vertex, error) {
		return adapter.ReadVertices(ctx, fixture)
	})
	if err != nil {
		return err
	}
	if len(vertices) != 0 {
		return errors.New("graphtest: cleanup left vertices")
	}
	edges, err := bounded(ctx, config.CaseTimeout, "cleanup read edges", func(ctx context.Context) ([]graph.Edge, error) {
		return adapter.ReadEdges(ctx, fixture)
	})
	if err != nil {
		return err
	}
	if len(edges) != 0 {
		return errors.New("graphtest: cleanup left edges")
	}
	return nil
}

func caseTraversal(ctx context.Context, adapter Adapter, fixture Fixture, config Config) error {
	if _, err := bounded(ctx, config.CaseTimeout, "create traversal fixture", func(ctx context.Context) (struct{}, error) {
		return struct{}{}, adapter.CreateFixture(ctx, fixture)
	}); err != nil {
		return err
	}
	keys, err := bounded(ctx, config.CaseTimeout, "traversal", func(ctx context.Context) ([]string, error) {
		return adapter.Traverse(ctx, fixture)
	})
	if err != nil {
		return err
	}
	if len(keys) > config.MaxTraversalResults {
		return errors.New("graphtest: traversal result limit exceeded")
	}
	if !slices.Equal(keys, []string{"left", "right"}) {
		return errors.New("graphtest: traversal semantic mismatch")
	}
	return nil
}
