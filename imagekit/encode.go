package imagekit

import (
	"image"
	"image/jpeg"
	"image/png"
	"io"
)

func encodeImage(writer io.Writer, img image.Image, req Request) error {
	if writer == nil {
		return errorWith(ErrInvalidOptions, "writer", "", nil)
	}
	switch req.OutputFormat {
	case OutputJPEG:
		if err := jpeg.Encode(writer, img, &jpeg.Options{Quality: req.JPEGQuality}); err != nil {
			return errorWith(ErrEncode, "encode", string(req.OutputFormat), err)
		}
	case OutputPNG:
		if err := png.Encode(writer, img); err != nil {
			return errorWith(ErrEncode, "encode", string(req.OutputFormat), err)
		}
	default:
		return errorWith(ErrUnsupportedFormat, "encode", string(req.OutputFormat), nil)
	}
	return nil
}
