package imagekitgovips

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"runtime/debug"
	"sync"

	"github.com/bluetape4k/bluetape-go/imagekit"
	"github.com/davidbyttow/govips/v2/vips"
)

const (
	defaultMaxInputBytes = int64(10 << 20)
	defaultJPEGQuality   = 85
)

var (
	startOnce  sync.Once
	startError error
)

// Runtime textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
// 세부 조건은 language script, tokenizer, normalization, example ownership 계약을 따른다.
type Runtime struct {
	LibvipsVersion string
	GovipsVersion  string
	SupportsJPEG   bool
	SupportsPNG    bool
	SupportsGIF    bool
}

// Startup textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
// 세부 조건은 language script, tokenizer, normalization, example ownership 계약을 따른다.
// 세부 조건은 language script, tokenizer, normalization, example ownership 계약을 따른다.
func Startup() error {
	startOnce.Do(func() {
		vips.LoggingSettings(func(string, vips.LogLevel, string) {}, vips.LogLevelError)
		startError = vips.Startup(&vips.Config{
			ConcurrencyLevel: 1,
			MaxCacheFiles:    64,
			MaxCacheMem:      50 << 20,
			MaxCacheSize:     200,
		})
	})
	if startError != nil {
		return fmt.Errorf("imagekit-govips: start libvips: %w", startError)
	}
	return nil
}

// RuntimeInfo textsearch language image example에서 반환값과 오류 의미를 설명한다.
func RuntimeInfo() Runtime {
	return Runtime{
		LibvipsVersion: vips.Version,
		GovipsVersion:  govipsVersion(),
		SupportsJPEG:   vips.IsTypeSupported(vips.ImageTypeJPEG),
		SupportsPNG:    vips.IsTypeSupported(vips.ImageTypePNG),
		SupportsGIF:    vips.IsTypeSupported(vips.ImageTypeGIF),
	}
}

// Transform textsearch language image example에서 반환값과 오류 의미를 설명한다.
// 세부 조건은 language script, tokenizer, normalization, example ownership 계약을 따른다.
func Transform(ctx context.Context, reader io.Reader, request imagekit.Request) (imagekit.Result, error) {
	var output bytes.Buffer
	result, err := TransformTo(ctx, &output, reader, request)
	if err != nil {
		return imagekit.Result{}, err
	}
	result.Bytes = output.Bytes()
	return result, nil
}

