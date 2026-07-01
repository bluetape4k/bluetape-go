package graph_test

import (
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/bluetape4k/bluetape-go/graph"
)

func graphPathFixtures(t *testing.T) (graph.Vertex, graph.Edge) {
	t.Helper()

	vertex, err := graph.ParseVertex("person-1", "Person", nil)
	if err != nil {
		t.Fatalf("ParseVertex error = %v", err)
	}
	edge, err := graph.ParseEdge(
		"edge-1",
		"KNOWS",
		graph.RawEdgeEndpoints{Start: "person-1", End: "person-2"},
		nil,
	)
	if err != nil {
		t.Fatalf("ParseEdge error = %v", err)
	}
	return vertex, edge
}

func TestZeroPathAndPathStepDoNotPanic(t *testing.T) {
	var path graph.Path
	if !path.IsEmpty() || path.Length() != 0 || path.TotalWeight() != 0 {
		t.Fatalf("zero path should be empty")
	}
	if path.Steps() != nil || path.Vertices() != nil || path.Edges() != nil {
		t.Fatalf("zero path accessors should return nil slices")
	}
	if err := path.Validate(); err != nil {
		t.Fatalf("zero path should be valid empty path: %v", err)
	}

	var step graph.PathStep
	if err := step.Validate(); !errors.Is(err, graph.ErrInvalidPath) {
		t.Fatalf("zero PathStep Validate = %v, want ErrInvalidPath", err)
	}
	if step.IsVertex() || step.IsEdge() {
		t.Fatalf("zero PathStep should not be vertex or edge")
	}
	if vertex, ok := step.Vertex(); ok || vertex.ID().String() != "" {
		t.Fatalf("zero PathStep Vertex = %v/%v", vertex, ok)
	}
	if edge, ok := step.Edge(); ok || edge.ID().String() != "" {
		t.Fatalf("zero PathStep Edge = %v/%v", edge, ok)
	}
}

func TestPathConstructionAndAccessors(t *testing.T) {
	vertex, edge := graphPathFixtures(t)

	vertexStep, err := graph.VertexStep(vertex)
	if err != nil {
		t.Fatalf("VertexStep error = %v", err)
	}
	edgeStep, err := graph.EdgeStep(edge)
	if err != nil {
		t.Fatalf("EdgeStep error = %v", err)
	}
	if _, err := graph.VertexStep(graph.Vertex{}); !errors.Is(err, graph.ErrInvalidPath) {
		t.Fatalf("invalid VertexStep error = %v, want ErrInvalidPath", err)
	}
	if _, err := graph.EdgeStep(graph.Edge{}); !errors.Is(err, graph.ErrInvalidPath) {
		t.Fatalf("invalid EdgeStep error = %v, want ErrInvalidPath", err)
	}

	path, err := graph.NewPath(vertexStep, edgeStep)
	if err != nil {
		t.Fatalf("NewPath error = %v", err)
	}
	if path.IsEmpty() || path.Length() != 1 || path.TotalWeight() != 1 {
		t.Fatalf("path empty/length/weight = %v/%d/%v", path.IsEmpty(), path.Length(), path.TotalWeight())
	}
	if got := path.Vertices(); len(got) != 1 || got[0].ID() != vertex.ID() {
		t.Fatalf("Vertices = %#v", got)
	}
	if got := path.Edges(); len(got) != 1 || got[0].ID() != edge.ID() {
		t.Fatalf("Edges = %#v", got)
	}

	steps := path.Steps()
	steps[0] = edgeStep
	if got := path.Steps(); !got[0].IsVertex() {
		t.Fatalf("Steps accessor did not return defensive copy")
	}
}

func TestWeightedPathValidation(t *testing.T) {
	vertex, edge := graphPathFixtures(t)
	vertexStep, _ := graph.VertexStep(vertex)
	edgeStep, _ := graph.EdgeStep(edge)

	path, err := graph.NewWeightedPath(2.5, vertexStep, edgeStep)
	if err != nil {
		t.Fatalf("NewWeightedPath error = %v", err)
	}
	if path.TotalWeight() != 2.5 {
		t.Fatalf("TotalWeight = %v, want 2.5", path.TotalWeight())
	}

	for _, weight := range []float64{-1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		t.Run("invalid", func(t *testing.T) {
			if _, err := graph.NewWeightedPath(weight, vertexStep); !errors.Is(err, graph.ErrInvalidPath) {
				t.Fatalf("NewWeightedPath(%v) error = %v, want ErrInvalidPath", weight, err)
			}
		})
	}
}

func TestPathJSONValidation(t *testing.T) {
	vertex, edge := graphPathFixtures(t)
	vertexStep, _ := graph.VertexStep(vertex)
	edgeStep, _ := graph.EdgeStep(edge)
	path, err := graph.NewWeightedPath(2, vertexStep, edgeStep)
	if err != nil {
		t.Fatalf("NewWeightedPath error = %v", err)
	}

	data, err := json.Marshal(path)
	if err != nil {
		t.Fatalf("Marshal Path error = %v", err)
	}
	if string(data) != `{"steps":[{"vertex":{"id":"person-1","label":"Person","properties":null}},{"edge":{"id":"edge-1","label":"KNOWS","start":"person-1","end":"person-2","properties":null}}],"total_weight":2}` {
		t.Fatalf("path JSON = %s", data)
	}
	var decoded graph.Path
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal Path error = %v", err)
	}
	if decoded.Length() != 1 || decoded.TotalWeight() != 2 {
		t.Fatalf("decoded path length/weight = %d/%v", decoded.Length(), decoded.TotalWeight())
	}

	var step graph.PathStep
	both := `{"vertex":{"id":"person-1","label":"Person"},"edge":{"id":"edge-1","label":"KNOWS","start":"person-1","end":"person-2"}}`
	if err := json.Unmarshal([]byte(both), &step); !errors.Is(err, graph.ErrInvalidPath) {
		t.Fatalf("both-shape PathStep error = %v, want ErrInvalidPath", err)
	}
	if err := json.Unmarshal([]byte(`{}`), &step); !errors.Is(err, graph.ErrInvalidPath) {
		t.Fatalf("empty PathStep error = %v, want ErrInvalidPath", err)
	}
	if err := json.Unmarshal([]byte(`{"steps":[],"total_weight":-1}`), &decoded); !errors.Is(err, graph.ErrInvalidPath) {
		t.Fatalf("negative weight Path error = %v, want ErrInvalidPath", err)
	}
	emptyData, err := json.Marshal(graph.EmptyPath())
	if err != nil {
		t.Fatalf("Marshal EmptyPath error = %v", err)
	}
	if string(emptyData) != `{"steps":[],"total_weight":0}` {
		t.Fatalf("empty path JSON = %s", emptyData)
	}
	if err := json.Unmarshal(emptyData, &decoded); err != nil {
		t.Fatalf("Unmarshal EmptyPath error = %v", err)
	}
	for _, payload := range []string{
		`null`,
		`{}`,
		`{"total_weight":0}`,
		`{"steps":[]}`,
		`{"steps":null,"total_weight":0}`,
		`{"steps":[{"edge":{"id":"edge-1","label":"KNOWS","start":"person-1","end":"person-2"}}]}`,
	} {
		t.Run(payload, func(t *testing.T) {
			if err := json.Unmarshal([]byte(payload), &decoded); !errors.Is(err, graph.ErrInvalidPath) {
				t.Fatalf("malformed Path JSON error = %v, want ErrInvalidPath", err)
			}
		})
	}
}
