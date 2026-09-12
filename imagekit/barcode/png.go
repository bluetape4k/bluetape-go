package barcode

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"

	"github.com/bluetape4k/bluetape-go/imagekit"
)

const maxPNGBytes = 4 << 20

var errPNGTooLarge = errors.New("barcode: png output too large")

type cappedWriter struct {
	ctx   context.Context
	limit int
	buf   bytes.Buffer
}

// EncodePNG 함수는 barcode를 렌더링해 bounded 표준 PNG 바이트로 encoding한다.
//
// 반환한 byte slice의 소유권은 caller에게 있다. provider, writer 또는 output-size
// 실패는 nil과 고정 ErrEncode 오류를 반환하며 cancellation과 deadline 오류는
// 변경하지 않고 반환한다.
func EncodePNG(ctx context.Context, req Request) ([]byte, error) {
	return encodePNGWithRenderer(ctx, req, Render, maxPNGBytes)
}

func encodePNGWithRenderer(ctx context.Context, req Request, renderer func(context.Context, Request) (image.Image, error), limit int) ([]byte, error) {
	if err := validateRequest(ctx, req); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if renderer == nil || limit <= 0 {
		return nil, newPNGEncodeError()
	}

	img, err := renderer(ctx, req)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}
	if err != nil {
		return nil, err
	}
	if img == nil {
		return nil, newPNGEncodeError()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	writer := &cappedWriter{ctx: ctx, limit: limit}
	if err := png.Encode(writer, img); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, newPNGEncodeError()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return append([]byte(nil), writer.buf.Bytes()...), nil
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	if len(p) > w.limit-w.buf.Len() {
		return 0, errPNGTooLarge
	}
	n, err := w.buf.Write(p)
	if err != nil {
		return n, err
	}
	if ctxErr := w.ctx.Err(); ctxErr != nil {
		return n, ctxErr
	}
	return n, nil
}

func newPNGEncodeError() error {
	return &imagekit.Error{Kind: ErrEncode, Operation: "encode", Format: "png", Cause: nil}
}
