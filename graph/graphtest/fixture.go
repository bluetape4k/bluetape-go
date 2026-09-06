package graphtest

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sort"
	"strings"

	"github.com/bluetape4k/bluetape-go/graph"
)

const logicalKeyProperty = "btgc_key"

func newFixture() (Fixture, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return Fixture{}, errors.New("graphtest: namespace generation failed")
	}
	namespace := "btgc_" + hex.EncodeToString(raw[:])
	left, err := graph.ParseVertex("left", "BTGCSource", graph.Properties{
		logicalKeyProperty: "left",
		"rank":             int64(1),
		"namespace":        namespace,
	})
	if err != nil {
		return Fixture{}, err
	}
	right, err := graph.ParseVertex("right", "BTGCTarget", graph.Properties{
		logicalKeyProperty: "right",
		"active":           true,
		"namespace":        namespace,
	})
	if err != nil {
		return Fixture{}, err
	}
	edge, err := graph.ParseEdge(
		"left-right",
		"BTGC_LINKS",
		graph.RawEdgeEndpoints{Start: "left", End: "right"},
		graph.Properties{logicalKeyProperty: "left-right", "weight": int64(7), "namespace": namespace},
	)
	if err != nil {
		return Fixture{}, err
	}
	return Fixture{
		namespace: namespace,
		vertices:  []graph.Vertex{left, right},
		edges:     []graph.Edge{edge},
	}, nil
}

func validateFixture(f Fixture) error {
	if len(f.namespace) != len("btgc_")+32 || !strings.HasPrefix(f.namespace, "btgc_") {
		return errInvalidHarness
	}
	if len(f.vertices) != 2 || len(f.edges) != 1 {
		return errInvalidHarness
	}
	for _, vertex := range f.vertices {
		if err := vertex.Validate(); err != nil {
			return errInvalidHarness
		}
	}
	for _, edge := range f.edges {
		if err := edge.Validate(); err != nil {
			return errInvalidHarness
		}
	}
	return nil
}

func cloneVertices(src []graph.Vertex) []graph.Vertex {
	out := make([]graph.Vertex, len(src))
	for i, vertex := range src {
		out[i], _ = graph.NewVertex(vertex.ID(), vertex.Label(), vertex.Properties())
	}
	return out
}

func cloneEdges(src []graph.Edge) []graph.Edge {
	out := make([]graph.Edge, len(src))
	for i, edge := range src {
		out[i], _ = graph.NewEdge(
			edge.ID(),
			edge.Label(),
			graph.EdgeEndpoints{Start: edge.StartID(), End: edge.EndID()},
			edge.Properties(),
		)
	}
	return out
}

func logicalKey(properties graph.Properties) (string, bool) {
	value, ok := properties[logicalKeyProperty].(string)
	return value, ok && value != ""
}

func canonicalVertices(values []graph.Vertex, limit int) ([]graph.Vertex, error) {
	if len(values) > limit {
		return nil, errors.New("graphtest: vertex result limit exceeded")
	}
	out := cloneVertices(values)
	for _, vertex := range out {
		if _, ok := logicalKey(vertex.Properties()); !ok {
			return nil, errInvalidHarness
		}
	}
	sort.Slice(out, func(i, j int) bool {
		left, _ := logicalKey(out[i].Properties())
		right, _ := logicalKey(out[j].Properties())
		return left < right
	})
	return out, nil
}

func canonicalEdges(values []graph.Edge, limit int) ([]graph.Edge, error) {
	if len(values) > limit {
		return nil, errors.New("graphtest: edge result limit exceeded")
	}
	out := cloneEdges(values)
	for _, edge := range out {
		if _, ok := logicalKey(edge.Properties()); !ok {
			return nil, errInvalidHarness
		}
	}
	sort.Slice(out, func(i, j int) bool {
		left, _ := logicalKey(out[i].Properties())
		right, _ := logicalKey(out[j].Properties())
		return left < right
	})
	return out, nil
}
