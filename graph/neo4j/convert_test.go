package neo4j_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	neo4jadapter "github.com/bluetape4k/bluetape-go/graph/neo4j"
	neo4jdriver "github.com/neo4j/neo4j-go-driver/v6/neo4j"
	"github.com/neo4j/neo4j-go-driver/v6/neo4j/dbtype"
)

func TestVertexFromNodeSelectsDeterministicLabelAndCopiesProperties(t *testing.T) {
	node := dbtype.Node{
		ElementId: "node-1",
		Labels:    []string{"Service", " API ", "Service"},
		Props: map[string]any{
			"name": "checkout",
		},
	}

	vertex, err := neo4jadapter.VertexFromNode(node)
	if err != nil {
		t.Fatalf("VertexFromNode() error = %v", err)
	}
	if vertex.ID().String() != "node-1" {
		t.Fatalf("vertex id = %q, want node-1", vertex.ID())
	}
	if vertex.Label().String() != "API" {
		t.Fatalf("vertex label = %q, want API", vertex.Label())
	}

	node.Props["name"] = "mutated"
	if vertex.Properties()["name"] != "checkout" {
		t.Fatalf("vertex properties were not copied")
	}
}

func TestVertexFromNodeRejectsMissingLabel(t *testing.T) {
	_, err := neo4jadapter.VertexFromNode(dbtype.Node{ElementId: "node-1"})
	if !errors.Is(err, neo4jadapter.ErrInvalidRecord) {
		t.Fatalf("VertexFromNode() error = %v, want ErrInvalidRecord", err)
	}
}

func TestVertexFromNodeRejectsMissingElementID(t *testing.T) {
	_, err := neo4jadapter.VertexFromNode(dbtype.Node{Labels: []string{"Service"}})
	if !errors.Is(err, neo4jadapter.ErrInvalidRecord) {
		t.Fatalf("VertexFromNode() error = %v, want ErrInvalidRecord", err)
	}
}

func TestEdgeFromRelationshipMapsDirectedEndpoints(t *testing.T) {
	relationship := dbtype.Relationship{
		ElementId:      "rel-1",
		StartElementId: "node-1",
		EndElementId:   "node-2",
		Type:           "CALLS",
		Props:          map[string]any{"weight": int64(7)},
	}

	edge, err := neo4jadapter.EdgeFromRelationship(relationship)
	if err != nil {
		t.Fatalf("EdgeFromRelationship() error = %v", err)
	}
	if edge.ID().String() != "rel-1" || edge.Label().String() != "CALLS" {
		t.Fatalf("edge = %s/%s", edge.ID(), edge.Label())
	}
	if edge.StartID().String() != "node-1" || edge.EndID().String() != "node-2" {
		t.Fatalf("edge endpoints = %s -> %s", edge.StartID(), edge.EndID())
	}
	if edge.Properties()["weight"] != int64(7) {
		t.Fatalf("edge property weight = %#v", edge.Properties()["weight"])
	}
}

func TestRecordsAdaptationRejectsMissingAndWrongColumns(t *testing.T) {
	record := &neo4jdriver.Record{
		Keys:   []string{"item"},
		Values: []any{"not-a-node"},
	}

	if _, err := neo4jadapter.VerticesFromRecords([]*neo4jdriver.Record{record}, "item"); !errors.Is(err, neo4jadapter.ErrInvalidRecord) {
		t.Fatalf("VerticesFromRecords(wrong type) error = %v, want ErrInvalidRecord", err)
	}
	if _, err := neo4jadapter.VerticesFromRecords([]*neo4jdriver.Record{record}, "missing"); !errors.Is(err, neo4jadapter.ErrInvalidRecord) {
		t.Fatalf("VerticesFromRecords(missing) error = %v, want ErrInvalidRecord", err)
	}
	if _, err := neo4jadapter.VerticesFromRecords([]*neo4jdriver.Record{nil}, "item"); !errors.Is(err, neo4jadapter.ErrInvalidRecord) {
		t.Fatalf("VerticesFromRecords(nil) error = %v, want ErrInvalidRecord", err)
	}
	if _, err := neo4jadapter.VerticesFromRecords(nil, " "); !errors.Is(err, neo4jadapter.ErrInvalidOptions) {
		t.Fatalf("VerticesFromRecords(blank column) error = %v, want ErrInvalidOptions", err)
	}
}

func TestRecordsAdaptationReturnsGraphValues(t *testing.T) {
	records := []*neo4jdriver.Record{
		{
			Keys: []string{"n", "r"},
			Values: []any{
				dbtype.Node{ElementId: "node-1", Labels: []string{"Service"}},
				dbtype.Relationship{ElementId: "rel-1", StartElementId: "node-1", EndElementId: "node-2", Type: "CALLS"},
			},
		},
	}

	vertices, err := neo4jadapter.VerticesFromRecords(records, "n")
	if err != nil {
		t.Fatalf("VerticesFromRecords() error = %v", err)
	}
	if len(vertices) != 1 || vertices[0].ID().String() != "node-1" {
		t.Fatalf("vertices = %#v", vertices)
	}

	edges, err := neo4jadapter.EdgesFromRecords(records, "r")
	if err != nil {
		t.Fatalf("EdgesFromRecords() error = %v", err)
	}
	if len(edges) != 1 || edges[0].StartID().String() != "node-1" {
		t.Fatalf("edges = %#v", edges)
	}
}

func TestErrorStringDoesNotRetainCypherOrProperties(t *testing.T) {
	const secret = "secret-token-123"
	err := fmt.Errorf("%w", &neo4jadapter.Error{
		Kind:      neo4jadapter.ErrInvalidRecord,
		Operation: "adapt node record",
		Column:    "n",
		Cause:     errors.New("driver cause with " + secret),
	})

	rendered := err.Error()
	if strings.Contains(rendered, secret) {
		t.Fatalf("error string leaked cause details: %s", rendered)
	}
	if !errors.Is(err, neo4jadapter.ErrInvalidRecord) {
		t.Fatalf("errors.Is did not match ErrInvalidRecord")
	}
}
