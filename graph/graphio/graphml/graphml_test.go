package graphml_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/graph"
	"github.com/bluetape4k/bluetape-go/graph/graphio"
	"github.com/bluetape4k/bluetape-go/graph/graphio/graphml"
)

func TestGraphMLRoundTrip(t *testing.T) {
	ctx := context.Background()
	records := []graphio.Record{
		mustEdgeRecord(t, testEdge(t, "e1", "v1", "v2")),
		mustVertexRecord(t, testVertex(t, "v2")),
		mustVertexRecord(t, testVertex(t, "v1")),
	}

	var output bytes.Buffer
	report, err := graphml.Write(ctx, &output, records, graphml.WriteOptions{})
	if err != nil {
		t.Fatalf("Write error = %v", err)
	}
	if report.Format != graphml.FormatGraphML || report.VerticesWritten != 2 || report.EdgesWritten != 1 {
		t.Fatalf("write report = %+v", report)
	}
	if !strings.Contains(output.String(), `edgedefault="directed"`) || !strings.Contains(output.String(), `<node id="v1">`) {
		t.Fatalf("unexpected graphml output:\n%s", output.String())
	}

	read, readReport, err := graphml.Read(ctx, strings.NewReader(output.String()), graphml.ReadOptions{})
	if err != nil {
		t.Fatalf("Read error = %v", err)
	}
	if readReport.Format != graphml.FormatGraphML || readReport.VerticesRead != 2 || readReport.EdgesRead != 1 {
		t.Fatalf("read report = %+v", readReport)
	}
	if len(read) != 3 {
		t.Fatalf("read records = %d, want 3", len(read))
	}
	if read[0].Kind != graphio.RecordVertex || read[0].Vertex.ID().String() != "v1" {
		t.Fatalf("first record = %+v, want v1 vertex", read[0])
	}
	if read[2].Kind != graphio.RecordEdge || read[2].Edge.ID().String() != "e1" {
		t.Fatalf("last record = %+v, want e1 edge", read[2])
	}
	if read[0].Vertex.Properties()["name"] != "v1" {
		t.Fatalf("vertex properties = %+v", read[0].Vertex.Properties())
	}
	if read[2].Edge.Properties()["since"] != int64(2020) {
		t.Fatalf("edge properties = %+v", read[2].Edge.Properties())
	}
}

