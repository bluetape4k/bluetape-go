package imagekit

import (
	"bytes"
	"context"
	"errors"
	"image"
	_ "image/gif"  // register GIF input decoding for the explicit allowlist
	_ "image/jpeg" // register JPEG input decoding for the explicit allowlist
	_ "image/png"  // register PNG input decoding for the explicit allowlist
	"io"
	"math"
)

// Transform reads a bounded image, resizes it, and returns encoded bytes plus
// metadata.
func Transform(ctx context.Context, reader io.Reader, request Request) (Result, error) {
	var output bytes.Buffer
	result, err := TransformTo(ctx, &output, reader, request)
	if err != nil {
		return Result{}, err
	}
	result.Bytes = output.Bytes()
	return result, nil
}

// TransformTo reads a bounded image, resizes it, and encodes directly to writer.
// The write is not atomic: a codec or writer failure can leave partial bytes in
// the writer. Use Transform or write to a caller-owned temporary buffer/object
// before publishing when final response or storage writes must be all-or-nothing.
func TransformTo(ctx context.Context, writer io.Writer, reader io.Reader, request Request) (Result, error) {
	ctx = normalizeContext(ctx)
	req, err := normalizeRequest(request)
	if err != nil {
		return Result{}, err
	}
	if writer == nil {
		return Result{}, errorWith(ErrInvalidOptions, "writer", "", nil)
	}
	if reader == nil {
		return Result{}, errorWith(ErrInvalidOptions, "reader", "", nil)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, errorWith(err, "read", "", err)
	}

	payload, err := readBounded(reader, req.MaxInputBytes)
	if err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, errorWith(err, "read", "", err)
	}

	cfg, inputFormat, err := decodeConfig(payload)
	if err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, errorWith(err, "decode", string(inputFormat), err)
	}
	if err := validateInputBounds(cfg, req); err != nil {
		return Result{}, err
	}

	src, _, err := image.Decode(bytes.NewReader(payload))
	if err != nil {
		return Result{}, errorWith(ErrDecode, "decode", string(inputFormat), err)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, errorWith(err, "resize", string(inputFormat), err)
	}

	dst, err := resizeImage(src, req)
	if err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, errorWith(err, "encode", string(req.OutputFormat), err)
	}
	if err := encodeImage(writer, dst, req); err != nil {
		return Result{}, err
	}

	bounds := dst.Bounds()
	return Result{
		InputFormat:  inputFormat,
		OutputFormat: req.OutputFormat,
		InputWidth:   cfg.Width,
		InputHeight:  cfg.Height,
		OutputWidth:  bounds.Dx(),
		OutputHeight: bounds.Dy(),
	}, nil
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func normalizeRequest(req Request) (Request, error) {
	if req.Width <= 0 || req.Height <= 0 {
		return Request{}, errorWith(ErrInvalidOptions, "size", "", nil)
	}
	if req.Mode < ModeFit || req.Mode > ModeExact {
		return Request{}, errorWith(ErrInvalidOptions, "mode", "", nil)
	}
	if req.OutputFormat == "" {
		req.OutputFormat = OutputJPEG
	}
	if req.OutputFormat != OutputJPEG && req.OutputFormat != OutputPNG {
		return Request{}, errorWith(ErrUnsupportedFormat, "output_format", string(req.OutputFormat), nil)
	}
	if req.ResampleFilter < FilterCubic || req.ResampleFilter > FilterNearest {
		return Request{}, errorWith(ErrInvalidOptions, "resample_filter", "", nil)
	}
	if req.JPEGQuality == 0 {
		req.JPEGQuality = defaultJPEGQuality
	}
	if req.JPEGQuality < 1 || req.JPEGQuality > 100 {
		return Request{}, errorWith(ErrInvalidOptions, "jpeg_quality", "", nil)
	}

	var err error
	if req.MaxInputBytes, err = normalizeInt64Limit(req.MaxInputBytes, defaultMaxInputBytes, "max_input_bytes"); err != nil {
		return Request{}, err
	}
	if req.MaxPixels, err = normalizeIntLimit(req.MaxPixels, defaultMaxPixels, "max_pixels"); err != nil {
		return Request{}, err
	}
	if req.MaxWidth, err = normalizeIntLimit(req.MaxWidth, defaultMaxWidth, "max_width"); err != nil {
		return Request{}, err
	}
	if req.MaxHeight, err = normalizeIntLimit(req.MaxHeight, defaultMaxHeight, "max_height"); err != nil {
		return Request{}, err
	}
	if req.MaxOutputPixels, err = normalizeIntLimit(req.MaxOutputPixels, defaultMaxPixels, "max_output_pixels"); err != nil {
		return Request{}, err
	}
	if req.MaxOutputWidth, err = normalizeIntLimit(req.MaxOutputWidth, defaultMaxWidth, "max_output_width"); err != nil {
		return Request{}, err
	}
	if req.MaxOutputHeight, err = normalizeIntLimit(req.MaxOutputHeight, defaultMaxHeight, "max_output_height"); err != nil {
		return Request{}, err
	}
	if req.MaxInputBytes > math.MaxInt64-1 {
		return Request{}, errorWith(ErrInvalidOptions, "max_input_bytes", "", nil)
	}
	if err := validateOutputBounds(req.Width, req.Height, req); err != nil {
		return Request{}, err
	}
	return req, nil
}

