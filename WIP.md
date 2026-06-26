# WIP

Snapshot: 2026-06-26 KST
Scope: `0.8.0` no-op release preflight and `0.9.0` text package planning.

## Current Target Release

`v0.9.0` - first text package slice for Go callers.

The next active milestone is `0.9.0`, covering text search and tokenizer
foundation work. The first slice should stay Go-native and narrow: multi-pattern
search, blockword detection and masking, tokenizer interfaces, and tokenizer or
language-detection research before any optional dependency is adopted.

## Current State

- `0.1.0`, `0.1.1`, `0.2.0`, `0.3.0`, `0.4.0`, `0.5.0`, `v0.5.1`,
  `v0.6.0`, `v0.6.1` through `v0.6.8`, and `v0.7.0` are tagged and
  released.
- Milestone `0.8.0` has no open issues, but its develop tree is byte-for-byte
  identical to the `v0.7.0` tag target tree. Its AWS and Floci work was already
  shipped by the broad `v0.7.0` protected-branch promotion.
- `v0.8.0` should not be tagged unless new commits land after `v0.7.0`; a tag
  on the same tree would be a no-op release with no Go module content delta.
- Issue #52, issue #53, issue #54, issue #55, and epic #45 are the active
  `0.9.0` text milestone candidates.

## Release Checklist

1. Close milestone `0.8.0` as already shipped in `v0.7.0`.
2. Do not create `v0.8.0` unless `git diff v0.7.0^{}..origin/develop` becomes
   non-empty.
3. Start `0.9.0` from the text milestone, beginning with the P0 implementation
   issues before the P1 research/design follow-ups.
