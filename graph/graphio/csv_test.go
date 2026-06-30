package graphio_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/graph/graphio"
)

func TestCSVStatefulWriterReaderAndTerminalState(t *testing.T) {
	ctx := context.Background()
	var vertices bytes.Buffer
	var edges bytes.Buffer

	writer := graphio.NewCSVWriter(ctx, graphio.CSVWriterStreams{Vertices: &vertices, Edges: &edges}, graphio.CSVWriteOptions{
		PropertyColumns: []string{"name", "since"},
		FormulaPolicy:   graphio.CSVFormulaRaw,
	})
	if err := writer.WriteVertex(testVertex(t, "v1")); err != nil {
		t.Fatalf("WriteVertex error = %v", err)
	}
	if err := writer.WriteVertex(testVertex(t, "v2")); err != nil {
		t.Fatalf("WriteVertex v2 error = %v", err)
	}
	if err := writer.WriteEdge(testEdge(t, "e1", "v1", "v2")); err != nil {
		t.Fatalf("WriteEdge error = %v", err)
	}
	report, err := writer.Close()
	if err != nil {
		t.Fatalf("Close error = %v", err)
	}
	if report.VerticesWritten != 2 || report.EdgesWritten != 1 {
		t.Fatalf("write report = %+v", report)
	}
	again, err := writer.Close()
	if err != nil {
		t.Fatalf("second Close error = %v", err)
	}
	if again.VerticesWritten != report.VerticesWritten || again.EdgesWritten != report.EdgesWritten {
		t.Fatalf("second close report = %+v, want %+v", again, report)
	}
	if err := writer.WriteVertex(testVertex(t, "v3")); !errors.Is(err, graphio.ErrStreamClosed) {
		t.Fatalf("post-close WriteVertex error = %v, want ErrStreamClosed", err)
	}

	reader := graphio.NewCSVReader(ctx, graphio.CSVReaderStreams{Vertices: strings.NewReader(vertices.String()), Edges: strings.NewReader(edges.String())}, graphio.CSVReadOptions{})
	if _, err := reader.ReadVertex(); err != nil {
		t.Fatalf("ReadVertex 1 error = %v", err)
	}
	if _, err := reader.ReadVertex(); err != nil {
		t.Fatalf("ReadVertex 2 error = %v", err)
	}
	if _, err := reader.ReadVertex(); !errors.Is(err, io.EOF) {
		t.Fatalf("vertex EOF error = %v, want EOF", err)
	}
	if _, err := reader.ReadVertex(); !errors.Is(err, io.EOF) {
		t.Fatalf("second vertex EOF error = %v, want EOF", err)
	}
	if _, err := reader.ReadEdge(); err != nil {
		t.Fatalf("ReadEdge error = %v", err)
	}
	if _, err := reader.ReadEdge(); !errors.Is(err, io.EOF) {
		t.Fatalf("edge EOF error = %v, want EOF", err)
	}
	readReport, err := reader.Close()
	if err != nil {
		t.Fatalf("reader Close error = %v", err)
	}
	if readReport.VerticesRead != 2 || readReport.EdgesRead != 1 {
		t.Fatalf("read report = %+v", readReport)
	}
	if _, err := reader.ReadEdge(); !errors.Is(err, graphio.ErrStreamClosed) {
		t.Fatalf("post-close ReadEdge error = %v, want ErrStreamClosed", err)
	}
}

