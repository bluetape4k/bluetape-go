package graph_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/graph"
)

func TestElementIDAndLabelValidation(t *testing.T) {
	id, err := graph.NewElementID(" node-1 ")
	if err != nil {
		t.Fatalf("NewElementID error = %v", err)
	}
	if id.String() != "node-1" {
		t.Fatalf("ElementID = %q, want node-1", id.String())
	}
	if err := id.Validate(); err != nil {
		t.Fatalf("ElementID.Validate error = %v", err)
	}

	if _, err := graph.NewElementID(" "); !errors.Is(err, graph.ErrInvalidElementID) {
		t.Fatalf("blank id error = %v, want ErrInvalidElementID", err)
	}
	intID, err := graph.ElementIDFromInt(42)
	if err != nil {
		t.Fatalf("ElementIDFromInt error = %v", err)
	}
	if intID.String() != "42" {
		t.Fatalf("ElementIDFromInt = %q, want 42", intID.String())
	}
	if _, err := graph.ElementIDFromInt(-1); !errors.Is(err, graph.ErrInvalidElementID) {
		t.Fatalf("negative id error = %v, want ErrInvalidElementID", err)
	}

	label, err := graph.NewLabel(" Person ")
	if err != nil {
		t.Fatalf("NewLabel error = %v", err)
	}
	if label.String() != "Person" {
		t.Fatalf("Label = %q, want Person", label.String())
	}
	if err := label.Validate(); err != nil {
		t.Fatalf("Label.Validate error = %v", err)
	}
	if _, err := graph.NewLabel(""); !errors.Is(err, graph.ErrInvalidLabel) {
		t.Fatalf("blank label error = %v, want ErrInvalidLabel", err)
	}
}

func TestElementIDAndLabelJSONValidateScalars(t *testing.T) {
	data, err := json.Marshal(graph.MustElementID("node-1"))
	if err != nil {
		t.Fatalf("Marshal ElementID error = %v", err)
	}
	if string(data) != `"node-1"` {
		t.Fatalf("ElementID JSON = %s", data)
	}

	var id graph.ElementID
	if err := json.Unmarshal([]byte(`" node-2 "`), &id); err != nil {
		t.Fatalf("Unmarshal ElementID error = %v", err)
	}
	if id.String() != "node-2" {
		t.Fatalf("decoded ElementID = %q", id.String())
	}
	if err := json.Unmarshal([]byte(`" "`), &id); !errors.Is(err, graph.ErrInvalidElementID) {
		t.Fatalf("blank ElementID JSON error = %v, want ErrInvalidElementID", err)
	}

	var label graph.Label
	if err := json.Unmarshal([]byte(`" Person "`), &label); err != nil {
		t.Fatalf("Unmarshal Label error = %v", err)
	}
	if label.String() != "Person" {
		t.Fatalf("decoded Label = %q", label.String())
	}
	if err := json.Unmarshal([]byte(`" "`), &label); !errors.Is(err, graph.ErrInvalidLabel) {
		t.Fatalf("blank Label JSON error = %v, want ErrInvalidLabel", err)
	}
}

func TestMustElementID(t *testing.T) {
	if graph.MustElementID("constant-id").String() != "constant-id" {
		t.Fatalf("MustElementID did not return constant-id")
	}

	defer func() {
		if recover() == nil {
			t.Fatalf("MustElementID did not panic for invalid id")
		}
	}()
	_ = graph.MustElementID(" ")
}

func TestValidationErrorRedactsValues(t *testing.T) {
	const secret = "token-secret-value"
	var vertex graph.Vertex
	err := json.Unmarshal(
		[]byte(`{"id":" ","label":"Person","properties":{"api_key":"`+secret+`"}}`),
		&vertex,
	)
	if !errors.Is(err, graph.ErrInvalidElementID) {
		t.Fatalf("error = %v, want ErrInvalidElementID", err)
	}
	var validation *graph.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("error = %T, want ValidationError", err)
	}

	rendered := err.Error() + " " + fmt.Sprintf("%+v", validation)
	for _, leaked := range []string{secret, "api_key"} {
		if strings.Contains(rendered, leaked) {
			t.Fatalf("validation error leaked %q: %s", leaked, rendered)
		}
	}
	if _, ok := reflect.TypeOf(*validation).FieldByName("Value"); ok {
		t.Fatalf("ValidationError must not expose raw Value field")
	}
}

func TestPropertiesCloneAndOwnership(t *testing.T) {
	if got := (graph.Properties)(nil).Clone(); got != nil {
		t.Fatalf("nil clone = %#v, want nil", got)
	}
	empty := graph.Properties{}
	if got := empty.Clone(); got == nil || len(got) != 0 {
		t.Fatalf("empty clone = %#v, want empty map", got)
	}

	nested := map[string]any{"flag": true}
	props := graph.Properties{"name": "Alice", "nested": nested}
	clone := props.Clone()
	clone["name"] = "Bob"
	if props["name"] != "Alice" {
		t.Fatalf("clone mutation changed source properties")
	}
	clone["nested"].(map[string]any)["flag"] = false
	if nested["flag"] != false {
		t.Fatalf("nested property should remain caller-owned and shallow-copied")
	}

	vertex, err := graph.ParseVertex("person-1", "Person", props)
	if err != nil {
		t.Fatalf("ParseVertex error = %v", err)
	}
	props["name"] = "Carol"
	if vertex.Properties()["name"] != "Alice" {
		t.Fatalf("constructor did not copy properties")
	}
	got := vertex.Properties()
	got["name"] = "Mallory"
	if vertex.Properties()["name"] != "Alice" {
		t.Fatalf("Properties accessor did not return defensive copy")
	}
}

