package gremlin

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/graph"
	"github.com/bluetape4k/bluetape-go/graph/graphtest"
	tinkerpoptestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/tinkerpop"
)

const (
	tinkerPopImage = "tinkerpop/gremlin-server:3.8.1@sha256:d7b23b4b6773a521cb70cf82c68584a6c68e35019c1357ab4c9371c4e843d651"
	gremlinVersion = "3.8.1"
)

func TestConformance(t *testing.T) {
	graphtest.RunWithConfig(t, gremlinHarness(), graphtest.DefaultConfig())
}

func gremlinHarness() graphtest.Harness {
	return graphtest.Harness{
		Provider: graphtest.ProviderMetadata{
			Name:           "tinkerpop",
			Version:        gremlinVersion,
			ImageReference: tinkerPopImage,
		},
		New: func(ctx context.Context, tb testing.TB, config graphtest.Config) (graphtest.Adapter, error) {
			endpoint := tinkerpoptestcontainer.Start(ctx, tb)
			client, err := NewRemoteClient(endpoint, WithMaxResults(graphtest.MaxResultLimit))
			if err != nil {
				return graphtest.Adapter{}, err
			}
			return newConformanceAdapter(client, config), nil
		},
		Capabilities: graphtest.Capabilities{
			graphtest.CapabilityTraversal: {Enabled: true},
		},
	}
}

func newConformanceAdapter(client *Client, config graphtest.Config) graphtest.Adapter {
	return graphtest.Adapter{
		VerifyConnectivity: func(ctx context.Context) error {
			return sanitizeProviderError("verify connectivity", client.VerifyConnectivity(ctx))
		},
		CreateFixture: func(ctx context.Context, fixture graphtest.Fixture) error {
			vertices := fixture.Vertices()
			for _, vertex := range vertices {
				properties := vertex.Properties()
				query := "g.addV('BTGraphConformance').property('namespace', namespace).property('btgc_key', btgc_key).property('rank', rank)"
				if properties["active"] != nil {
					query += ".property('active', active)"
				}
				if err := client.Execute(ctx, query, map[string]any{
					"namespace": properties["namespace"],
					"btgc_key":  properties["btgc_key"],
					"rank":      properties["rank"],
					"active":    properties["active"],
				}); err != nil {
					return sanitizeProviderError("create vertex", err)
				}
			}
			edge := fixture.Edges()[0]
			properties := edge.Properties()
			query := "g.V().has('namespace', namespace).has('btgc_key', start).as('source').V().has('namespace', namespace).has('btgc_key', end).as('target').addE('BTGC_LINKS').from('source').to('target').property('namespace', namespace).property('btgc_key', btgc_key).property('weight', weight)"
			if err := client.Execute(ctx, query, map[string]any{
				"namespace": properties["namespace"],
				"start":     edge.StartID().String(),
				"end":       edge.EndID().String(),
				"btgc_key":  properties["btgc_key"],
				"weight":    properties["weight"],
			}); err != nil {
				return sanitizeProviderError("create edge", err)
			}
			return nil
		},
		ReadVertices: func(ctx context.Context, fixture graphtest.Fixture) ([]graph.Vertex, error) {
			result, err := client.ReadVertices(ctx, "g.V().has('namespace', namespace).hasLabel('BTGraphConformance').elementMap()", map[string]any{"namespace": fixture.Namespace()})
			return result, sanitizeProviderError("read vertices", err)
		},
		ReadEdges: func(ctx context.Context, fixture graphtest.Fixture) ([]graph.Edge, error) {
			result, err := client.ReadEdges(ctx, "g.E().has('namespace', namespace).elementMap()", map[string]any{"namespace": fixture.Namespace()})
			return result, sanitizeProviderError("read edges", err)
		},
		InvalidOperation: func(ctx context.Context, _ graphtest.Fixture) error {
			return sanitizeProviderError("invalid operation", client.Execute(ctx, "g.V().has("))
		},
		BlockUntilCanceled: func(ctx context.Context, _ graphtest.Fixture, started graphtest.Started) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			stream, err := client.executor.Submit("g.inject(1).repeat(__.identity()).times(1000000)", nil, config.CaseTimeout)
			if err != nil {
				return sanitizeProviderError("submit cancellation traversal", err)
			}
			if stream == nil || stream.Results() == nil {
				return classified(ErrInvalidResult, "cancellation result stream", nil)
			}
			defer stream.Close()
			results := stream.Results()
			started()
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case _, ok := <-results:
					if !ok {
						if err := stream.Err(); err != nil {
							return sanitizeProviderError("cancellation result stream", err)
						}
						return nil
					}
				}
			}
		},
		CleanupFixture: func(ctx context.Context, fixture graphtest.Fixture) error {
			return sanitizeProviderError("cleanup fixture", client.Execute(ctx, "g.V().has('namespace', namespace).drop()", map[string]any{"namespace": fixture.Namespace()}))
		},
		Close: func(ctx context.Context) error {
			return sanitizeProviderError("close", client.Close(ctx))
		},
		Traverse: func(ctx context.Context, fixture graphtest.Fixture) ([]string, error) {
			query := "g.V().has('namespace', namespace).has('btgc_key', start).repeat(__.out()).times(1).path().by('btgc_key')"
			keys, err := client.Traverse(ctx, query, map[string]any{"namespace": fixture.Namespace(), "start": "left"})
			return keys, sanitizeProviderError("traversal", err)
		},
		IsProviderError: func(err error) bool {
			return errors.Is(err, ErrProvider)
		},
	}
}

func sanitizeProviderError(phase string, err error) error {
	if err == nil {
		return nil
	}
	return classified(ErrProvider, phase, err)
}

func TestConformanceMetadataIsDigestPinned(t *testing.T) {
	if !strings.Contains(tinkerPopImage, "@sha256:") || !strings.HasSuffix(tinkerPopImage, "d7b23b4b6773a521cb70cf82c68584a6c68e35019c1357ab4c9371c4e843d651") {
		t.Fatalf("image reference is not pinned: %s", tinkerPopImage)
	}
	if time.Duration(graphtest.DefaultConfig().CaseTimeout) <= 0 {
		t.Fatal("conformance timeout is not bounded")
	}
}
