package barcode_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"strings"
	"sync"
	"testing"

	"github.com/bluetape4k/bluetape-go/imagekit/barcode"
)

func TestRenderRejects(t *testing.T) {
	tests := []struct {
		name string
		req  barcode.Request
		want error
	}{
		{
			name: "unknown kind",
			req:  barcode.Request{Kind: barcode.Kind(99), Content: "x", Width: 128, Height: 128},
			want: barcode.ErrInvalidOptions,
		},
		{
			name: "empty content",
			req:  barcode.Request{Kind: barcode.QR, Content: "", Width: 128, Height: 128},
			want: barcode.ErrInvalidOptions,
		},
		{
			name: "zero size",
			req:  barcode.Request{Kind: barcode.QR, Content: "x", Width: 0, Height: 128},
			want: barcode.ErrInvalidOptions,
		},
		{
			name: "non-square QR",
			req:  barcode.Request{Kind: barcode.QR, Content: "x", Width: 128, Height: 127},
			want: barcode.ErrInvalidOptions,
		},
		{
			name: "QR content limit",
			req:  barcode.Request{Kind: barcode.QR, Content: strings.Repeat("x", 1025), Width: 128, Height: 128},
			want: barcode.ErrInputTooLarge,
		},
		{
			name: "Code128 control",
			req:  barcode.Request{Kind: barcode.Code128, Content: "A\nB", Width: 256, Height: 64},
			want: barcode.ErrInvalidOptions,
		},
		{
			name: "width limit",
			req:  barcode.Request{Kind: barcode.Code128, Content: "A", Width: 4097, Height: 64},
			want: barcode.ErrImageTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := barcode.Render(context.Background(), tt.req)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Render(%s) error = %v, want errors.Is(..., %v)", tt.name, err, tt.want)
			}
		})
	}
}

func TestRenderRejectsNilContext(t *testing.T) {
	req := barcode.Request{Kind: barcode.QR, Content: "x", Width: 128, Height: 128}
	//nolint:staticcheck // nil context is an explicit API contract test.
	_, err := barcode.Render(nil, req)
	if !errors.Is(err, barcode.ErrInvalidOptions) {
		t.Fatalf("Render(nil, req) error = %v, want errors.Is(..., %v)", err, barcode.ErrInvalidOptions)
	}
}

func TestRenderRejectsBoundaries(t *testing.T) {
	tests := []struct {
		name string
		req  barcode.Request
		want error
	}{
		{
			name: "QR content over 1024 bytes",
			req:  barcode.Request{Kind: barcode.QR, Content: strings.Repeat("x", 1025), Width: 128, Height: 128},
			want: barcode.ErrInputTooLarge,
		},
		{
			name: "Code128 content over 80 bytes",
			req:  barcode.Request{Kind: barcode.Code128, Content: strings.Repeat("A", 81), Width: 256, Height: 64},
			want: barcode.ErrInputTooLarge,
		},
		{
			name: "invalid UTF-8",
			req:  barcode.Request{Kind: barcode.QR, Content: string([]byte{0xff}), Width: 128, Height: 128},
			want: barcode.ErrInvalidOptions,
		},
		{
			name: "width over 4096",
			req:  barcode.Request{Kind: barcode.Code128, Content: "A", Width: 4097, Height: 64},
			want: barcode.ErrImageTooLarge,
		},
		{
			name: "height over 4096",
			req:  barcode.Request{Kind: barcode.Code128, Content: "A", Width: 64, Height: 4097},
			want: barcode.ErrImageTooLarge,
		},
		{
			name: "pixels over 4194304",
			req:  barcode.Request{Kind: barcode.Code128, Content: "A", Width: 4096, Height: 1025},
			want: barcode.ErrImageTooLarge,
		},
		{
			name: "unknown QR level",
			req:  barcode.Request{Kind: barcode.QR, Content: "x", Width: 128, Height: 128, QRLevel: barcode.QRLevel(4)},
			want: barcode.ErrInvalidOptions,
		},
		{
			name: "Code128 QR level",
			req:  barcode.Request{Kind: barcode.Code128, Content: "A", Width: 256, Height: 64, QRLevel: barcode.QRLevelL},
			want: barcode.ErrInvalidOptions,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := barcode.Render(context.Background(), tt.req)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Render(%s) error = %v, want errors.Is(..., %v)", tt.name, err, tt.want)
			}
		})
	}
}

