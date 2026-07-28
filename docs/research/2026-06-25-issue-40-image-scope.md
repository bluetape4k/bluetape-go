# Issue 40 Image 연구 범위

Issue #40은 `bluetape4k-image` concept를 Go package로 만들지, example로 둘지,
defer할지 결정하는 0.7.0 research gate다. Source repository는 넓은 JVM coverage를
갖지만, Go track은 runtime, test, dependency, caller value가 분명한 곳에서만
시작해야 한다.

## 소스 인벤토리

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

## 현재 Go Ecosystem 증거

- `disintegration/imaging`은 `image.Image` 위의 단순한 pure-Go package다. resize,
  crop, rotate, blur, sharpen, brightness, contrast, common load/save workflow를
  제공한다. 최신 module release는 오래됐지만 API는 작고 idiomatic하다.
- Go standard library와 `golang.org/x/image/draw`만으로도 third-party runtime
  dependency 없이 좁은 first-party thumbnail/resize/conversion helper를 만들 수 있다.
- `davidbyttow/govips/v2`는 현재 가장 강한 libvips binding 후보이다. active release,
  load/export, resize, crop, smart crop, metadata, cgo를 통한 native libvips
  integration을 제공한다.
- `h2non/bimg`는 compact API와 installed libvips build에 묶인 optional codec support를
  가진 mature high-level libvips wrapper다. `govips`보다 덜 active하지만 비교 후보로
  여전히 의미가 있다.
- libvips 자체는 빠르고 demand-driven, threaded, low-memory지만 native
  LGPL-2.1-or-later library, codec packaging, deployment check, cgo build/runtime
  constraint를 가져온다.
- CAPTCHA package는 있지만 선택지가 동등하지 않다. 단순한 `dchest/captcha` package는
  advanced OCR이 generated challenge를 깰 수 있다고 명시적으로 경고한다.
  `wenlng/go-captcha`는 더 풍부한 interactive CAPTCHA type을 제공하지만 UX와 integration
  선택지가 커진다. Lightweight generate/verify package도 있으나 adoption signal이 약하다.
- OCR은 첫 Go image package가 되면 안 된다. Go parity track도 native Tesseract,
  language data, containerized smoke test, 명확한 operator guidance가 필요하다.

## 순위

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

## 구현

- Create a narrow pure-Go image helper package before any native backend:
  bounded decode, thumbnail/resize, output format selection, error contracts,
  cancellation through `context.Context`, golden fixtures, fuzz inputs where
  useful, and benchmarks for small and medium images.
- Keep dependency policy conservative. Prefer the standard library plus
  `golang.org/x/image/draw`; compare `disintegration/imaging` only if the
  first-party code becomes larger or lower quality than the dependency.
- Publish performance claims only after `go test -bench` evidence is stored in
  the repo.

## 채택

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

## 필요한 후속 Issue

- #309: pure-Go image helper issue for bounded thumbnail, resize, and
  conversion.
- #310: separate libvips research/adapter issue for optional native image
  processing after benchmark and runtime proof.
- Do not create a CAPTCHA implementation issue from #40. It remains
  example-only until a concrete service workflow asks for it.

## 검증 계획

- Documentation-only PR에서는 `git diff --check`와 targeted `rg`를 실행한다.
- 새 follow-up issue가 #40을 reference하고 implementation gate를 보존하는지 확인한다.
- External evidence는 `bluetape4k-wiki`에 보존하고
  `gno update`, `gno embed --collection bluetape4k-wiki`, and representative
  `gno search`로 검증한다.

## 후속 권고

작은 pure-Go package와 measured behavior에서 시작한다. Benchmark evidence가 native
dependency management를 정당화할 충분한 가치를 증명한 뒤에만 libvips를 optional
accelerated path로 추가한다.
