# Issue #309 ImageKit Design

## Context

Issue #309 is the first implementation task for the `0.11.0` image track. It
asks for a small pure-Go image helper package for bounded thumbnails, resize,
and stdlib-supported format conversion.

Current live issue evidence:

- #309 is a P1 implementation task in milestone `0.11.0`.
- The required behavior is bounded decode, deterministic fit/fill resizing,
  explicit output format and quality options, `context.Context` cancellation
  before expensive work, typed or documented errors, malformed input coverage,
  golden fixtures, and small/medium benchmarks.
- #310 depends on this work because libvips comparison requires benchmark
  evidence against the pure-Go baseline.

Current repository evidence:

- The repository has no image helper package yet.
- `docs/research/2026-06-25-issue-40-image-scope.md` ranks bounded
  thumbnail/resize/conversion as the first Go image package and keeps libvips,
  OCR, CAPTCHA, SVG rasterization, AVIF, HEIC, and TIFF out of the default
  package.
- `docs/lessons/2026-06-25-image-research-order.md` says image work should
  start with fixture-testable pure-Go behavior and benchmark evidence before
  any native dependency.
- Go's `image` package documentation warns that untrusted images should use
  `DecodeConfig` before `Decode` to avoid resource exhaustion.
- `go.mod` currently does not require `golang.org/x/image`; adding it must be
  justified by resizing quality and kept as the only new dependency.

## Problem

Callers need a predictable way to load a small bounded image, resize or
thumbnail it, and encode it to a supported output format without adopting a
native image stack or a broad JVM-shaped image toolkit. The dangerous failure
modes are unbounded memory growth during decode, confusing fit/fill semantics,
unsafe unsupported-format claims, cancellation that arrives too late to matter,
and benchmark claims without committed measurement code.

## Goals

- Add a new package `imagekit` with a small public API.
- Support only JPEG, PNG, and GIF input formats after an explicit decoded
  format allowlist check.
- Support JPEG and PNG encode paths, with GIF output explicitly unsupported in
  the first slice unless the implementation can prove correct palette behavior
  without broadening scope.
- Bound input bytes, decoded pixel count, requested output dimensions, and
  requested output pixel count before full decode or resize.
- Provide deterministic resize modes:
  - fit inside a box while preserving aspect ratio
  - fill a box while preserving aspect ratio and center-cropping
  - exact resize only when the caller explicitly chooses distortion
- Accept `context.Context` and check cancellation before bounded read, after
  bounded read, before decode, before resize, and before encode. The package
  must document that it cannot preempt a blocked caller-owned `io.Reader` or a
  caller-owned `io.Writer`, stdlib decoder, or stdlib encoder while that call
  is already executing.
- Return `errors.Is` / `errors.As` compatible errors for invalid options,
  unsupported formats, input-size limits, dimension limits, decode failures,
  encode failures, and cancellation. Canceled and deadline-exceeded operations
  must preserve `context.Canceled` or `context.DeadlineExceeded` through
  `errors.Is`.
- Commit golden fixture tests, malformed input tests, dimension/format tests,
  and benchmarks for small and medium images before performance claims.
- Add `README.md`, `README.ko.md`, and package docs that state supported
  formats and non-goals.

## Non-Goals

- No libvips, cgo, native codec detection, or operator install guidance in
  this package.
- No AVIF, HEIC, TIFF, SVG rasterization, OCR, CAPTCHA, EXIF, filters,
  histograms, similarity metrics, watermarks, or framework integration.
- No streaming transform API in the first slice. The API reads bounded bytes
  from an `io.Reader`, transforms one image, and writes or returns one encoded
  result.
- No throughput ranking against libvips. Benchmarks are baseline evidence for
  #310, not production performance claims.

## Design Options

### Option A: Standard library plus `golang.org/x/image/draw`

Create `imagekit` with stdlib decode/encode and `x/image/draw` scalers.

Pros:

- Keeps the default package pure Go and free of native runtime state.
- Uses Go's existing image model and a focused official extension package.
- Provides better resize algorithms than `image/draw` while avoiding a broad
  third-party image toolkit.
- Leaves libvips optional for #310.

Cons:

- Adds one dependency that is not currently in `go.mod`.
- Still requires careful bounds and fixture tests because Go image decoders can
  allocate large images after `DecodeConfig`.

### Option B: Standard library only

Use only `image`, `image/draw`, and stdlib encoders.

Pros:

- No dependency change.
- Very small dependency and security surface.

Cons:

- Resize quality and algorithm selection are weaker than `x/image/draw`.
- More likely to push callers toward unrelated third-party helpers later.
- Makes benchmark evidence less useful for #310 because the baseline would be
  intentionally low quality.

### Option C: Adopt a high-level package such as `disintegration/imaging`

Use an existing pure-Go image package and wrap it with bounded decode and
context behavior.

Pros:

- Richer operations and less first-party resizing code.
- Mature caller ergonomics for common image transformations.

Cons:

- Broadens scope beyond #309.
- Adds an indirect public dependency shape before this repo proves its own
  minimal contract.
- Risks duplicating third-party API decisions instead of creating a small
  bluetape-go surface.

## Decision

Choose Option A: create `imagekit` using the standard library plus
`golang.org/x/image/draw`.

The dependency is narrow and official enough for the first package. The public
API stays bluetape-go owned: callers do not see scaler internals unless they
choose a resample filter. If `x/image/draw` proves unnecessary or problematic
during implementation review, the plan can fall back to Option B before any
release.

## Proposed API Shape

Files:

- `imagekit/doc.go`
- `imagekit/errors.go`
- `imagekit/options.go`
- `imagekit/resize.go`
- `imagekit/encode.go`
- `imagekit/README.md`
- `imagekit/README.ko.md`

Core concepts:

- `InputFormat` enum or metadata value for detected `jpeg`, `png`, and `gif`
  input.
- `OutputFormat` enum: `OutputJPEG`, `OutputPNG`.
- `Mode` enum: `ModeFit`, `ModeFill`, `ModeExact`.
- `ResampleFilter` enum: `FilterNearest`, `FilterLinear`, `FilterCubic`.
- `Request` with target behavior, format, scaler, encoder, and safety limits:
  - `Width`
  - `Height`
  - `Mode`
  - `OutputFormat`
  - `ResampleFilter`
  - `JPEGQuality`
  - `MaxInputBytes`
  - `MaxPixels`
  - `MaxWidth`
  - `MaxHeight`
  - `MaxOutputPixels`
  - `MaxOutputWidth`
  - `MaxOutputHeight`
- `Result` with dimensions, input format, output format, and encoded bytes
  when using byte-returning helpers.
- `Transform(ctx, reader, request)` returning encoded bytes and metadata.
- `TransformTo(ctx, writer, reader, request)` for callers that want writer
  ownership.
- Convenience helpers `Thumbnail` and `Resize` may be added only if they do
  not duplicate `Transform` ambiguously.

Error contracts:

- `ErrInvalidOptions`
- `ErrUnsupportedFormat`
- `ErrInputTooLarge`
- `ErrImageTooLarge`
- `ErrDecode`
- `ErrEncode`
- `Error` struct with `Kind`, `Operation`, `Format`, and optional `Cause`.
  Error messages must not include raw image bytes or caller file paths.
- Context cancellation should return or wrap `ctx.Err()` directly enough that
  `errors.Is(err, context.Canceled)` and
  `errors.Is(err, context.DeadlineExceeded)` both work.

## Decode And Bounds Policy

The implementation must check `ctx` before starting the bounded read, read at
most `MaxInputBytes + 1` bytes from the reader, then check `ctx` again before
decode work. It then calls `image.DecodeConfig` on the bounded bytes, rejects
formats outside the explicit input allowlist (`jpeg`, `png`, `gif`), rejects
input dimensions above configured limits, rejects requested output dimensions
above configured output limits, and only then calls `image.Decode`.

The bounded read cannot preempt a blocked caller-owned reader, and encode cannot
preempt a blocked caller-owned writer. Callers that need hard I/O deadlines must
enforce them on the reader/writer or request context before calling the package.
Documentation must state this caveat instead of implying hard preemption inside
arbitrary `io.Reader`, `io.Writer`, or stdlib codec calls.

Default limits should be conservative enough for services:

