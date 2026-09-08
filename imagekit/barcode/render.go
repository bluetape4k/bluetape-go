package barcode

import (
	"context"
	"image"

	"github.com/bluetape4k/bluetape-go/imagekit"
)

// Render validates a barcode request and returns its rendered image.
//
// Provider rendering is added after the validation boundary in Task 3.
func Render(ctx context.Context, req Request) (image.Image, error) {
	if err := validateRequest(ctx, req); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, newEncodeError(formatForKind(req.Kind))
}

// EncodePNG validates a barcode request and reserves the PNG output contract.
//
// PNG encoding is added after the renderer boundary in Task 4.
func EncodePNG(ctx context.Context, req Request) ([]byte, error) {
	if err := validateRequest(ctx, req); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, newEncodeError("png")
}

func formatForKind(kind Kind) string {
	switch kind {
	case QR:
		return "qr"
	case Code128:
		return "code128"
	default:
		return "barcode"
	}
}

func newEncodeError(format string) error {
	return &imagekit.Error{Kind: ErrEncode, Operation: "render", Format: format}
}
