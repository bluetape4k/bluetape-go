package barcode

import (
	"context"
	"image"
	"image/color"

	"github.com/bluetape4k/bluetape-go/imagekit"
	"github.com/boombuler/barcode/code128"
	"github.com/boombuler/barcode/qr"
)

const (
	qrQuietModules      = 4
	code128QuietModules = 10
)

type encodeFunc func(content string, level QRLevel) (image.Image, error)

// Render validates a request, invokes the selected barcode provider, and
// returns a detached black-and-white image with its fixed quiet zone.
func Render(ctx context.Context, req Request) (image.Image, error) {
	return renderWithEncoder(ctx, req, encoderForKind(req.Kind))
}

func encoderForKind(kind Kind) encodeFunc {
	switch kind {
	case QR:
		return func(content string, level QRLevel) (image.Image, error) {
			return qr.Encode(content, qrLevel(level), qr.Unicode)
		}
	case Code128:
		return func(content string, _ QRLevel) (image.Image, error) {
			return code128.Encode(content)
		}
	default:
		return nil
	}
}

func qrLevel(level QRLevel) qr.ErrorCorrectionLevel {
	switch level {
	case QRLevelL:
		return qr.L
	case QRLevelQ:
		return qr.Q
	case QRLevelH:
		return qr.H
	default:
		return qr.M
	}
}

func renderWithEncoder(ctx context.Context, req Request, encoder encodeFunc) (image.Image, error) {
	if err := validateRequest(ctx, req); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if encoder == nil {
		return nil, newEncodeError(formatForKind(req.Kind))
	}

	encoded, encodeErr := encoder(req.Content, req.QRLevel)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if encodeErr != nil || encoded == nil {
		return nil, newEncodeError(formatForKind(req.Kind))
	}

	bounds := encoded.Bounds()
	symbolWidth, ok := checkedSpan(bounds.Min.X, bounds.Max.X)
	if !ok || symbolWidth <= 0 {
		if bounds.Max.X > bounds.Min.X {
			return nil, imageTooLargeError("geometry")
		}
		return nil, malformedEncodeError(formatForKind(req.Kind))
	}
	symbolHeight, ok := checkedSpan(bounds.Min.Y, bounds.Max.Y)
	if !ok || symbolHeight <= 0 {
		if bounds.Max.Y > bounds.Min.Y {
			return nil, imageTooLargeError("geometry")
		}
		return nil, malformedEncodeError(formatForKind(req.Kind))
	}

	// The provider has no context-aware API. This checkpoint intentionally
	// precedes every geometry decision and image allocation.
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	switch req.Kind {
	case QR:
		if symbolWidth != symbolHeight {
			return nil, malformedEncodeError("qr")
		}
		return renderQR(ctx, req, encoded, bounds, symbolWidth)
	case Code128:
		if symbolHeight != 1 {
			return nil, malformedEncodeError("code128")
		}
		return renderCode128(ctx, req, encoded, bounds, symbolWidth)
	default:
		return nil, invalidError("kind")
	}
}

func renderQR(ctx context.Context, req Request, source image.Image, bounds image.Rectangle, symbolSize int) (image.Image, error) {
	quietWidth := qrQuietModules
	quietTotal, ok := checkedMul(quietWidth, 2)
	if !ok {
		return nil, imageTooLargeError("geometry")
	}
	requiredWidth, ok := checkedAdd(symbolSize, quietTotal)
	if !ok {
		return nil, imageTooLargeError("geometry")
	}
	requiredHeight := requiredWidth
	scaleX := req.Width / requiredWidth
	scaleY := req.Height / requiredHeight
	scale := scaleX
	if scaleY < scale {
		scale = scaleY
	}
	if scale < 1 {
		return nil, invalidError("size")
	}

	scaledSymbol, ok := checkedMul(symbolSize, scale)
	if !ok {
		return nil, imageTooLargeError("geometry")
	}
	quietPixels, ok := checkedMul(quietWidth, scale)
	if !ok {
		return nil, imageTooLargeError("geometry")
	}
	renderedWidth, ok := checkedAdd(scaledSymbol, quietPixels)
	if ok {
		renderedWidth, ok = checkedAdd(renderedWidth, quietPixels)
	}
	if !ok || renderedWidth > req.Width || renderedWidth > req.Height {
		return nil, imageTooLargeError("geometry")
	}
	offsetX := (req.Width - renderedWidth) / 2
	offsetY := (req.Height - renderedWidth) / 2
	return copyQR(ctx, req, source, bounds, scaledSymbol, scale, quietPixels, offsetX, offsetY)
}

