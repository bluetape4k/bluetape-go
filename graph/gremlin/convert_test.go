package gremlin

import (
	"errors"
	"testing"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
	"github.com/bluetape4k/bluetape-go/graph"
)

func TestVertexAndEdgeConversionCopiesProperties(t *testing.T) {
	vertex, err := VertexFromValue(&gremlingo.Vertex{Element: gremlingo.Element{
		Id: int64(1), Label: "person", Properties: map[string]any{"name": "Ada"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	properties := vertex.Properties()
	properties["name"] = "Grace"
	if vertex.Properties()["name"] != "Ada" {
		t.Fatal("vertex properties were not copied")
	}
	edge, err := EdgeFromValue(gremlingo.Edge{
		Element: gremlingo.Element{Id: "e1", Label: "knows", Properties: map[string]any{"weight": int64(7)}},
		OutV:    gremlingo.Vertex{Element: gremlingo.Element{Id: "1", Label: "person"}},
		InV:     gremlingo.Vertex{Element: gremlingo.Element{Id: "2", Label: "person"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if edge.StartID() != graph.MustElementID("1") || edge.EndID() != graph.MustElementID("2") {
		t.Fatalf("edge endpoints=%s,%s", edge.StartID(), edge.EndID())
	}
}

func TestMapAndPathConversion(t *testing.T) {
	vertex, err := VertexFromValue(map[string]any{
		"id": "v1", "label": "person", "properties": map[string]any{"name": "Ada"},
	})
	if err != nil || vertex.ID() != graph.MustElementID("v1") {
		t.Fatalf("vertex=%#v err=%v", vertex, err)
	}
	edge, err := EdgeFromValue(map[any]any{
		"id": "e1", "label": "knows", "outV": "v1", "inV": "v2",
	})
	if err != nil || edge.ID() != graph.MustElementID("e1") {
		t.Fatalf("edge=%#v err=%v", edge, err)
	}
	path := gremlingo.Path{Objects: []interface{}{"left", "right"}}
	keys, err := traversalKeys(path)
	if err != nil || len(keys) != 2 || keys[0] != "left" || keys[1] != "right" {
		t.Fatalf("keys=%#v err=%v", keys, err)
	}
}

func TestConversionRejectsPartialValues(t *testing.T) {
	if _, err := VertexFromValue(map[string]any{"id": "v1"}); !errors.Is(err, ErrInvalidResult) {
		t.Fatalf("partial vertex error=%v", err)
	}
	if _, err := EdgeFromValue(map[string]any{"id": "e1", "label": "knows"}); !errors.Is(err, ErrInvalidResult) {
		t.Fatalf("partial edge error=%v", err)
	}
	if _, err := traversalKeys(map[string]any{}); !errors.Is(err, ErrInvalidResult) {
		t.Fatalf("partial traversal error=%v", err)
	}
}
