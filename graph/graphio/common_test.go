package graphio_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/graph"
	"github.com/bluetape4k/bluetape-go/graph/graphio"
)

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
	edge, err := graph.ParseEdge(id, "KNOWS", graph.RawEdgeEndpoints{Start: start, End: end}, graph.Properties{"since": float64(2020)})
	if err != nil {
		t.Fatalf("ParseEdge(%q) error = %v", id, err)
	}
	return edge
}

func TestRecordValidationAndErrorContracts(t *testing.T) {
	vertex := testVertex(t, "v1")
	edge := testEdge(t, "e1", "v1", "v2")

	vertexRecord, err := graphio.VertexRecord(vertex)
	if err != nil {
		t.Fatalf("VertexRecord error = %v", err)
	}
	if err := vertexRecord.Validate(); err != nil {
		t.Fatalf("vertex record Validate error = %v", err)
	}

	edgeRecord, err := graphio.EdgeRecord(edge)
	if err != nil {
		t.Fatalf("EdgeRecord error = %v", err)
	}
	if err := edgeRecord.Validate(); err != nil {
		t.Fatalf("edge record Validate error = %v", err)
	}

	invalid := graphio.Record{Kind: graphio.RecordVertex, Vertex: vertex, Edge: edge}
	if err := invalid.Validate(); !errors.Is(err, graphio.ErrInvalidRecord) {
		t.Fatalf("both-value record error = %v, want ErrInvalidRecord", err)
	}

	err = graphio.NewError(graphio.ErrInvalidRecord, graphio.FormatNDJSON, graphio.PhaseValidate, graphio.Location{Line: 3, FileRole: graphio.FileRoleStream}, "id", "token-secret-value", "bad record", graph.ErrInvalidVertex)
	if !errors.Is(err, graphio.ErrInvalidRecord) {
		t.Fatalf("errors.Is graphio sentinel = false: %v", err)
	}
	if !errors.Is(err, graph.ErrInvalidVertex) {
		t.Fatalf("errors.Is graph sentinel = false: %v", err)
	}
	var graphErr *graphio.Error
	if !errors.As(err, &graphErr) {
		t.Fatalf("errors.As = false for %T", err)
	}
	if strings.Contains(err.Error(), "token-secret-value") || strings.Contains(fmt.Sprintf("%+v", graphErr), "token-secret-value") {
		t.Fatalf("graphio error leaked secret: %v %+v", err, graphErr)
	}
}

func TestReadOptionsDefaultsAndInvalidLimits(t *testing.T) {
	options, err := graphio.NormalizeReadOptions(graphio.ReadOptions{})
	if err != nil {
		t.Fatalf("NormalizeReadOptions zero error = %v", err)
	}
	if options.MaxLineBytes != 1<<20 || options.MaxRecordBytes != 1<<20 || options.MaxFieldBytes != 256<<10 {
		t.Fatalf("unexpected byte defaults: %+v", options)
	}
	if options.MaxColumns != 1024 || options.MaxRecords != 1_000_000 || options.MaxFailures != 100 {
		t.Fatalf("unexpected count defaults: %+v", options)
	}
	if options.DuplicateVertexPolicy != graphio.DuplicateVertexFail || options.MissingEndpointPolicy != graphio.MissingEndpointFail {
		t.Fatalf("zero options must fail closed: %+v", options)
	}

	options, err = graphio.NormalizeReadOptions(graphio.ReadOptions{MaxRecords: graphio.UnlimitedRecords})
	if err != nil {
		t.Fatalf("UnlimitedRecords error = %v", err)
	}
	if options.MaxRecords != graphio.UnlimitedRecords {
		t.Fatalf("MaxRecords = %d, want UnlimitedRecords", options.MaxRecords)
	}

	_, err = graphio.NormalizeReadOptions(graphio.ReadOptions{MaxLineBytes: -1})
	if !errors.Is(err, graphio.ErrInvalidOptions) {
		t.Fatalf("negative MaxLineBytes error = %v, want ErrInvalidOptions", err)
	}
	_, err = graphio.NormalizeReadOptions(graphio.ReadOptions{MaxRecords: -2})
	if !errors.Is(err, graphio.ErrInvalidOptions) {
		t.Fatalf("negative MaxRecords error = %v, want ErrInvalidOptions", err)
	}
}

func TestFailureReportCapsAndRedaction(t *testing.T) {
	report := graphio.Report{Format: graphio.FormatCSV}
	for i := 0; i < 3; i++ {
		report.AddFailure(graphio.Failure{
			Phase:    graphio.PhaseReadVertex,
			Severity: graphio.SeverityError,
			RecordID: "token-secret-value",
			Summary:  "redacted failure",
		}, 2)
	}
	if len(report.Failures) != 2 {
		t.Fatalf("retained failures = %d, want 2", len(report.Failures))
	}
	if report.OmittedFailures != 1 {
		t.Fatalf("OmittedFailures = %d, want 1", report.OmittedFailures)
	}
	for _, failure := range report.Failures {
		if strings.Contains(failure.RecordID, "token-secret-value") || strings.Contains(fmt.Sprintf("%+v", failure), "token-secret-value") {
			t.Fatalf("failure leaked secret: %+v", failure)
		}
	}
}
