# Issue 309 ImageKit Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a bounded pure-Go `imagekit` package for thumbnail, resize, and JPEG/PNG conversion with fixture tests, examples, benchmarks, and synced documentation.

**Architecture:** The package reads bounded input bytes, checks context before and after each coarse phase, validates decoded and requested output dimensions before full decode, resizes with `golang.org/x/image/draw`, and encodes only JPEG/PNG output. Public API is one `Request` struct plus `Transform` and `TransformTo`, with typed sentinel errors and metadata in `Result`.

**Tech Stack:** Go standard library image codecs, `golang.org/x/image/draw`, `context.Context`, table-driven tests, Go examples, package/root READMEs, `make ci`.

---

## File Map

- Create `imagekit/doc.go`: package overview, supported formats, cancellation caveat, service guidance.
- Create `imagekit/types.go`: `InputFormat`, `OutputFormat`, `Mode`, `ResampleFilter`, `Request`, `Result`, defaults.
- Create `imagekit/errors.go`: sentinel errors, typed `Error`, `Is`, `Unwrap`, safe messages.
- Create `imagekit/transform.go`: codec registration, bounded read, config decode, allowlist, bounds validation, resize, encode orchestration.
- Create `imagekit/resize.go`: fit/fill/exact dimension math, center crop, draw scaler selection.
- Create `imagekit/encode.go`: JPEG/PNG encode behavior and JPEG quality validation.
- Create `imagekit/imagekit_test.go`: success, limits, malformed input, cancellation, error contracts.
- Create `imagekit/example_test.go`: compile-checked fit/fill/exact and `TransformTo` examples.
- Create `imagekit/benchmark_test.go`: small/medium benchmarks with `-benchmem`.
- Create `imagekit/README.md` and `imagekit/README.ko.md`: synced public docs.
- Modify `go.mod` and `go.sum`: add `golang.org/x/image`.
- Modify `README.md` and `README.ko.md`: package table and package documentation grouping.
- Create `docs/benchmarks/2026-07-01-issue-309-imagekit.md`: raw benchmark command output and environment notes after implementation.

## Task 1: Dependency And Public Types

**Files:**
- Modify: `go.mod`
- Create: `imagekit/doc.go`
- Create: `imagekit/types.go`

- [ ] **Step 1: Add the focused resize dependency**

Run:

```bash
go get golang.org/x/image@v0.43.0
```

Expected: `go.mod` gains `golang.org/x/image` and `go.sum` gains module hashes.

- [ ] **Step 2: Create package docs**

Create `imagekit/doc.go` with this package contract:

```go
// Package imagekit provides bounded pure-Go helpers for resizing one image and
// encoding it as JPEG or PNG.
//
// Supported input formats are JPEG, PNG, and GIF as reported by the standard
// library image decoders. Supported output formats are JPEG and PNG.
//
// The package checks context cancellation before the bounded read, after the
// bounded read, before decode, before resize, and before encode. It cannot
// preempt a blocked caller-owned io.Reader or io.Writer, nor a standard-library
// codec call that is already executing; callers that need hard I/O deadlines
// should enforce them at the I/O boundary.
//
// Transform returns encoded bytes for convenience. TransformTo is preferred for
// service handlers that already own an output writer and want to avoid one extra
// encoded byte slice.
package imagekit
```

- [ ] **Step 3: Create the public type surface**

Create `imagekit/types.go` with these exported names and zero-value behavior:

