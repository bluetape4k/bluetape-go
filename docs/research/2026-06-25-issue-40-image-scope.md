# Issue 40 Image Research Scope

Issue #40 is the 0.7.0 research gate for deciding whether
`bluetape4k-image` concepts should become Go packages, examples, or deferred
work. The source repository has broad JVM coverage, but the Go track should
start only where the runtime, test, dependency, and caller value are clear.

## Source Inventory

Source repository: `/Users/debop/work/bluetape4k/bluetape4k-image`

- `images` provides the main pure-JVM/scrimage path: load/save, async I/O,
  Okio integration, resize, thumbnailing, tile/split, filters, watermarking,
  captions, color transforms, dominant color, blur detection, EXIF,
  similarity/hash/MSSIM/histogram/keypoint helpers, and SVG rasterization.
- `images-captcha` provides Java2D CAPTCHA generation, one-shot verification,
  bounded challenge configuration, pluggable storage, and coroutine entry
  points. The application remains responsible for rate limits, id generation,
  tenant scoping, and abuse controls.
- `images-ocr` wraps Tess4J/Tesseract for OCR over `ImmutableImage`. It
  requires native Tesseract and traineddata files and uses container smoke
  tests.
- `images-ktor` and `images-spring-boot` provide JVM framework integration.
  The Spring Boot module also covers storage/CDN-style behavior, health, and
  metrics.
- `images-vips-api`, `images-vips-java21`, and `images-vips-java25` provide a
  libvips abstraction with JNI and FFM backends, runtime limits, format
  allowlists, Okio support, and native dependency gates.
- `benchmark/images-benchmark` shows large libvips gains for resize and large
  streaming paths, but mixed results for some encode paths. The evidence
  supports libvips for large native-backed processing, not as a blanket
  replacement for every image operation.

## Current Go Ecosystem Evidence

- `disintegration/imaging` is a simple pure-Go package over `image.Image`. It
  covers resize, crop, rotate, blur, sharpen, brightness, contrast, and common
  load/save workflows. Its latest module release is old, but the API remains
  small and idiomatic.
- The Go standard library plus `golang.org/x/image/draw` can cover a narrow
  first-party thumbnail/resize/conversion helper without adding a third-party
  runtime dependency.
- `davidbyttow/govips/v2` is the strongest current libvips binding candidate.
  It has active releases, load/export, resize, crop, smart crop, metadata, and
  native libvips integration through cgo.
- `h2non/bimg` is a mature high-level libvips wrapper with a compact API and
  optional codec support tied to the installed libvips build. It is less active
  than `govips`, but remains relevant for comparison.
- libvips itself is fast, demand-driven, threaded, and low-memory, but brings a
  native LGPL-2.1-or-later library, codec packaging, deployment checks, and
  cgo build/runtime constraints.
- CAPTCHA packages are available, but the choices are not equivalent. The
  simple `dchest/captcha` package explicitly warns that advanced OCR can break
  generated challenges. `wenlng/go-captcha` has richer interactive CAPTCHA
  types, but brings larger UX and integration choices. Lightweight generate
  and verify packages exist, but have much weaker adoption signals.
- OCR should not be a first Go image package. A Go parity track would still
  require native Tesseract, language data, containerized smoke tests, and clear
  operator guidance.

## Ranking

| Area | Go fit | Risk | Decision |
|---|---:|---:|---|
| Bounded thumbnail/resize/conversion | High | Medium | Implement first as a small pure-Go package with decode-size limits and format tests. |
| Pure-Go image examples | High | Low/medium | Add examples around stdlib or a small helper API; keep supported formats explicit. |
| libvips adapter | Medium/high | High | Create optional follow-up gated by benchmarks, native detection, codec matrix, and lifecycle tests. |
| Metadata/EXIF | Medium | Medium | Add only if a concrete caller needs it; avoid broad parity helpers first. |
| SVG rasterization | Low/medium | High | Defer unless a safe, maintained Go dependency and XXE-style safety story are proven. |
| AVIF/HEIC/TIFF | Medium | High | Defer to backend-specific codec support and document native build gates. |
| CAPTCHA | Medium | High | Treat as example or optional package only; do not present it as an authentication boundary. |
| OCR | Low/medium | High | Defer until a separate OCR issue defines native Tesseract support and testcontainers shape. |
| HTTP/Spring/Ktor parity | Low | Medium | Translate to small `net/http` examples only when packages exist; do not port framework auto-config. |

## Implement

- Create a narrow pure-Go image helper package before any native backend:
  bounded decode, thumbnail/resize, output format selection, error contracts,
  cancellation through `context.Context`, golden fixtures, fuzz inputs where
  useful, and benchmarks for small and medium images.
- Keep dependency policy conservative. Prefer the standard library plus
  `golang.org/x/image/draw`; compare `disintegration/imaging` only if the
  first-party code becomes larger or lower quality than the dependency.
- Publish performance claims only after `go test -bench` evidence is stored in
  the repo.

## Adopt

- Evaluate `govips/v2` for an optional native package after the pure-Go package
  defines the caller contract. The adapter must prove native library detection,
  cleanup/lifecycle behavior, bounded memory, codec support, and benchmark
  value against the pure-Go path.
- Compare `bimg` as the high-level libvips alternative, especially for
  transformation ergonomics and codec support, but do not adopt both.

## Example-only

- CAPTCHA can be demonstrated as a bounded example if a service sample needs
  it. The example must document replay storage, expiry, rate limiting, and OCR
  bypass limits.
- HTTP upload/thumbnail examples should use `net/http` and the first-party
  package contract. Framework parity belongs outside the core image package.

## Defer

- Full `bluetape4k-image` parity: filters, histograms, similarity metrics,
  keypoints, captions, watermarks, SVG rasterization, OCR, and framework
  integration.
- AVIF/HEIC/TIFF promises until the selected backend and build environment
  prove codec support.
- Runtime mutable or global native state until race tests and shutdown tests
  cover concurrent callers.

## Follow-up Issues Required

- #309: pure-Go image helper issue for bounded thumbnail, resize, and
  conversion.
- #310: separate libvips research/adapter issue for optional native image
  processing after benchmark and runtime proof.
- Do not create a CAPTCHA implementation issue from #40. It remains
  example-only until a concrete service workflow asks for it.

## Validation Plan

- Documentation-only PR: `git diff --check` and targeted `rg`.
- Verify the new follow-up issues reference #40 and preserve implementation
  gates.
- Preserve external evidence in `bluetape4k-wiki` and validate with
  `gno update`, `gno embed --collection bluetape4k-wiki`, and representative
  `gno search`.

## Follow-up Recommendation

Start with a small pure-Go package and measured behavior. Add libvips only as
an optional accelerated path after benchmark evidence proves enough value to
justify native dependency management.
