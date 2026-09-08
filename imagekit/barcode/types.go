package barcode

import (
	"context"
	"unicode/utf8"

	"github.com/bluetape4k/bluetape-go/imagekit"
)

const (
	maxContentBytes = 1024
	maxCode128Bytes = 80
	maxDimension    = 4096
	maxPixels       = 4_194_304
)

var (
	// ErrInvalidOptions reports a malformed barcode request or nil context.
	ErrInvalidOptions = imagekit.ErrInvalidOptions
	// ErrInputTooLarge reports content that exceeds the barcode input limit.
	ErrInputTooLarge = imagekit.ErrInputTooLarge
	// ErrImageTooLarge reports dimensions or pixels that exceed the image limit.
	ErrImageTooLarge = imagekit.ErrImageTooLarge
	// ErrEncode reports a provider or output encoding failure.
	ErrEncode = imagekit.ErrEncode
)

// Kind identifies the barcode family to render.
type Kind uint8

const (
	// QR selects a UTF-8 QR code.
	QR Kind = iota + 1
	// Code128 selects a printable-ASCII Code128 barcode.
	Code128
)

// QRLevel selects the QR error-correction level.
type QRLevel uint8

const (
	// QRLevelM selects the default medium QR error correction.
	QRLevelM QRLevel = iota
	// QRLevelL selects low QR error correction.
	QRLevelL
	// QRLevelQ selects quartile QR error correction.
	QRLevelQ
	// QRLevelH selects high QR error correction.
	QRLevelH
)

// Request describes one bounded barcode image request.
type Request struct {
	// Kind selects QR or Code128 rendering.
	Kind Kind
	// Content is the payload encoded into the barcode.
	Content string
	// Width is the requested output width in pixels.
	Width int
	// Height is the requested output height in pixels.
	Height int
	// QRLevel selects QR error correction; Code128 accepts only QRLevelM.
	QRLevel QRLevel
}

func validateRequest(ctx context.Context, req Request) error {
	if ctx == nil {
		return invalidError("context")
	}

	if req.Kind != QR && req.Kind != Code128 {
		return invalidError("kind")
	}
	if !utf8.ValidString(req.Content) {
		return invalidError("content")
	}
	if req.Content == "" {
		return invalidError("content")
	}

	switch req.Kind {
	case QR:
		if len(req.Content) > maxContentBytes {
			return inputTooLargeError("content")
		}
		if req.QRLevel > QRLevelH {
			return invalidError("qr_level")
		}
	case Code128:
		if len(req.Content) > maxCode128Bytes {
			return inputTooLargeError("content")
		}
		if req.QRLevel != QRLevelM {
			return invalidError("qr_level")
		}
		for i := 0; i < len(req.Content); i++ {
			if req.Content[i] < 0x20 || req.Content[i] > 0x7e {
				return invalidError("content")
			}
		}
	}

	if req.Width <= 0 || req.Height <= 0 {
		return invalidError("size")
	}
	if req.Kind == QR && req.Width != req.Height {
		return invalidError("size")
	}
	if req.Width > maxDimension || req.Height > maxDimension {
		return imageTooLargeError("dimensions")
	}
	if req.Width > maxPixels/req.Height {
		return imageTooLargeError("pixels")
	}
	return nil
}

func invalidError(operation string) error {
	return &imagekit.Error{Kind: ErrInvalidOptions, Operation: operation}
}

func inputTooLargeError(operation string) error {
	return &imagekit.Error{Kind: ErrInputTooLarge, Operation: operation}
}

func imageTooLargeError(operation string) error {
	return &imagekit.Error{Kind: ErrImageTooLarge, Operation: operation}
}