```go
package imagekit

const (
	defaultMaxInputBytes  = int64(10 << 20)
	defaultMaxPixels      = 16_000_000
	defaultMaxWidth       = 8192
	defaultMaxHeight      = 8192
	defaultJPEGQuality    = 85
)

type InputFormat string

const (
	InputJPEG InputFormat = "jpeg"
	InputPNG  InputFormat = "png"
	InputGIF  InputFormat = "gif"
)

type OutputFormat string

const (
	OutputJPEG OutputFormat = "jpeg"
	OutputPNG  OutputFormat = "png"
)

type Mode int

const (
	ModeFit Mode = iota
	ModeFill
	ModeExact
)

type ResampleFilter int

const (
	FilterCubic ResampleFilter = iota
	FilterLinear
	FilterNearest
)

type Request struct {
	Width          int
	Height         int
	Mode           Mode
	OutputFormat   OutputFormat
	ResampleFilter ResampleFilter
	JPEGQuality    int
	MaxInputBytes  int64
	MaxPixels      int
	MaxWidth       int
	MaxHeight      int
	MaxOutputPixels  int
	MaxOutputWidth   int
	MaxOutputHeight  int
}

type Result struct {
	InputFormat  InputFormat
	OutputFormat OutputFormat
	InputWidth   int
	InputHeight  int
	OutputWidth  int
	OutputHeight int
	Bytes        []byte
}

func Transform(ctx context.Context, r io.Reader, req Request) (Result, error)
func TransformTo(ctx context.Context, w io.Writer, r io.Reader, req Request) (Result, error)
```

`Transform` must populate `Result.Bytes` with encoded bytes. `TransformTo`
keeps writer ownership by encoding directly to `w` and returning metadata with
`Result.Bytes == nil`. The writer argument stays before the reader in
`TransformTo` to mirror `io.Copy(dst, src)` style APIs.

- [ ] **Step 4: Run formatting for created package files**

Run:

```bash
gofmt -w imagekit/doc.go imagekit/types.go
go test -count=1 ./imagekit
```

Expected: package compiles or fails only because implementation functions are not created yet.

## Task 2: Errors And Validation Tests

**Files:**
- Create: `imagekit/errors.go`
- Create: `imagekit/imagekit_test.go`

- [ ] **Step 1: Write failing tests for error identity and invalid requests**

Add tests named:

```go
func TestTransformRejectsInvalidRequest(t *testing.T)
func TestTransformPreservesContextErrors(t *testing.T)
func TestTransformToReturnsMetadataWithoutBytes(t *testing.T)
func TestErrorContracts(t *testing.T)
func TestErrorMessagesDoNotLeakPayloadsOrPaths(t *testing.T)
```

Assertions:

```go
if !errors.Is(err, ErrInvalidOptions) { t.Fatalf("expected ErrInvalidOptions, got %v", err) }
if !errors.Is(err, context.Canceled) { t.Fatalf("expected context.Canceled, got %v", err) }
var kitErr *Error
if !errors.As(err, &kitErr) { t.Fatalf("expected *Error, got %T", err) }
```

The no-leak test must use a sentinel raw payload and path-like reader/writer
errors. `Error.Error()` must omit raw payload and `Cause` text while `Unwrap`
still preserves the cause for `errors.Is` / `errors.As`.

- [ ] **Step 2: Implement typed errors**

Create `imagekit/errors.go` with sentinel errors:

```go
var (
	ErrInvalidOptions    = errors.New("imagekit: invalid options")
	ErrUnsupportedFormat = errors.New("imagekit: unsupported format")
	ErrInputTooLarge     = errors.New("imagekit: input too large")
	ErrImageTooLarge     = errors.New("imagekit: image too large")
	ErrDecode            = errors.New("imagekit: decode failed")
	ErrEncode            = errors.New("imagekit: encode failed")
)
```

Add `Error` with `Kind`, `Operation`, `Format`, `Cause`, `Error()`, `Unwrap()`, and `Is(target error) bool` so callers can use `errors.Is` for sentinel and context errors.

- [ ] **Step 3: Run targeted tests**

Run:

```bash
go test -count=1 ./imagekit
```

Expected: validation tests pass after Task 3 implementation; before Task 3, failures must reference missing `Transform`.

## Task 3: Transform Pipeline

**Files:**
- Create: `imagekit/transform.go`
- Create: `imagekit/resize.go`
- Create: `imagekit/encode.go`
- Modify: `imagekit/imagekit_test.go`