func TestVertexEdgeValidationAndJSON(t *testing.T) {
	vertex, err := graph.ParseVertex("person-1", "Person", graph.Properties{"name": "Alice"})
	if err != nil {
		t.Fatalf("ParseVertex error = %v", err)
	}
	if vertex.ID().String() != "person-1" || vertex.Label().String() != "Person" {
		t.Fatalf("vertex = %v/%v", vertex.ID(), vertex.Label())
	}
	if err := vertex.Validate(); err != nil {
		t.Fatalf("Vertex.Validate error = %v", err)
	}
	vertexData, err := json.Marshal(vertex)
	if err != nil {
		t.Fatalf("Marshal Vertex error = %v", err)
	}
	if string(vertexData) != `{"id":"person-1","label":"Person","properties":{"name":"Alice"}}` {
		t.Fatalf("vertex JSON = %s", vertexData)
	}

	endpoints := graph.EdgeEndpoints{Start: graph.MustElementID("person-1"), End: graph.MustElementID("person-1")}
	if err := endpoints.Validate(); err != nil {
		t.Fatalf("self-loop endpoints should be valid: %v", err)
	}
	edge, err := graph.ParseEdge(
		"edge-1",
		"KNOWS",
		graph.RawEdgeEndpoints{Start: "person-1", End: "person-2"},
		graph.Properties{"since": 2026},
	)
	if err != nil {
		t.Fatalf("ParseEdge error = %v", err)
	}
	if edge.StartID().String() != "person-1" || edge.EndID().String() != "person-2" {
		t.Fatalf("edge endpoints = %v -> %v", edge.StartID(), edge.EndID())
	}

	data, err := json.Marshal(edge)
	if err != nil {
		t.Fatalf("Marshal Edge error = %v", err)
	}
	if !strings.Contains(string(data), `"start":"person-1"`) || !strings.Contains(string(data), `"end":"person-2"`) {
		t.Fatalf("edge JSON missing endpoint fields: %s", data)
	}

	edgeWithoutProperties, err := graph.ParseEdge(
		"edge-2",
		"KNOWS",
		graph.RawEdgeEndpoints{Start: "person-1", End: "person-2"},
		nil,
	)
	if err != nil {
		t.Fatalf("ParseEdge without properties error = %v", err)
	}
	edgeData, err := json.Marshal(edgeWithoutProperties)
	if err != nil {
		t.Fatalf("Marshal Edge without properties error = %v", err)
	}
	if string(edgeData) != `{"id":"edge-2","label":"KNOWS","start":"person-1","end":"person-2","properties":null}` {
		t.Fatalf("edge JSON = %s", edgeData)
	}

	var decoded graph.Edge
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal Edge error = %v", err)
	}
	if decoded.StartID() != edge.StartID() || decoded.EndID() != edge.EndID() {
		t.Fatalf("decoded edge endpoints = %v -> %v", decoded.StartID(), decoded.EndID())
	}

	if _, err := graph.ParseVertex("", "Person", nil); !errors.Is(err, graph.ErrInvalidElementID) {
		t.Fatalf("ParseVertex blank id error = %v, want ErrInvalidElementID", err)
	}
	if _, err := graph.NewEdge(graph.ElementID(""), graph.Label("KNOWS"), endpoints, nil); !errors.Is(err, graph.ErrInvalidElementID) {
		t.Fatalf("NewEdge zero id error = %v, want ErrInvalidElementID", err)
	}
	if err := json.Unmarshal([]byte(`{"id":"edge-1","label":"KNOWS","start":" ","end":"person-2"}`), &decoded); !errors.Is(err, graph.ErrInvalidElementID) {
		t.Fatalf("invalid endpoint JSON error = %v, want ErrInvalidElementID", err)
	}
}

func TestZeroVertexAndEdgeDoNotPanic(t *testing.T) {
	var vertex graph.Vertex
	if err := vertex.Validate(); !errors.Is(err, graph.ErrInvalidVertex) {
		t.Fatalf("zero Vertex Validate = %v, want ErrInvalidVertex", err)
	}
	if vertex.ID().String() != "" || vertex.Label().String() != "" || vertex.Properties() != nil {
		t.Fatalf("zero Vertex accessors should return zero values")
	}

	var edge graph.Edge
	if err := edge.Validate(); !errors.Is(err, graph.ErrInvalidEdge) {
		t.Fatalf("zero Edge Validate = %v, want ErrInvalidEdge", err)
	}
	if edge.ID().String() != "" || edge.Label().String() != "" || edge.Properties() != nil {
		t.Fatalf("zero Edge accessors should return zero values")
	}
}

func TestUnsupportedCapabilityIsReserved(t *testing.T) {
	if !errors.Is(fmt.Errorf("reserved: %w", graph.ErrUnsupportedCapability), graph.ErrUnsupportedCapability) {
		t.Fatalf("ErrUnsupportedCapability should work with errors.Is")
	}

	constructorErrors := []error{
		func() error { _, err := graph.NewElementID(" "); return err }(),
		func() error { _, err := graph.NewLabel(" "); return err }(),
		func() error { _, err := graph.ParseVertex("", "Person", nil); return err }(),
	}
	for _, err := range constructorErrors {
		if errors.Is(err, graph.ErrUnsupportedCapability) {
			t.Fatalf("#48 constructor returned reserved ErrUnsupportedCapability: %v", err)
		}
	}
}
