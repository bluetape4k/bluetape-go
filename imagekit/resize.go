package imagekit

import (
	"image"
	"math"

	xdraw "golang.org/x/image/draw"
)

func resizeImage(src image.Image, req Request) (image.Image, error) {
	srcBounds := src.Bounds()
	if srcBounds.Empty() {
		return nil, errorWith(ErrImageTooLarge, "resize", "", nil)
	}
	dstWidth, dstHeight := targetSize(srcBounds.Dx(), srcBounds.Dy(), req)
	if err := validateOutputBounds(dstWidth, dstHeight, req); err != nil {
		return nil, err
	}

	dst := image.NewRGBA(image.Rect(0, 0, dstWidth, dstHeight))
	srcRect := srcBounds
	if req.Mode == ModeFill {
		srcRect = fillCropRect(srcBounds, req.Width, req.Height)
	}
	scaler(req.ResampleFilter).Scale(dst, dst.Bounds(), src, srcRect, xdraw.Over, nil)
	return dst, nil
}

func targetSize(srcWidth int, srcHeight int, req Request) (int, int) {
	switch req.Mode {
	case ModeExact, ModeFill:
		return req.Width, req.Height
	default:
		scale := math.Min(float64(req.Width)/float64(srcWidth), float64(req.Height)/float64(srcHeight))
		width := int(math.Round(float64(srcWidth) * scale))
		height := int(math.Round(float64(srcHeight) * scale))
		if width < 1 {
			width = 1
		}
		if height < 1 {
			height = 1
		}
		return width, height
	}
}

func fillCropRect(bounds image.Rectangle, targetWidth int, targetHeight int) image.Rectangle {
	srcWidth := bounds.Dx()
	srcHeight := bounds.Dy()
	srcRatio := float64(srcWidth) / float64(srcHeight)
	targetRatio := float64(targetWidth) / float64(targetHeight)
	if srcRatio > targetRatio {
		cropWidth := clampCropDimension(int(math.Round(float64(srcHeight)*targetRatio)), srcWidth)
		left := bounds.Min.X + (srcWidth-cropWidth)/2
		return image.Rect(left, bounds.Min.Y, left+cropWidth, bounds.Max.Y)
	}

	cropHeight := clampCropDimension(int(math.Round(float64(srcWidth)/targetRatio)), srcHeight)
	top := bounds.Min.Y + (srcHeight-cropHeight)/2
	return image.Rect(bounds.Min.X, top, bounds.Max.X, top+cropHeight)
}

func clampCropDimension(value int, maxValue int) int {
	if value < 1 {
		return 1
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func scaler(filter ResampleFilter) xdraw.Scaler {
	switch filter {
	case FilterNearest:
		return xdraw.NearestNeighbor
	case FilterLinear:
		return xdraw.ApproxBiLinear
	default:
		return xdraw.CatmullRom
	}
}
