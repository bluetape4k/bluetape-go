package graphio_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"

	"github.com/bluetape4k/bluetape-go/graph/graphio"
)

func TestGraphIORoundTripStress(t *testing.T) {
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       4,
		RoundsPerTask: 5,
		Timeout:       5 * time.Second,
	})
	tester.RunT(t, func(ctx context.Context) error {
		var ndjson bytes.Buffer
		records := []graphio.Record{
			mustVertexRecord(t, testVertex(t, "v1")),
			mustVertexRecord(t, testVertex(t, "v2")),
			mustEdgeRecord(t, testEdge(t, "e1", "v1", "v2")),
		}
		if _, err := graphio.WriteNDJSON(ctx, &ndjson, records, graphio.WriteOptions{}); err != nil {
			return err
		}
		_, _, err := graphio.ReadNDJSON(ctx, bytes.NewReader(ndjson.Bytes()), graphio.ReadOptions{})
		return err
	})
}

func TestGraphIOCancellationWithAsyncJobTester(t *testing.T) {
	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers:       1,
		RoundsPerTask: 1,
		Timeout:       time.Second,
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	report, err := tester.Run(ctx, func(ctx context.Context) error {
		_, _, err := graphio.ReadNDJSON(ctx, bytes.NewBufferString(""), graphio.ReadOptions{})
		return err
	})
	if err == nil {
		t.Fatalf("AsyncJobTester error = nil, report = %+v", report)
	}
}
