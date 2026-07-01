package imagekit

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"strings"
	"testing"
	"time"
)

func TestTransformFitFillExactDimensions(t *testing.T) {
	input := mustJPEG(t, quadrantImage(400, 200))

	tests := []struct {
		name       string
		mode       Mode
		wantWidth  int
		wantHeight int
	}{
		{name: "fit preserves aspect ratio", mode: ModeFit, wantWidth: 100, wantHeight: 50},
		{name: "fill center crops", mode: ModeFill, wantWidth: 100, wantHeight: 100},
		{name: "exact distorts only when requested", mode: ModeExact, wantWidth: 100, wantHeight: 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Transform(context.Background(), bytes.NewReader(input), Request{
				Width:        100,
				Height:       100,
				Mode:         tt.mode,
				OutputFormat: OutputPNG,
			})
			if err != nil {
				t.Fatalf("Transform() error = %v", err)
			}

			if result.OutputWidth != tt.wantWidth || result.OutputHeight != tt.wantHeight {
				t.Fatalf("output dimensions = %dx%d, want %dx%d", result.OutputWidth, result.OutputHeight, tt.wantWidth, tt.wantHeight)
			}
			if result.OutputFormat != OutputPNG {
				t.Fatalf("output format = %q, want %q", result.OutputFormat, OutputPNG)
			}
			if len(result.Bytes) == 0 {
				t.Fatal("expected encoded bytes")
			}
		})
	}
}

func TestTransformFillUsesCenterCrop(t *testing.T) {
	input := mustPNG(t, verticalBandsImage(400, 100))

	result, err := Transform(context.Background(), bytes.NewReader(input), Request{
		Width:        100,
		Height:       100,
		Mode:         ModeFill,
		OutputFormat: OutputPNG,
	})
	if err != nil {
		t.Fatalf("Transform() error = %v", err)
	}

	output := mustDecodeImage(t, result.Bytes)
	left := rgbaAt(output, 10, 50)
	center := rgbaAt(output, 50, 50)
	right := rgbaAt(output, 90, 50)
	if !nearColor(left, color.RGBA{G: 255, A: 255}, 8) {
		t.Fatalf("left pixel = %#v, want center-cropped green band", left)
	}
	if !nearColor(center, color.RGBA{B: 255, A: 255}, 8) {
		t.Fatalf("center pixel = %#v, want center-cropped blue band", center)
	}
	if !nearColor(right, color.RGBA{B: 255, A: 255}, 8) {
		t.Fatalf("right pixel = %#v, want center-cropped blue band", right)
	}
}

func TestTransformSupportsJPEGPNGAndGIFInput(t *testing.T) {
	tests := []struct {
		name      string
		input     []byte
		wantInput InputFormat
	}{
		{name: "jpeg", input: mustJPEG(t, quadrantImage(32, 24)), wantInput: InputJPEG},
		{name: "png", input: mustPNG(t, quadrantImage(32, 24)), wantInput: InputPNG},
		{name: "gif", input: mustGIF(t, quadrantImage(32, 24)), wantInput: InputGIF},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Transform(context.Background(), bytes.NewReader(tt.input), Request{
				Width:        16,
				Height:       16,
				OutputFormat: OutputPNG,
			})
			if err != nil {
				t.Fatalf("Transform() error = %v", err)
			}
			if result.InputFormat != tt.wantInput {
				t.Fatalf("input format = %q, want %q", result.InputFormat, tt.wantInput)
			}
		})
	}
}

func TestTransformRejectsMalformedInput(t *testing.T) {
	payload := []byte("secret-raw-payload")
	_, err := Transform(context.Background(), bytes.NewReader(payload), Request{Width: 10, Height: 10})
	if err == nil {
		t.Fatal("expected malformed input error")
	}
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("errors.Is(err, ErrUnsupportedFormat) = false; err=%v", err)
	}
	if !errors.Is(err, image.ErrFormat) {
		t.Fatalf("errors.Is(err, image.ErrFormat) = false; err=%v", err)
	}
}

func TestTransformPreservesDecodeFailures(t *testing.T) {
	payload := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 'b', 'a', 'd'}
	_, err := Transform(context.Background(), bytes.NewReader(payload), Request{Width: 10, Height: 10})
	if err == nil {
		t.Fatal("expected decode failure")
	}
	if !errors.Is(err, ErrDecode) {
		t.Fatalf("errors.Is(err, ErrDecode) = false; err=%v", err)
	}
}