func TestGraphMLReadsNamedProducerSubset(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<graphml xmlns="http://graphml.graphdrawing.org/xmlns">
  <key id="node_label" for="node" attr.name="label" attr.type="string"/>
  <key id="name" for="node" attr.name="name" attr.type="string"/>
  <key id="edge_label" for="edge" attr.name="label" attr.type="string"/>
  <key id="weight" for="edge" attr.name="weight" attr.type="double"/>
  <graph id="G" edgedefault="directed">
    <node id="n0"><data key="node_label">Person</data><data key="name">Alice</data></node>
    <node id="n1"><data key="node_label">Person</data><data key="name">Bob</data></node>
    <edge id="e0" source="n0" target="n1"><data key="edge_label">KNOWS</data><data key="weight">1.5</data></edge>
  </graph>
</graphml>`

	records, report, err := graphml.Read(context.Background(), strings.NewReader(input), graphml.ReadOptions{})
	if err != nil {
		t.Fatalf("Read producer subset error = %v", err)
	}
	if len(records) != 3 || report.VerticesRead != 2 || report.EdgesRead != 1 {
		t.Fatalf("records/report = %d %+v", len(records), report)
	}
	if records[2].Edge.Properties()["weight"] != 1.5 {
		t.Fatalf("edge properties = %+v", records[2].Edge.Properties())
	}
}

func TestGraphMLFailClosedInputs(t *testing.T) {
	tests := []struct {
		name string
		xml  string
		want error
	}{
		{
			name: "malformed xml",
			xml:  `<graphml><graph edgedefault="directed"><node id="n1"></graph>`,
			want: graphio.ErrMalformedInput,
		},
		{
			name: "doctype rejected",
			xml:  `<!DOCTYPE graphml [<!ENTITY xxe SYSTEM "file:///etc/passwd">]><graphml/>`,
			want: graphio.ErrMalformedInput,
		},
		{
			name: "undirected graph rejected",
			xml:  `<graphml><graph edgedefault="undirected"/></graphml>`,
			want: graphio.ErrMalformedInput,
		},
		{
			name: "nested graph rejected",
			xml:  `<graphml><graph edgedefault="directed"><node id="n1"><graph edgedefault="directed"/></node></graph></graphml>`,
			want: graphio.ErrMalformedInput,
		},
		{
			name: "hyperedge rejected",
			xml:  `<graphml><graph edgedefault="directed"><hyperedge id="h1"/></graph></graphml>`,
			want: graphio.ErrMalformedInput,
		},
		{
			name: "port rejected",
			xml:  `<graphml><graph edgedefault="directed"><node id="n1"><port name="p"/></node></graph></graphml>`,
			want: graphio.ErrMalformedInput,
		},
		{
			name: "extension payload rejected",
			xml:  `<graphml><key id="d0" for="node" attr.name="label" attr.type="string"/><graph edgedefault="directed"><node id="n1"><data key="d0"><y:ShapeNode xmlns:y="http://www.yworks.com/xml/graphml"/></data></node></graph></graphml>`,
			want: graphio.ErrMalformedInput,
		},
		{
			name: "unknown key rejected",
			xml:  `<graphml><graph edgedefault="directed"><node id="n1"><data key="missing">Person</data></node></graph></graphml>`,
			want: graphio.ErrMalformedInput,
		},
		{
			name: "duplicate vertex rejected",
			xml:  `<graphml><key id="label" for="node" attr.name="label" attr.type="string"/><graph edgedefault="directed"><node id="n1"><data key="label">Person</data></node><node id="n1"><data key="label">Person</data></node></graph></graphml>`,
			want: graphio.ErrDuplicateVertex,
		},
		{
			name: "missing endpoint rejected",
			xml:  `<graphml><key id="nl" for="node" attr.name="label" attr.type="string"/><key id="el" for="edge" attr.name="label" attr.type="string"/><graph edgedefault="directed"><node id="n1"><data key="nl">Person</data></node><edge id="e1" source="n1" target="n2"><data key="el">KNOWS</data></edge></graph></graphml>`,
			want: graphio.ErrMissingEndpoint,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := graphml.Read(context.Background(), strings.NewReader(tt.xml), graphml.ReadOptions{})
			if !errors.Is(err, tt.want) {
				t.Fatalf("Read error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestGraphMLLimitsAndCancellation(t *testing.T) {
	input := `<graphml><graph edgedefault="directed"/></graphml>`
	_, _, err := graphml.Read(context.Background(), strings.NewReader(input), graphml.ReadOptions{MaxInputBytes: 8})
	if !errors.Is(err, graphio.ErrMalformedInput) {
		t.Fatalf("over limit error = %v, want ErrMalformedInput", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var output bytes.Buffer
	_, err = graphml.Write(ctx, &output, []graphio.Record{mustVertexRecord(t, testVertex(t, "v1"))}, graphml.WriteOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Write error = %v, want context.Canceled", err)
	}

	_, _, err = graphml.Read(ctx, strings.NewReader(input), graphml.ReadOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Read error = %v, want context.Canceled", err)
	}
}

func testVertex(t *testing.T, id string) graph.Vertex {
	t.Helper()
	vertex, err := graph.ParseVertex(id, "Person", graph.Properties{"name": id})
	if err != nil {
		t.Fatalf("ParseVertex(%q) error = %v", id, err)
	}
	return vertex
}

func testEdge(t *testing.T, id, start, end string) graph.Edge {
	t.Helper()
	edge, err := graph.ParseEdge(id, "KNOWS", graph.RawEdgeEndpoints{Start: start, End: end}, graph.Properties{"since": int64(2020)})
	if err != nil {
		t.Fatalf("ParseEdge(%q) error = %v", id, err)
	}
	return edge
}

func mustVertexRecord(t *testing.T, vertex graph.Vertex) graphio.Record {
	t.Helper()
	record, err := graphio.VertexRecord(vertex)
	if err != nil {
		t.Fatalf("VertexRecord error = %v", err)
	}
	return record
}

func mustEdgeRecord(t *testing.T, edge graph.Edge) graphio.Record {
	t.Helper()
	record, err := graphio.EdgeRecord(edge)
	if err != nil {
		t.Fatalf("EdgeRecord error = %v", err)
	}
	return record
}