func TestCSVSliceRoundTripsPoliciesAndLimits(t *testing.T) {
	ctx := context.Background()
	records := []graphio.Record{
		mustVertexRecord(t, testVertex(t, "=v1")),
		mustVertexRecord(t, testVertex(t, "v2")),
		mustEdgeRecord(t, testEdge(t, "e1", "=v1", "v2")),
	}
	streams := graphio.CSVWriterStreams{Vertices: &bytes.Buffer{}, Edges: &bytes.Buffer{}}
	report, err := graphio.WriteCSV(ctx, streams, records, graphio.CSVWriteOptions{PropertyColumns: []string{"name", "since"}})
	if err != nil {
		t.Fatalf("WriteCSV escape default error = %v", err)
	}
	if report.VerticesWritten != 2 || report.EdgesWritten != 1 {
		t.Fatalf("write report = %+v", report)
	}
	verticesCSV := streams.Vertices.(*bytes.Buffer).String()
	if !strings.Contains(verticesCSV, "'=v1") {
		t.Fatalf("default CSVFormulaEscape did not escape formula-like ID:\n%s", verticesCSV)
	}

	rawStreams := graphio.CSVWriterStreams{Vertices: &bytes.Buffer{}, Edges: &bytes.Buffer{}}
	if _, err := graphio.WriteCSV(ctx, rawStreams, records, graphio.CSVWriteOptions{PropertyColumns: []string{"name", "since"}, FormulaPolicy: graphio.CSVFormulaRaw}); err != nil {
		t.Fatalf("WriteCSV raw error = %v", err)
	}
	read, _, err := graphio.ReadCSV(ctx, graphio.CSVReaderStreams{Vertices: strings.NewReader(rawStreams.Vertices.(*bytes.Buffer).String()), Edges: strings.NewReader(rawStreams.Edges.(*bytes.Buffer).String())}, graphio.CSVReadOptions{})
	if err != nil {
		t.Fatalf("ReadCSV raw error = %v", err)
	}
	if len(read) != 3 {
		t.Fatalf("read records = %d, want 3", len(read))
	}

	_, _, err = graphio.ReadCSV(ctx, graphio.CSVReaderStreams{Vertices: strings.NewReader("id,label\nv1,Person\nv2,Person\n"), Edges: strings.NewReader("id,label,from,to\ne1,KNOWS,v1,missing\n")}, graphio.CSVReadOptions{})
	if !errors.Is(err, graphio.ErrMissingEndpoint) {
		t.Fatalf("missing endpoint error = %v, want ErrMissingEndpoint", err)
	}
	_, _, err = graphio.ReadCSV(ctx, graphio.CSVReaderStreams{Vertices: strings.NewReader("id,label\nv1,Person\n"), Edges: strings.NewReader("id,label,from,to\n")}, graphio.CSVReadOptions{ReadOptions: graphio.ReadOptions{MaxRecordBytes: 8}})
	if !errors.Is(err, graphio.ErrMalformedInput) {
		t.Fatalf("oversized record error = %v, want ErrMalformedInput", err)
	}
	_, _, err = graphio.ReadCSV(ctx, graphio.CSVReaderStreams{Vertices: strings.NewReader("id,label,prop.x\nv1,Person,\"long\nquoted\"\n"), Edges: strings.NewReader("id,label,from,to\n")}, graphio.CSVReadOptions{ReadOptions: graphio.ReadOptions{MaxRecordBytes: 18}})
	if !errors.Is(err, graphio.ErrMalformedInput) {
		t.Fatalf("oversized quoted record error = %v, want ErrMalformedInput", err)
	}
	_, _, err = graphio.ReadCSV(ctx, graphio.CSVReaderStreams{Vertices: strings.NewReader("id,label\nv1,Person\nv2,Person\n"), Edges: strings.NewReader("id,label,from,to\n")}, graphio.CSVReadOptions{ReadOptions: graphio.ReadOptions{MaxRecords: 1}})
	if !errors.Is(err, graphio.ErrMalformedInput) {
		t.Fatalf("record limit error = %v, want ErrMalformedInput", err)
	}
}

func TestCSVReaderRequiresVertexEOFBeforeEdges(t *testing.T) {
	reader := graphio.NewCSVReader(context.Background(), graphio.CSVReaderStreams{
		Vertices: strings.NewReader("id,label\nv1,Person\n"),
		Edges:    strings.NewReader("id,label,from,to\ne1,KNOWS,v1,v1\n"),
	}, graphio.CSVReadOptions{})

	if _, err := reader.ReadEdge(); !errors.Is(err, graphio.ErrInvalidRecord) {
		t.Fatalf("ReadEdge before vertex EOF error = %v, want ErrInvalidRecord", err)
	}
}
