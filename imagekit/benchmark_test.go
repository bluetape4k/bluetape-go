package imagekit

import (
	"bytes"
	"context"
	"testing"
)

func BenchmarkTransformSmallJPEGToJPEG(b *testing.B) {
	benchmarkTransform(b, mustJPEG(b, quadrantImage(320, 180)), Request{
		Width:        160,
		Height:       90,
		OutputFormat: OutputJPEG,
	})
}

func BenchmarkTransformSmallPNGToPNG(b *testing.B) {
	benchmarkTransform(b, mustPNG(b, quadrantImage(320, 180)), Request{
		Width:        160,
		Height:       90,
		OutputFormat: OutputPNG,
	})
}

func BenchmarkTransformMediumJPEGToPNG(b *testing.B) {
	benchmarkTransform(b, mustJPEG(b, quadrantImage(1600, 900)), Request{
		Width:        320,
		Height:       180,
		OutputFormat: OutputPNG,
	})
}

func BenchmarkTransformToMediumJPEGToPNG(b *testing.B) {
	input := mustJPEG(b, quadrantImage(1600, 900))
	req := Request{
		Width:        320,
		Height:       180,
		OutputFormat: OutputPNG,
	}
	var output bytes.Buffer
	result, err := TransformTo(context.Background(), &output, bytes.NewReader(input), req)
	if err != nil {
		b.Fatalf("TransformTo() error = %v", err)
	}
	if result.OutputWidth != 320 || result.OutputHeight != 180 || result.OutputFormat != OutputPNG {
		b.Fatalf("unexpected result: %+v", result)
	}

	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		output.Reset()
		if _, err := TransformTo(context.Background(), &output, bytes.NewReader(input), req); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkTransform(b *testing.B, input []byte, req Request) {
	b.Helper()
	result, err := Transform(context.Background(), bytes.NewReader(input), req)
	if err != nil {
		b.Fatalf("Transform() error = %v", err)
	}
	if result.OutputWidth <= 0 || result.OutputHeight <= 0 || result.OutputFormat != req.OutputFormat {
		b.Fatalf("unexpected result: %+v", result)
	}

	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Transform(context.Background(), bytes.NewReader(input), req); err != nil {
			b.Fatal(err)
		}
	}
}
