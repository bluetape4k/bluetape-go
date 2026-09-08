package barcode_test

import (
	"context"
	"errors"
	"strings"
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
