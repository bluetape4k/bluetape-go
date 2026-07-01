package graphio_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/graph"
	"github.com/bluetape4k/bluetape-go/graph/graphio"
)

func TestNDJSONRoundTripAndOrdering(t *testing.T) {
	ctx := context.Background()
	records := []graphio.Record{
		mustEdgeRecord(t, testEdge(t, "e1", "v1", "v2")),
		mustVertexRecord(t, testVertex(t, "v2")),
		mustVertexRecord(t, testVertex(t, "v1")),
	}

	var buffer bytes.Buffer
	report, err := graphio.WriteNDJSON(ctx, &buffer, records, graphio.WriteOptions{})
	if err != nil {
		t.Fatalf("WriteNDJSON error = %v", err)
	}
	if report.VerticesWritten != 2 || report.EdgesWritten != 1 {
		t.Fatalf("write report = %+v", report)
	}
	lines := strings.Split(strings.TrimSpace(buffer.String()), "\n")
	if len(lines) != 3 || !strings.Contains(lines[0], `"type":"vertex"`) || !strings.Contains(lines[2], `"type":"edge"`) {
		t.Fatalf("unexpected NDJSON order:\n%s", buffer.String())
	}

	read, readReport, err := graphio.ReadNDJSON(ctx, strings.NewReader(buffer.String()), graphio.ReadOptions{})
	if err != nil {
		t.Fatalf("ReadNDJSON error = %v", err)
	}
	if len(read) != 3 || readReport.VerticesRead != 2 || readReport.EdgesRead != 1 {
		t.Fatalf("read len/report = %d %+v", len(read), readReport)
	}
}

func TestNDJSONReaderFailuresAndTerminalState(t *testing.T) {
	var invalid bytes.Buffer
	writer := graphio.NewNDJSONWriter(context.Background(), &invalid, graphio.WriteOptions{MaxFailures: -1})
	if err := writer.WriteRecord(mustVertexRecord(t, testVertex(t, "invalid-options"))); !errors.Is(err, graphio.ErrInvalidOptions) {
		t.Fatalf("invalid writer options error = %v, want ErrInvalidOptions", err)
	}

	nilReader := graphio.NewNDJSONReader(context.Background(), nil, graphio.ReadOptions{})
	if _, err := nilReader.ReadRecord(); !errors.Is(err, graphio.ErrInvalidOptions) {
		t.Fatalf("nil reader error = %v, want ErrInvalidOptions", err)
	}

	reader := graphio.NewNDJSONReader(context.Background(), strings.NewReader("{\"type\":\"edge\",\"id\":\"e1\",\"label\":\"KNOWS\",\"from\":\"v1\",\"to\":\"v2\"}\n"), graphio.ReadOptions{})
	if _, err := reader.ReadRecord(); !errors.Is(err, graphio.ErrMissingEndpoint) {
		t.Fatalf("missing endpoint error = %v, want ErrMissingEndpoint", err)
	}

	report, err := reader.Close()
	if err != nil {
		t.Fatalf("Close error = %v", err)
	}
	again, err := reader.Close()
	if err != nil {
		t.Fatalf("second Close error = %v", err)
	}
	if again.VerticesRead != report.VerticesRead || again.EdgesRead != report.EdgesRead {
		t.Fatalf("second Close report = %+v, want %+v", again, report)
	}
	if _, err := reader.ReadRecord(); !errors.Is(err, graphio.ErrStreamClosed) {
		t.Fatalf("post-close ReadRecord error = %v, want ErrStreamClosed", err)
	}

	clean := graphio.NewNDJSONReader(context.Background(), strings.NewReader(""), graphio.ReadOptions{})
	if _, err := clean.ReadRecord(); !errors.Is(err, io.EOF) {
		t.Fatalf("empty ReadRecord error = %v, want EOF", err)
	}
	if _, err := clean.ReadRecord(); !errors.Is(err, io.EOF) {
		t.Fatalf("second EOF ReadRecord error = %v, want EOF", err)
	}
}

func TestNDJSONLimitsCancellationAndRedaction(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var buffer bytes.Buffer
	_, err := graphio.WriteNDJSON(ctx, &buffer, []graphio.Record{mustVertexRecord(t, testVertex(t, "v1"))}, graphio.WriteOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled WriteNDJSON error = %v, want context.Canceled", err)
	}

	_, _, err = graphio.ReadNDJSON(context.Background(), strings.NewReader("{\"type\":\"vertex\",\"id\":\"token-secret-value\",\"label\":\"Person\"}\n"), graphio.ReadOptions{MaxLineBytes: 8})
	if !errors.Is(err, graphio.ErrMalformedInput) {
		t.Fatalf("over-limit line error = %v, want ErrMalformedInput", err)
	}
	if strings.Contains(err.Error(), "token-secret-value") {
		t.Fatalf("over-limit error leaked raw input: %v", err)
	}

	_, _, err = graphio.ReadNDJSON(context.Background(), strings.NewReader("{\"type\":\"vertex\",\"id\":\"v1\",\"label\":\"Person\"}\n"), graphio.ReadOptions{MaxLineBytes: 128, MaxRecordBytes: 16})
	if !errors.Is(err, graphio.ErrMalformedInput) {
		t.Fatalf("over-limit record error = %v, want ErrMalformedInput", err)
	}
}

func mustVertexRecord(t *testing.T, vertex graph.Vertex) graphio.Record {
	t.Helper()
	record, err := graphio.VertexRecord(vertex)
	return mustRecord(t, record, err)
}

func mustEdgeRecord(t *testing.T, edge graph.Edge) graphio.Record {
	t.Helper()
	record, err := graphio.EdgeRecord(edge)
	return mustRecord(t, record, err)
}

func mustRecord(t *testing.T, record graphio.Record, err error) graphio.Record {
	t.Helper()
	if err != nil {
		t.Fatalf("record error = %v", err)
	}
	return record
}
