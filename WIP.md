# WIP

Snapshot: 2026-06-26 KST
Scope: `0.8.0` text search package implementation.

## Current Target Release

`v0.8.0` - text search, blockword masking, and tokenizer research.

The milestone starts with deterministic multi-pattern search. `textsearch`
provides immutable compiled matchers, overlap-aware first/all match modes,
replacement/masking hooks, Unicode normalization, and explicit boundary modes
without adding external search-engine dependencies.

## Current State

- `0.1.0`, `0.1.1`, `0.2.0`, `0.3.0`, `0.4.0`, `0.5.0`, `v0.5.1`,
  `v0.6.0`, `v0.6.1` through `v0.6.8`, and `v0.7.0` are tagged and released.
- Milestone `0.8.0` is the active text milestone after the post-`v0.7.0`
  downshift.
- Issue #52 adds the first-party multi-pattern search package.
- Issue #53 should build blockword detection and masking on top of #52.
- Issues #54 and #55 own tokenizer interface and Korean/Japanese/language
  detection research follow-ups.

## Release Checklist

1. Finish #52 with package tests, race/stress coverage, docs, and P0/P1 review.
2. Use #52 matcher APIs as the foundation for #53 blockword masking.
3. Keep tokenizer and language-specific dependencies out of core until #54/#55
   close their design/research gates.
