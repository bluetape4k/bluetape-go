# Lessons Learned - Issue #310 Libvips Evaluation (2026-07-02)

**Related PR**: TBD
**Affected modules**: `examples/imagekit-govips`, `imagekit` docs/research

## L1: Keep native image adapters outside the default Go module until CI owns them

### Problem

libvips-backed adapters need cgo, `pkg-config`, native library installation, and
host-specific codec availability. Adding govips directly to the root module or
default `imagekit` package would make normal callers pay for native dependency
and CI complexity even when they only need small pure-Go thumbnails.

### Lesson

For bluetape-go native image experiments, start with an isolated nested example
module. Promote it to a root package only after native CI, codec support policy,
and release packaging are explicitly accepted.

## L2: govips lifecycle must be process-owned

### Problem

`govips` can start libvips explicitly, but its source rejects restart after
shutdown. A request-scoped start/stop helper would be unsafe and hard to test.

### Lesson

Use a process-level `sync.Once` startup, do not call shutdown on normal request
paths, and close every `ImageRef` after export. Document that context
cancellation is checked around native work but cannot preempt a libvips call
already in progress.
