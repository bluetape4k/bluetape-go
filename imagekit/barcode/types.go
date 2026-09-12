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
	// ErrInvalidOptions 오류는 잘못된 barcode 요청 또는 nil context를 나타낸다.
	ErrInvalidOptions = imagekit.ErrInvalidOptions
	// ErrInputTooLarge 오류는 barcode 입력 한도를 초과한 content를 나타낸다.
	ErrInputTooLarge = imagekit.ErrInputTooLarge
	// ErrImageTooLarge 오류는 이미지 크기 또는 pixel 한도 초과를 나타낸다.
	ErrImageTooLarge = imagekit.ErrImageTooLarge
	// ErrEncode 오류는 provider 또는 출력 encoding 실패를 나타낸다.
	ErrEncode = imagekit.ErrEncode
)

// Kind 값은 렌더링할 barcode family를 식별한다.
type Kind uint8

const (
	// QR 값은 UTF-8 QR code를 선택한다.
	QR Kind = iota + 1
	// Code128 값은 printable ASCII Code128 barcode를 선택한다.
	Code128
)

// QRLevel 값은 QR error-correction level을 선택한다.
type QRLevel uint8

const (
	// QRLevelM 값은 기본 medium QR error correction을 선택한다.
	QRLevelM QRLevel = iota
	// QRLevelL 값은 low QR error correction을 선택한다.
	QRLevelL
	// QRLevelQ 값은 quartile QR error correction을 선택한다.
	QRLevelQ
	// QRLevelH 값은 high QR error correction을 선택한다.
	QRLevelH
)

// Request 값은 하나의 bounded barcode 이미지 요청을 설명한다.
type Request struct {
	// Kind 필드는 QR 또는 Code128 렌더링을 선택한다.
	Kind Kind
	// Content 필드는 barcode에 encoding할 payload다.
	Content string
	// Width 필드는 요청한 출력 width(pixel)다.
	Width int
	// Height 필드는 요청한 출력 height(pixel)다.
	Height int
	// QRLevel 필드는 QR error correction을 선택하며 Code128은 QRLevelM만 허용한다.
	QRLevel QRLevel
}

func validateRequest(ctx context.Context, req Request) error {
	if ctx == nil {
		return invalidError("context")
	}

	if req.Kind != QR && req.Kind != Code128 {
		return invalidError("kind")
	}
	if req.Content == "" {
		return invalidError("content")
	}

	switch req.Kind {
	case QR:
		if len(req.Content) > maxContentBytes {
			return inputTooLargeError("content")
		}
		if !utf8.ValidString(req.Content) {
			return invalidError("content")
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