func TestEncodePNGRejects(t *testing.T) {
	req := barcode.Request{Kind: barcode.QR, Content: "x", Width: 0, Height: 128}
	_, err := barcode.EncodePNG(context.Background(), req)
	if !errors.Is(err, barcode.ErrInvalidOptions) {
		t.Fatalf("EncodePNG invalid request error = %v, want errors.Is(..., %v)", err, barcode.ErrInvalidOptions)
	}
}

func TestRenderActualProviders(t *testing.T) {
	tests := []struct {
		name   string
		req    barcode.Request
		quiet  int
		checkY bool
	}{
		{
			name:  "QR",
			req:   barcode.Request{Kind: barcode.QR, Content: "한글 QR payload", Width: 256, Height: 256},
			quiet: 4,
		},
		{
			name:   "Code128",
			req:    barcode.Request{Kind: barcode.Code128, Content: "BLUETAPE-546", Width: 512, Height: 128},
			quiet:  10,
			checkY: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img, err := barcode.Render(context.Background(), tt.req)
			if err != nil {
				t.Fatalf("Render error = %v", err)
			}
			if got, want := img.Bounds(), image.Rect(0, 0, tt.req.Width, tt.req.Height); got != want {
				t.Fatalf("bounds = %v, want %v", got, want)
			}
			if _, ok := img.(*image.Gray); !ok {
				t.Fatalf("image type = %T, want *image.Gray", img)
			}
			if _, ok := img.(interface{ Content() string }); ok {
				t.Fatal("returned image unexpectedly exposes provider Content")
			}
			dark, ok := darkBounds(img)
			if !ok {
				t.Fatal("rendered image has no dark modules")
			}
			if dark.Min.X < tt.quiet || tt.req.Width-dark.Max.X < tt.quiet {
				t.Fatalf("dark bounds = %v, horizontal quiet zone %d pixels is not preserved", dark, tt.quiet)
			}
			if !tt.checkY && (dark.Min.Y < tt.quiet || tt.req.Height-dark.Max.Y < tt.quiet) {
				t.Fatalf("dark bounds = %v, vertical quiet zone %d pixels is not preserved", dark, tt.quiet)
			}
			if tt.checkY {
				for y := 1; y < tt.req.Height; y++ {
					for x := 0; x < tt.req.Width; x++ {
						if img.At(x, y) != img.At(x, 0) {
							t.Fatalf("Code128 row %d differs at x=%d", y, x)
						}
					}
				}
			}
		})
	}
}

func TestRenderIsDeterministicAndDetached(t *testing.T) {
	req := barcode.Request{Kind: barcode.QR, Content: "same payload", Width: 192, Height: 192}
	first, err := barcode.Render(context.Background(), req)
	if err != nil {
		t.Fatalf("first Render error = %v", err)
	}
	second, err := barcode.Render(context.Background(), req)
	if err != nil {
		t.Fatalf("second Render error = %v", err)
	}
	firstGray := first.(*image.Gray)
	secondGray := second.(*image.Gray)
	if !equalPixels(firstGray, secondGray) {
		t.Fatal("identical requests produced different pixels")
	}
	original := secondGray.Pix[0]
	firstGray.Pix[0] = 0
	if secondGray.Pix[0] != original {
		t.Fatal("rendered images share a pixel buffer")
	}
}