func TestTransformRejectsNilReaderAndWriter(t *testing.T) {
	_, err := Transform(context.Background(), nil, Request{Width: 10, Height: 10})
	if !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("nil reader error = %v, want ErrInvalidOptions", err)
	}

	err = transformToError(context.Background(), io.Discard, nil, Request{Width: 10, Height: 10})
	if !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("nil reader TransformTo error = %v, want ErrInvalidOptions", err)
	}

	err = transformToError(context.Background(), nil, bytes.NewReader(mustPNG(t, quadrantImage(10, 10))), Request{Width: 10, Height: 10})
	if !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("nil writer error = %v, want ErrInvalidOptions", err)
	}
}

func TestTransformWrapsReaderAndWriterFailures(t *testing.T) {
	readErr := errors.New("reader failed /tmp/secret-reader-path")
	_, err := Transform(context.Background(), failingReader{err: readErr}, Request{Width: 10, Height: 10})
	if !errors.Is(err, ErrDecode) || !errors.Is(err, readErr) {
		t.Fatalf("reader failure = %v, want ErrDecode wrapping original", err)
	}

	writeErr := errors.New("writer failed /tmp/secret-writer-path")
	err = transformToError(context.Background(), failingWriter{err: writeErr}, bytes.NewReader(mustPNG(t, quadrantImage(10, 10))), Request{
		Width:        10,
		Height:       10,
		OutputFormat: OutputPNG,
	})
	if !errors.Is(err, ErrEncode) || !errors.Is(err, writeErr) {
		t.Fatalf("writer failure = %v, want ErrEncode wrapping original", err)
	}
}

func TestTransformRejectsUnsupportedOutputFormat(t *testing.T) {
	_, err := Transform(context.Background(), bytes.NewReader(mustPNG(t, quadrantImage(10, 10))), Request{
		Width:        10,
		Height:       10,
		OutputFormat: OutputFormat("gif"),
	})
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("unsupported output error = %v, want ErrUnsupportedFormat", err)
	}
}

func TestTransformEnforcesInputAndOutputLimits(t *testing.T) {
	input := mustPNG(t, quadrantImage(64, 32))

	tests := []struct {
		name string
		req  Request
		want error
	}{
		{name: "input bytes", req: Request{Width: 10, Height: 10, MaxInputBytes: 8}, want: ErrInputTooLarge},
		{name: "input width", req: Request{Width: 10, Height: 10, MaxWidth: 8}, want: ErrImageTooLarge},
		{name: "input pixels", req: Request{Width: 10, Height: 10, MaxPixels: 32}, want: ErrImageTooLarge},
		{name: "output width", req: Request{Width: 100, Height: 10, MaxOutputWidth: 50}, want: ErrImageTooLarge},
		{name: "output pixels", req: Request{Width: 100, Height: 100, MaxOutputPixels: 1000}, want: ErrImageTooLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Transform(context.Background(), bytes.NewReader(input), tt.req)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Transform() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestTransformRejectsNegativeLimits(t *testing.T) {
	input := mustPNG(t, quadrantImage(10, 10))
	requests := []Request{
		{Width: 10, Height: 10, MaxInputBytes: -1},
		{Width: 10, Height: 10, MaxPixels: -1},
		{Width: 10, Height: 10, MaxWidth: -1},
		{Width: 10, Height: 10, MaxHeight: -1},
		{Width: 10, Height: 10, MaxOutputPixels: -1},
		{Width: 10, Height: 10, MaxOutputWidth: -1},
		{Width: 10, Height: 10, MaxOutputHeight: -1},
	}

	for _, req := range requests {
		_, err := Transform(context.Background(), bytes.NewReader(input), req)
		if !errors.Is(err, ErrInvalidOptions) {
			t.Fatalf("Transform(%+v) error = %v, want ErrInvalidOptions", req, err)
		}
	}
}

func TestTransformRejectsOverflowingLimits(t *testing.T) {
	input := mustPNG(t, quadrantImage(1, 1))
	_, err := Transform(context.Background(), bytes.NewReader(input), Request{
		Width:         1,
		Height:        1,
		MaxInputBytes: int64(^uint64(0) >> 1),
	})
	if !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("MaxInputBytes overflow error = %v, want ErrInvalidOptions", err)
	}

	_, err = Transform(context.Background(), bytes.NewReader(input), Request{
		Width:           int(^uint(0) >> 1),
		Height:          2,
		MaxOutputPixels: int(^uint(0) >> 1),
	})
	if !errors.Is(err, ErrImageTooLarge) && !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("output pixel overflow error = %v, want ErrImageTooLarge or ErrInvalidOptions", err)
	}
}

func TestTransformFillExtremeAspectDoesNotAllocateCoverImage(t *testing.T) {
	input := mustPNG(t, quadrantImage(4000, 40))

	result, err := Transform(context.Background(), bytes.NewReader(input), Request{
		Width:           20,
		Height:          20,
		Mode:            ModeFill,
		OutputFormat:    OutputPNG,
		MaxPixels:       4000 * 40,
		MaxOutputPixels: 20 * 20,
	})
	if err != nil {
		t.Fatalf("Transform() error = %v", err)
	}
	if result.OutputWidth != 20 || result.OutputHeight != 20 {
		t.Fatalf("output dimensions = %dx%d, want 20x20", result.OutputWidth, result.OutputHeight)
	}
}

func TestFillCropRectAvoidsIntegerOverflow(t *testing.T) {
	huge := int(^uint(0)>>1)/2 + 100
	bounds := image.Rect(0, 0, huge, 3)

	crop := fillCropRect(bounds, huge, 2)

	if crop.Empty() {
		t.Fatal("crop rect is empty")
	}
	if !crop.In(bounds) {
		t.Fatalf("crop rect = %v, want inside %v", crop, bounds)
	}
	if got := crop.Dy(); got != 2 {
		t.Fatalf("crop height = %d, want 2", got)
	}
}

func TestTransformPreservesCancellationAtPhaseBoundaries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Transform(ctx, bytes.NewReader(mustPNG(t, quadrantImage(10, 10))), Request{Width: 10, Height: 10})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("already canceled error = %v, want context.Canceled", err)
	}

	ctx, cancel = context.WithCancel(context.Background())
	reader := cancelAfterEOFReader{Reader: bytes.NewReader(mustPNG(t, quadrantImage(10, 10))), cancel: cancel}
	_, err = Transform(ctx, reader, Request{Width: 10, Height: 10})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("post-read cancellation error = %v, want context.Canceled", err)
	}
}

