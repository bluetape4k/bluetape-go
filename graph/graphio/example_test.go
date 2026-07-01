package graphio_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bluetape4k/bluetape-go/graph"
	"github.com/bluetape4k/bluetape-go/graph/graphio"
)

func ExampleNDJSONWriter() {
	ctx := context.Background()
	vertex, _ := graph.ParseVertex("v1", "Person", graph.Properties{"name": "Alice"})

	var output bytes.Buffer
	writer := graphio.NewNDJSONWriter(ctx, &output, graphio.WriteOptions{})
	_ = writer.WriteRecord(mustExampleRecord(graphio.VertexRecord(vertex)))
	_, _ = writer.Close()

	fmt.Println(output.String() != "")
	// Output: true
}

func ExampleCSVWriter() {
	ctx := context.Background()
	vertex, _ := graph.ParseVertex("v1", "Person", graph.Properties{"name": "Alice"})

	var vertices bytes.Buffer
	var edges bytes.Buffer
	writer := graphio.NewCSVWriter(ctx, graphio.CSVWriterStreams{Vertices: &vertices, Edges: &edges}, graphio.CSVWriteOptions{PropertyColumns: []string{"name"}})
	_ = writer.WriteVertex(vertex)
	_, _ = writer.Close()

	fmt.Println(vertices.String() != "")
	// Output: true
}

func ExampleCSVReader() {
	ctx := context.Background()
	reader := graphio.NewCSVReader(ctx, graphio.CSVReaderStreams{
		Vertices: strings.NewReader("id,label,prop.name\nv1,Person,Alice\nv2,Person,Bob\n"),
		Edges:    strings.NewReader("id,label,from,to\ne1,KNOWS,v1,v2\n"),
	}, graphio.CSVReadOptions{})

	for {
		_, err := reader.ReadVertex()
		if err != nil {
			break
		}
	}
	edge, _ := reader.ReadEdge()
	_, _ = reader.Close()

	fmt.Println(edge.ID().String())
	// Output: e1
}

func Example_errorHandling() {
	_, _, err := graphio.ReadNDJSON(context.Background(), bytes.NewBufferString("\n"), graphio.ReadOptions{})
	fmt.Println(errors.Is(err, graphio.ErrMalformedInput))

	var graphErr *graphio.Error
	fmt.Println(errors.As(err, &graphErr))
	// Output:
	// true
	// true
}

func mustExampleRecord(record graphio.Record, err error) graphio.Record {
	if err != nil {
		panic(err)
	}
	return record
}