// TransformTo textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
// 세부 조건은 language script, tokenizer, normalization, example ownership 계약을 따른다.
// 세부 조건은 language script, tokenizer, normalization, example ownership 계약을 따른다.
func TransformTo(ctx context.Context, writer io.Writer, reader io.Reader, request imagekit.Request) (imagekit.Result, error) {
	ctx = normalizeContext(ctx)
	req, err := normalizeRequest(request)
	if err != nil {
		return imagekit.Result{}, err
	}
	if writer == nil {
		return imagekit.Result{}, wrapImagekit(imagekit.ErrInvalidOptions, "writer", nil)
	}
	if reader == nil {
		return imagekit.Result{}, wrapImagekit(imagekit.ErrInvalidOptions, "reader", nil)
	}
	if err := ctx.Err(); err != nil {
		return imagekit.Result{}, wrapImagekit(err, "read", err)
	}

	payload, err := readBounded(reader, req.MaxInputBytes)
	if err != nil {
		return imagekit.Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return imagekit.Result{}, wrapImagekit(err, "read", err)
	}

	cfg, inputFormat, err := decodeSupportedConfig(payload)
	if err != nil {
		return imagekit.Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return imagekit.Result{}, wrapImagekit(err, "native", err)
	}
	if err := Startup(); err != nil {
		return imagekit.Result{}, err
	}

	img, err := thumbnail(payload, req)
	if err != nil {
		return imagekit.Result{}, mapNativeError("thumbnail", err)
	}
	defer img.Close()

	encoded, err := exportImage(img, req)
	if err != nil {
		return imagekit.Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return imagekit.Result{}, wrapImagekit(err, "write", err)
	}
	if _, err := writer.Write(encoded); err != nil {
		return imagekit.Result{}, wrapImagekit(imagekit.ErrEncode, "write", err)
	}

	return imagekit.Result{
		InputFormat:  inputFormat,
		OutputFormat: req.OutputFormat,
		InputWidth:   cfg.Width,
		InputHeight:  cfg.Height,
		OutputWidth:  img.Width(),
		OutputHeight: img.Height(),
	}, nil
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func normalizeRequest(req imagekit.Request) (imagekit.Request, error) {
	if req.Width <= 0 || req.Height <= 0 {
		return imagekit.Request{}, wrapImagekit(imagekit.ErrInvalidOptions, "size", nil)
	}
	if req.Mode < imagekit.ModeFit || req.Mode > imagekit.ModeExact {
		return imagekit.Request{}, wrapImagekit(imagekit.ErrInvalidOptions, "mode", nil)
	}
	if req.OutputFormat == "" {
		req.OutputFormat = imagekit.OutputJPEG
	}
	if req.OutputFormat != imagekit.OutputJPEG && req.OutputFormat != imagekit.OutputPNG {
		return imagekit.Request{}, wrapImagekit(imagekit.ErrUnsupportedFormat, "output_format", nil)
	}
	if req.JPEGQuality == 0 {
		req.JPEGQuality = defaultJPEGQuality
	}
	if req.JPEGQuality < 1 || req.JPEGQuality > 100 {
		return imagekit.Request{}, wrapImagekit(imagekit.ErrInvalidOptions, "jpeg_quality", nil)
	}
	if req.MaxInputBytes < 0 || req.MaxInputBytes > math.MaxInt64-1 {
		return imagekit.Request{}, wrapImagekit(imagekit.ErrInvalidOptions, "max_input_bytes", nil)
	}
	if req.MaxInputBytes == 0 {
		req.MaxInputBytes = defaultMaxInputBytes
	}
	return req, nil
}

func readBounded(reader io.Reader, maxInputBytes int64) ([]byte, error) {
	payload, err := io.ReadAll(io.LimitReader(reader, maxInputBytes+1))
	if err != nil {
		return nil, wrapImagekit(imagekit.ErrDecode, "read", err)
	}
	if int64(len(payload)) > maxInputBytes {
		return nil, wrapImagekit(imagekit.ErrInputTooLarge, "read", nil)
	}
	return payload, nil
}

func decodeSupportedConfig(payload []byte) (image.Config, imagekit.InputFormat, error) {
	imageType := vips.DetermineImageType(payload)
	var inputFormat imagekit.InputFormat
	switch imageType {
	case vips.ImageTypeJPEG:
		inputFormat = imagekit.InputJPEG
	case vips.ImageTypePNG:
		inputFormat = imagekit.InputPNG
	case vips.ImageTypeGIF:
		inputFormat = imagekit.InputGIF
	default:
		return image.Config{}, "", wrapImagekit(imagekit.ErrUnsupportedFormat, "decode_config", nil)
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(payload))
	if err != nil {
		return image.Config{}, "", wrapImagekit(imagekit.ErrDecode, "decode_config", err)
	}
	return cfg, inputFormat, nil
}

func thumbnail(payload []byte, req imagekit.Request) (*vips.ImageRef, error) {
	crop := vips.InterestingNone
	size := vips.SizeBoth
	if req.Mode == imagekit.ModeFill {
		crop = vips.InterestingCentre
	}
	if req.Mode == imagekit.ModeExact {
		size = vips.SizeForce
	}
	return vips.LoadThumbnailFromBuffer(payload, req.Width, req.Height, crop, size, nil)
}

func exportImage(img *vips.ImageRef, req imagekit.Request) ([]byte, error) {
	switch req.OutputFormat {
	case imagekit.OutputJPEG:
		out, _, err := img.ExportJpeg(&vips.JpegExportParams{Quality: req.JPEGQuality})
		if err != nil {
			return nil, wrapImagekit(imagekit.ErrEncode, "jpeg", err)
		}
		return out, nil
	case imagekit.OutputPNG:
		out, _, err := img.ExportPng(vips.NewPngExportParams())
		if err != nil {
			return nil, wrapImagekit(imagekit.ErrEncode, "png", err)
		}
		return out, nil
	default:
		return nil, wrapImagekit(imagekit.ErrUnsupportedFormat, "encode", nil)
	}
}

func mapNativeError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return wrapImagekit(imagekit.ErrDecode, operation, err)
}

func wrapImagekit(kind error, operation string, cause error) error {
	if cause == nil {
		return fmt.Errorf("%w: %s", kind, operation)
	}
	return fmt.Errorf("%w: %s: %w", kind, operation, cause)
}

func govipsVersion() string {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, dep := range buildInfo.Deps {
		if dep.Path == "github.com/davidbyttow/govips/v2" {
			return dep.Version
		}
	}
	return "unknown"
}