func TestTransformPreservesDeadlineExceededAtPhaseBoundaries(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	_, err := Transform(ctx, bytes.NewReader(mustPNG(t, quadrantImage(10, 10))), Request{Width: 10, Height: 10})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline error = %v, want context.DeadlineExceeded", err)
	}
}

func TestTransformDoesNotWriteAfterPreEncodeCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	writer := cancelOnFirstWriteWriter{cancel: cancel}

	err := transformToError(ctx, &writer, bytes.NewReader(mustPNG(t, quadrantImage(10, 10))), Request{
		Width:        10,
		Height:       10,
		OutputFormat: OutputPNG,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("TransformTo() error = %v, want context.Canceled", err)
	}
	if writer.writeCalls != 0 {
		t.Fatalf("write calls = %d, want 0", writer.writeCalls)
	}
}

func TestTransformRejectsUnallowlistedRegisteredFormat(t *testing.T) {
	const fakeMagic = "IKT309"
	image.RegisterFormat("imagekit-test-309", fakeMagic, decodeFakeImage, decodeFakeConfig)

	payload := []byte(fakeMagic + "payload")
	_, err := Transform(context.Background(), bytes.NewReader(payload), Request{Width: 1, Height: 1})
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("registered fake format error = %v, want ErrUnsupportedFormat", err)
	}
}

func TestTransformRejectsInvalidRequest(t *testing.T) {
	input := mustPNG(t, quadrantImage(10, 10))
	requests := []Request{
		{Width: 0, Height: 10},
		{Width: 10, Height: 0},
		{Width: 10, Height: 10, Mode: Mode(99)},
		{Width: 10, Height: 10, ResampleFilter: ResampleFilter(99)},
		{Width: 10, Height: 10, JPEGQuality: -1},
		{Width: 10, Height: 10, JPEGQuality: 101},
	}

	for _, req := range requests {
		_, err := Transform(context.Background(), bytes.NewReader(input), req)
		if !errors.Is(err, ErrInvalidOptions) {
			t.Fatalf("Transform(%+v) error = %v, want ErrInvalidOptions", req, err)
		}
	}
}

func TestTransformPreservesContextErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Transform(ctx, bytes.NewReader(mustPNG(t, quadrantImage(10, 10))), Request{Width: 10, Height: 10})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	_, err = Transform(ctx, bytes.NewReader(mustPNG(t, quadrantImage(10, 10))), Request{Width: 10, Height: 10})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
}

