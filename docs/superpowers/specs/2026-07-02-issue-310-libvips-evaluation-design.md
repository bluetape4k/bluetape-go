# Issue #310 Libvips Evaluation Design

## Context

Issue #310 asks whether bluetape-go should offer an optional libvips-backed
image path after the pure-Go `imagekit` baseline from issue #309. The default
`imagekit` package must stay pure Go and must not require native libvips for
small thumbnail callers.

Current baseline evidence:

- `imagekit` supports bounded JPEG, PNG, and GIF input with JPEG/PNG output.
- `docs/benchmarks/2026-07-01-issue-309-imagekit.md` records pure-Go benchmark
  rows for small JPEG, small PNG, and medium JPEG-to-PNG transforms.
- Local native environment for this evaluation has `vips-8.18.3`,
  `pkg-config vips=8.18.3`, Go `go1.26.4`, `darwin/arm64`, and
  `CGO_ENABLED=1`.

## Requirements

- Compare `github.com/davidbyttow/govips/v2` and `github.com/h2non/bimg/v2`
  or the closest available bimg module.
- Verify native libvips detection and document actionable setup failure modes.
- Document lifecycle and cleanup rules for native image handles.
- Provide concurrency and race evidence for any example adapter.
- Benchmark against the pure-Go `imagekit` baseline.
- Keep libvips out of the default `imagekit` package and default root module
  build/test path.
- Do not claim AVIF, HEIC, TIFF, or other advanced codec support without native
  codec proof on the running host.

## Options

### Option A: Research-only decision record

This is the smallest change and keeps root builds untouched, but it does not
prove the lifecycle, concurrency, or caller ergonomics of a real adapter.

### Option B: Root build-tagged adapter package

A build-tagged `imagekit/govips` package would be discoverable, but it would add
the govips dependency to the root module graph and would be easy for CI or
callers to compile accidentally without native libvips.

### Option C: Isolated optional example module

An `examples/imagekit-govips` nested module can demonstrate the adapter shape,
tests, race coverage, native installation checks, and benchmark evidence while
keeping the default module and `imagekit` package pure Go.

## Decision

Use Option C for #310. Select `govips/v2` for the example because it is actively
maintained, versioned as `v2.18.0`, has current Go module metadata, documents
`Startup`/`Shutdown`, and exposes explicit image reference cleanup. Do not
select bimg for implementation because `github.com/h2non/bimg/v2` has no
available module version, the closest module is `github.com/h2non/bimg v1.1.9`,
and the repository lacks the same module hygiene for new bluetape-go code.

## Adapter Scope

- Mirror a narrow subset of `imagekit.Request`: width, height, mode, JPEG
  quality, output format, and input byte limit.
- Support JPEG and PNG output only.
- Use libvips thumbnail operations:
  - `ModeFit`: fit inside the requested box.
  - `ModeFill`: center crop to the requested box.
  - `ModeExact`: force exact dimensions.
- Read bounded input before handing bytes to libvips.
- Check context before bounded read, after bounded read, before native work, and
  before returning. Native libvips calls cannot be preempted once started.
- Call `Close` on every `ImageRef`.
- Start govips once with low default concurrency and quiet logging. The example
  intentionally avoids `Shutdown` during normal process lifetime because govips
  cannot be restarted after shutdown.

## Acceptance Checks

- Decision record compares govips and bimg with maintenance, API, native burden,
  license, testability, and benchmark evidence.
- Example module tests cover transform dimensions, output formats, invalid
  input handling, input byte limit handling, context cancellation before native
  work, and parallel caller use.
- Example module passes `go test` and `go test -race` when libvips is installed.
- Root `go test ./...` remains independent from libvips.
- External research evidence is preserved in `bluetape4k-wiki`.
