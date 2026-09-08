package barcode

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/imagekit"
)

type fakeBarcode struct {
	rect   image.Rectangle
	pixels map[image.Point]color.Color
	at     func(int, int) color.Color
}

func (f *fakeBarcode) Bounds() image.Rectangle {
	return f.rect
}

func (f *fakeBarcode) At(x, y int) color.Color {
	if f.at != nil {
		return f.at(x, y)
	}
	if pixel, ok := f.pixels[image.Point{X: x, Y: y}]; ok {
		return pixel
	}
	return color.White
}

func (f *fakeBarcode) ColorModel() color.Model {
	return color.GrayModel
}

type providerFailure struct {
	message string
}

func (e *providerFailure) Error() string {
	return e.message
}

func code128Request(width, height int) Request {
	return Request{Kind: Code128, Content: "ABC", Width: width, Height: height}
}

func qrRequest(width, height int) Request {
	return Request{Kind: QR, Content: "payload", Width: width, Height: height}
}

func TestRenderWithEncoderRedactsProviderError(t *testing.T) {
	ctx := context.Background()
	cause := &providerFailure{message: `provider secret "payload"`}
	_, err := renderWithEncoder(ctx, code128Request(64, 32), func(string, QRLevel) (image.Image, error) {
		return nil, cause
	})
	if !errors.Is(err, ErrEncode) {
		t.Fatalf("error = %v, want ErrEncode", err)
	}
	if errors.Is(err, cause) {
		t.Fatal("provider error was unexpectedly wrapped")
	}
	var restored *providerFailure
	if errors.As(err, &restored) {
		t.Fatal("provider error was unexpectedly restored by errors.As")
	}
	if errors.Unwrap(err) != nil {
		t.Fatalf("errors.Unwrap(error) = %v, want nil", errors.Unwrap(err))
	}
	for _, formatted := range []string{err.Error(), fmt.Sprintf("%+v", err)} {
		if strings.Contains(formatted, cause.message) || strings.Contains(formatted, "payload") {
			t.Fatalf("formatted error leaked provider data: %q", formatted)
		}
	}
	var imageError *imagekit.Error
	if !errors.As(err, &imageError) || imageError.Cause != nil {
		t.Fatalf("error = %#v, want imagekit.Error with nil Cause", err)
	}
}

func TestRenderWithEncoderRedactsOutputPlusProviderError(t *testing.T) {
	source := &fakeBarcode{rect: image.Rect(0, 0, 1, 1)}
	cause := &providerFailure{message: "secret output failure"}
	_, err := renderWithEncoder(context.Background(), code128Request(21, 4), func(string, QRLevel) (image.Image, error) {
		return source, cause
	})
	if !errors.Is(err, ErrEncode) || strings.Contains(err.Error(), "secret") {
		t.Fatalf("error = %v, want redacted ErrEncode", err)
	}
}

func TestRenderWithEncoderRejectsMalformedProviderOutput(t *testing.T) {
	tests := []struct {
		name   string
		req    Request
		source image.Image
	}{
		{name: "nil", req: code128Request(64, 32)},
		{name: "empty bounds", req: code128Request(64, 32), source: &fakeBarcode{rect: image.Rectangle{}}},
		{name: "QR nonsquare", req: qrRequest(64, 64), source: &fakeBarcode{rect: image.Rect(0, 0, 2, 1)}},
		{name: "Code128 height", req: code128Request(64, 32), source: &fakeBarcode{rect: image.Rect(0, 0, 3, 2)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := renderWithEncoder(context.Background(), tt.req, func(string, QRLevel) (image.Image, error) {
				return tt.source, nil
			})
			if !errors.Is(err, ErrEncode) {
				t.Fatalf("error = %v, want ErrEncode", err)
			}
		})
	}
}

