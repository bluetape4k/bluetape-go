# Issue 40 Image Research Review

Date: 2026-06-25
Scope: issue #40 research note, follow-up image issues, and preserved external
research evidence.

## Verdict

P0: 0
P1: 0

This is a documentation and tracker-alignment change. It does not add Go
package code, exported APIs, dependencies, benchmark claims, or runtime
behavior.

## 7-Tier Review

### Performance

P0: 0
P1: 0

The research separates measured libvips acceleration from default package
adoption. Native-backed performance claims are blocked on Go benchmarks against
the pure-Go path.

### Stability

P0: 0
P1: 0

The first package path avoids global native runtime state, cgo, and codec
availability variance. Optional libvips work requires lifecycle, cleanup, and
concurrent caller tests before adoption.

### Security

P0: 0
P1: 0

The first implementation issue requires bounded decode behavior and explicit
format support. CAPTCHA remains example-only and is not described as an
authentication or abuse-prevention boundary.

### Operator/Ops

P0: 0
P1: 0

Native libvips, codec support, and OCR/Tesseract deployment are kept out of the
default image package. The optional adapter issue requires detection and
operator guidance before merge.

### Developer/API

P0: 0
P1: 0

The recommendation is Go-shaped: `context.Context`, small first-party helper
contracts, stdlib-compatible formats, and optional native acceleration. It does
not port Kotlin/JVM framework facades.

### User/Caller

P0: 0
P1: 0

The first useful caller value is predictable thumbnail, resize, and conversion
behavior. Broader image analysis, OCR, and CAPTCHA surfaces are deferred until
caller workflows justify them.

### Integration

P0: 0
P1: 0

Evidence sources include current `bluetape4k-image` module inventory, Go image
and libvips library metadata, the repo's `go 1.26.3` module contract, duplicate
issue search, and preserved wiki research notes.