func TestEncodePNGMatchesRender(t *testing.T) {
	tests := []barcode.Request{
		{Kind: barcode.QR, Content: "한글 PNG", Width: 128, Height: 128},
		{Kind: barcode.Code128, Content: "PNG-546", Width: 256, Height: 64},
	}
	for _, req := range tests {
		t.Run(req.Content, func(t *testing.T) {
			rendered, err := barcode.Render(context.Background(), req)
			if err != nil {
				t.Fatalf("Render error = %v", err)
			}
			payload, err := barcode.EncodePNG(context.Background(), req)
			if err != nil {
				t.Fatalf("EncodePNG error = %v", err)
			}
			decoded, err := png.Decode(bytes.NewReader(payload))
			if err != nil {
				t.Fatalf("png.Decode error = %v", err)
			}
			if decoded.Bounds() != rendered.Bounds() {
				t.Fatalf("decoded bounds = %v, rendered = %v", decoded.Bounds(), rendered.Bounds())
			}
			for y := rendered.Bounds().Min.Y; y < rendered.Bounds().Max.Y; y++ {
				for x := rendered.Bounds().Min.X; x < rendered.Bounds().Max.X; x++ {
					if decoded.At(x, y) != rendered.At(x, y) {
						t.Fatalf("pixel mismatch at (%d,%d): decoded=%v rendered=%v", x, y, decoded.At(x, y), rendered.At(x, y))
					}
				}
			}
		})
	}
}

func TestEncodePNGRejectsNilContext(t *testing.T) {
	req := barcode.Request{Kind: barcode.QR, Content: "x", Width: 64, Height: 64}
	//nolint:staticcheck // nil context is an explicit API contract test.
	_, err := barcode.EncodePNG(nil, req)
	if !errors.Is(err, barcode.ErrInvalidOptions) {
		t.Fatalf("EncodePNG(nil, req) error = %v, want ErrInvalidOptions", err)
	}
}

func TestEncodePNGReturnsIndependentBuffers(t *testing.T) {
	req := barcode.Request{Kind: barcode.Code128, Content: "buffer", Width: 128, Height: 32}
	first, err := barcode.EncodePNG(context.Background(), req)
	if err != nil {
		t.Fatalf("first EncodePNG error = %v", err)
	}
	second, err := barcode.EncodePNG(context.Background(), req)
	if err != nil {
		t.Fatalf("second EncodePNG error = %v", err)
	}
	original := second[0]
	first[0] ^= 0xff
	if second[0] != original {
		t.Fatal("EncodePNG results share a byte buffer")
	}
}

func TestConcurrent(t *testing.T) {
	const workers = 32
	errs := make(chan error, workers)
	var group sync.WaitGroup
	group.Add(workers)
	for i := 0; i < workers; i++ {
		i := i
		go func() {
			defer group.Done()
			req := barcode.Request{Kind: barcode.Code128, Content: "CONCURRENT", Width: 256, Height: 64}
			if i%2 == 0 {
				req = barcode.Request{Kind: barcode.QR, Content: "동시 호출", Width: 128, Height: 128}
			}
			img, err := barcode.Render(context.Background(), req)
			if err != nil {
				errs <- err
				return
			}
			if img.Bounds().Dx() != req.Width || img.Bounds().Dy() != req.Height {
				errs <- fmt.Errorf("bounds = %v, want %dx%d", img.Bounds(), req.Width, req.Height)
				return
			}
			if _, ok := img.(*image.Gray); !ok {
				errs <- fmt.Errorf("image type = %T, want *image.Gray", img)
			}
		}()
	}
	group.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent render error: %v", err)
	}
}

func darkBounds(img image.Image) (image.Rectangle, bool) {
	bounds := img.Bounds()
	minX, minY := bounds.Max.X, bounds.Max.Y
	maxX, maxY := bounds.Min.X, bounds.Min.Y
	found := false
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, _, _, _ := img.At(x, y).RGBA()
			if r >= 0x8000 {
				continue
			}
			found = true
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x+1 > maxX {
				maxX = x + 1
			}
			if y+1 > maxY {
				maxY = y + 1
			}
		}
	}
	return image.Rect(minX, minY, maxX, maxY), found
}

func equalPixels(left, right *image.Gray) bool {
	if left.Bounds() != right.Bounds() || len(left.Pix) != len(right.Pix) {
		return false
	}
	for i := range left.Pix {
		if left.Pix[i] != right.Pix[i] {
			return false
		}
	}
	return true
}