func TestRenderWithEncoderRejectsProviderOutputWithOverflowingBounds(t *testing.T) {
	maxValue := int(^uint(0) >> 1)
	minValue := -maxValue - 1
	source := &fakeBarcode{rect: image.Rect(minValue, 0, maxValue, 1)}
	_, err := renderWithEncoder(context.Background(), code128Request(64, 32), func(string, QRLevel) (image.Image, error) {
		return source, nil
	})
	if !errors.Is(err, ErrImageTooLarge) {
		t.Fatalf("error = %v, want ErrImageTooLarge", err)
	}
}

func TestRenderWithEncoderCancellationAfterProviderWins(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	called := 0
	_, err := renderWithEncoder(ctx, code128Request(64, 32), func(string, QRLevel) (image.Image, error) {
		called++
		cancel()
		return &fakeBarcode{rect: image.Rect(0, 0, 3, 1)}, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if called != 1 {
		t.Fatalf("provider calls = %d, want 1", called)
	}
}

func TestRenderWithEncoderCancellationDuringRowCopyWins(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	source := &fakeBarcode{
		rect: image.Rect(0, 0, 1, 1),
		at: func(int, int) color.Color {
			cancel()
			return color.Black
		},
	}
	_, err := renderWithEncoder(ctx, code128Request(21, 4), func(string, QRLevel) (image.Image, error) {
		return source, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestRenderWithEncoderRejectsNilContextWithoutProviderCall(t *testing.T) {
	calls := 0
	//nolint:staticcheck // nil context is an explicit API contract test.
	_, err := renderWithEncoder(nil, code128Request(64, 32), func(string, QRLevel) (image.Image, error) {
		calls++
		return &fakeBarcode{rect: image.Rect(0, 0, 3, 1)}, nil
	})
	if !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("error = %v, want ErrInvalidOptions", err)
	}
	if calls != 0 {
		t.Fatalf("provider calls = %d, want 0", calls)
	}
}

func TestRenderWithEncoderChecksContextBeforeProvider(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	_, err := renderWithEncoder(ctx, code128Request(64, 32), func(string, QRLevel) (image.Image, error) {
		calls++
		return &fakeBarcode{rect: image.Rect(0, 0, 3, 1)}, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if calls != 0 {
		t.Fatalf("provider calls = %d, want 0", calls)
	}
}

func TestRenderWithEncoderPreservesProviderInput(t *testing.T) {
	req := Request{Kind: QR, Content: "한글 payload", Width: 11, Height: 11, QRLevel: QRLevelH}
	var gotContent string
	var gotLevel QRLevel
	_, err := renderWithEncoder(context.Background(), req, func(content string, level QRLevel) (image.Image, error) {
		gotContent = content
		gotLevel = level
		return &fakeBarcode{rect: image.Rect(0, 0, 3, 3)}, nil
	})
	if err != nil {
		t.Fatalf("render error = %v", err)
	}
	if gotContent != req.Content {
		t.Fatalf("provider content = %q, want %q", gotContent, req.Content)
	}
	if gotLevel != req.QRLevel {
		t.Fatalf("provider QR level = %v, want %v", gotLevel, req.QRLevel)
	}
}

func TestRenderWithEncoderQRGeometryAndNormalization(t *testing.T) {
	source := &fakeBarcode{
		rect: image.Rect(2, 4, 5, 7),
		pixels: map[image.Point]color.Color{
			{X: 2, Y: 4}: color.Gray{Y: 127},
			{X: 3, Y: 4}: color.Gray{Y: 128},
		},
	}
	img, err := renderWithEncoder(context.Background(), qrRequest(11, 11), func(string, QRLevel) (image.Image, error) {
		return source, nil
	})
	if err != nil {
		t.Fatalf("render error = %v", err)
	}
	gray, ok := img.(*image.Gray)
	if !ok {
		t.Fatalf("image type = %T, want *image.Gray", img)
	}
	if got, want := gray.Bounds(), image.Rect(0, 0, 11, 11); got != want {
		t.Fatalf("bounds = %v, want %v", got, want)
	}
	if gray.GrayAt(4, 4).Y != 0 || gray.GrayAt(5, 4).Y != 255 {
		t.Fatalf("normalized pixels = %d/%d, want 0/255", gray.GrayAt(4, 4).Y, gray.GrayAt(5, 4).Y)
	}
	for y := 0; y < 11; y++ {
		if gray.GrayAt(0, y).Y != 255 || gray.GrayAt(10, y).Y != 255 {
			t.Fatalf("quiet zone at x edge y=%d is not white", y)
		}
	}
	for x := 0; x < 11; x++ {
		if gray.GrayAt(x, 0).Y != 255 || gray.GrayAt(x, 10).Y != 255 {
			t.Fatalf("quiet zone at y edge x=%d is not white", x)
		}
	}
}

func TestRenderWithEncoderCode128GeometryFillsHeight(t *testing.T) {
	source := &fakeBarcode{
		rect:   image.Rect(5, 9, 8, 10),
		pixels: map[image.Point]color.Color{{X: 5, Y: 9}: color.Black},
	}
	img, err := renderWithEncoder(context.Background(), code128Request(23, 5), func(string, QRLevel) (image.Image, error) {
		return source, nil
	})
	if err != nil {
		t.Fatalf("render error = %v", err)
	}
	gray := img.(*image.Gray)
	for y := 0; y < 5; y++ {
		if gray.GrayAt(10, y).Y != 0 {
			t.Fatalf("dark bar at x=10 y=%d = %d, want 0", y, gray.GrayAt(10, y).Y)
		}
		if gray.GrayAt(0, y).Y != 255 || gray.GrayAt(22, y).Y != 255 {
			t.Fatalf("quiet zone at y=%d is not white", y)
		}
	}
}

func TestRenderWithEncoderRejectsOnePixelShortCanvas(t *testing.T) {
	qrSource := &fakeBarcode{rect: image.Rect(0, 0, 3, 3)}
	_, err := renderWithEncoder(context.Background(), qrRequest(10, 10), func(string, QRLevel) (image.Image, error) {
		return qrSource, nil
	})
	if !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("QR error = %v, want ErrInvalidOptions", err)
	}

	codeSource := &fakeBarcode{rect: image.Rect(0, 0, 3, 1)}
	_, err = renderWithEncoder(context.Background(), code128Request(22, 5), func(string, QRLevel) (image.Image, error) {
		return codeSource, nil
	})
	if !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("Code128 error = %v, want ErrInvalidOptions", err)
	}
}

func TestCappedWriterHonorsLimitWithoutPartialWrite(t *testing.T) {
	writer := &cappedWriter{ctx: context.Background(), limit: 3}
	if n, err := writer.Write([]byte("abc")); n != 3 || err != nil {
		t.Fatalf("first Write = (%d, %v), want (3, nil)", n, err)
	}
	if n, err := writer.Write([]byte("d")); n != 0 || !errors.Is(err, errPNGTooLarge) {
		t.Fatalf("second Write = (%d, %v), want (0, errPNGTooLarge)", n, err)
	}
	if got := writer.buf.String(); got != "abc" {
		t.Fatalf("buffer = %q, want %q", got, "abc")
	}
}

func TestCappedWriterCancellationPrecedesLimit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	writer := &cappedWriter{ctx: ctx, limit: 1}
	n, err := writer.Write([]byte("too large"))
	if n != 0 || !errors.Is(err, context.Canceled) {
		t.Fatalf("Write = (%d, %v), want (0, context.Canceled)", n, err)
	}
}

func TestEncodePNGWithRendererRejectsPreCanceledContextWithoutRender(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	img, err := encodePNGWithRenderer(ctx, code128Request(64, 32), func(context.Context, Request) (image.Image, error) {
		calls++
		return image.NewGray(image.Rect(0, 0, 1, 1)), nil
	}, maxPNGBytes)
	if img != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("result = (%v, %v), want (nil, context.Canceled)", img, err)
	}
	if calls != 0 {
		t.Fatalf("renderer calls = %d, want 0", calls)
	}
}

func TestEncodePNGWithRendererPreservesDeadlineAfterRender(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Nanosecond))
	defer cancel()
	calls := 0
	img, err := encodePNGWithRenderer(ctx, code128Request(64, 32), func(context.Context, Request) (image.Image, error) {
		calls++
		return image.NewGray(image.Rect(0, 0, 1, 1)), nil
	}, maxPNGBytes)
	if img != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("result = (%v, %v), want (nil, context.DeadlineExceeded)", img, err)
	}
	if calls != 0 {
		t.Fatalf("renderer calls = %d, want 0", calls)
	}
}