func normalizeInt64Limit(value int64, defaultValue int64, operation string) (int64, error) {
	if value < 0 {
		return 0, errorWith(ErrInvalidOptions, operation, "", nil)
	}
	if value == 0 {
		return defaultValue, nil
	}
	return value, nil
}

func normalizeIntLimit(value int, defaultValue int, operation string) (int, error) {
	if value < 0 {
		return 0, errorWith(ErrInvalidOptions, operation, "", nil)
	}
	if value == 0 {
		return defaultValue, nil
	}
	return value, nil
}

func readBounded(reader io.Reader, maxInputBytes int64) ([]byte, error) {
	payload, err := io.ReadAll(io.LimitReader(reader, maxInputBytes+1))
	if err != nil {
		return nil, errorWith(ErrDecode, "read", "", err)
	}
	if int64(len(payload)) > maxInputBytes {
		return nil, errorWith(ErrInputTooLarge, "read", "", nil)
	}
	return payload, nil
}

func decodeConfig(payload []byte) (image.Config, InputFormat, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(payload))
	if err != nil {
		if errors.Is(err, image.ErrFormat) {
			return image.Config{}, "", errorWith(ErrUnsupportedFormat, "decode_config", "", err)
		}
		return image.Config{}, "", errorWith(ErrDecode, "decode_config", "", err)
	}
	if format != string(InputJPEG) && format != string(InputPNG) && format != string(InputGIF) {
		return image.Config{}, "", errorWith(ErrUnsupportedFormat, "decode_config", format, nil)
	}
	return cfg, InputFormat(format), nil
}

func validateInputBounds(cfg image.Config, req Request) error {
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return errorWith(ErrImageTooLarge, "input_dimensions", "", nil)
	}
	if cfg.Width > req.MaxWidth || cfg.Height > req.MaxHeight {
		return errorWith(ErrImageTooLarge, "input_dimensions", "", nil)
	}
	if exceedsPixels(cfg.Width, cfg.Height, req.MaxPixels) {
		return errorWith(ErrImageTooLarge, "input_pixels", "", nil)
	}
	return nil
}

func validateOutputBounds(width int, height int, req Request) error {
	if width > req.MaxOutputWidth || height > req.MaxOutputHeight {
		return errorWith(ErrImageTooLarge, "output_dimensions", "", nil)
	}
	if exceedsPixels(width, height, req.MaxOutputPixels) {
		return errorWith(ErrImageTooLarge, "output_pixels", "", nil)
	}
	return nil
}

func exceedsPixels(width int, height int, maxPixels int) bool {
	if width <= 0 || height <= 0 || maxPixels <= 0 {
		return true
	}
	return width > maxPixels/height
}