- [ ] **Step 1: Add fixture builders in tests**

In `imagekit/imagekit_test.go`, create helper functions that generate deterministic JPEG, PNG, and GIF bytes from `image.NewRGBA(image.Rect(0, 0, w, h))`. Use colored pixels so crop/resize paths decode real images without committed binary files.

- [ ] **Step 2: Write failing transform behavior tests**

Add table-driven tests named:

```go
func TestTransformFitFillExactDimensions(t *testing.T)
func TestTransformSupportsJPEGPNGAndGIFInput(t *testing.T)
func TestTransformRejectsMalformedInput(t *testing.T)
func TestTransformRejectsNilReaderAndWriter(t *testing.T)
func TestTransformWrapsReaderAndWriterFailures(t *testing.T)
func TestTransformRejectsUnsupportedOutputFormat(t *testing.T)
func TestTransformEnforcesInputAndOutputLimits(t *testing.T)
func TestTransformRejectsNegativeLimits(t *testing.T)
func TestTransformRejectsOverflowingLimits(t *testing.T)
func TestTransformFillExtremeAspectDoesNotAllocateCoverImage(t *testing.T)
func TestTransformPreservesCancellationAtPhaseBoundaries(t *testing.T)
func TestTransformPreservesDeadlineExceededAtPhaseBoundaries(t *testing.T)
func TestTransformDoesNotWriteAfterPreEncodeCancellation(t *testing.T)
func TestTransformRejectsUnallowlistedRegisteredFormat(t *testing.T)
```

Expected output dimensions:

```go
// 400x200 into 100x100
ModeFit   => 100x50
ModeFill  => 100x100
ModeExact => 100x100
```

Use deterministic quadrant-colored fixtures and assert decoded output pixels for
`ModeFill` center-crop behavior, not only dimensions. A top-left crop or
distorted fill must fail the test.

- [ ] **Step 3: Implement request defaulting and bounds**

Implement internal `normalizeRequest(Request) (Request, error)`:

- zero `OutputFormat` defaults to `OutputJPEG`
- zero `Mode` defaults to `ModeFit`
- zero `ResampleFilter` defaults to `FilterCubic`
- `JPEGQuality == 0` defaults to `85`
- nonzero `JPEGQuality` outside `1..100` returns `ErrInvalidOptions`
- zero limit fields default to the conservative constants
- `Width <= 0`, `Height <= 0`, negative limit fields, invalid JPEG quality, invalid format, invalid mode, or invalid filter returns `ErrInvalidOptions`
- `MaxInputBytes == math.MaxInt64` is rejected before adding `+1`
- width/height and pixel multiplication uses checked integer math and rejects overflow as `ErrInvalidOptions` or `ErrImageTooLarge` before allocation
- pixel checks happen before `image.Rect`, destination allocation, decode pixel acceptance, and resize math
- requested output width/height and output pixels are checked before allocation
Implement a checked pixel helper using `int64` division or widened arithmetic;
do not multiply untrusted `int` dimensions before proving the product fits.

- [ ] **Step 4: Implement bounded decode and allowlist**

Implement `readBounded`, `decodeConfig`, and allowlist checks. `transform.go`
must import `_ "image/gif"` so production builds support GIF input even when
callers do not import the GIF codec themselves.

```go
limited := io.LimitReader(reader, req.MaxInputBytes+1)
payload, err := io.ReadAll(limited)
if int64(len(payload)) > req.MaxInputBytes { return errorWithKind(ErrInputTooLarge) }
cfg, format, err := image.DecodeConfig(bytes.NewReader(payload))
if format != "jpeg" && format != "png" && format != "gif" { return errorWithKind(ErrUnsupportedFormat) }
```

Rules:

