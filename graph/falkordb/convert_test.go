package falkordb_test

import (
	"errors"
	"testing"

	"github.com/bluetape4k/bluetape-go/graph"
	"github.com/bluetape4k/bluetape-go/graph/falkordb"
)

func TestVertexAndEdgeConversionCopiesProperties(t *testing.T) {
	vertex, err := falkordb.VertexFromValue(map[string]any{
		"id": "v1", "label": "Person", "properties": map[string]any{"name": "Ada"},
	})
	if err != nil {
		t.Fatal(err)
	}
	props := vertex.Properties()
	props["name"] = "caller"
	if vertex.Properties()["name"] != "Ada" {
		t.Fatalf("vertex properties were not copied")
	}
	edge, err := falkordb.EdgeFromValue(map[string]any{
		"id": "e1", "label": "KNOWS", "start_id": "v1", "end_id": "v2",
		"properties": map[string]any{"since": int64(2020)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if edge.StartID() != graph.ElementID("v1") || edge.EndID() != graph.ElementID("v2") {
		t.Fatalf("edge endpoints=%s/%s", edge.StartID(), edge.EndID())
	}
}

func TestConversionRejectsPartialValues(t *testing.T) {
	if _, err := falkordb.VertexFromValue(map[string]any{"label": "Person"}); !errors.Is(err, falkordb.ErrInvalidResult) {
		t.Fatalf("partial vertex err=%v", err)
	}
	if _, err := falkordb.EdgeFromValue(map[string]any{"id": "e1"}); !errors.Is(err, falkordb.ErrInvalidResult) {
		t.Fatalf("partial edge err=%v", err)
	}
}
