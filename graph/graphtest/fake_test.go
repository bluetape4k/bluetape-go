package graphtest

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/bluetape4k/bluetape-go/graph"
)

type fakeState struct {
	mu       sync.Mutex
	vertices []graph.Vertex
	edges    []graph.Edge
	queries  map[string]int
	closed   int
}

func validFakeHarness(mutate func(*Adapter)) Harness {
	return Harness{
		Provider: ProviderMetadata{
			Name:           "fake",
			Version:        "1.0.0",
			ImageReference: "fake:1@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		New: func(context.Context, testing.TB, Config) (Adapter, error) {
			state := &fakeState{queries: make(map[string]int)}
			adapter := Adapter{
				VerifyConnectivity: func(ctx context.Context) error { return ctx.Err() },
				CreateFixture: func(ctx context.Context, fixture Fixture) error {
					if err := ctx.Err(); err != nil {
						return err
					}
					state.mu.Lock()
					defer state.mu.Unlock()
					state.queries["create"]++
					state.vertices, state.edges = fixture.Vertices(), fixture.Edges()
					return nil
				},
				ReadVertices: func(ctx context.Context, _ Fixture) ([]graph.Vertex, error) {
					if err := ctx.Err(); err != nil {
						return nil, err
					}
					state.mu.Lock()
					defer state.mu.Unlock()
					state.queries["vertices"]++
					return cloneVertices(state.vertices), nil
				},
				ReadEdges: func(ctx context.Context, _ Fixture) ([]graph.Edge, error) {
					if err := ctx.Err(); err != nil {
						return nil, err
					}
					state.mu.Lock()
					defer state.mu.Unlock()
					state.queries["edges"]++
					return cloneEdges(state.edges), nil
				},
				InvalidOperation: func(context.Context, Fixture) error {
					return &providerProbeError{cause: errors.New("fake cause")}
				},
				BlockUntilCanceled: func(ctx context.Context, _ Fixture, started Started) error {
					started()
					<-ctx.Done()
					return ctx.Err()
				},
				CleanupFixture: func(context.Context, Fixture) error {
					state.mu.Lock()
					defer state.mu.Unlock()
					state.queries["cleanup"]++
					state.vertices, state.edges = nil, nil
					return nil
				},
				Close: func(context.Context) error {
					state.mu.Lock()
					defer state.mu.Unlock()
					state.closed++
					if state.closed > 1 {
						return errors.New("fake adapter closed more than once")
					}
					return nil
				},
				Traverse: func(context.Context, Fixture) ([]string, error) {
					state.mu.Lock()
					defer state.mu.Unlock()
					state.queries["traverse"]++
					return []string{"left", "right"}, nil
				},
				IsProviderError: func(err error) bool {
					var target *providerProbeError
					return errors.As(err, &target)
				},
			}
			if mutate != nil {
				mutate(&adapter)
			}
			return adapter, nil
		},
		Capabilities: Capabilities{CapabilityTraversal: {Enabled: true}},
	}
}
