package graphtest_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/bluetape4k/bluetape-go/graph"
	"github.com/bluetape4k/bluetape-go/graph/graphtest"
)

var errExampleProvider = errors.New("example provider")

type exampleState struct {
	mu       sync.Mutex
	vertices []graph.Vertex
	edges    []graph.Edge
}

func ExampleRun() {
	_ = exampleHarness() // TestBackend(t)에서 graphtest.Run(t, exampleHarness())로 호출한다.
}

func ExampleRunWithConfig() {
	config := graphtest.DefaultConfig()
	config.MaxVertices = graphtest.MaxResultLimit
	_ = config // TestBackend(t)에서 graphtest.RunWithConfig(t, exampleHarness(), config)로 호출한다.
}

func ExampleCapabilities() {
	capabilities := graphtest.Capabilities{
		graphtest.CapabilityTraversal: {Enabled: false, ReasonCode: "query-language-limit"},
	}
	_ = capabilities
}

func exampleHarness() graphtest.Harness {
	return graphtest.Harness{
		Provider: graphtest.ProviderMetadata{
			Name:           "example",
			Version:        "1.0.0",
			ImageReference: "example:1@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		New: func(context.Context, testing.TB, graphtest.Config) (graphtest.Adapter, error) {
			state := &exampleState{}
			return graphtest.Adapter{
				VerifyConnectivity: func(context.Context) error { return nil },
				CreateFixture: func(_ context.Context, fixture graphtest.Fixture) error {
					state.mu.Lock()
					defer state.mu.Unlock()
					state.vertices, state.edges = fixture.Vertices(), fixture.Edges()
					return nil
				},
				ReadVertices: func(context.Context, graphtest.Fixture) ([]graph.Vertex, error) {
					state.mu.Lock()
					defer state.mu.Unlock()
					return append([]graph.Vertex(nil), state.vertices...), nil
				},
				ReadEdges: func(context.Context, graphtest.Fixture) ([]graph.Edge, error) {
					state.mu.Lock()
					defer state.mu.Unlock()
					return append([]graph.Edge(nil), state.edges...), nil
				},
				InvalidOperation: func(context.Context, graphtest.Fixture) error {
					return fmt.Errorf("example operation: %w", errExampleProvider)
				},
				BlockUntilCanceled: func(ctx context.Context, _ graphtest.Fixture, started graphtest.Started) error {
					started()
					<-ctx.Done()
					return ctx.Err()
				},
				CleanupFixture: func(context.Context, graphtest.Fixture) error {
					state.mu.Lock()
					defer state.mu.Unlock()
					state.vertices, state.edges = nil, nil
					return nil
				},
				Close: func(context.Context) error { return nil },
				IsProviderError: func(err error) bool {
					return errors.Is(err, errExampleProvider)
				},
			}, nil
		},
		Capabilities: graphtest.Capabilities{
			graphtest.CapabilityTraversal: {Enabled: false, ReasonCode: "query-language-limit"},
		},
	}
}

func TestExampleHarnessConforms(t *testing.T) {
	graphtest.Run(t, exampleHarness())
}
