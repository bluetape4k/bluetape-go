package graphio_test

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/bluetape4k/bluetape-go/graph"
	"github.com/bluetape4k/bluetape-go/graph/graphio"
	"github.com/bluetape4k/bluetape-go/graph/graphio/graphml"
	"github.com/google/go-cmp/cmp"
)

const (
	benchmarkMaxRecords   = 30_000
	benchmarkMaxLineBytes = 64 << 10
	benchmarkMaxInput     = 64 << 20
)

type benchmarkGraphShape struct {
	name       string
	vertices   int
	edges      int
	properties int
}

type benchmarkEncodedGraph struct {
	vertices []byte
	edges    []byte
	stream   []byte
}

func (encoded benchmarkEncodedGraph) totalBytes() int64 {
	return int64(len(encoded.vertices) + len(encoded.edges) + len(encoded.stream))
}

type benchmarkGraphFormat struct {
	name  string
	write func(testing.TB, []graphio.Record) benchmarkEncodedGraph
	read  func(testing.TB, benchmarkEncodedGraph) []graphio.Record
}

type normalizedBenchmarkProperty struct {
	Key   string
	Value string
}

type normalizedBenchmarkRecord struct {
	Kind       graphio.RecordKind
	ID         string
	Label      string
	Start      string
	End        string
	Properties []normalizedBenchmarkProperty
}

var (
	benchmarkEncodedGraphSink benchmarkEncodedGraph
	benchmarkGraphRecordsSink []graphio.Record
)

func TestBenchmarkGraphShapeIsDeterministic(t *testing.T) {
	for _, shape := range benchmarkGraphShapes() {
		t.Run(shape.name, func(t *testing.T) {
			first := benchmarkGraphRecords(t, shape)
			second := benchmarkGraphRecords(t, shape)
			if diff := cmp.Diff(normalizeBenchmarkRecords(first), normalizeBenchmarkRecords(second)); diff != "" {
				t.Fatalf("records are not deterministic (-first +second):\n%s", diff)
			}
			assertBenchmarkRecords(t, first, shape)
		})
	}
}

func TestBenchmarkFormatsRoundTripEquivalentRecords(t *testing.T) {
	for _, format := range benchmarkGraphFormats() {
		format := format
		t.Run(format.name, func(t *testing.T) {
			for _, shape := range benchmarkGraphShapes() {
				shape := shape
				t.Run(shape.name, func(t *testing.T) {
					records := benchmarkGraphRecords(t, shape)
					encoded := format.write(t, records)
					decoded := format.read(t, encoded)
					assertBenchmarkRecords(t, decoded, shape)
					if diff := cmp.Diff(normalizeBenchmarkRecords(records), normalizeBenchmarkRecords(decoded)); diff != "" {
						t.Fatalf("%s round trip (-want +got):\n%s", format.name, diff)
					}
				})
			}
		})
	}
}

func benchmarkGraphShapes() []benchmarkGraphShape {
	return []benchmarkGraphShape{
		{name: "Small/100V-200E-3P", vertices: 100, edges: 200, properties: 3},
		{name: "Medium/10000V-20000E-5P", vertices: 10_000, edges: 20_000, properties: 5},
		{name: "WideProperties/1000V-2000E-20P", vertices: 1_000, edges: 2_000, properties: 20},
	}
}

