package imagekitgovips

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"sync"
	"testing"

	"github.com/bluetape4k/bluetape-go/imagekit"
)

func TestTransformModesAndFormats(t *testing.T) {
	input := mustJPEG(t, quadrantImage(400, 200))

	tests := []struct {
		name       string
		mode       imagekit.Mode
		format     imagekit.OutputFormat
		wantWidth  int
		wantHeight int
	}{
		{name: "fit jpeg", mode: imagekit.ModeFit, format: imagekit.OutputJPEG, wantWidth: 100, wantHeight: 50},
		{name: "fill png", mode: imagekit.ModeFill, format: imagekit.OutputPNG, wantWidth: 100, wantHeight: 100},
		{name: "exact png", mode: imagekit.ModeExact, format: imagekit.OutputPNG, wantWidth: 100, wantHeight: 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Transform(context.Background(), bytes.NewReader(input), imagekit.Request{
				Width:        100,
				Height:       100,
				Mode:         tt.mode,
				OutputFormat: tt.format,
			})
			if err != nil {
				t.Fatalf("Transform() error = %v", err)
			}
			if result.OutputWidth != tt.wantWidth || result.OutputHeight != tt.wantHeight {
				t.Fatalf("dimensions = %dx%d, want %dx%d", result.OutputWidth, result.OutputHeight, tt.wantWidth, tt.wantHeight)
			}
			if result.OutputFormat != tt.format {
				t.Fatalf("output format = %q, want %q", result.OutputFormat, tt.format)
			}
			if len(result.Bytes) == 0 {
				t.Fatal("expected output bytes")
			}
		})
	}
}

func TestTransformToWritesOutputAndLeavesBytesNil(t *testing.T) {
	var output bytes.Buffer
	result, err := TransformTo(context.Background(), &output, bytes.NewReader(mustPNG(t, quadrantImage(40, 20))), imagekit.Request{
		Width:        20,
		Height:       20,
		OutputFormat: imagekit.OutputPNG,
	})
	if err != nil {
		t.Fatalf("TransformTo() error = %v", err)
	}
	if output.Len() == 0 {
		t.Fatal("expected writer output")
	}
	if result.Bytes != nil {
		t.Fatalf("TransformTo bytes = %d, want nil", len(result.Bytes))
	}
}

func TestTransformRejectsInvalidInputAndInputLimit(t *testing.T) {
	_, err := Transform(context.Background(), bytes.NewReader([]byte("not an image")), imagekit.Request{Width: 10, Height: 10})
	if !errors.Is(err, imagekit.ErrDecode) && !errors.Is(err, imagekit.ErrUnsupportedFormat) {
		t.Fatalf("malformed input error = %v, want decode or unsupported format", err)
	}

	_, err = Transform(context.Background(), bytes.NewReader(mustPNG(t, quadrantImage(32, 32))), imagekit.Request{
		Width:         10,
		Height:        10,
		MaxInputBytes: 8,
	})
	if !errors.Is(err, imagekit.ErrInputTooLarge) {
		t.Fatalf("input limit error = %v, want ErrInputTooLarge", err)
	}
}

func TestTransformHonorsCanceledContextBeforeNativeWork(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Transform(ctx, bytes.NewReader(mustPNG(t, quadrantImage(32, 32))), imagekit.Request{Width: 10, Height: 10})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled context error = %v, want context.Canceled", err)
	}
}

func TestStartupReportsRuntimeInfo(t *testing.T) {
	if err := Startup(); err != nil {
		t.Fatalf("Startup() error = %v", err)
	}
	info := RuntimeInfo()
	if info.LibvipsVersion == "" {
		t.Fatal("expected libvips version")
	}
	if info.GovipsVersion == "" {
		t.Fatal("expected govips version")
	}
}

func TestTransformParallelCallers(t *testing.T) {
	input := mustJPEG(t, quadrantImage(320, 180))
	var wg sync.WaitGroup
	errs := make(chan error, 16)

	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := Transform(context.Background(), bytes.NewReader(input), imagekit.Request{
				Width:        160,
				Height:       90,
				OutputFormat: imagekit.OutputJPEG,
			})
			if err != nil {
				errs <- err
				return
			}
			if result.OutputWidth != 160 || result.OutputHeight != 90 || len(result.Bytes) == 0 {
				errs <- errors.New("unexpected transform result")
			}
		}()
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}

func BenchmarkGovipsSmallJPEGToJPEG(b *testing.B) {
	benchmarkTransform(b, mustJPEG(b, quadrantImage(320, 180)), imagekit.Request{
		Width:        160,
		Height:       90,
		OutputFormat: imagekit.OutputJPEG,
	})
}

func BenchmarkGovipsMediumJPEGToPNG(b *testing.B) {
	benchmarkTransform(b, mustJPEG(b, quadrantImage(1600, 900)), imagekit.Request{
		Width:        320,
		Height:       180,
		OutputFormat: imagekit.OutputPNG,
	})
}

func benchmarkTransform(b *testing.B, input []byte, req imagekit.Request) {
	b.Helper()
	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Transform(context.Background(), bytes.NewReader(input), req); err != nil {
			b.Fatal(err)
		}
	}
}

func quadrantImage(width int, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			switch {
			case x < width/2 && y < height/2:
				img.Set(x, y, color.RGBA{R: 255, A: 255})
			case x >= width/2 && y < height/2:
				img.Set(x, y, color.RGBA{G: 255, A: 255})
			case x < width/2 && y >= height/2:
				img.Set(x, y, color.RGBA{B: 255, A: 255})
			default:
				img.Set(x, y, color.RGBA{R: 255, G: 255, A: 255})
			}
		}
	}
	return img
}

func mustJPEG(tb testing.TB, img image.Image) []byte {
	tb.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		tb.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

func mustPNG(tb testing.TB, img image.Image) []byte {
	tb.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		tb.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func TestTransformRejectsNilReaderAndWriter(t *testing.T) {
	_, err := Transform(context.Background(), nil, imagekit.Request{Width: 10, Height: 10})
	if !errors.Is(err, imagekit.ErrInvalidOptions) {
		t.Fatalf("nil reader error = %v, want ErrInvalidOptions", err)
	}

	err = transformToError(context.Background(), nil, bytes.NewReader(mustPNG(t, quadrantImage(10, 10))), imagekit.Request{Width: 10, Height: 10})
	if !errors.Is(err, imagekit.ErrInvalidOptions) {
		t.Fatalf("nil writer error = %v, want ErrInvalidOptions", err)
	}
}

func transformToError(ctx context.Context, writer io.Writer, reader io.Reader, req imagekit.Request) error {
	_, err := TransformTo(ctx, writer, reader, req)
	return err
}