- `MaxInputBytes`: 10 MiB
- `MaxPixels`: 16 megapixels
- `MaxWidth`: 8192
- `MaxHeight`: 8192
- `MaxOutputPixels`: 16 megapixels
- `MaxOutputWidth`: 8192
- `MaxOutputHeight`: 8192

The exact defaults may be adjusted in implementation if tests show they are
awkward. Go-idiomatic zero-value options must apply these conservative defaults;
explicitly invalid options must fail closed.

## Testing And Benchmarks

Tests must cover:

- JPEG and PNG decode/encode round trips.
- GIF input decode to JPEG or PNG output.
- unsupported output format.
- malformed input.
- empty/nil reader or writer behavior.
- max input byte limit.
- max pixel and dimension limit.
- requested output width/height and output pixel limit for fit, fill, and exact
  modes.
- registered-but-not-allowlisted input format rejection.
- zero or negative requested dimensions.
- fit, fill, and exact output dimensions within the output limit policy.
- cancellation before bounded read, after bounded read, before decode, before
  resize, and before encode where feasible.
- error wrapping through `errors.Is` and `errors.As`.
- `errors.Is` compatibility for `context.Canceled` and
  `context.DeadlineExceeded`.
- no raw byte/path leakage in errors.

Benchmarks:

- `BenchmarkTransformSmallJPEGToJPEG`
- `BenchmarkTransformSmallPNGToPNG`
- `BenchmarkTransformMediumJPEGToPNG`
- include `-benchmem` compatibility and no production ranking language.
- preserve representative raw benchmark output and environment notes under
  `docs/benchmarks/` so #310 can compare libvips against a reproducible
  pure-Go baseline.

Validation commands:

```bash
go test -count=1 ./imagekit
go test -race -count=1 ./imagekit
go test -run '^$' -bench '^BenchmarkTransform' -benchmem ./imagekit
git diff --check
make ci
```

## Documentation Impact

- Add `imagekit/README.md` and `imagekit/README.ko.md`.
- Update root `README.md` and `README.ko.md` consistently in both the package
  table and package documentation grouping.
- Add package docs for import path, supported formats, bounds, non-goals, and
  libvips handoff to #310.
- Add compile-checked README examples or mirrored package examples for fit,
  fill, exact, output format, and conservative default limits.
- Document byte-returning versus writer-owned helper tradeoffs, encoded-output
  memory behavior, and recommend `TransformTo` for service handlers that should
  avoid avoidable heap pressure.
- Document the cancellation caveat for caller-owned readers and stdlib
  decode/encode calls.
- No README diagram is required for #309 because the workflow is a simple
  linear decode-bound-transform-encode helper. If a diagram is later added,
  load `bluetape4k-diagram` before producing assets.

## Risks And Mitigations

| Risk | Mitigation |
|---|---|
| Decode resource exhaustion | `MaxInputBytes`, `DecodeConfig`, explicit input allowlist, max pixel/dimension checks before `Decode` |
| Resize/encode resource exhaustion | reject requested output width/height/pixels before allocating destination image |
| Misleading format claims | Document JPEG/PNG output and JPEG/PNG/GIF input allowlist only |
| Late cancellation | Check `context.Context` before/after bounded read and before decode, resize, and encode; preserve `ctx.Err()` through `errors.Is`; document no preemption inside blocked readers/writers or stdlib codecs |
| Scope creep toward full image toolkit | Keep API transform-focused; defer libvips, OCR, CAPTCHA, metadata, filters, SVG |
| Benchmark overclaim | Commit benchmark functions but avoid throughput recommendations in README |

## Acceptance Criteria Mapping

| Issue #309 requirement | Design response |
|---|---|
| small Go-shaped API | `imagekit` package with transform-focused options and typed errors |
| bounds and formats | bounded read, `DecodeConfig`, max pixel/dimension checks, supported format docs |
| cancellation-before-work | context checks before each major phase |
| malformed input and output dimensions | targeted tests for decode failure and resize modes |
| benchmarks committed before performance claims | benchmark functions included, no performance claim text |
| README pair in sync | English and Korean package READMEs plus root README updates |

## Open Questions

No user-blocking question remains for the first slice. The issue already
chooses pure-Go bounded helpers and explicitly defers native/libvips behavior.
