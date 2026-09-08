package graphtest

import (
	"context"
	"errors"
	"testing"

	"github.com/bluetape4k/bluetape-go/graph"
)

func completeAdapter() Adapter {
	return Adapter{
		VerifyConnectivity: func(context.Context) error { return nil },
		CreateFixture:      func(context.Context, Fixture) error { return nil },
		ReadVertices:       func(context.Context, Fixture) ([]graph.Vertex, error) { return []graph.Vertex{}, nil },
		ReadEdges:          func(context.Context, Fixture) ([]graph.Edge, error) { return []graph.Edge{}, nil },
		InvalidOperation:   func(context.Context, Fixture) error { return errors.New("provider") },
		BlockUntilCanceled: func(ctx context.Context, _ Fixture, started Started) error {
			started()
			<-ctx.Done()
			return ctx.Err()
		},
		CleanupFixture: func(context.Context, Fixture) error { return nil },
		Close:          func(context.Context) error { return nil },
		Traverse:       func(context.Context, Fixture) ([]string, error) { return []string{"left", "right"}, nil },
		IsProviderError: func(err error) bool {
			return err != nil && err.Error() == "provider"
		},
	}
}

func TestValidateAdapterRejectsMissingAndOverbroadClassifier(t *testing.T) {
	a := completeAdapter()
	caps := Capabilities{CapabilityTraversal: {Enabled: true}}
	if err := validateAdapter(a, caps); err != nil {
		t.Fatalf("validateAdapter() error = %v", err)
	}
	a.ReadEdges = nil
	if err := validateAdapter(a, caps); err == nil {
		t.Fatal("nil ReadEdges accepted")
	}
	a = completeAdapter()
	a.IsProviderError = func(error) bool { return true }
	if err := validateAdapter(a, caps); err == nil {
		t.Fatal("always-true classifier accepted")
	}
}

func TestSnapshotCapabilitiesIsIndependent(t *testing.T) {
	source := Capabilities{CapabilityTraversal: {Enabled: false, ReasonCode: "unsupported-by-fake"}}
	got := snapshotCapabilities(source)
	source[CapabilityTraversal] = Support{Enabled: true}
	if got[CapabilityTraversal].Enabled {
		t.Fatal("snapshot observed caller mutation")
	}
}

func TestValidatePositiveClassifierRequiresDirectAndWrappedMatches(t *testing.T) {
	providerErr := errors.New("provider")
	if err := validatePositiveClassifier(func(err error) bool {
		return errors.Is(err, providerErr)
	}, providerErr); err != nil {
		t.Fatalf("validatePositiveClassifier() error = %v", err)
	}
	if err := validatePositiveClassifier(func(err error) bool {
		return err != nil && err.Error() == providerErr.Error()
	}, providerErr); err == nil {
		t.Fatal("direct-only classifier accepted")
	}
	if err := validatePositiveClassifier(func(error) bool { return true }, nil); err == nil {
		t.Fatal("nil provider error accepted")
	}
}

func TestValidateAdapterRejectsEveryMissingCoreCallback(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		mutate func(*Adapter)
	}{
		{"verify-connectivity", func(adapter *Adapter) { adapter.VerifyConnectivity = nil }},
		{"create-fixture", func(adapter *Adapter) { adapter.CreateFixture = nil }},
		{"read-vertices", func(adapter *Adapter) { adapter.ReadVertices = nil }},
		{"read-edges", func(adapter *Adapter) { adapter.ReadEdges = nil }},
		{"invalid-operation", func(adapter *Adapter) { adapter.InvalidOperation = nil }},
		{"block-until-canceled", func(adapter *Adapter) { adapter.BlockUntilCanceled = nil }},
		{"cleanup-fixture", func(adapter *Adapter) { adapter.CleanupFixture = nil }},
		{"close", func(adapter *Adapter) { adapter.Close = nil }},
		{"classifier", func(adapter *Adapter) { adapter.IsProviderError = nil }},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			adapter := completeAdapter()
			testCase.mutate(&adapter)
			if err := validateAdapter(adapter, Capabilities{CapabilityTraversal: {Enabled: true}}); err == nil {
				t.Fatal("validateAdapter() error = nil")
			}
		})
	}
}

func TestValidateAdapterRejectsClassifierPanic(t *testing.T) {
	adapter := completeAdapter()
	adapter.IsProviderError = func(error) bool { panic("classifier-secret") }
	if err := validateAdapter(adapter, Capabilities{CapabilityTraversal: {Enabled: true}}); err == nil {
		t.Fatal("validateAdapter() error = nil")
	}
}
