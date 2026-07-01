package graph_test

import (
	"errors"
	"fmt"

	"github.com/bluetape4k/bluetape-go/graph"
)

func ExampleParseVertex() {
	vertex, err := graph.ParseVertex("person-1", "Person", graph.Properties{"name": "Alice"})
	if err != nil {
		return
	}

	fmt.Println(vertex.ID())
	fmt.Println(vertex.Label())

	// Output:
	// person-1
	// Person
}

func ExampleNewPath() {
	vertex, _ := graph.ParseVertex("person-1", "Person", nil)
	edge, _ := graph.ParseEdge(
		"edge-1",
		"KNOWS",
		graph.RawEdgeEndpoints{Start: "person-1", End: "person-2"},
		nil,
	)
	vertexStep, _ := graph.VertexStep(vertex)
	edgeStep, _ := graph.EdgeStep(edge)
	path, _ := graph.NewPath(vertexStep, edgeStep)

	fmt.Println(path.Length())
	fmt.Println(path.TotalWeight())

	// Output:
	// 1
	// 1
}

func ExampleValidationError() {
	_, err := graph.ParseVertex("", "Person", nil)

	fmt.Println(errors.Is(err, graph.ErrInvalidElementID))
	var validation *graph.ValidationError
	fmt.Println(errors.As(err, &validation))

	// Output:
	// true
	// true
}

func Example_rawRecordAdaptation() {
	type rawNode struct {
		ID     string
		Labels []string
		Props  map[string]any
	}
	type rawRelationship struct {
		ID    string
		Type  string
		Start string
		End   string
		Props map[string]any
	}

	node := rawNode{ID: "person-1", Labels: []string{"Person"}, Props: map[string]any{"name": "Alice"}}
	relationship := rawRelationship{ID: "edge-1", Type: "KNOWS", Start: "person-1", End: "person-2"}

	vertex, _ := graph.ParseVertex(node.ID, node.Labels[0], graph.Properties(node.Props))
	edge, _ := graph.ParseEdge(
		relationship.ID,
		relationship.Type,
		graph.RawEdgeEndpoints{Start: relationship.Start, End: relationship.End},
		graph.Properties(relationship.Props),
	)

	fmt.Println(vertex.ID())
	fmt.Println(edge.StartID(), edge.EndID())

	// Output:
	// person-1
	// person-1 person-2
}