- nil reader returns `ErrInvalidOptions`
- read failure wraps `ErrDecode` plus the original reader error
- check `ctx.Err()` before bounded read and after bounded read
- check `ctx.Err()` before `image.DecodeConfig`, before `image.Decode`, before resize, and before encode
- context failures return or wrap the exact `ctx.Err()` so `errors.Is(err, context.Canceled)` and `errors.Is(err, context.DeadlineExceeded)` pass
- input width, height, and pixels are checked before `image.Decode`
- all sentinel failures return `*Error` so `errors.As(err, *Error)` exposes `Kind`, `Operation`, `Format`, and `Cause`
- phase cancellation tests must cover already-canceled context, deadline context, cancellation after bounded read, deterministic pre-resize cancellation, and deterministic pre-encode cancellation; if a phase cannot be isolated without implementation hooks, record the exact reason in the test comment and still prove the adjacent checkpoint
- `TestTransformRejectsUnallowlistedRegisteredFormat` must use a package-unique fake format name and magic, must not call `t.Parallel`, and must avoid relying on order with JPEG/PNG/GIF tests

- [ ] **Step 5: Implement resize math and scaling**

Implement:

```go
func targetRect(src image.Rectangle, req Request) image.Rectangle
func resizeImage(src image.Image, req Request) image.Image
```

Use `draw.NearestNeighbor`, `draw.ApproxBiLinear`, and `draw.CatmullRom` from `golang.org/x/image/draw`.
For `ModeFill`, compute a centered source crop rectangle in source coordinates
and scale directly into the bounded final destination image. Do not allocate a
larger cover image and crop it afterward. Add the extreme-aspect test before
implementation and assert output dimensions plus bounded allocation behavior.

- [ ] **Step 6: Implement encoding**

Implement `encodeImage(w io.Writer, img image.Image, req Request) error`:

- `OutputJPEG` uses `jpeg.Encode` with normalized `JPEGQuality`
- `OutputPNG` uses `png.Encode`
- unsupported output returns `*Error` with `ErrUnsupportedFormat`
- nil writer returns `ErrInvalidOptions`
- writer failure wraps `ErrEncode` plus the original writer error

- [ ] **Step 7: Run targeted tests and race test**

Run:

```bash
go test -count=1 ./imagekit
go test -race -count=1 ./imagekit
```

Expected: both pass.

## Task 4: Examples, Benchmarks, And Benchmark Evidence

**Files:**
- Create: `imagekit/example_test.go`
- Create: `imagekit/benchmark_test.go`
- Create: `docs/benchmarks/2026-07-01-issue-309-imagekit.md`

- [ ] **Step 1: Add compile-checked examples**

Create examples:

```go
func ExampleTransform_fit()
func ExampleTransform_fill()
func ExampleTransform_exact()
func ExampleTransform_outputFormat()
func ExampleTransform_zeroValueDefaults()
func ExampleTransformTo()
```

Each example prints output dimensions and output format, not encoded bytes.

- [ ] **Step 2: Add benchmarks**

Create benchmark functions:

```go
func BenchmarkTransformSmallJPEGToJPEG(b *testing.B)
func BenchmarkTransformSmallPNGToPNG(b *testing.B)
func BenchmarkTransformMediumJPEGToPNG(b *testing.B)
func BenchmarkTransformToMediumJPEGToPNG(b *testing.B)
```

Use generated in-memory fixtures with fixed dimensions: small `320x180`,
medium `1600x900`. Generate inputs and validate one output before
`b.ResetTimer()`, call `b.ReportAllocs()`, call `b.SetBytes(int64(len(input)))`,
and keep timed loop work to the transform call. `TransformTo` should write to a
fresh reusable `bytes.Buffer` reset inside the loop.

- [ ] **Step 3: Run benchmark command and preserve output**

Run:

```bash
go test -run '^$' -bench '^BenchmarkTransform' -benchmem ./imagekit | tee /tmp/issue-309-imagekit-bench.txt
```