func benchmarkGraphRecords(tb testing.TB, shape benchmarkGraphShape) []graphio.Record {
	tb.Helper()
	records := make([]graphio.Record, 0, shape.vertices+shape.edges)
	for i := 0; i < shape.vertices; i++ {
		id := benchmarkVertexID(i)
		vertex, err := graph.ParseVertex(id, "BenchmarkVertex", benchmarkProperties("vertex", i, shape.properties))
		if err != nil {
			tb.Fatalf("parse vertex %q: %v", id, err)
		}
		record, err := graphio.VertexRecord(vertex)
		if err != nil {
			tb.Fatalf("create vertex record %q: %v", id, err)
		}
		records = append(records, record)
	}
	for i := 0; i < shape.edges; i++ {
		id := benchmarkEdgeID(i)
		start := benchmarkVertexID(i % shape.vertices)
		end := benchmarkVertexID((i*17 + 1) % shape.vertices)
		edge, err := graph.ParseEdge(id, "BenchmarkEdge", graph.RawEdgeEndpoints{Start: start, End: end}, benchmarkProperties("edge", i, shape.properties))
		if err != nil {
			tb.Fatalf("parse edge %q: %v", id, err)
		}
		record, err := graphio.EdgeRecord(edge)
		if err != nil {
			tb.Fatalf("create edge record %q: %v", id, err)
		}
		records = append(records, record)
	}
	return records
}

func benchmarkProperties(kind string, recordIndex int, count int) graph.Properties {
	properties := make(graph.Properties, count)
	for i := 0; i < count; i++ {
		properties[fmt.Sprintf("property_%02d", i)] = fmt.Sprintf("%s-value-%02d-%06d", kind, i, recordIndex)
	}
	return properties
}

func benchmarkVertexID(index int) string {
	return fmt.Sprintf("vertex-%06d", index)
}

func benchmarkEdgeID(index int) string {
	return fmt.Sprintf("edge-%06d", index)
}

func benchmarkGraphFormats() []benchmarkGraphFormat {
	readOptions := graphio.ReadOptions{
		MaxLineBytes:   benchmarkMaxLineBytes,
		MaxRecordBytes: benchmarkMaxLineBytes,
		MaxFieldBytes:  benchmarkMaxLineBytes,
		MaxColumns:     64,
		MaxRecords:     benchmarkMaxRecords,
		MaxFailures:    1,
	}
	csvWriteOptions := graphio.CSVWriteOptions{
		PropertyMode:  graphio.CSVPropertiesRawJSONColumn,
		FormulaPolicy: graphio.CSVFormulaEscape,
	}
	csvReadOptions := graphio.CSVReadOptions{
		ReadOptions:  readOptions,
		PropertyMode: graphio.CSVPropertiesRawJSONColumn,
	}

	return []benchmarkGraphFormat{
		{
			name: "CSV",
			write: func(tb testing.TB, records []graphio.Record) benchmarkEncodedGraph {
				tb.Helper()
				var vertices bytes.Buffer
				var edges bytes.Buffer
				_, err := graphio.WriteCSV(context.Background(), graphio.CSVWriterStreams{Vertices: &vertices, Edges: &edges}, records, csvWriteOptions)
				if err != nil {
					tb.Fatalf("write CSV: %v", err)
				}
				return benchmarkEncodedGraph{vertices: vertices.Bytes(), edges: edges.Bytes()}
			},
			read: func(tb testing.TB, encoded benchmarkEncodedGraph) []graphio.Record {
				tb.Helper()
				records, _, err := graphio.ReadCSV(context.Background(), graphio.CSVReaderStreams{
					Vertices: bytes.NewReader(encoded.vertices),
					Edges:    bytes.NewReader(encoded.edges),
				}, csvReadOptions)
				if err != nil {
					tb.Fatalf("read CSV: %v", err)
				}
				return records
			},
		},
		{
			name: "NDJSON",
			write: func(tb testing.TB, records []graphio.Record) benchmarkEncodedGraph {
				tb.Helper()
				var output bytes.Buffer
				if _, err := graphio.WriteNDJSON(context.Background(), &output, records, graphio.WriteOptions{}); err != nil {
					tb.Fatalf("write NDJSON: %v", err)
				}
				return benchmarkEncodedGraph{stream: output.Bytes()}
			},
			read: func(tb testing.TB, encoded benchmarkEncodedGraph) []graphio.Record {
				tb.Helper()
				records, _, err := graphio.ReadNDJSON(context.Background(), bytes.NewReader(encoded.stream), readOptions)
				if err != nil {
					tb.Fatalf("read NDJSON: %v", err)
				}
				return records
			},
		},
		{
			name: "GraphML",
			write: func(tb testing.TB, records []graphio.Record) benchmarkEncodedGraph {
				tb.Helper()
				var output bytes.Buffer
				if _, err := graphml.Write(context.Background(), &output, records, graphml.WriteOptions{}); err != nil {
					tb.Fatalf("write GraphML: %v", err)
				}
				return benchmarkEncodedGraph{stream: output.Bytes()}
			},
			read: func(tb testing.TB, encoded benchmarkEncodedGraph) []graphio.Record {
				tb.Helper()
				records, _, err := graphml.Read(context.Background(), bytes.NewReader(encoded.stream), graphml.ReadOptions{
					ReadOptions:   readOptions,
					MaxInputBytes: benchmarkMaxInput,
				})
				if err != nil {
					tb.Fatalf("read GraphML: %v", err)
				}
				return records
			},
		},
	}
}