func TestEncodePNGWithRendererContextAfterRenderWins(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	img, err := encodePNGWithRenderer(ctx, code128Request(64, 32), func(context.Context, Request) (image.Image, error) {
		cancel()
		return image.NewGray(image.Rect(0, 0, 1, 1)), nil
	}, maxPNGBytes)
	if img != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("result = (%v, %v), want (nil, context.Canceled)", img, err)
	}
}

func TestEncodePNGWithRendererRejectsNilImageAndSmallCap(t *testing.T) {
	valid := func(context.Context, Request) (image.Image, error) {
		return nil, nil
	}
	img, err := encodePNGWithRenderer(context.Background(), code128Request(64, 32), valid, maxPNGBytes)
	if img != nil || !errors.Is(err, ErrEncode) {
		t.Fatalf("nil image result = (%v, %v), want (nil, ErrEncode)", img, err)
	}

	checkerboard := image.NewGray(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			if (x+y)%2 == 0 {
				checkerboard.SetGray(x, y, color.Gray{Y: 0})
			}
		}
	}
	img, err = encodePNGWithRenderer(context.Background(), code128Request(64, 32), func(context.Context, Request) (image.Image, error) {
		return checkerboard, nil
	}, 1)
	if img != nil || !errors.Is(err, ErrEncode) {
		t.Fatalf("capped result = (%v, %v), want (nil, ErrEncode)", img, err)
	}
	var imageError *imagekit.Error
	if !errors.As(err, &imageError) || imageError.Cause != nil {
		t.Fatalf("capped error = %#v, want imagekit.Error with nil Cause", err)
	}
}

