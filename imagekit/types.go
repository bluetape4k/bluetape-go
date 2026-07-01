package imagekit

const (
	defaultMaxInputBytes = int64(10 << 20)
	defaultMaxPixels     = 16_000_000
	defaultMaxWidth      = 8192
	defaultMaxHeight     = 8192
	defaultJPEGQuality   = 85
)

// InputFormat reports the decoded input format.
type InputFormat string

const (
	// InputJPEG reports a JPEG input image.
	InputJPEG InputFormat = "jpeg"
	// InputPNG reports a PNG input image.
	InputPNG InputFormat = "png"
	// InputGIF reports a GIF input image.
	InputGIF InputFormat = "gif"
)

// OutputFormat selects the encoded output format.
type OutputFormat string

const (
	// OutputJPEG encodes the result as JPEG.
	OutputJPEG OutputFormat = "jpeg"
	// OutputPNG encodes the result as PNG.
	OutputPNG OutputFormat = "png"
)

// Mode controls how the source image is mapped into the requested size.
type Mode int

const (
	// ModeFit preserves aspect ratio and fits inside the requested box.
	ModeFit Mode = iota
	// ModeFill preserves aspect ratio, center-crops, and fills the requested box.
	ModeFill
	// ModeExact resizes to the exact requested size and may distort the image.
	ModeExact
)

// ResampleFilter selects the resize algorithm.
type ResampleFilter int

const (
	// FilterCubic uses Catmull-Rom resampling.
	FilterCubic ResampleFilter = iota
	// FilterLinear uses approximate bilinear resampling.
	FilterLinear
	// FilterNearest uses nearest-neighbor resampling.
	FilterNearest
)

// Request describes one bounded image transform.
//
// Zero limit fields use conservative defaults. JPEGQuality defaults to 85.
type Request struct {
	Width           int
	Height          int
	Mode            Mode
	OutputFormat    OutputFormat
	ResampleFilter  ResampleFilter
	JPEGQuality     int
	MaxInputBytes   int64
	MaxPixels       int
	MaxWidth        int
	MaxHeight       int
	MaxOutputPixels int
	MaxOutputWidth  int
	MaxOutputHeight int
}

// Result reports transform metadata. Transform populates Bytes; TransformTo
// writes to the caller-owned writer and leaves Bytes nil.
type Result struct {
	InputFormat  InputFormat
	OutputFormat OutputFormat
	InputWidth   int
	InputHeight  int
	OutputWidth  int
	OutputHeight int
	Bytes        []byte
}