func normalizeBenchmarkRecords(records []graphio.Record) []normalizedBenchmarkRecord {
	normalized := make([]normalizedBenchmarkRecord, 0, len(records))
	for _, record := range records {
		entry := normalizedBenchmarkRecord{Kind: record.Kind}
		var properties graph.Properties
		switch record.Kind {
		case graphio.RecordVertex:
			entry.ID = record.Vertex.ID().String()
			entry.Label = record.Vertex.Label().String()
			properties = record.Vertex.Properties()
		case graphio.RecordEdge:
			entry.ID = record.Edge.ID().String()
			entry.Label = record.Edge.Label().String()
			entry.Start = record.Edge.StartID().String()
			entry.End = record.Edge.EndID().String()
			properties = record.Edge.Properties()
		}
		entry.Properties = make([]normalizedBenchmarkProperty, 0, len(properties))
		for key, value := range properties {
			entry.Properties = append(entry.Properties, normalizedBenchmarkProperty{
				Key:   key,
				Value: fmt.Sprintf("%T:%v", value, value),
			})
		}
		sort.Slice(entry.Properties, func(i, j int) bool {
			return entry.Properties[i].Key < entry.Properties[j].Key
		})
		normalized = append(normalized, entry)
	}
	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].Kind != normalized[j].Kind {
			return normalized[i].Kind < normalized[j].Kind
		}
		return normalized[i].ID < normalized[j].ID
	})
	return normalized
}

func assertBenchmarkRecords(tb testing.TB, records []graphio.Record, shape benchmarkGraphShape) {
	tb.Helper()
	if got, want := len(records), shape.vertices+shape.edges; got != want {
		tb.Fatalf("record count = %d, want %d", got, want)
	}

	vertices := make(map[string]graph.Vertex, shape.vertices)
	edges := make(map[string]graph.Edge, shape.edges)
	for _, record := range records {
		switch record.Kind {
		case graphio.RecordVertex:
			vertices[record.Vertex.ID().String()] = record.Vertex
			if got := len(record.Vertex.Properties()); got != shape.properties {
				tb.Fatalf("vertex %q property count = %d, want %d", record.Vertex.ID(), got, shape.properties)
			}
		case graphio.RecordEdge:
			edges[record.Edge.ID().String()] = record.Edge
			if got := len(record.Edge.Properties()); got != shape.properties {
				tb.Fatalf("edge %q property count = %d, want %d", record.Edge.ID(), got, shape.properties)
			}
		default:
			tb.Fatalf("unknown record kind %q", record.Kind)
		}
	}
	if len(vertices) != shape.vertices || len(edges) != shape.edges {
		tb.Fatalf("vertex/edge counts = %d/%d, want %d/%d", len(vertices), len(edges), shape.vertices, shape.edges)
	}

	firstVertex, ok := vertices[benchmarkVertexID(0)]
	if !ok {
		tb.Fatalf("representative vertex %q missing", benchmarkVertexID(0))
	}
	if got, want := firstVertex.Properties()["property_00"], "vertex-value-00-000000"; got != want {
		tb.Fatalf("representative vertex property = %v, want %q", got, want)
	}
	lastVertexID := benchmarkVertexID(shape.vertices - 1)
	if _, ok := vertices[lastVertexID]; !ok {
		tb.Fatalf("last vertex %q missing", lastVertexID)
	}

	firstEdge, ok := edges[benchmarkEdgeID(0)]
	if !ok {
		tb.Fatalf("representative edge %q missing", benchmarkEdgeID(0))
	}
	if got, want := firstEdge.StartID().String(), benchmarkVertexID(0); got != want {
		tb.Fatalf("representative edge start = %q, want %q", got, want)
	}
	if got, want := firstEdge.EndID().String(), benchmarkVertexID(1); got != want {
		tb.Fatalf("representative edge end = %q, want %q", got, want)
	}
	if got, want := firstEdge.Properties()["property_00"], "edge-value-00-000000"; got != want {
		tb.Fatalf("representative edge property = %v, want %q", got, want)
	}
}