func TestEncodePNGWithRendererCancellationDuringWrite(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	imageToEncode := &cancelingImage{ctx: cancel, image: image.NewGray(image.Rect(0, 0, 16, 16))}
	img, err := encodePNGWithRenderer(ctx, code128Request(64, 32), func(context.Context, Request) (image.Image, error) {
		return imageToEncode, nil
	}, maxPNGBytes)
	if img != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("result = (%v, %v), want (nil, context.Canceled)", img, err)
	}
}

type cancelingImage struct {
	ctx   context.CancelFunc
	image *image.Gray
}

func (c *cancelingImage) ColorModel() color.Model {
	return c.image.ColorModel()
}

func (c *cancelingImage) Bounds() image.Rectangle {
	return c.image.Bounds()
}

func (c *cancelingImage) At(x, y int) color.Color {
	c.ctx()
	return c.image.At(x, y)
}

func TestEncodePNGWithRendererProducesDecodablePNG(t *testing.T) {
	source := image.NewGray(image.Rect(0, 0, 3, 3))
	source.SetGray(1, 1, color.Gray{Y: 0})
	payload, err := encodePNGWithRenderer(context.Background(), code128Request(64, 32), func(context.Context, Request) (image.Image, error) {
		return source, nil
	}, maxPNGBytes)
	if err != nil {
		t.Fatalf("encode error = %v", err)
	}
	decoded, err := png.Decode(bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if decoded.Bounds() != source.Bounds() {
		t.Fatalf("decoded bounds = %v, want %v", decoded.Bounds(), source.Bounds())
	}
}