func TestTransformToReturnsMetadataWithoutBytes(t *testing.T) {
	var output bytes.Buffer
	result, err := TransformTo(context.Background(), &output, bytes.NewReader(mustPNG(t, quadrantImage(10, 10))), Request{
		Width:        5,
		Height:       5,
		OutputFormat: OutputPNG,
	})
	if err != nil {
		t.Fatalf("TransformTo() error = %v", err)
	}
	if len(result.Bytes) != 0 {
		t.Fatalf("Result.Bytes length = %d, want 0", len(result.Bytes))
	}
	if output.Len() == 0 {
		t.Fatal("expected writer output")
	}
}

func TestErrorContracts(t *testing.T) {
	const failMagic = "ZKT309PNGFAIL"
	image.RegisterFormat("png", failMagic, decodeFailImage, decodeFakeConfig)

	_, err := Transform(context.Background(), bytes.NewReader([]byte(failMagic+"payload")), Request{Width: 10, Height: 10})
	if !errors.Is(err, ErrDecode) {
		t.Fatalf("expected ErrDecode, got %v", err)
	}

	var kitErr *Error
	if !errors.As(err, &kitErr) {
		t.Fatalf("expected *Error, got %T", err)
	}
	if kitErr.Operation == "" {
		t.Fatal("expected operation metadata")
	}
}

func TestErrorMessagesDoNotLeakPayloadsOrPaths(t *testing.T) {
	payload := []byte("secret-raw-payload")
	_, err := Transform(context.Background(), bytes.NewReader(payload), Request{Width: 10, Height: 10})
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), string(payload)) {
		t.Fatalf("error leaked payload: %q", err.Error())
	}

	pathErr := errors.New("/tmp/secret/path.png")
	_, err = Transform(context.Background(), failingReader{err: pathErr}, Request{Width: 10, Height: 10})
	if err == nil {
		t.Fatal("expected reader error")
	}
	if strings.Contains(err.Error(), pathErr.Error()) {
		t.Fatalf("error leaked cause text: %q", err.Error())
	}
	if !errors.Is(err, pathErr) {
		t.Fatalf("expected cause preserved through errors.Is, got %v", err)
	}
}

func transformToError(ctx context.Context, w io.Writer, r io.Reader, req Request) error {
	_, err := TransformTo(ctx, w, r, req)
	return err
}

func quadrantImage(width, height int) *image.RGBA {
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

func verticalBandsImage(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			switch {
			case x < width/4:
				img.Set(x, y, color.RGBA{R: 255, A: 255})
			case x < width/2:
				img.Set(x, y, color.RGBA{G: 255, A: 255})
			case x < width*3/4:
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

func mustGIF(tb testing.TB, img image.Image) []byte {
	tb.Helper()
	var buf bytes.Buffer
	if err := gif.Encode(&buf, img, nil); err != nil {
		tb.Fatalf("encode gif: %v", err)
	}
	return buf.Bytes()
}

func mustDecodeImage(tb testing.TB, payload []byte) image.Image {
	tb.Helper()
	img, _, err := image.Decode(bytes.NewReader(payload))
	if err != nil {
		tb.Fatalf("decode image: %v", err)
	}
	return img
}

func rgbaAt(img image.Image, x int, y int) color.RGBA {
	r, g, b, a := img.At(x, y).RGBA()
	return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
}

func nearColor(got color.RGBA, want color.RGBA, tolerance uint8) bool {
	return near(got.R, want.R, tolerance) &&
		near(got.G, want.G, tolerance) &&
		near(got.B, want.B, tolerance) &&
		near(got.A, want.A, tolerance)
}

func near(got uint8, want uint8, tolerance uint8) bool {
	if got > want {
		return got-want <= tolerance
	}
	return want-got <= tolerance
}

type failingReader struct {
	err error
}

func (r failingReader) Read([]byte) (int, error) {
	return 0, r.err
}

type failingWriter struct {
	err error
}

func (w failingWriter) Write([]byte) (int, error) {
	return 0, w.err
}

type cancelAfterEOFReader struct {
	*bytes.Reader
	cancel context.CancelFunc
}

func (r cancelAfterEOFReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if err == io.EOF {
		r.cancel()
	}
	return n, err
}

type cancelOnFirstWriteWriter struct {
	cancel     context.CancelFunc
	writeCalls int
}

func (w *cancelOnFirstWriteWriter) Write(p []byte) (int, error) {
	w.writeCalls++
	w.cancel()
	return len(p), nil
}

func decodeFakeImage(io.Reader) (image.Image, error) {
	return image.NewRGBA(image.Rect(0, 0, 1, 1)), nil
}

func decodeFailImage(io.Reader) (image.Image, error) {
	return nil, errors.New("decode failed")
}

func decodeFakeConfig(io.Reader) (image.Config, error) {
	return image.Config{Width: 1, Height: 1, ColorModel: color.RGBAModel}, nil
}
