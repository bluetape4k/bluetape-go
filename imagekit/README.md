# imagekit

[English](README.md) | [한국어](README.ko.md)

`imagekit` provides bounded pure-Go helpers for resizing one image and encoding
it as JPEG or PNG.

![imagekit bounded transform flow](../docs/images/readme-diagrams/imagekit-transform-flow.png)

## Install

```bash
go get github.com/bluetape4k/bluetape-go
```

## Supported Formats

| Direction | Formats |
|---|---|
| Input | JPEG, PNG, GIF |
| Output | JPEG, PNG |

The input allowlist is explicit. Process-global image decoders registered by
other dependencies are rejected unless they report JPEG, PNG, or GIF.

## Default Bounds

Zero limit fields use conservative service defaults:

| Limit | Default |
|---|---:|
| `MaxInputBytes` | 10 MiB |
| `MaxPixels` | 16,000,000 |
| `MaxWidth` | 8192 |
| `MaxHeight` | 8192 |
| `MaxOutputPixels` | 16,000,000 |
| `MaxOutputWidth` | 8192 |
| `MaxOutputHeight` | 8192 |

`JPEGQuality` defaults to `85`. Nonzero JPEG quality must be in `1..100`.

## Usage

```go
result, err := imagekit.Transform(ctx, reader, imagekit.Request{
    Width:        320,
    Height:       180,
    Mode:         imagekit.ModeFit,
    OutputFormat: imagekit.OutputJPEG,
})
```

Use `TransformTo` only when direct writer output is acceptable. It encodes
directly to the writer and returns metadata with `Result.Bytes == nil`, avoiding
the extra encoded byte slice returned by `Transform`. The write is not atomic:
a codec or writer failure can leave partial bytes in the writer. For HTTP
responses or final storage objects, prefer `Transform` or write `TransformTo`
into a temporary buffer/object and publish it only after `err == nil`.

```go
var staged bytes.Buffer
result, err := imagekit.TransformTo(ctx, &staged, reader, imagekit.Request{
    Width:        320,
    Height:       180,
    Mode:         imagekit.ModeFill,
    OutputFormat: imagekit.OutputPNG,
})
if err == nil {
    _, err = writer.Write(staged.Bytes())
}
```

## Modes

| Mode | Behavior |
|---|---|
| `ModeFit` | Preserve aspect ratio and fit inside the requested box. |
| `ModeFill` | Preserve aspect ratio, center-crop, and fill the requested box. |
| `ModeExact` | Resize exactly to the requested size and allow distortion. |

## Cancellation

`imagekit` checks `context.Context` before the bounded read, after the bounded
read, before decode, before resize, and before encode. It cannot preempt a
blocked caller-owned `io.Reader` or `io.Writer`, nor a standard-library codec
call that is already executing. Services that need hard I/O deadlines should
enforce them at the I/O boundary.

## Errors

Errors support `errors.Is` and `errors.As` with imagekit sentinels:

- `ErrInvalidOptions`
- `ErrUnsupportedFormat`
- `ErrInputTooLarge`
- `ErrImageTooLarge`
- `ErrDecode`
- `ErrEncode`

Cancellation preserves `context.Canceled` and `context.DeadlineExceeded`.
Error messages do not include raw image bytes, caller file paths, or wrapped
cause text.

## Benchmarks

The pure-Go baseline is recorded in
[`docs/benchmarks/2026-07-01-issue-309-imagekit.md`](../docs/benchmarks/2026-07-01-issue-309-imagekit.md).
It exists to support the follow-up libvips evaluation in issue #310 and does not
claim libvips-level throughput.

![imagekit pure-Go benchmark baseline](../docs/images/readme-charts/imagekit-benchmark-baseline.png)

## Non-Goals

This package does not provide libvips/cgo integration, OCR, CAPTCHA, SVG
rasterization, AVIF, HEIC, TIFF, EXIF processing, image effects or filters
beyond resize resampling, metadata editing, watermarks, similarity metrics, or
framework integration.
