package barcode

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/imagekit"
	providerbarcode "github.com/boombuler/barcode"
)

type fakeBarcode struct {
	rect     image.Rectangle
	dims     byte
	content  string
	pixels   map[image.Point]color.Color
	at       func(int, int) color.Color
	metadata providerbarcode.Metadata
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

func (f *fakeBarcode) Content() string {
	return f.content
}

func (f *fakeBarcode) Metadata() providerbarcode.Metadata {
	if f.metadata.Dimensions != 0 {
		return f.metadata
	}
	return providerbarcode.Metadata{CodeKind: "fake", Dimensions: f.dims}
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
	_, err := renderWithEncoder(ctx, code128Request(64, 32), func(string, QRLevel) (providerbarcode.Barcode, error) {
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
	source := &fakeBarcode{rect: image.Rect(0, 0, 1, 1), dims: 1}
	cause := &providerFailure{message: "secret output failure"}
	_, err := renderWithEncoder(context.Background(), code128Request(21, 4), func(string, QRLevel) (providerbarcode.Barcode, error) {
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
		source providerbarcode.Barcode
	}{
		{name: "nil", req: code128Request(64, 32)},
		{name: "empty bounds", req: code128Request(64, 32), source: &fakeBarcode{rect: image.Rectangle{}, dims: 1}},
		{name: "QR nonsquare", req: qrRequest(64, 64), source: &fakeBarcode{rect: image.Rect(0, 0, 2, 1), dims: 2}},
		{name: "Code128 height", req: code128Request(64, 32), source: &fakeBarcode{rect: image.Rect(0, 0, 3, 2), dims: 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := renderWithEncoder(context.Background(), tt.req, func(string, QRLevel) (providerbarcode.Barcode, error) {
				return tt.source, nil
			})
			if !errors.Is(err, ErrEncode) {
				t.Fatalf("error = %v, want ErrEncode", err)
			}
		})
	}
}

func TestRenderWithEncoderRejectsProviderOutputWithOverflowingBounds(t *testing.T) {
	max := int(^uint(0) >> 1)
	min := -max - 1
	source := &fakeBarcode{rect: image.Rect(min, 0, max, 1), dims: 1}
	_, err := renderWithEncoder(context.Background(), code128Request(64, 32), func(string, QRLevel) (providerbarcode.Barcode, error) {
		return source, nil
	})
	if !errors.Is(err, ErrImageTooLarge) {
		t.Fatalf("error = %v, want ErrImageTooLarge", err)
	}
}

func TestRenderWithEncoderCancellationAfterProviderWins(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	called := 0
	_, err := renderWithEncoder(ctx, code128Request(64, 32), func(string, QRLevel) (providerbarcode.Barcode, error) {
		called++
		cancel()
		return &fakeBarcode{rect: image.Rect(0, 0, 3, 1), dims: 1}, nil
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
		dims: 1,
		at: func(int, int) color.Color {
			cancel()
			return color.Black
		},
	}
	_, err := renderWithEncoder(ctx, code128Request(21, 4), func(string, QRLevel) (providerbarcode.Barcode, error) {
		return source, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestRenderWithEncoderRejectsNilContextWithoutProviderCall(t *testing.T) {
	calls := 0
	_, err := renderWithEncoder(nil, code128Request(64, 32), func(string, QRLevel) (providerbarcode.Barcode, error) {
		calls++
		return &fakeBarcode{rect: image.Rect(0, 0, 3, 1), dims: 1}, nil
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
	_, err := renderWithEncoder(ctx, code128Request(64, 32), func(string, QRLevel) (providerbarcode.Barcode, error) {
		calls++
		return &fakeBarcode{rect: image.Rect(0, 0, 3, 1), dims: 1}, nil
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
	_, err := renderWithEncoder(context.Background(), req, func(content string, level QRLevel) (providerbarcode.Barcode, error) {
		gotContent = content
		gotLevel = level
		return &fakeBarcode{rect: image.Rect(0, 0, 3, 3), dims: 2}, nil
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
		rect:    image.Rect(2, 4, 5, 7),
		dims:    2,
		content: "secret payload",
		pixels: map[image.Point]color.Color{
			{X: 2, Y: 4}: color.Gray{Y: 127},
			{X: 3, Y: 4}: color.Gray{Y: 128},
		},
	}
	img, err := renderWithEncoder(context.Background(), qrRequest(11, 11), func(string, QRLevel) (providerbarcode.Barcode, error) {
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
		rect:    image.Rect(5, 9, 8, 10),
		dims:    1,
		pixels:  map[image.Point]color.Color{{X: 5, Y: 9}: color.Black},
		content: "secret payload",
	}
	img, err := renderWithEncoder(context.Background(), code128Request(23, 5), func(string, QRLevel) (providerbarcode.Barcode, error) {
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
	qrSource := &fakeBarcode{rect: image.Rect(0, 0, 3, 3), dims: 2}
	_, err := renderWithEncoder(context.Background(), qrRequest(10, 10), func(string, QRLevel) (providerbarcode.Barcode, error) {
		return qrSource, nil
	})
	if !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("QR error = %v, want ErrInvalidOptions", err)
	}

	codeSource := &fakeBarcode{rect: image.Rect(0, 0, 3, 1), dims: 1}
	_, err = renderWithEncoder(context.Background(), code128Request(22, 5), func(string, QRLevel) (providerbarcode.Barcode, error) {
		return codeSource, nil
	})
	if !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("Code128 error = %v, want ErrInvalidOptions", err)
	}
}