func renderCode128(ctx context.Context, req Request, source image.Image, bounds image.Rectangle, symbolWidth int) (image.Image, error) {
	quietWidth := code128QuietModules
	quietTotal, ok := checkedMul(quietWidth, 2)
	if !ok {
		return nil, imageTooLargeError("geometry")
	}
	requiredWidth, ok := checkedAdd(symbolWidth, quietTotal)
	if !ok {
		return nil, imageTooLargeError("geometry")
	}
	scale := req.Width / requiredWidth
	if scale < 1 {
		return nil, invalidError("size")
	}
	scaledSymbolWidth, ok := checkedMul(symbolWidth, scale)
	if !ok {
		return nil, imageTooLargeError("geometry")
	}
	quietPixels, ok := checkedMul(quietWidth, scale)
	if !ok {
		return nil, imageTooLargeError("geometry")
	}
	renderedWidth, ok := checkedAdd(scaledSymbolWidth, quietPixels)
	if ok {
		renderedWidth, ok = checkedAdd(renderedWidth, quietPixels)
	}
	if !ok || renderedWidth > req.Width {
		return nil, imageTooLargeError("geometry")
	}
	offsetX := (req.Width - renderedWidth) / 2
	return copyCode128(ctx, req, source, bounds, scaledSymbolWidth, scale, quietPixels, offsetX)
}

func copyQR(ctx context.Context, req Request, source image.Image, bounds image.Rectangle, scaledSymbol, scale, quietPixels, offsetX, offsetY int) (image.Image, error) {
	symbolStartX, ok := checkedAdd(offsetX, quietPixels)
	if !ok {
		return nil, imageTooLargeError("geometry")
	}
	symbolEndX, ok := checkedAdd(symbolStartX, scaledSymbol)
	if !ok {
		return nil, imageTooLargeError("geometry")
	}
	symbolStartY, ok := checkedAdd(offsetY, quietPixels)
	if !ok {
		return nil, imageTooLargeError("geometry")
	}
	symbolEndY, ok := checkedAdd(symbolStartY, scaledSymbol)
	if !ok {
		return nil, imageTooLargeError("geometry")
	}
	dst := image.NewGray(image.Rect(0, 0, req.Width, req.Height))
	fillWhite(dst)
	for y := 0; y < req.Height; y++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if y < symbolStartY || y >= symbolEndY {
			continue
		}
		sourceY := bounds.Min.Y + (y-symbolStartY)/scale
		for x := 0; x < req.Width; x++ {
			if x < symbolStartX || x >= symbolEndX {
				continue
			}
			sourceX := bounds.Min.X + (x-symbolStartX)/scale
			if isDark(source.At(sourceX, sourceY)) {
				dst.SetGray(x, y, color.Gray{Y: 0})
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return dst, nil
}

func copyCode128(ctx context.Context, req Request, source image.Image, bounds image.Rectangle, scaledSymbolWidth, scale, quietPixels, offsetX int) (image.Image, error) {
	symbolStartX, ok := checkedAdd(offsetX, quietPixels)
	if !ok {
		return nil, imageTooLargeError("geometry")
	}
	symbolEndX, ok := checkedAdd(symbolStartX, scaledSymbolWidth)
	if !ok {
		return nil, imageTooLargeError("geometry")
	}
	dst := image.NewGray(image.Rect(0, 0, req.Width, req.Height))
	fillWhite(dst)
	for y := 0; y < req.Height; y++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for x := 0; x < req.Width; x++ {
			if x < symbolStartX || x >= symbolEndX {
				continue
			}
			sourceX := bounds.Min.X + (x-symbolStartX)/scale
			if isDark(source.At(sourceX, bounds.Min.Y)) {
				dst.SetGray(x, y, color.Gray{Y: 0})
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return dst, nil
}

func fillWhite(img *image.Gray) {
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}
}

func isDark(value color.Color) bool {
	gray := color.GrayModel.Convert(value).(color.Gray)
	return gray.Y < 128
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

func malformedEncodeError(format string) error {
	return newEncodeError(format)
}

func checkedSpan(minimum, maximum int) (int, bool) {
	if maximum <= minimum {
		return 0, false
	}
	span := maximum - minimum
	if span <= 0 {
		return 0, false
	}
	return span, true
}

func checkedAdd(left, right int) (int, bool) {
	if left < 0 || right < 0 || right > maxInt-left {
		return 0, false
	}
	return left + right, true
}

func checkedMul(left, right int) (int, bool) {
	if left < 0 || right < 0 || (left != 0 && right > maxInt/left) {
		return 0, false
	}
	return left * right, true
}

const maxInt = int(^uint(0) >> 1)
