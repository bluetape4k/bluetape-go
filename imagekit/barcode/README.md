# imagekit/barcode

[English](README.md) | [한국어](README.ko.md)

`imagekit/barcode` provides bounded, deterministic QR and Code128 rendering
helpers for callers that need a standard `image.Image` or PNG bytes. It keeps
the provider behind a small package boundary and returns a new black-and-white
`*image.Gray` for every call.

## Install

The package is part of the `bluetape-go` module. Add the module as usual:

```bash
go get github.com/bluetape4k/bluetape-go
```

The renderer uses the pinned `github.com/boombuler/barcode v1.1.0` provider;
callers do not configure provider clients or credentials.

## Usage

```go
img, err := barcode.Render(ctx, barcode.Request{
    Kind:    barcode.QR,
    Content: "블루테이프 QR",
    Width:   256,
    Height:  256,
})
```

```go
payload, err := barcode.EncodePNG(ctx, barcode.Request{
    Kind:    barcode.Code128,
    Content: "BLUETAPE-546",
    Width:   512,
    Height:  128,
})
```

`Render` returns a caller-owned `*image.Gray`. `EncodePNG` returns a new byte
slice containing standard `image/png` output. See `example_test.go` for
compile-checked QR and Code128 examples.

## Input and size limits

| Request | Contract |
|---|---|
| `Kind` | `QR` or `Code128` only |
| QR `Content` | valid UTF-8, 1–1,024 bytes; no trimming or normalization |
| Code128 `Content` | printable ASCII `0x20`–`0x7e`, 1–80 bytes |
| `QRLevel` | `QRLevelM`, `QRLevelL`, `QRLevelQ`, or `QRLevelH` for QR; `QRLevelM` only for Code128 |
| `Width`, `Height` | 1–4,096 pixels each; at most 4,194,304 pixels |
| QR shape | `Width == Height` |
| PNG output | at most 4 MiB; oversized output is discarded |

Input and image limits are checked before the provider call and before image
allocation. A payload being accepted does not guarantee that every requested
canvas can contain its symbol and quiet zone.

## Geometry and quiet zones

QR output uses a centered integer scale with at least four white modules on all
four sides. Code128 uses an integer horizontal scale, at least ten white
modules on both horizontal sides, and repeats each bar across the requested
height. No interpolation, crop, or forced downscaling is used when the symbol
does not fit.

The quiet-zone contract helps callers compose an image, but it is not a
certification for a physical scanner or any industry-specific symbology
profile. Independent decoder round trips and physical scanner tests are not
part of this helper.

## Context and errors

`ctx` must be non-nil. Cancellation and deadline errors are returned unchanged
before dispatch and at rendering/PNG checkpoints. The provider has no
context-aware API, so a call already in progress cannot be preempted; the
helper does not hide that limitation with a detached goroutine.

`ErrInvalidOptions`, `ErrInputTooLarge`, `ErrImageTooLarge`, and `ErrEncode` are
available for `errors.Is` classification. Provider and PNG failures return a
fixed `ErrEncode` with no provider cause or raw provider message. Failed calls
return a nil image or nil byte slice.

## Combining with imagekit

PNG bytes can be passed to `imagekit.Transform` through a
`bytes.NewReader`. Arbitrary resize or crop, and converting to JPEG, can damage
module edges or quiet zones and therefore can reduce scanability. The barcode
package does not promise scanner compatibility after such transformations.

## Non-goals

The package does not provide custom colors, logos, captions, decoding, OCR,
CAPTCHA, file/network I/O, provider client configuration, or a global cache.