func BenchmarkGraphIOFormats(b *testing.B) {
	formats := benchmarkGraphFormats()
	for _, format := range formats {
		format := format
		b.Run(format.name, func(b *testing.B) {
			for _, shape := range benchmarkGraphShapes() {
				shape := shape
				b.Run(shape.name, func(b *testing.B) {
					records := benchmarkGraphRecords(b, shape)
					encoded := format.write(b, records)
					assertBenchmarkRecords(b, format.read(b, encoded), shape)

					b.Run("Write", func(b *testing.B) {
						b.ReportAllocs()
						b.SetBytes(encoded.totalBytes())
						var last benchmarkEncodedGraph
						b.ResetTimer()
						for i := 0; i < b.N; i++ {
							last = format.write(b, records)
							benchmarkEncodedGraphSink = last
						}
						b.StopTimer()
						if last.totalBytes() != encoded.totalBytes() {
							b.Fatalf("encoded bytes = %d, want %d", last.totalBytes(), encoded.totalBytes())
						}
						assertBenchmarkRecords(b, format.read(b, last), shape)
					})

					b.Run("Read", func(b *testing.B) {
						b.ReportAllocs()
						b.SetBytes(encoded.totalBytes())
						var last []graphio.Record
						b.ResetTimer()
						for i := 0; i < b.N; i++ {
							last = format.read(b, encoded)
							benchmarkGraphRecordsSink = last
						}
						b.StopTimer()
						assertBenchmarkRecords(b, last, shape)
					})

					b.Run("RoundTrip", func(b *testing.B) {
						b.ReportAllocs()
						b.SetBytes(encoded.totalBytes())
						var lastEncoded benchmarkEncodedGraph
						var lastRecords []graphio.Record
						b.ResetTimer()
						for i := 0; i < b.N; i++ {
							lastEncoded = format.write(b, records)
							lastRecords = format.read(b, lastEncoded)
							benchmarkEncodedGraphSink = lastEncoded
							benchmarkGraphRecordsSink = lastRecords
						}
						b.StopTimer()
						if lastEncoded.totalBytes() != encoded.totalBytes() {
							b.Fatalf("round-trip encoded bytes = %d, want %d", lastEncoded.totalBytes(), encoded.totalBytes())
						}
						assertBenchmarkRecords(b, lastRecords, shape)
					})

					b.Run("RecordConstructionBaseline", func(b *testing.B) {
						b.ReportAllocs()
						var last []graphio.Record
						b.ResetTimer()
						for i := 0; i < b.N; i++ {
							last = benchmarkGraphRecords(b, shape)
							benchmarkGraphRecordsSink = last
						}
						b.StopTimer()
						assertBenchmarkRecords(b, last, shape)
					})
				})
			}
		})
	}
}