Create `docs/benchmarks/2026-07-01-issue-309-imagekit.md` with command, Go version, OS/arch, CPU line from output, and the raw benchmark rows copied from `/tmp/issue-309-imagekit-bench.txt`.

## Task 5: Documentation

**Files:**
- Create: `imagekit/README.md`
- Create: `imagekit/README.ko.md`
- Modify: `README.md`
- Modify: `README.ko.md`

- [ ] **Step 1: Write package READMEs**

Both package READMEs must include:

- import path `github.com/bluetape4k/bluetape-go/imagekit`
- supported input: JPEG, PNG, GIF
- supported output: JPEG, PNG
- default bounds
- `TransformTo` service guidance
- cancellation caveat for caller-owned readers/writers and stdlib codecs
- non-goals: libvips, OCR, CAPTCHA, SVG, AVIF, HEIC, TIFF, EXIF, image effects or filters beyond resize resampling, framework integration
- links to benchmark evidence and issue #310 handoff

- [ ] **Step 2: Update root README package tables**

Add `imagekit` row to both root package tables:

```markdown
| [`imagekit`](imagekit/README.md) | active | Bounded pure-Go thumbnail, resize, and JPEG/PNG conversion helpers for service inputs. |
```

Korean:

```markdown
| [`imagekit`](imagekit/README.ko.md) | active | 서비스 입력을 위한 bounded pure-Go thumbnail, resize, JPEG/PNG conversion helper. |
```

- [ ] **Step 3: Update root package documentation grouping**

Add an Image bullet to both root READMEs near portable utilities:

```markdown
- Image: [`imagekit`](imagekit/README.md) for bounded pure-Go resize, thumbnail, and JPEG/PNG conversion helpers with explicit format and memory boundaries.
```

Korean:

```markdown
- Image: 명시적 format과 memory boundary를 가진 bounded pure-Go resize, thumbnail, JPEG/PNG conversion helper인 [`imagekit`](imagekit/README.ko.md).
```

## Task 6: Final Verification And PR

**Files:**
- Modify only files changed by earlier tasks if verification exposes a defect.

- [ ] **Step 1: Run full validation**

Run:

```bash
go test -count=1 ./imagekit
go test -race -count=1 ./imagekit
go test -run '^$' -bench '^BenchmarkTransform' -benchmem ./imagekit
go mod verify
govulncheck ./imagekit
git diff --check
make ci
```

If `govulncheck` is not installed, run:

```bash
go run golang.org/x/vuln/cmd/govulncheck@latest ./imagekit
```

Expected: all commands pass.

- [ ] **Step 2: Commit with Lore protocol**

Run:

```bash
git status --short
git add go.mod go.sum imagekit README.md README.ko.md docs/benchmarks docs/superpowers/specs/2026-07-01-issue-309-imagekit-design.md docs/superpowers/plans/2026-07-01-issue-309-imagekit-implementation.md
git commit -m "Add bounded image helpers for service uploads" -m "Constraint: issue #309 requires pure-Go bounded thumbnail, resize, and conversion before libvips research.
Rejected: broad third-party image toolkit | would expand the API beyond the first service-safe package slice.
Confidence: high
Scope-risk: moderate
Directive: Keep imagekit bounded and pure-Go until #310 proves a native backend is warranted.
Tested: go test -count=1 ./imagekit; go test -race -count=1 ./imagekit; go test -run '^$' -bench '^BenchmarkTransform' -benchmem ./imagekit; go mod verify; govulncheck ./imagekit; git diff --check; make ci
Not-tested: native libvips comparison deferred to #310"
```

- [ ] **Step 3: Push and open PR**

Run:

```bash
git push -u origin feat/issue-309-image-helpers
gh pr create --base develop --head feat/issue-309-image-helpers --title "Implement bounded pure-Go image helpers" --body-file /tmp/issue-309-pr.md
```

PR body must include issue closure `Closes #309`, validation evidence, benchmark evidence path, and a final `## DoD Status` section.
